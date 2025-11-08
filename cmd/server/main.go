package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	intrnl "termchat/internal"
	"termchat/internal/storage"
)

func main() {
	addr := flag.String("addr", getEnv("TERMCHAT_ADDR", ":8080"), "server listen address")
	path := flag.String("path", getEnv("TERMCHAT_PATH", "/join"), "websocket join path")
	dbPath := flag.String("db", getEnv("TERMCHAT_DB_PATH", "termchat.db"), "sqlite database path")
	flag.Parse()

	store, err := storage.NewStore(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	server := intrnl.NewServer(store)
	mux := http.NewServeMux()
	mux.HandleFunc(*path, server.ServeWS)
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

	log.Printf("TermChat server listening on %s (ws path %s)", *addr, *path)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
