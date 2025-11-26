package internal

import "sync"

// all active rooms state
type Hub struct {
	mutex sync.RWMutex
	rooms map[string]*Room
}

// builds an empty hub ready to serve websocket requests
func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*Room)}
}

// takes a peek into the room map. We use it for the lightweight /exists
func (hub *Hub) Exists(key string) bool {
	hub.mutex.RLock()
	defer hub.mutex.RUnlock()
	_, ok := hub.rooms[key]
	return ok
}

// ensures there is a live Room for the given ke
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

// getRoom retrieves a room by key (may return nil)
func (hub *Hub) getRoom(key string) *Room {
	hub.mutex.RLock()
	defer hub.mutex.RUnlock()
	return hub.rooms[key]
}

// deleteRoomIfEmptyWithCleanup removes room and cleans up files
func (hub *Hub) deleteRoomIfEmptyWithCleanup(key string, uploadDir string) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	if room, exists := hub.rooms[key]; exists {
		if room.size() == 0 {
			// Clean up files before deleting room
			room.deleteAllFiles(uploadDir)
			delete(hub.rooms, key)
		}
	}
}
