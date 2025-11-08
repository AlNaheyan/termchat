package internal

import "sync"

// PresenceTracker keeps counts of active websocket connections per user.
type PresenceTracker struct {
	mu     sync.Mutex
	online map[int64]int
}

func NewPresenceTracker() *PresenceTracker {
	return &PresenceTracker{online: make(map[int64]int)}
}

func (p *PresenceTracker) Increment(userID int64) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.online[userID]++
	return p.online[userID]
}

func (p *PresenceTracker) Decrement(userID int64) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if count, ok := p.online[userID]; ok {
		if count <= 1 {
			delete(p.online, userID)
			return 0
		}
		p.online[userID] = count - 1
		return p.online[userID]
	}
	return 0
}

func (p *PresenceTracker) Online(userID int64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.online[userID] > 0
}

func (p *PresenceTracker) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.online)
}
