package internal

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
)

// tui model struct for all the components and modes
type TUIModel struct {
	textInput       textinput.Model
	messages        []ChatMessage
	serverJoinURL   string
	apiBaseURL      string
	sessionPath     string
	roomKey         string
	username        string
	currentFriend   string
	sessionToken    string
	friends         []Friend
	incomingReqs    []string
	outgoingReqs    []string
	selectedFriend  int
	selectedRequest int
	// Version checking
	latestVersion     string
	updateAvailable   bool
	versionCheckDone  bool
	requestView     requestViewType
	pendingUsername string
	authIntent      authIntent
	websocketConn   *websocket.Conn
	writeMutex      sync.Mutex
	isConnected     bool
	connectionError error
	mode            appMode
	pendingAction   actionType
	loading         bool

	// File upload state
	uploadingFile  bool
	uploadProgress float64
	uploadFilename string
	uploadError    string
	roomFiles      []FileMetadata
	filePicker     filepicker.Model
}

type appMode int

const (
	modeAuthMenu appMode = iota
	modeAuthUsername
	modeAuthPassword
	modeFriends
	modeAddFriend
	modeManualRoom
	modeRequestsIncoming
	modeRequestsOutgoing
	modeChat
	modeFileSelect
)

type actionType int

const (
	actionNone actionType = iota
	actionJoin
	actionCreate
)

type authIntent int

const (
	authIntentLogin authIntent = iota
	authIntentSignup
)

type requestViewType int

const (
	requestViewIncoming requestViewType = iota
	requestViewOutgoing
)

type Friend struct {
	Username string
	Online   bool
}

// FileMetadata represents a file uploaded to the current room
type FileMetadata struct {
	ID         string
	Filename   string
	SizeBytes  int64
	UploadedBy string
	UploadedAt int64
}

// FileItem represents an item in the file browser
type FileItem struct {
	Name  string
	Path  string
	IsDir bool
	Size  int64
}

func NewTUIModel(serverJoinURL, roomKey, username string) *TUIModel {
	input := textinput.New()
	input.Placeholder = "Type a message…"
	input.CharLimit = 0
	input.Focus()
	input.Prompt = "> "

	if username == "" {
		username = defaultUsername()
	}

	apiBase, err := httpBaseFromJoinURL(serverJoinURL)
	if err != nil {
		apiBase = ""
	}

	// Initialize filepicker
	fp := filepicker.New()
	fp.AllowedTypes = []string{} // Allow all file types
	fp.FileAllowed = true
	fp.DirAllowed = false
	fp.ShowHidden = false
	fp.AutoHeight = false
	fp.Height = 15

	// Set starting directory to user's home directory
	if home, err := os.UserHomeDir(); err == nil {
		fp.CurrentDirectory = home
	}


	model := &TUIModel{
		textInput:     input,
		messages:      make([]ChatMessage, 0, 64),
		serverJoinURL: serverJoinURL,
		apiBaseURL:    apiBase,
		sessionPath:   defaultSessionPath(),
		roomKey:       roomKey,
		username:      username,
		filePicker:    fp,
	}

	if session, err := loadSessionFromDisk(model.sessionPath); err == nil {
		model.sessionToken = session.Token
		model.username = session.Username
	}

	switch {
	case roomKey != "" && model.sessionToken != "":
		model.mode = modeChat
	case model.sessionToken != "":
		model.mode = modeFriends
		model.textInput.Blur()
		model.textInput.Prompt = ""
		model.textInput.Placeholder = ""
	default:
		model.mode = modeAuthMenu
		model.textInput.Blur()
		model.textInput.Prompt = ""
		model.textInput.Placeholder = ""
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
	// Always check for updates on startup (non-blocking)
	cmds := []tea.Cmd{checkVersionCmd()}
	
	switch model.mode {
	case modeChat:
		cmds = append(cmds, model.connectCmd())
	case modeFriends:
		cmds = append(cmds, model.fetchFriendsCmd(), model.fetchFriendRequestsCmd())
	}
	
	return tea.Batch(cmds...)
}

func defaultSessionPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".termchat", "session.json")
	}
	return filepath.Join(".termchat", "session.json")
}

func (model *TUIModel) appendSystemNotice(body string) {
	model.messages = append(model.messages, ChatMessage{User: "system", Body: body, Ts: time.Now().Unix()})
}

func (model *TUIModel) resetChatLog() {
	filtered := model.messages[:0]
	for _, msg := range model.messages {
		if msg.Room == "" {
			filtered = append(filtered, msg)
		}
	}
	model.messages = filtered
}

func (model *TUIModel) persistSession() error {
	if model.sessionPath == "" {
		return nil
	}
	return saveSessionToDisk(model.sessionPath, sessionFile{Username: model.username, Token: model.sessionToken})
}

func (model *TUIModel) removeSessionFile() error {
	return deleteSessionFile(model.sessionPath)
}
