package internal

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// pre styled colors// all from lipglpss
var (
	appTitleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")).Padding(0, 1)
	subtitleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("110")).MarginTop(1)
	menuBoxStyle        = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(1, 2).MarginTop(1)
	menuItemStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).PaddingLeft(1)
	menuHotkeyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	menuHintStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).MarginTop(1)
	noticeBoxStyle      = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("95")).Padding(1, 2).MarginTop(1)
	chatHeaderStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(lipgloss.Color("63")).Padding(0, 1)
	statusStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("109")).MarginTop(1)
	connectedStyle      = statusStyle.Copy().Foreground(lipgloss.Color("42")).Bold(true)
	connectingStyle     = statusStyle.Copy().Foreground(lipgloss.Color("178")).Italic(true)
	messageBodyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("253"))
	messageBoxStyle     = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("60")).Padding(1, 2).MarginTop(1)
	inputBoxStyle       = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(0, 1).MarginTop(1)
	timestampStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	usernameStyle       = lipgloss.NewStyle().Bold(true)
	activeUserStyle     = usernameStyle.Copy().Foreground(lipgloss.Color("213"))
	systemMessageStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Italic(true)
	errorStyle          = statusStyle.Copy().Foreground(lipgloss.Color("196")).Bold(true)
	dividerStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("237")).Render(" ┃ ")
	friendSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	friendItemStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	userColorPalette    = []lipgloss.Color{
		lipgloss.Color("45"),
		lipgloss.Color("81"),
		lipgloss.Color("141"),
		lipgloss.Color("98"),
		lipgloss.Color("63"),
		lipgloss.Color("135"),
		lipgloss.Color("32"),
	}
)

func (model TUIModel) View() string {
	switch model.mode {
	case modeAuthMenu:
		return model.renderAuthMenuView()
	case modeAuthUsername, modeAuthPassword:
		return model.renderAuthPromptView()
	case modeFriends:
		return model.renderFriendsView()
	case modeAddFriend:
		return model.renderInputView("Add a friend", "Enter the username you want to add.")
	case modeManualRoom:
		return model.renderInputView("Join a room", "Enter a room code and press Enter.")
	case modeRequestsIncoming:
		return model.renderRequestsView(requestViewIncoming)
	case modeRequestsOutgoing:
		return model.renderRequestsView(requestViewOutgoing)
	default:
		return model.renderChatView()
	}
}

func (model TUIModel) renderAuthMenuView() string {
	title := appTitleStyle.Render("TermChat")
	subtitle := subtitleStyle.Render("Chat with trusted friends from your terminal")

	options := []string{
		renderMenuOption("1", "Log in"),
		renderMenuOption("2", "Sign up"),
		renderMenuOption("q", "Quit"),
	}

	viewSections := []string{
		lipgloss.JoinVertical(lipgloss.Left, title, subtitle),
		menuBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, options...)),
	}

	if model.loading {
		viewSections = append(viewSections, connectingStyle.Render("Working…"))
	}

	if notices := model.renderSystemNotices(); notices != "" {
		viewSections = append(viewSections, notices)
	}

	viewSections = append(viewSections, menuHintStyle.Render("1) Log in  •  2) Sign up  •  q) Quit"))

	return lipgloss.JoinVertical(lipgloss.Left, viewSections...)
}

func (model TUIModel) renderAuthPromptView() string {
	title := "Log in"
	if model.authIntent == authIntentSignup {
		title = "Create an account"
	}
	hint := "Enter your username"
	if model.mode == modeAuthPassword {
		hint = "Enter your password"
	}

	return model.renderPrompt(title, hint)
}

func (model TUIModel) renderInputView(title, hint string) string {
	return model.renderPrompt(title, hint)
}

func (model TUIModel) renderPrompt(title, hint string) string {
	header := appTitleStyle.Render(title)
	hintText := menuHintStyle.Render(hint)

	viewSections := []string{header, hintText}

	if model.loading {
		viewSections = append(viewSections, connectingStyle.Render("Working…"))
	}

	if notices := model.renderSystemNotices(); notices != "" {
		viewSections = append(viewSections, notices)
	}

	viewSections = append(viewSections, inputBoxStyle.Render(model.textInput.View()))

	return lipgloss.JoinVertical(lipgloss.Left, viewSections...)
}

func (model TUIModel) renderFriendsView() string {
	title := appTitleStyle.Render(fmt.Sprintf("Welcome, %s", model.username))
	subtitle := subtitleStyle.Render(fmt.Sprintf("Friends online: %d  |  Incoming requests: %d  |  Outgoing requests: %d", model.countOnlineFriends(), len(model.incomingReqs), len(model.outgoingReqs)))

	viewSections := []string{title, subtitle}

	if model.loading {
		viewSections = append(viewSections, connectingStyle.Render("Loading friends…"))
	}

	if notices := model.renderSystemNotices(); notices != "" {
		viewSections = append(viewSections, notices)
	}

	var friendLines []string
	if len(model.friends) == 0 {
		friendLines = append(friendLines, menuHintStyle.Render("No friends yet. Press A to add someone."))
	} else {
		for idx, friend := range model.friends {
			if idx == model.selectedFriend {
				friendLines = append(friendLines, friendSelectedStyle.Render(fmt.Sprintf("➤ %s %s", presenceDot(friend.Online), friend.Username)))
			} else {
				friendLines = append(friendLines, friendItemStyle.Render(fmt.Sprintf("  %s %s", presenceDot(friend.Online), friend.Username)))
			}
		}
	}
	viewSections = append(viewSections, menuBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, friendLines...)))

	hints := menuHintStyle.Render("↑/↓ select • Enter chat • A add friend • I incoming requests • O outgoing requests • M join room • N new room • R refresh • L logout • Q quit")
	viewSections = append(viewSections, hints)

	return lipgloss.JoinVertical(lipgloss.Left, viewSections...)
}

func (model TUIModel) renderRequestsView(view requestViewType) string {
	title := "Incoming friend requests"
	list := model.incomingReqs
	if view == requestViewOutgoing {
		title = "Outgoing friend requests"
		list = model.outgoingReqs
	}
	header := appTitleStyle.Render(title)
	viewSections := []string{header, menuHintStyle.Render("Enter to accept (incoming only) • D decline/cancel • Esc back")}
	if notices := model.renderSystemNotices(); notices != "" {
		viewSections = append(viewSections, notices)
	}
	var lines []string
	if len(list) == 0 {
		lines = append(lines, menuHintStyle.Render("No requests."))
	} else {
		for idx, name := range list {
			prefix := "  "
			if idx == model.selectedRequest {
				prefix = "➤ "
				lines = append(lines, friendSelectedStyle.Render(prefix+name))
			} else {
				lines = append(lines, friendItemStyle.Render(prefix+name))
			}
		}
	}
	viewSections = append(viewSections, menuBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...)))
	return lipgloss.JoinVertical(lipgloss.Left, viewSections...)
}

func (model TUIModel) renderChatView() string {
	headerSegments := []string{"TermChat"}
	if model.currentFriend != "" {
		headerSegments = append(headerSegments, fmt.Sprintf("Chat with %s", model.currentFriend))
	} else if model.roomKey != "" {
		headerSegments = append(headerSegments, fmt.Sprintf("Room %s", model.roomKey))
	}
	headerSegments = append(headerSegments, fmt.Sprintf("User %s", model.username))
	headerSegments = append(headerSegments, fmt.Sprintf("Server %s", model.serverJoinURL))
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
	footerHint := menuHintStyle.Render("Esc or /leave to return to menu")

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

// renderChatMessage renders a single log line. It stamps the timestamp, picks
// a color for the sender, and indents multi-line messages so they stay legible.
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

func presenceDot(online bool) string {
	if online {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("●")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○")
}

func (model TUIModel) countOnlineFriends() int {
	count := 0
	for _, f := range model.friends {
		if f.Online {
			count++
		}
	}
	return count
}

// color for users
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
