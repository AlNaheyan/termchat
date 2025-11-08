package internal

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

type Metrics struct {
	signups     atomic.Uint64
	logins      atomic.Uint64
	activeConns atomic.Int64
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) IncSignup() {
	m.signups.Add(1)
}

func (m *Metrics) IncLogin() {
	m.logins.Add(1)
}

func (m *Metrics) IncConn() {
	m.activeConns.Add(1)
}

func (m *Metrics) DecConn() {
	m.activeConns.Add(-1)
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	payload := map[string]any{
		"signups_total":      m.signups.Load(),
		"logins_total":       m.logins.Load(),
		"active_connections": m.activeConns.Load(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
