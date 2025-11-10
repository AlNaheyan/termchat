# Termchat
Want a lightweight chat app right inside your terminal? Termchat is a Bubble Tea–powered TUI that connects to a Go backend, authenticates with SQLite, and lets you trade real-time messages without leaving the command line.

## Technology
- **Go 1.24** for both server and client
- **Bubble Tea + Bubbles + Lipgloss** for the terminal interface
- **Gorilla WebSocket** for real-time chats
- **SQLite (modernc driver)** for user accounts, sessions, and friend lists
- **Fly.io / Docker** for deployment targets
