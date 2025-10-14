package internal

import (
	"os"
	"sync"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
)

// tui model struct for all the components and modes
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

// init user
func defaultUsername() string {
	if user := os.Getenv("TERMCHAT_USER"); user != "" {
		return user
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "anon"
}

func (model *TUIModel) Init() tea.Cmd {
	if model.mode == modeChat {
		return model.connectCmd()
	}
	return nil
}
