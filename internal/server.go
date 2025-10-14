package internal

import (
    "encoding/json"
    "log"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

// this struct describes the json envelope that both the client and server exchange during a chat session.
type ChatMessage struct {
    Room string `json:"room"`
    User string `json:"user"`
    Body string `json:"body"`
    Ts   int64  `json:"ts"`
}

// the hub keeps track of rooms by their key and creates or removes them as needed.
type Hub struct {
    mutex sync.RWMutex
    rooms map[string]*Room
}

func NewHub() *Hub {
    return &Hub{rooms: make(map[string]*Room)}
}

// Exists returns true if a room with the given key currently exists in memory.
func (hub *Hub) Exists(key string) bool {
    hub.mutex.RLock()
    defer hub.mutex.RUnlock()
    _, ok := hub.rooms[key]
    return ok
}

func (hub *Hub) getOrCreateRoom(key string) *Room {
    hub.mutex.Lock()
    defer hub.mutex.Unlock()
    if room, exists := hub.rooms[key]; exists {
        return room
    }
    room := newRoom(key)
    hub.rooms[key] = room
    go room.run()
    return room
}

func (hub *Hub) deleteRoomIfEmpty(key string) {
    hub.mutex.Lock()
    defer hub.mutex.Unlock()
    if room, exists := hub.rooms[key]; exists {
        if room.size() == 0 {
            delete(hub.rooms, key)
        }
    }
}

// a room broadcasts incoming messages to all currently connected clients and handles membership changes.
type Room struct {
    key        string
    clients    map[*Client]bool
    register   chan *Client
    unregister chan *Client
    broadcast  chan []byte
    mutex      sync.RWMutex
}

func newRoom(key string) *Room {
    return &Room{
        key:        key,
        clients:    make(map[*Client]bool),
        register:   make(chan *Client),
        unregister: make(chan *Client),
        broadcast:  make(chan []byte, 256),
    }
}

func (room *Room) size() int {
    room.mutex.RLock()
    defer room.mutex.RUnlock()
    return len(room.clients)
}

func (room *Room) run() {
    for {
        select {
        case client := <-room.register:
            room.mutex.Lock()
            room.clients[client] = true
            room.mutex.Unlock()
        case client := <-room.unregister:
            room.mutex.Lock()
            if _, exists := room.clients[client]; exists {
                delete(room.clients, client)
                close(client.send)
            }
            room.mutex.Unlock()
        case messagePayload := <-room.broadcast:
            // we fan out the message to all connected clients, and if a client's send buffer is full we drop that connection to keep the room healthy.
            room.mutex.Lock()
            for client := range room.clients {
                select {
                case client.send <- messagePayload:
                default:
                    // this client is too slow to read; we drop the connection to avoid backpressure on the room.
                    close(client.send)
                    delete(room.clients, client)
                }
            }
            room.mutex.Unlock()
        }
    }
}

// a client wraps a single websocket connection and a buffered send queue.
type Client struct {
    room *Room
    conn *websocket.Conn
    send chan []byte
}

const (
    writeWait  = 10 * time.Second
    pongWait   = 60 * time.Second
    pingPeriod = (pongWait * 9) / 10
    maxMsgSize = 8192
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        // we allow all origins in development; in production you should tighten this if the server is exposed publicly.
        return true
    },
}

// this handler upgrades the http request to a websocket and then joins or creates the target room based on the "room" query parameter.
func ServeWS(hub *Hub, writer http.ResponseWriter, request *http.Request) {
    roomKey := request.URL.Query().Get("room")
    if roomKey == "" {
        http.Error(writer, "missing room query param", http.StatusBadRequest)
        return
    }
    websocketConn, err := upgrader.Upgrade(writer, request, nil)
    if err != nil {
        log.Printf("upgrade error: %v", err)
        return
    }

    room := hub.getOrCreateRoom(roomKey)
    client := &Client{room: room, conn: websocketConn, send: make(chan []byte, 256)}
    room.register <- client

    go client.writePump()
    go client.readPump(hub, roomKey)
}

func (client *Client) readPump(hub *Hub, roomKey string) {
    defer func() {
        client.room.unregister <- client
        client.conn.Close()
        hub.deleteRoomIfEmpty(roomKey)
    }()
    client.conn.SetReadLimit(maxMsgSize)
    _ = client.conn.SetReadDeadline(time.Now().Add(pongWait))
    client.conn.SetPongHandler(func(string) error {
        return client.conn.SetReadDeadline(time.Now().Add(pongWait))
    })
    for {
        _, payload, err := client.conn.ReadMessage()
        if err != nil {
            // this is a normal close or read error, so we break and let the deferred cleanup run.
            break
        }
        // we try to decode the payload as a chat message and fill in missing fields like timestamp and room; if decoding fails, we broadcast the raw payload as-is.
        var chatMessage ChatMessage
        if err := json.Unmarshal(payload, &chatMessage); err == nil {
            if chatMessage.Ts == 0 {
                chatMessage.Ts = time.Now().Unix()
            }
            if chatMessage.Room == "" {
                chatMessage.Room = roomKey
            }
            encoded, _ := json.Marshal(chatMessage)
            client.room.broadcast <- encoded
        } else {
            // if it was not json, we still broadcast the payload so other clients can see something useful.
            client.room.broadcast <- payload
        }
    }
}

func (client *Client) writePump() {
    ticker := time.NewTicker(pingPeriod)
    defer func() {
        ticker.Stop()
        client.conn.Close()
    }()
    for {
        select {
        case message, ok := <-client.send:
            _ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if !ok {
                // if the send channel has been closed, we ask the peer to close and then return.
                _ = client.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
                return
            }
        case <-ticker.C:
            _ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
