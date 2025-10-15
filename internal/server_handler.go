package internal

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

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
	client := newClient(room, websocketConn)
	room.register <- client

	go client.writePump()
	go client.readPump(hub, roomKey)
}
