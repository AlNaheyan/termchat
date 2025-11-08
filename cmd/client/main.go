package main

import (
	"flag"
	"fmt"
	"os"

	intrnl "termchat/internal"
)

func main() {
	defaultServer := getenvDefault("TERMCHAT_SERVER", "wss://termchat-server-al.fly.dev/join")
	defaultUser := getenvDefault("TERMCHAT_USER", "")

	serverJoinURL := flag.String("server", defaultServer, "WebSocket join URL (e.g., ws://localhost:8080/join)")
	username := flag.String("user", defaultUser, "default username for login prompts")
	flag.Parse()

	args := flag.Args()
	var roomKey string
	if len(args) >= 1 {
		roomKey = args[0]
	}

	if err := intrnl.RunClient(*serverJoinURL, roomKey, *username); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func getenvDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
