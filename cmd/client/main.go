package main

import (
	"flag"
	"fmt"
	"os"

	"termchat/internal"
	"termchat/internal/app"
)

func main() {
	defaultServer := envOrDefault("TERMCHAT_SERVER", "wss://termchat-server-al.fly.dev/join")
	defaultUser := envOrDefault("TERMCHAT_USER", "")

	version := flag.Bool("version", false, "Show version information")
	update := flag.Bool("update", false, "Update to the latest version")
	serverJoinURL := flag.String("server", defaultServer, "WebSocket join URL (e.g., ws://localhost:8080/join)")
	username := flag.String("user", defaultUser, "default username for login prompts")
	flag.Parse()

	// Handle version flag
	if *version {
		fmt.Printf("termchat v%s\n", internal.Version)
		os.Exit(0)
	}

	// Handle update flag
	if *update {
		if err := internal.UpdateToLatest(); err != nil {
			fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "\nTry updating manually:\n")
			fmt.Fprintf(os.Stderr, "curl -fsSL https://raw.githubusercontent.com/AlNaheyan/termchat/main/install.sh | sh\n")
			os.Exit(1)
		}
		os.Exit(0)
	}

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
