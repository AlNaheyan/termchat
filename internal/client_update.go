package internal

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
)

// chat message struct
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
	authResultMsg struct {
		token    string
		username string
		err      error
	}
	friendsLoadedMsg struct {
		friends []Friend
		err     error
	}
	friendRequestsLoadedMsg struct {
		incoming []string
		outgoing []string
		err      error
	}
	friendRequestActionMsg struct {
		username string
		action   string
		err      error
	}
	logoutResultMsg struct {
		err error
	}
)

func (model *TUIModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			model.closeConnection()
			return model, tea.Quit
		}
		return model.handleKeyMsg(msg)

	case connectedMsg:
		model.isConnected = true
		model.connectionError = nil
		return model, model.readOnceCmd()

	case incomingMsg:
		model.messages = append(model.messages, ChatMessage(msg))
		return model, model.readOnceCmd()

	case errorMsg:
		model.connectionError = msg
		model.isConnected = false
		if model.mode == modeChat {
			model.appendSystemNotice(fmt.Sprintf("Connection closed: %v", msg))
			model.mode = modeFriends
			model.roomKey = ""
			model.currentFriend = ""
			model.textInput.Blur()
		}
		return model, nil

	case connectFailedMsg:
		model.connectionError = msg.err
		if model.mode == modeChat {
			model.appendSystemNotice(fmt.Sprintf("Connect failed: %v", msg.err))
			return model, model.scheduleReconnect()
		}
		return model, nil

	case reconnectMsg:
		if model.mode == modeChat && !model.isConnected {
			return model, model.connectCmd()
		}
		return model, nil

	case existsMsg:
		return model.handleExistsMsg(msg)

	case authResultMsg:
		model.loading = false
		if msg.err != nil {
			model.appendSystemNotice(fmt.Sprintf("Auth failed: %v", msg.err))
			model.mode = modeAuthMenu
			model.textInput.Blur()
			model.textInput.SetValue("")
			model.textInput.EchoMode = textinput.EchoNormal
			return model, nil
		}
		model.sessionToken = msg.token
		model.username = msg.username
		model.mode = modeFriends
		model.textInput.Blur()
		model.textInput.SetValue("")
		_ = model.persistSession()
		model.loading = true
		return model, tea.Batch(model.fetchFriendsCmd(), model.fetchFriendRequestsCmd())

	case friendsLoadedMsg:
		model.loading = false
		if msg.err != nil {
			if errors.Is(msg.err, errUnauthorized) {
				model.appendSystemNotice("Session expired. Please log in again.")
				model.clearSessionState()
				return model, nil
			}
			model.appendSystemNotice(fmt.Sprintf("Failed to load friends: %v", msg.err))
			return model, nil
		}
		model.friends = msg.friends
		if len(model.friends) == 0 {
			model.selectedFriend = 0
		} else if model.selectedFriend >= len(model.friends) {
			model.selectedFriend = len(model.friends) - 1
		}
		return model, nil

	case friendRequestsLoadedMsg:
		model.loading = false
		if msg.err != nil {
			if errors.Is(msg.err, errUnauthorized) {
				model.appendSystemNotice("Session expired. Please log in again.")
				model.clearSessionState()
				return model, nil
			}
			model.appendSystemNotice(fmt.Sprintf("Failed to load friend requests: %v", msg.err))
			return model, nil
		}
		model.incomingReqs = msg.incoming
		model.outgoingReqs = msg.outgoing
		if model.requestView == requestViewIncoming {
			if len(model.incomingReqs) == 0 {
				model.selectedRequest = 0
			} else if model.selectedRequest >= len(model.incomingReqs) {
				model.selectedRequest = len(model.incomingReqs) - 1
			}
		} else {
			if len(model.outgoingReqs) == 0 {
				model.selectedRequest = 0
			} else if model.selectedRequest >= len(model.outgoingReqs) {
				model.selectedRequest = len(model.outgoingReqs) - 1
			}
		}
		return model, nil

	case friendRequestActionMsg:
		model.loading = false
		if msg.err != nil {
			if errors.Is(msg.err, errUnauthorized) {
				model.appendSystemNotice("Session expired. Please log in again.")
				model.clearSessionState()
				return model, nil
			}
			model.appendSystemNotice(fmt.Sprintf("Friend request action failed: %v", msg.err))
			return model, nil
		}
		switch msg.action {
		case "sent":
			model.appendSystemNotice(fmt.Sprintf("Friend request sent to %s.", msg.username))
		case "accept":
			model.appendSystemNotice(fmt.Sprintf("You are now friends with %s.", msg.username))
		case "decline":
			model.appendSystemNotice(fmt.Sprintf("Declined friend request from %s.", msg.username))
		case "cancel":
			model.appendSystemNotice(fmt.Sprintf("Canceled friend request to %s.", msg.username))
		}
		model.mode = modeFriends
		model.selectedRequest = 0
		model.loading = true
		return model, tea.Batch(model.fetchFriendsCmd(), model.fetchFriendRequestsCmd())

	case logoutResultMsg:
		if msg.err != nil {
			model.appendSystemNotice(fmt.Sprintf("Logout error: %v", msg.err))
		}
		model.clearSessionState()
		return model, nil
	}

	return model, nil
}

func (model *TUIModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch model.mode {
	case modeAuthMenu:
		return model.handleAuthMenuKeys(msg)
	case modeAuthUsername:
		return model.handleAuthUsernameKeys(msg)
	case modeAuthPassword:
		return model.handleAuthPasswordKeys(msg)
	case modeFriends:
		return model.handleFriendsKeys(msg)
	case modeAddFriend:
		return model.handleAddFriendKeys(msg)
	case modeManualRoom:
		return model.handleManualRoomKeys(msg)
	case modeRequestsIncoming:
		return model.handleRequestListKeys(msg, requestViewIncoming)
	case modeRequestsOutgoing:
		return model.handleRequestListKeys(msg, requestViewOutgoing)
	case modeChat:
		return model.handleChatKeys(msg)
	default:
		return model, nil
	}
}

func (model *TUIModel) handleAuthMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "1", "l":
		return model.startAuthPrompt(authIntentLogin)
	case "2", "s":
		return model.startAuthPrompt(authIntentSignup)
	case "q":
		model.closeConnection()
		return model, tea.Quit
	}
	return model, nil
}

func (model *TUIModel) startAuthPrompt(intent authIntent) (tea.Model, tea.Cmd) {
	model.authIntent = intent
	model.mode = modeAuthUsername
	model.textInput.SetValue(model.username)
	model.textInput.Placeholder = "Username"
	model.textInput.Prompt = "user> "
	model.textInput.EchoMode = textinput.EchoNormal
	return model, model.textInput.Focus()
}

func (model *TUIModel) handleAuthUsernameKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		trimmed := strings.TrimSpace(model.textInput.Value())
		if trimmed == "" {
			model.appendSystemNotice("Username cannot be empty.")
			return model, nil
		}
		model.pendingUsername = trimmed
		model.mode = modeAuthPassword
		model.textInput.SetValue("")
		model.textInput.Placeholder = "Password"
		model.textInput.Prompt = "pass> "
		model.textInput.EchoMode = textinput.EchoPassword
		return model, model.textInput.Focus()
	case tea.KeyEsc:
		model.mode = modeAuthMenu
		model.textInput.Blur()
		model.textInput.SetValue("")
		model.textInput.EchoMode = textinput.EchoNormal
		return model, nil
	default:
		var cmd tea.Cmd
		model.textInput, cmd = model.textInput.Update(msg)
		return model, cmd
	}
}

func (model *TUIModel) handleAuthPasswordKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		password := model.textInput.Value()
		if strings.TrimSpace(password) == "" {
			model.appendSystemNotice("Password cannot be empty.")
			return model, nil
		}
		model.loading = true
		model.textInput.SetValue("")
		model.textInput.Blur()
		return model, model.submitCredentialsCmd(model.pendingUsername, password)
	case tea.KeyEsc:
		model.mode = modeAuthMenu
		model.textInput.Blur()
		model.textInput.SetValue("")
		model.textInput.EchoMode = textinput.EchoNormal
		return model, nil
	default:
		var cmd tea.Cmd
		model.textInput, cmd = model.textInput.Update(msg)
		return model, cmd
	}
}

func (model *TUIModel) handleFriendsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if len(model.friends) == 0 {
			return model, nil
		}
		friend := model.friends[model.selectedFriend]
		return model.startChatWithRoom(directRoomKey(model.username, friend.Username), friend.Username)
	case tea.KeyUp:
		if len(model.friends) > 0 {
			model.selectedFriend--
			if model.selectedFriend < 0 {
				model.selectedFriend = len(model.friends) - 1
			}
		}
		return model, nil
	case tea.KeyDown:
		if len(model.friends) > 0 {
			model.selectedFriend = (model.selectedFriend + 1) % len(model.friends)
		}
		return model, nil
	}

	switch strings.ToLower(msg.String()) {
	case "a":
		model.mode = modeAddFriend
		model.textInput.SetValue("")
		model.textInput.Placeholder = "Friend username"
		model.textInput.Prompt = "friend> "
		model.textInput.EchoMode = textinput.EchoNormal
		return model, model.textInput.Focus()
	case "i":
		if len(model.incomingReqs) == 0 {
			model.appendSystemNotice("No incoming requests.")
			return model, nil
		}
		model.mode = modeRequestsIncoming
		model.requestView = requestViewIncoming
		model.selectedRequest = 0
		return model, nil
	case "o":
		if len(model.outgoingReqs) == 0 {
			model.appendSystemNotice("No outgoing requests.")
			return model, nil
		}
		model.mode = modeRequestsOutgoing
		model.requestView = requestViewOutgoing
		model.selectedRequest = 0
		return model, nil
	case "m":
		model.pendingAction = actionJoin
		model.mode = modeManualRoom
		model.textInput.SetValue("")
		model.textInput.Placeholder = "Enter room code"
		model.textInput.Prompt = "room> "
		model.textInput.EchoMode = textinput.EchoNormal
		return model, model.textInput.Focus()
	case "n":
		key := generateSecureKey(12)
		model.resetChatLog()
		model.roomKey = key
		model.currentFriend = ""
		model.mode = modeChat
		model.textInput.Placeholder = "Type a message…"
		model.textInput.Prompt = "> "
		model.textInput.EchoMode = textinput.EchoNormal
		model.messages = append(model.messages, ChatMessage{Room: key, User: "system", Body: inviteText(model.serverJoinURL, key), Ts: time.Now().Unix()})
		return model, tea.Batch(model.textInput.Focus(), model.connectCmd())
	case "r":
		model.loading = true
		return model, tea.Batch(model.fetchFriendsCmd(), model.fetchFriendRequestsCmd())
	case "l":
		model.loading = true
		cmd := model.logoutCmd()
		model.sessionToken = ""
		model.friends = nil
		model.mode = modeAuthMenu
		model.textInput.Blur()
		return model, cmd
	case "q":
		model.closeConnection()
		return model, tea.Quit
	}
	return model, nil
}

func (model *TUIModel) handleAddFriendKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		trimmed := strings.TrimSpace(model.textInput.Value())
		if trimmed == "" {
			return model, nil
		}
		model.loading = true
		model.textInput.Blur()
		model.mode = modeFriends
		model.textInput.SetValue("")
		return model, model.sendFriendRequestCmd(trimmed)
	case tea.KeyEsc:
		model.mode = modeFriends
		model.textInput.Blur()
		model.textInput.SetValue("")
		return model, nil
	default:
		var cmd tea.Cmd
		model.textInput, cmd = model.textInput.Update(msg)
		return model, cmd
	}
}

func (model *TUIModel) handleManualRoomKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		trimmed := strings.TrimSpace(model.textInput.Value())
		if trimmed == "" {
			return model, nil
		}
		model.textInput.SetValue("")
		return model, model.existsCmd(trimmed)
	case tea.KeyEsc:
		model.mode = modeFriends
		model.textInput.Blur()
		model.textInput.SetValue("")
		return model, nil
	default:
		var cmd tea.Cmd
		model.textInput, cmd = model.textInput.Update(msg)
		return model, cmd
	}
}

func (model *TUIModel) handleChatKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		trimmed := strings.TrimSpace(model.textInput.Value())
		if strings.HasPrefix(trimmed, "/") {
			switch strings.ToLower(trimmed) {
			case "/leave":
				model.leaveChat()
				return model, nil
			}
			return model, nil
		}
		if trimmed != "" && model.isConnected {
			chat := ChatMessage{Room: model.roomKey, User: model.username, Body: trimmed, Ts: time.Now().Unix()}
			return model, model.sendCmd(chat)
		}
	case tea.KeyEsc:
		model.leaveChat()
		return model, nil
	}
	var cmd tea.Cmd
	model.textInput, cmd = model.textInput.Update(msg)
	return model, cmd
}

func (model *TUIModel) startChatWithRoom(roomKey, friend string) (tea.Model, tea.Cmd) {
	model.resetChatLog()
	model.roomKey = roomKey
	model.currentFriend = friend
	model.mode = modeChat
	model.isConnected = false
	model.textInput.Placeholder = "Type a message…"
	model.textInput.Prompt = "> "
	model.textInput.EchoMode = textinput.EchoNormal
	model.textInput.SetValue("")
	return model, tea.Batch(model.textInput.Focus(), model.connectCmd())
}

func (model *TUIModel) handleExistsMsg(msg existsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		model.appendSystemNotice(fmt.Sprintf("Error checking room: %v", msg.err))
		return model, nil
	}
	if !msg.exists {
		model.appendSystemNotice("Room not found. Try again or create one.")
		return model, nil
	}
	model.mode = modeChat
	model.resetChatLog()
	model.roomKey = msg.key
	model.currentFriend = ""
	model.textInput.Placeholder = "Type a message…"
	model.textInput.Prompt = "> "
	model.textInput.EchoMode = textinput.EchoNormal
	model.textInput.SetValue("")
	return model, tea.Batch(model.textInput.Focus(), model.connectCmd())
}

func (model *TUIModel) clearSessionState() {
	model.sessionToken = ""
	model.friends = nil
	model.selectedFriend = 0
	model.mode = modeAuthMenu
	model.roomKey = ""
	model.currentFriend = ""
	model.loading = false
	model.textInput.Blur()
	model.textInput.SetValue("")
	_ = model.removeSessionFile()
	model.closeConnection()
}

func (model *TUIModel) leaveChat() {
	model.closeConnection()
	model.mode = modeFriends
	model.roomKey = ""
	model.currentFriend = ""
	model.textInput.Blur()
	model.textInput.SetValue("")
}

func (model *TUIModel) closeConnection() {
	if model.websocketConn != nil {
		_ = model.websocketConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = model.websocketConn.Close()
		model.websocketConn = nil
	}
	model.isConnected = false
}

func (model *TUIModel) handleRequestListKeys(msg tea.KeyMsg, view requestViewType) (tea.Model, tea.Cmd) {
	var list []string
	switch view {
	case requestViewIncoming:
		list = model.incomingReqs
	case requestViewOutgoing:
		list = model.outgoingReqs
	}
	if len(list) == 0 {
		model.mode = modeFriends
		return model, nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		model.mode = modeFriends
		return model, nil
	case tea.KeyUp:
		model.selectedRequest--
		if model.selectedRequest < 0 {
			model.selectedRequest = len(list) - 1
		}
		return model, nil
	case tea.KeyDown:
		model.selectedRequest = (model.selectedRequest + 1) % len(list)
		return model, nil
	case tea.KeyEnter:
		if view == requestViewIncoming {
			model.loading = true
			return model, model.friendRequestActionCmd(list[model.selectedRequest], "accept")
		}
		return model, nil
	}
	switch strings.ToLower(msg.String()) {
	case "d":
		model.loading = true
		if view == requestViewIncoming {
			return model, model.friendRequestActionCmd(list[model.selectedRequest], "decline")
		}
		return model, model.friendRequestActionCmd(list[model.selectedRequest], "cancel")
	}
	return model, nil
}
