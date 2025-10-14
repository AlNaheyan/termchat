package internal

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// pre styled colors// all from lipglpss
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
