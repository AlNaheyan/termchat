# termchat
Lightweight terminal-based chat that lets users create or join private rooms and talk in real time from the command line. A Bubble Tea TUI handles navigation, name/room prompts, and the live message log; the server keeps room state in memory and broadcasts messages to all connected clients.

*ref: [go/lipgloss](https://github.com/charmbracelet/lipgloss)*

## Running the server

1. Clone the repo and pull the modules:
   ```bash
   git clone https://github.com/alnaheyan/termchat.git
   cd termchat
   go mod download
   ```
2. Start the HTTP/WebSocket server. The first run will create a SQLite database (defaults to `termchat.db`, override with `TERMCHAT_DB_PATH`):
   ```bash
   TERMCHAT_DB_PATH=termchat.db go run ./cmd/server \
     --addr :8080 \
     --path /join
   ```
   Endpoints exposed:
   - `POST /signup`, `POST /login`, `POST /logout`
   - `GET /friends`, `POST /friends/{username}`
   - `GET /exists?room=ROOM_CODE`
   - `GET /join?room=ROOM_CODE` (upgraded to WebSocket)

## Running the client

1. Launch the TUI and point it at your server’s join endpoint:
   ```bash
   go run ./cmd/client --server ws://localhost:8080/join
   ```
2. First-time users can **Sign up** inside the TUI. Returning users pick **Log in**; credentials are cached at `~/.termchat/session.json` (0600 permissions) for convenience.
3. After authentication the client loads your friends list:
   - `↑/↓` highlight a friend and hit `Enter` to open a direct chat (room key `chat:<sorted usernames>`).
   - `A` adds a friend by username; friendships are mutual.
   - `M` joins a manual room by code; `N` creates a random shareable room while keeping chats ephemeral.
   - `R` refreshes, `L` logs out (also clears the cached session), `Q` quits.
4. Inside a chat:
   - Type messages and press `Enter`.
   - `Esc` returns to the friends list, `/quit` exits the client entirely.

## docker
build and run everything through docker compose:

```
docker compose build server client
```

start the websocket server:

```
docker compose up server
```

launch a chat client runs in an interactive tty (ghostty):

```
docker compose run --rm client
```

## Tests

Storage migrations and helpers are covered with unit tests. Run them (or the whole suite) with:

```bash
GOCACHE=$(pwd)/.gocache go test ./...
```
