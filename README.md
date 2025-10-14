# termchat
simple bi-directional chatting platform right at your terminal built with go and bubbletea.

tui built with lipgloss

*ref: [go/lipgloss](https://github.com/charmbracelet/lipgloss)*

## setup
run the tui against a locally running server using go directly:

```
go run ./cmd/server &
go run ./cmd/client
```

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
