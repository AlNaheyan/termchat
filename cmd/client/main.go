package main

import (
	"flag"
	"fmt"
	"os"

	"termchat/internal/app"
)

func main() {
	defaultServer := envOrDefault("TERMCHAT_SERVER", "wss://termchat-server-al.fly.dev/join")
	defaultUser := envOrDefault("TERMCHAT_USER", "")

	serverJoinURL := flag.String("server", defaultServer, "WebSocket join URL (e.g., ws://localhost:8080/join)")
	username := flag.String("user", defaultUser, "default username for login prompts")
	flag.Parse()

	args := flag.Args()
	var roomKey string
	if len(args) >= 1 {
		roomKey = args[0]
	}

	cfg := app.ClientConfig{
		ServerURL: *serverJoinURL,
		RoomKey:   roomKey,
		Username:  *username,
	}

	if err := app.RunClient(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
