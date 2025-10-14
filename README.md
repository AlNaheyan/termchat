# termchat
Lightweight terminal-based chat that lets users create or join private rooms and talk in real time from the command line. A Bubble Tea TUI handles navigation, name/room prompts, and the live message log; the server keeps room state in memory and broadcasts messages to all connected clients.
tui built with lipgloss

*ref: [go/lipgloss](https://github.com/charmbracelet/lipgloss)*

## setup
WIP: Binary 

1. Clone or download the repo:
    ```
    git clone https://github.com/alnaheyan/termchat.git
    cd termchat
    ```
2. Pull dependencies:
    ```
    go mod download
    ```
3. Join or create a chat room
    ```
    go run ./cmd/client --server wss://termchat-server-al.fly.dev/join
    ```
4. Enjoy chatting either sharing the code if creating or joining with the shared code. 

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
