package internal

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// single room strucut
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
			// Broadcast to every connected client. If a client can't keep up we
			// close its send channel, which will trigger cleanup in writePump.
			room.mutex.Lock()
			for client := range room.clients {
				select {
				case client.send <- messagePayload:
				default:
					close(client.send)
					delete(room.clients, client)
				}
			}
			room.mutex.Unlock()
		}
	}
}


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
			// read error ends the loop so the deferred cleanup can fire.
			break
		}
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
