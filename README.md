# Termchat
Want a lightweight chat app right inside your terminal? Termchat is a Bubble Tea–powered TUI that connects to a Go backend, authenticates with SQLite, and lets you trade real-time messages without leaving the command line.

## Technology
- **Go 1.24** for both server and client
- **Bubble Tea + Bubbles + Lipgloss** for the terminal interface
- **Gorilla WebSocket** for real-time chats
- **SQLite (modernc driver)** for user accounts, sessions, and friend lists
- **Fly.io / Docker** for deployment targets

## Quick Setup
1. Clone the repo and fetch deps:
   ```bash
   git clone https://github.com/alnaheyan/termchat.git
   cd termchat
   go mod download
   ```
2. Run the server locally (stores data in `termchat.db` by default):
   ```bash
   TERMCHAT_DB_PATH=termchat.db go run ./cmd/server
   ```
3. Run the client; it defaults to the hosted backend but you can point it at your local server with `TERMCHAT_SERVER`:
   ```bash
   go run ./cmd/client
   # or
   TERMCHAT_SERVER=ws://localhost:8080/join go run ./cmd/client
   ```
4. Containerized? Use Docker Compose:
   ```bash
   docker compose up server
   docker compose run --rm client
   ```

That’s it—launch the TUI, sign up, send a friend request, and start chatting directly from your terminal.
