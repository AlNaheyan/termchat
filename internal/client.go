package internal

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

// this model holds the bubbletea state for the chat client, including the input, message log, and websocket connection.
type TUIModel struct {
	textInput       textinput.Model
	messages        []ChatMessage
	serverJoinURL   string
	roomKey         string
	username        string
	websocketConn   *websocket.Conn
	writeMutex      sync.Mutex
	isConnected     bool
	connectionError error
	mode            appMode
	pendingAction   actionType
	menuIndex       int
}

// these are bubbletea messages that represent asynchronous events like connecting, receiving a chat message, or encountering an error.
type (
	connectedMsg     struct{}
	incomingMsg      ChatMessage
	errorMsg         error
	connectFailedMsg struct{ err error }
	reconnectMsg     struct{}
	existsMsg        struct {
		key    string
		exists bool
		err    error
	}
)

type appMode int

const (
	modeMenu appMode = iota
	modeNamePrompt
	modeJoinPrompt
	modeChat
)

type actionType int

const (
	actionNone actionType = iota
	actionJoin
	actionCreate
)

var (
	appTitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")).Padding(0, 1)
	subtitleStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("110")).MarginTop(1)
	menuBoxStyle       = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(1, 2).MarginTop(1)
	menuItemStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).PaddingLeft(1)
	menuHotkeyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	menuHintStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).MarginTop(1)
	noticeBoxStyle     = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("95")).Padding(1, 2).MarginTop(1)
	chatHeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(lipgloss.Color("63")).Padding(0, 1)
	statusStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("109")).MarginTop(1)
	connectedStyle     = statusStyle.Copy().Foreground(lipgloss.Color("42")).Bold(true)
	connectingStyle    = statusStyle.Copy().Foreground(lipgloss.Color("178")).Italic(true)
	messageBodyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("253"))
	messageBoxStyle    = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("60")).Padding(1, 2).MarginTop(1)
	inputBoxStyle      = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(0, 1).MarginTop(1)
	timestampStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	usernameStyle      = lipgloss.NewStyle().Bold(true)
	activeUserStyle    = usernameStyle.Copy().Foreground(lipgloss.Color("213"))
	systemMessageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Italic(true)
	errorStyle         = statusStyle.Copy().Foreground(lipgloss.Color("196")).Bold(true)
	dividerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("237")).Render(" ┃ ")
	userColorPalette   = []lipgloss.Color{
		lipgloss.Color("45"),
		lipgloss.Color("81"),
		lipgloss.Color("141"),
		lipgloss.Color("98"),
		lipgloss.Color("63"),
		lipgloss.Color("135"),
		lipgloss.Color("32"),
	}
)

// this constructor builds a new chat ui model with a focused input and a sensible default username.
func NewTUIModel(serverJoinURL, roomKey, username string) *TUIModel {
	input := textinput.New()
	input.Placeholder = "Type a message…"
	input.CharLimit = 0
	input.Focus()
	input.Prompt = "> "

	if username == "" {
		username = defaultUsername()
	}

	model := &TUIModel{
		textInput:     input,
		messages:      make([]ChatMessage, 0, 64),
		serverJoinURL: serverJoinURL,
		roomKey:       roomKey,
		username:      username,
	}
	if roomKey == "" {
		model.mode = modeMenu
		model.textInput.Blur()
		model.textInput.Prompt = ""
		model.textInput.Placeholder = ""
	} else {
		model.mode = modeChat
	}
	return model
}

func defaultUsername() string {
	if user := os.Getenv("TERMCHAT_USER"); user != "" {
		return user
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "anon"
}

// when the program starts we kick off a command that dials the websocket.
func (model *TUIModel) Init() tea.Cmd {
	if model.mode == modeChat {
		return model.connectCmd()
	}
	return nil
}

// update reacts to key presses and asynchronous events to drive the application state.
func (model *TUIModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMessage := message.(type) {
	case tea.KeyMsg:
		// global quit
		if typedMessage.Type == tea.KeyCtrlC || typedMessage.Type == tea.KeyEsc {
			if model.websocketConn != nil {
				_ = model.websocketConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				_ = model.websocketConn.Close()
			}
			return model, tea.Quit
		}
		switch model.mode {
		case modeMenu:
			switch typedMessage.String() {
			case "1", "j", "J":
				model.pendingAction = actionJoin
				model.mode = modeNamePrompt
				model.textInput.SetValue(model.username)
				model.textInput.Placeholder = "Enter display name…"
				model.textInput.Prompt = "name> "
				focusCmd := model.textInput.Focus()
				return model, focusCmd
			case "2", "c", "C":
				model.pendingAction = actionCreate
				model.mode = modeNamePrompt
				model.textInput.SetValue(model.username)
				model.textInput.Placeholder = "Enter display name…"
				model.textInput.Prompt = "name> "
				focusCmd := model.textInput.Focus()
				return model, focusCmd
			case "q", "Q", "3":
				// allow 3 as quit for a simple 1/2/3 menu
				return model, tea.Quit
			}
			return model, nil
		case modeNamePrompt:
			switch typedMessage.Type {
			case tea.KeyEnter:
				trimmed := strings.TrimSpace(model.textInput.Value())
				if trimmed == "" {
					model.messages = append(model.messages, ChatMessage{Room: "", User: "system", Body: "Display name cannot be empty.", Ts: time.Now().Unix()})
					return model, nil
				}
				model.username = trimmed
				model.textInput.SetValue("")
				nextAction := model.pendingAction
				model.pendingAction = actionNone
				switch nextAction {
				case actionJoin:
					model.mode = modeJoinPrompt
					model.textInput.Placeholder = "Enter room key…"
					model.textInput.Prompt = "room> "
					focusCmd := model.textInput.Focus()
					return model, focusCmd
				case actionCreate:
					key := generateSecureKey(12)
					model.roomKey = key
					model.messages = append(model.messages, ChatMessage{Room: key, User: "system", Body: inviteText(model.serverJoinURL, key), Ts: time.Now().Unix()})
					model.mode = modeChat
					model.textInput.Placeholder = "Type a message…"
					model.textInput.Prompt = "> "
					focusCmd := model.textInput.Focus()
					return model, tea.Batch(focusCmd, model.connectCmd())
				default:
					model.mode = modeMenu
					model.textInput.Blur()
					model.textInput.Placeholder = ""
					model.textInput.Prompt = ""
					return model, nil
				}
			case tea.KeyEsc:
				model.pendingAction = actionNone
				model.mode = modeMenu
				model.textInput.SetValue("")
				model.textInput.Blur()
				model.textInput.Placeholder = ""
				model.textInput.Prompt = ""
				return model, nil
			default:
				var cmd tea.Cmd
				model.textInput, cmd = model.textInput.Update(typedMessage)
				return model, cmd
			}
		case modeJoinPrompt:
			if typedMessage.Type == tea.KeyEsc {
				model.mode = modeMenu
				model.textInput.SetValue("")
				model.textInput.Blur()
				model.textInput.Placeholder = ""
				model.textInput.Prompt = ""
				return model, nil
			}
			if typedMessage.Type == tea.KeyEnter {
				trimmed := strings.TrimSpace(model.textInput.Value())
				if trimmed == "" {
					return model, nil
				}
				// check if room exists before connecting
				return model, model.existsCmd(trimmed)
			}
			var cmd tea.Cmd
			model.textInput, cmd = model.textInput.Update(typedMessage)
			return model, cmd
		case modeChat:
			switch typedMessage.Type {
			case tea.KeyEnter:
				trimmed := strings.TrimSpace(model.textInput.Value())
				// if the user typed a command that starts with '/', handle locally.
				if strings.HasPrefix(trimmed, "/") {
					lower := strings.ToLower(trimmed)
					if lower == "/quit" || lower == "/exit" {
						if model.websocketConn != nil {
							_ = model.websocketConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client quit"))
							_ = model.websocketConn.Close()
						}
						return model, tea.Quit
					}
					return model, nil
				}
				if trimmed != "" && model.isConnected {
					chat := ChatMessage{Room: model.roomKey, User: model.username, Body: trimmed, Ts: time.Now().Unix()}
					return model, model.sendCmd(chat)
				}
			}
			var command tea.Cmd
			model.textInput, command = model.textInput.Update(typedMessage)
			return model, command
		}

	case connectedMsg:
		model.isConnected = true
		model.connectionError = nil
		// start a read loop that delivers one message at a time
		return model, model.readOnceCmd()

	case incomingMsg:
		model.messages = append(model.messages, ChatMessage(typedMessage))
		// chain next read
		return model, model.readOnceCmd()

	case errorMsg:
		model.connectionError = typedMessage
		// if there is a connection error, stop the program
		return model, tea.Quit

	case connectFailedMsg:
		model.connectionError = typedMessage.err
		if model.mode == modeChat {
			return model, model.scheduleReconnect()
		}
		return model, nil

	case reconnectMsg:
		if model.mode == modeChat && !model.isConnected {
			return model, model.connectCmd()
		}
		return model, nil

	case existsMsg:
		if typedMessage.err != nil {
			// show error and stay in join prompt
			model.messages = append(model.messages, ChatMessage{Room: "", User: "system", Body: fmt.Sprintf("Error checking room: %v", typedMessage.err), Ts: time.Now().Unix()})
			return model, nil
		}
		if !typedMessage.exists {
			model.messages = append(model.messages, ChatMessage{Room: "", User: "system", Body: "Room not found. Try again or create a room.", Ts: time.Now().Unix()})
			return model, nil
		}
		// exists → set and connect
		model.roomKey = typedMessage.key
		model.mode = modeChat
		model.textInput.SetValue("")
		model.textInput.Placeholder = "Type a message…"
		model.textInput.Prompt = "> "
		model.textInput.Focus()
		return model, model.connectCmd()
	}
	return model, nil
}

// the view renders a simple header, the list of messages, and the input box so the user can type.
func (model TUIModel) View() string {
	switch model.mode {
	case modeMenu:
		return model.renderMenuView()
	case modeNamePrompt:
		return model.renderNamePromptView()
	case modeJoinPrompt:
		return model.renderJoinPromptView()
	default:
		return model.renderChatView()
	}
}

func (model TUIModel) renderMenuView() string {
	title := appTitleStyle.Render("TermChat")
	subtitle := subtitleStyle.Render("Lightweight terminal chat rooms")

	options := []string{
		renderMenuOption("1", "Join a room"),
		renderMenuOption("2", "Create a private room"),
		renderMenuOption("3", "Quit"),
	}

	viewSections := []string{
		lipgloss.JoinVertical(lipgloss.Left, title, subtitle),
		menuBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, options...)),
	}

	if notices := model.renderSystemNotices(); notices != "" {
		viewSections = append(viewSections, notices)
	}

	if model.textInput.Focused() {
		viewSections = append(viewSections, inputBoxStyle.Render(model.textInput.View()))
	}

	viewSections = append(viewSections, menuHintStyle.Render("Press 1, 2, or 3 to choose an option."))

	return lipgloss.JoinVertical(lipgloss.Left, viewSections...)
}

func (model TUIModel) renderNamePromptView() string {
	title := appTitleStyle.Render("Choose a display name")
	hint := menuHintStyle.Render("Enter the name others will see, then press Enter.")

	viewSections := []string{title, hint}

	if notices := model.renderSystemNotices(); notices != "" {
		viewSections = append(viewSections, notices)
	}

	viewSections = append(viewSections, inputBoxStyle.Render(model.textInput.View()))

	return lipgloss.JoinVertical(lipgloss.Left, viewSections...)
}

func (model TUIModel) renderJoinPromptView() string {
	title := appTitleStyle.Render("Join a room")
	hint := menuHintStyle.Render("Enter the room key and press Enter to connect.")

	viewSections := []string{
		title,
		hint,
	}

	if notices := model.renderSystemNotices(); notices != "" {
		viewSections = append(viewSections, notices)
	}

	viewSections = append(viewSections, inputBoxStyle.Render(model.textInput.View()))

	return lipgloss.JoinVertical(lipgloss.Left, viewSections...)
}

func (model TUIModel) renderChatView() string {
	headerSegments := []string{
		"TermChat",
		fmt.Sprintf("Room %s", model.roomKey),
		fmt.Sprintf("User %s", model.username),
		fmt.Sprintf("Server %s", model.serverJoinURL),
	}
	header := chatHeaderStyle.Render(strings.Join(headerSegments, dividerStyle))

	var statusLine string
	switch {
	case model.connectionError != nil:
		statusLine = errorStyle.Render("Connection error: " + model.connectionError.Error())
	case model.isConnected:
		statusLine = connectedStyle.Render("Connected")
	default:
		statusLine = connectingStyle.Render("Connecting…")
	}

	var messageLines []string
	for _, chat := range model.messages {
		messageLines = append(messageLines, model.renderChatMessage(chat))
	}
	if len(messageLines) == 0 {
		messageLines = append(messageLines, systemMessageStyle.Render("No messages yet. Say hi and start the conversation."))
	}

	messagesView := messageBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, messageLines...))
	inputView := inputBoxStyle.Render(model.textInput.View())
	footerHint := menuHintStyle.Render("Commands: /quit to leave the room")

	sections := []string{header}
	if statusLine != "" {
		sections = append(sections, statusLine)
	}
	sections = append(sections, messagesView)
	sections = append(sections, inputView, footerHint)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderMenuOption(hotkey string, label string) string {
	key := menuHotkeyStyle.Render(hotkey)
	return lipgloss.JoinHorizontal(lipgloss.Left, key, menuItemStyle.Render(label))
}

func (model TUIModel) renderSystemNotices() string {
	var notices []string
	for _, msg := range model.messages {
		if msg.User == "system" && msg.Room == "" {
			notices = append(notices, systemMessageStyle.Render(msg.Body))
		}
	}
	if len(notices) == 0 {
		return ""
	}
	return noticeBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, notices...))
}

func (model *TUIModel) scheduleReconnect() tea.Cmd {
	const retryDelay = 2 * time.Second
	// we use tea.Tick instead of time.After to integrate with bubbletea's event loop.
	return tea.Tick(retryDelay, func(time.Time) tea.Msg {
		return reconnectMsg{}
	})
}

func (model TUIModel) renderChatMessage(chat ChatMessage) string {
	timestamp := timestampStyle.Render(fmt.Sprintf("[%s]", time.Unix(chat.Ts, 0).Format("15:04:05")))
	if chat.User == "system" {
		body := systemMessageStyle.Render(chat.Body)
		return lipgloss.JoinHorizontal(lipgloss.Left, timestamp, " ", body)
	}

	var nameStyle lipgloss.Style
	if chat.User == model.username {
		nameStyle = activeUserStyle
	} else {
		nameStyle = usernameStyle.Copy().Foreground(colorForUser(chat.User))
	}

	name := nameStyle.Render(chat.User)
	bodyText := messageBodyStyle.Render(strings.ReplaceAll(chat.Body, "\n", "\n   "))

	return lipgloss.JoinHorizontal(lipgloss.Left, timestamp, " ", name, ": ", bodyText)
}

func colorForUser(name string) lipgloss.Color {
	if len(userColorPalette) == 0 {
		return lipgloss.Color("249")
	}
	if name == "" {
		return userColorPalette[0]
	}
	var sum int
	for _, r := range name {
		sum += int(r)
	}
	return userColorPalette[sum%len(userColorPalette)]
}

// this command dials the websocket join url and returns either a connected message or an error.
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

// existsCmd checks if a room exists via an HTTP endpoint without creating it, then emits existsMsg
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

// this command reads a single message from the websocket and converts it into a bubbletea message; we schedule it repeatedly to keep reading.
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
		// if decoding fails, we render the payload as plain text so the user still sees something meaningful.
		chat = ChatMessage{Room: model.roomKey, User: "server", Body: string(payload), Ts: time.Now().Unix()}
		return incomingMsg(chat)
	}
}

// this command encodes a chat message and writes it to the websocket, and it clears the input when the write succeeds.
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
		// we clear the input field after a successful send so the user can type the next message immediately.
		model.textInput.SetValue("")
		return nil
	}
}

// runClient launches the bubbletea program with the chat model so the user can chat from the terminal.
func RunClient(serverJoinURL, roomKey, username string) error {
	program := tea.NewProgram(NewTUIModel(serverJoinURL, roomKey, username))
	_, err := program.Run()
	return err
}

func buildJoinURL(base string, roomKey string) (string, error) {
	// we accept a full websocket url and ensure the room query parameter is set so the server knows which room to join.
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

func buildExistsURL(wsBase string, roomKey string) (string, error) {
	// convert ws(s)://host[:port]/join to http(s)://host[:port]/exists?room=KEY
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

func generateSecureKey(length int) string {
	if length < 8 {
		length = 8
	}
	// base32 encoding yields ~1.6 bytes per char; compute needed bytes
	byteLen := (length * 5) / 8
	if (length*5)%8 != 0 {
		byteLen++
	}
	b := make([]byte, byteLen)
	_, _ = rand.Read(b)
	// RFC4648 base32 without padding, uppercase A-Z2-7
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
