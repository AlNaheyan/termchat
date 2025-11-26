package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	intrnl "termchat/internal"
	"termchat/internal/storage"
)

// ServerHandle represents a running HTTP/WebSocket server instance.
type ServerHandle struct {
	addr   string
	server *http.Server
	store  *storage.Store
	done   chan struct{}
	err    error
}

// Addr returns the actual listen address (after the OS allocated a port).
func (h *ServerHandle) Addr() string {
	return h.addr
}

// Stop triggers a graceful shutdown with the provided context deadline.
func (h *ServerHandle) Stop(ctx context.Context) error {
	if h == nil || h.server == nil {
		return nil
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	return h.server.Shutdown(ctx)
}

// Wait blocks until the server exits.
func (h *ServerHandle) Wait() error {
	if h == nil {
		return nil
	}
	<-h.done
	return h.err
}

// RunServer wires handlers, opens the SQLite store, runs migrations, and
// starts serving in the background. Call Stop/Wait to manage its lifecycle.
func RunServer(ctx context.Context, cfg ServerConfig) (*ServerHandle, error) {
	if cfg.DBPath == "" {
		return nil, errors.New("database path is required")
	}
	cfg.Path = NormalizeJoinPath(cfg.Path)

	// Set defaults for file upload config
	if cfg.UploadDir == "" {
		cfg.UploadDir = DefaultUploadDir()
	}
	if cfg.MaxFileSize == 0 {
		cfg.MaxFileSize = 10 * 1024 * 1024 // 10MB default
	}

	// Ensure upload directory exists
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload directory: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	store, err := storage.NewStore(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	server := intrnl.NewServerWithConfig(store, cfg.UploadDir, cfg.MaxFileSize)
	mux := http.NewServeMux()
	registerHandlers(mux, cfg.Path, server)

	httpServer := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("listen: %w", err)
	}

	handle := &ServerHandle{
		addr:   listener.Addr().String(),
		server: httpServer,
		store:  store,
		done:   make(chan struct{}),
	}

	go func() {
		if ctx == nil {
			return
		}
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	go handle.serve(listener)

	return handle, nil
}

func (h *ServerHandle) serve(listener net.Listener) {
	defer close(h.done)
	err := h.server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	if err := h.store.Close(); err != nil {
		log.Printf("store close error: %v", err)
	}
	h.err = err
}

func registerHandlers(mux *http.ServeMux, wsPath string, server *intrnl.Server) {
	mux.HandleFunc(wsPath, server.ServeWS)
	mux.HandleFunc("/signup", server.HandleSignup)
	mux.HandleFunc("/login", server.HandleLogin)
	mux.HandleFunc("/logout", server.HandleLogout)
	mux.HandleFunc("/friends", server.HandleFriends)
	mux.HandleFunc("/friends/", server.HandleAddFriend)
	mux.HandleFunc("/friend-requests", server.HandleFriendRequests)
	mux.HandleFunc("/friend-requests/", func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/friend-requests/")
		if strings.Contains(trimmed, "/") {
			server.HandleRespondFriendRequest(w, r)
			return
		}
		server.HandleCreateFriendRequest(w, r)
	})
	mux.HandleFunc("/password/change", server.HandlePasswordChange)
	mux.HandleFunc("/exists", server.HandleRoomExists)
	mux.Handle("/metrics", server.MetricsHandler())

	// File upload/download routes
	mux.HandleFunc("/api/upload", server.HandleFileUpload)
	mux.HandleFunc("/api/files/", server.HandleFileDownload)
}
