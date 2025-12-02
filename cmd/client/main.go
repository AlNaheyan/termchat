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
	help := flag.Bool("help", false, "Show help and keyboard shortcuts")
	serverJoinURL := flag.String("server", defaultServer, "WebSocket join URL (e.g., ws://localhost:8080/join)")
	username := flag.String("user", defaultUser, "default username for login prompts")
	flag.Parse()

	// Handle help flag
	if *help {
		showHelp()
		os.Exit(0)
	}

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

func showHelp() {
	fmt.Printf("termchat v%s - Terminal-based chat application\n\n", internal.Version)
	
	fmt.Println("USAGE:")
	fmt.Println("  termchat [room]              Join or create a room")
	fmt.Println("  termchat --help              Show this help message")
	fmt.Println("  termchat --version           Show version information")
	fmt.Println("  termchat --update            Update to the latest version")
	fmt.Println()
	
	fmt.Println("AUTHENTICATION SCREEN:")
	fmt.Println("  1 or L     Log in")
	fmt.Println("  2 or S     Sign up")
	fmt.Println("  Q          Quit")
	fmt.Println()
	
	fmt.Println("FRIENDS SCREEN:")
	fmt.Println("  ↑ / ↓      Navigate friend list")
	fmt.Println("  Enter      Start chat with selected friend")
	fmt.Println("  A          Add a friend")
	fmt.Println("  I          View incoming friend requests")
	fmt.Println("  O          View outgoing friend requests")
	fmt.Println("  M          Manually join a room by code")
	fmt.Println("  N          Create a new room")
	fmt.Println("  R          Refresh friends list")
	fmt.Println("  L          Logout")
	fmt.Println("  Q          Quit")
	fmt.Println()
	
	fmt.Println("FRIEND REQUESTS SCREEN:")
	fmt.Println("  ↑ / ↓      Navigate requests")
	fmt.Println("  Enter      Accept request (incoming only)")
	fmt.Println("  D          Decline (incoming) or Cancel (outgoing)")
	fmt.Println("  Esc        Go back to Friends screen")
	fmt.Println()
	
	fmt.Println("CHAT SCREEN:")
	fmt.Println("  Esc        Leave chat room")
	fmt.Println("  Enter      Send message")
	fmt.Println("  Ctrl+C     Force quit")
	fmt.Println()
	
	fmt.Println("CHAT COMMANDS:")
	fmt.Println("  /upload           Open file picker to select and upload a file")
	fmt.Println("  /upload <path>    Upload a specific file")
	fmt.Println("  /download <file>  Download a file from the room")
	fmt.Println("  /leave            Exit the current chat room")
	fmt.Println()
	
	fmt.Println("FILE PICKER:")
	fmt.Println("  ↑ / ↓      Navigate files")
	fmt.Println("  Enter      Select file to upload")
	fmt.Println("  Esc        Cancel file selection")
	fmt.Println()
	
	fmt.Println("For more information, visit: https://github.com/AlNaheyan/termchat")
}
