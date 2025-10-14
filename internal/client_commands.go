package internal

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
)

func (model *TUIModel) scheduleReconnect() tea.Cmd {
	const retryDelay = 2 * time.Second
	// we schedule a future poke that nudges Update to try the connection again.
	return tea.Tick(retryDelay, func(time.Time) tea.Msg {
		return reconnectMsg{}
	})
}

// websocket dial
func (model *TUIModel) connectCmd() tea.Cmd {
	return func() tea.Msg {
		joinURL, err := buildJoinURL(model.serverJoinURL, model.roomKey)
		if err != nil {
			return connectFailedMsg{err: err}
		}
		conn, _, err := websocket.DefaultDialer.Dial(joinURL, http.Header{})
		if err != nil {
			return connectFailedMsg{err: err}
		}
		model.websocketConn = conn
		return connectedMsg{}
	}
}

// HTTP GET against /exists so we can warn the user
func (model *TUIModel) existsCmd(key string) tea.Cmd {
	return func() tea.Msg {
		urlStr, err := buildExistsURL(model.serverJoinURL, key)
		if err != nil {
			return existsMsg{key: key, exists: false, err: err}
		}
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(urlStr)
		if err != nil {
			return existsMsg{key: key, exists: false, err: err}
		}
		_ = resp.Body.Close()
		return existsMsg{key: key, exists: resp.StatusCode == http.StatusOK, err: nil}
	}
}

// if the payload is JSON we turn it into a ChatMessage
func (model *TUIModel) readOnceCmd() tea.Cmd {
	return func() tea.Msg {
		if model.websocketConn == nil {
			return errorMsg(fmt.Errorf("websocket not connected"))
		}
		messageType, payload, err := model.websocketConn.ReadMessage()
		if err != nil {
			return errorMsg(err)
		}
		if messageType != websocket.TextMessage {
			return nil
		}
		var chat ChatMessage
		if err := json.Unmarshal(payload, &chat); err == nil {
			return incomingMsg(chat)
		}

		chat = ChatMessage{Room: model.roomKey, User: "server", Body: string(payload), Ts: time.Now().Unix()}
		return incomingMsg(chat)
	}
}

func (model *TUIModel) sendCmd(chat ChatMessage) tea.Cmd {
	return func() tea.Msg {
		if model.websocketConn == nil {
			return errorMsg(fmt.Errorf("websocket not connected"))
		}
		encoded, err := json.Marshal(chat)
		if err != nil {
			return errorMsg(err)
		}
		model.writeMutex.Lock()
		err = model.websocketConn.WriteMessage(websocket.TextMessage, encoded)
		model.writeMutex.Unlock()
		if err != nil {
			return errorMsg(err)
		}
		model.textInput.SetValue("")
		return nil
	}
}

//entry for bubbletea
func RunClient(serverJoinURL, roomKey, username string) error {
	program := tea.NewProgram(NewTUIModel(serverJoinURL, roomKey, username))
	_, err := program.Run()
	return err
}


func buildJoinURL(base string, roomKey string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("room", roomKey)
	parsed.RawQuery = query.Encode()
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return "", fmt.Errorf("invalid scheme for websocket: %s", parsed.Scheme)
	}
	return parsed.String(), nil
}

//quich exist check for a room with http://localhost:8080/exists?room=ROOM_ID
func buildExistsURL(wsBase string, roomKey string) (string, error) {
	parsed, err := url.Parse(wsBase)
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "ws":
		parsed.Scheme = "http"
	case "wss":
		parsed.Scheme = "https"
	default:
		return "", fmt.Errorf("invalid scheme for websocket: %s", parsed.Scheme)
	}
	parsed.Path = "/exists"
	q := parsed.Query()
	q.Set("room", roomKey)
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

// make shareable room code using base32
func generateSecureKey(length int) string {
	if length < 8 {
		length = 8
	}
	// base32 encoding gets 1.6 bytes per char
	byteLen := (length * 5) / 8
	if (length*5)%8 != 0 {
		byteLen++
	}
	b := make([]byte, byteLen)
	_, _ = rand.Read(b)
	//  base32 without padding, uppercase A-Z2-7
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	if len(enc) >= length {
		return enc[:length]
	}
	return enc
}


func inviteText(serverJoinURL, roomKey string) string {
	var sb strings.Builder
	sb.WriteString("Invite others with:\n  ")
	sb.WriteString("go run ./cmd/client --user <name> ")
	sb.WriteString(roomKey)
	sb.WriteString("\nURL: ")
	// best effort to render the raw ws URL
	if u, err := buildJoinURL(serverJoinURL, roomKey); err == nil {
		sb.WriteString(u)
	} else {
		sb.WriteString("ws://localhost:8080/join?room=")
		sb.WriteString(roomKey)
	}
	return sb.String()
}
