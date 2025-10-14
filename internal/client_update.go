package internal

import (
	"fmt"
	"strings"
	"time"

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
)

// 
func (model *TUIModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMessage := message.(type) {
	case tea.KeyMsg:
		// Any mode should respect Ctrl+C or Esc so the user can bail out quickly.
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
				// The menu screens all read the same keys, so "3" acts as an easy quit.
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
				// Before we try to dial the websocket, hit the lightweight HTTP probe.
				return model, model.existsCmd(trimmed)
			}
			var cmd tea.Cmd
			model.textInput, cmd = model.textInput.Update(typedMessage)
			return model, cmd
		case modeChat:
			switch typedMessage.Type {
			case tea.KeyEnter:
				trimmed := strings.TrimSpace(model.textInput.Value())

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

		return model, model.readOnceCmd()

	case incomingMsg:
		model.messages = append(model.messages, ChatMessage(typedMessage))

		return model, model.readOnceCmd()

	case errorMsg:
		model.connectionError = typedMessage
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
			model.messages = append(model.messages, ChatMessage{Room: "", User: "system", Body: fmt.Sprintf("Error checking room: %v", typedMessage.err), Ts: time.Now().Unix()})
			return model, nil
		}
		if !typedMessage.exists {
			model.messages = append(model.messages, ChatMessage{Room: "", User: "system", Body: "Room not found. Try again or create a room.", Ts: time.Now().Unix()})
			return model, nil
		}
		
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
