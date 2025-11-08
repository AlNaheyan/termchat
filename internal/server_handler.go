package internal

import (
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"termchat/internal/storage"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Server bundles the hub with the persistent store and exposes HTTP handlers.
type Server struct {
	store       *storage.Store
	hub         *Hub
	tokenTTL    time.Duration
	presence    *PresenceTracker
	metrics     *Metrics
	authLimiter *RateLimiter
}

// AuthContext represents the authenticated user resolved from a session token.
type AuthContext struct {
	UserID   int64
	Token    string
	Username string
}

// NewServer wires the hub and store together.
func NewServer(store *storage.Store) *Server {
	return &Server{
		store:       store,
		hub:         NewHub(),
		tokenTTL:    30 * 24 * time.Hour,
		presence:    NewPresenceTracker(),
		metrics:     NewMetrics(),
		authLimiter: NewRateLimiter(10, time.Minute),
	}
}

// ServeWS upgrades the HTTP connection after verifying the bearer token.
func (s *Server) ServeWS(writer http.ResponseWriter, request *http.Request) {
	roomKey := request.URL.Query().Get("room")
	if roomKey == "" {
		http.Error(writer, "missing room query param", http.StatusBadRequest)
		return
	}
	authCtx, err := s.authenticateRequest(request)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnauthorized) {
			status = http.StatusUnauthorized
		}
		http.Error(writer, http.StatusText(status), status)
		return
	}

	websocketConn, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	room := s.hub.getOrCreateRoom(roomKey)
	s.presence.Increment(authCtx.UserID)
	s.metrics.IncConn()
	client := newClient(room, websocketConn, authCtx.Username, authCtx.UserID, func() {
		s.presence.Decrement(authCtx.UserID)
		s.metrics.DecConn()
	})
	room.register <- client

	go client.writePump()
	go client.readPump(s.hub, roomKey)
}

var errUnauthorized = errors.New("unauthorized")

func (s *Server) authenticateRequest(r *http.Request) (*AuthContext, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, errUnauthorized
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, errUnauthorized
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return nil, errUnauthorized
	}
	ctx := r.Context()
	session, err := s.store.GetSession(ctx, token)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errUnauthorized
	}
	if time.Now().After(session.ExpiresAt) {
		_ = s.store.DeleteSession(ctx, token)
		return nil, errUnauthorized
	}
	user, err := s.store.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errUnauthorized
	}
	return &AuthContext{UserID: user.ID, Username: user.Username, Token: token}, nil
}

func (s *Server) clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) MetricsHandler() http.Handler {
	return s.metrics
}
