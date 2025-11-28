package internal

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
		headers := http.Header{}
		if model.sessionToken != "" {
			headers.Set("Authorization", "Bearer "+model.sessionToken)
		}
		conn, _, err := websocket.DefaultDialer.Dial(joinURL, headers)
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

		// Try to parse as FileUploadMessage first
		var fileMsg FileUploadMessage
		if err := json.Unmarshal(payload, &fileMsg); err == nil && fileMsg.Type == "file_uploaded" {
			// Add to room files list
			model.roomFiles = append(model.roomFiles, FileMetadata{
				ID:         fileMsg.FileID,
				Filename:   fileMsg.Filename,
				SizeBytes:  fileMsg.SizeBytes,
				UploadedBy: fileMsg.UploadedBy,
				UploadedAt: fileMsg.UploadedAt,
			})
			// Display as system message
			sizeStr := formatFileSize(fileMsg.SizeBytes)
			chat := ChatMessage{
				Room: model.roomKey,
				User: "system",
				Body: fmt.Sprintf("📎 %s uploaded: %s (%s)", fileMsg.UploadedBy, fileMsg.Filename, sizeStr),
				Ts:   fileMsg.UploadedAt,
			}
			return incomingMsg(chat)
		}

		// Try to parse as regular ChatMessage
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

// entry for bubbletea
func RunClient(serverJoinURL, roomKey, username string) error {
	program := tea.NewProgram(
		NewTUIModel(serverJoinURL, roomKey, username),
		tea.WithAltScreen(), // render on an isolated canvas so we don't leave scrollback noise
	)
	_, err := program.Run()
	return err
}

func (model *TUIModel) submitCredentialsCmd(username, password string) tea.Cmd {
	intent := model.authIntent
	base := model.apiBaseURL
	return func() tea.Msg {
		if base == "" {
			return authResultMsg{err: fmt.Errorf("invalid server URL")}
		}
		if intent == authIntentSignup {
			if err := apiSignup(base, username, password); err != nil {
				return authResultMsg{err: err}
			}
		}
		resp, err := apiLogin(base, username, password)
		if err != nil {
			return authResultMsg{err: err}
		}
		return authResultMsg{token: resp.Token, username: resp.Username}
	}
}

func (model *TUIModel) fetchFriendsCmd() tea.Cmd {
	token := model.sessionToken
	base := model.apiBaseURL
	return func() tea.Msg {
		if base == "" || token == "" {
			return friendsLoadedMsg{err: fmt.Errorf("missing session")}
		}
		friends, err := apiGetFriends(base, token)
		return friendsLoadedMsg{friends: friends, err: err}
	}
}

func (model *TUIModel) sendFriendRequestCmd(friendUsername string) tea.Cmd {
	token := model.sessionToken
	base := model.apiBaseURL
	return func() tea.Msg {
		if base == "" || token == "" {
			return friendRequestActionMsg{username: friendUsername, err: fmt.Errorf("missing session")}
		}
		err := apiSendFriendRequest(base, token, friendUsername)
		return friendRequestActionMsg{username: friendUsername, action: "sent", err: err}
	}
}

func (model *TUIModel) logoutCmd() tea.Cmd {
	token := model.sessionToken
	base := model.apiBaseURL
	return func() tea.Msg {
		if base == "" || token == "" {
			return logoutResultMsg{err: nil}
		}
		err := apiLogout(base, token)
		return logoutResultMsg{err: err}
	}
}

func (model *TUIModel) fetchFriendRequestsCmd() tea.Cmd {
	token := model.sessionToken
	base := model.apiBaseURL
	return func() tea.Msg {
		if base == "" || token == "" {
			return friendRequestsLoadedMsg{err: fmt.Errorf("missing session")}
		}
		resp, err := apiGetFriendRequests(base, token)
		return friendRequestsLoadedMsg{incoming: resp.Incoming, outgoing: resp.Outgoing, err: err}
	}
}

func (model *TUIModel) friendRequestActionCmd(username, action string) tea.Cmd {
	token := model.sessionToken
	base := model.apiBaseURL
	return func() tea.Msg {
		if base == "" || token == "" {
			return friendRequestActionMsg{username: username, action: action, err: fmt.Errorf("missing session")}
		}
		err := apiRespondFriendRequest(base, token, username, action)
		return friendRequestActionMsg{username: username, action: action, err: err}
	}
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

// quich exist check for a room with http://localhost:8080/exists?room=ROOM_ID
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

func directRoomKey(a, b string) string {
	if strings.Compare(a, b) < 0 {
		return fmt.Sprintf("chat:%s:%s", a, b)
	}
	return fmt.Sprintf("chat:%s:%s", b, a)
}

// formatFileSize returns a human-readable file size
func formatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// uploadFileCmd uploads selected file
func (model *TUIModel) uploadFileCmd(filePath string) tea.Cmd {
	return func() tea.Msg {
		// Progress callback
		progressFn := func(progress float64) {
			// This would ideally send progress updates via channel
			// For simplicity, we'll handle it in one shot
		}

		fileID, err := apiUploadFile(
			model.apiBaseURL,
			model.sessionToken,
			filePath,
			model.roomKey,
			model.username,
			progressFn,
		)

		if err != nil {
			return fileUploadErrorMsg{err: err, filename: filePath}
		}

		return fileUploadedMsg{fileID: fileID, filename: filepath.Base(filePath)}
	}
}

// downloadFileCmd downloads a file from the server
func (model *TUIModel) downloadFileCmd(fileID, filename string) tea.Cmd {
	return func() tea.Msg {
		// Download to ~/Downloads
		destDir := filepath.Join(os.Getenv("HOME"), "Downloads")
		if _, err := os.Stat(destDir); os.IsNotExist(err) {
			destDir = "."
		}
		destPath := filepath.Join(destDir, filename)

		err := apiDownloadFile(
			model.apiBaseURL,
			model.sessionToken,
			fileID,
			model.roomKey,
			destPath,
		)

		if err != nil {
			return fileDownloadErrorMsg{err: err, filename: filename}
		}

		return fileDownloadedMsg{filename: filename, path: destPath}
	}
}
