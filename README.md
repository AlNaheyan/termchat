# Termchat

A lightweight, real-time terminal-based chat application built with Go and Bubble Tea.

## ✨ Features

- 🔐 **Secure Authentication** - User accounts with encrypted sessions
- 👥 **Friend System** - Add friends and manage friend requests
- 💬 **Real-time Chat** - WebSocket-powered instant messaging
- 📎 **File Sharing** - Upload and download files in chat rooms
- 🔒 **Privacy-First** - Ephemeral rooms and messages (deleted when empty)
- 🎨 **Beautiful TUI** - Clean terminal interface with Bubble Tea

## 🚀 Quick Install

### One-Liner (macOS/Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/AlNaheyan/termchat/main/install.sh | sh
```

### Manual Install

**macOS:**
```bash
# Intel Mac
curl -L https://github.com/AlNaheyan/termchat/releases/latest/download/termchat-macos-amd64 -o termchat

# Apple Silicon Mac
curl -L https://github.com/AlNaheyan/termchat/releases/latest/download/termchat-macos-arm64 -o termchat

chmod +x termchat
sudo mv termchat /usr/local/bin/
```

**Linux:**
```bash
curl -L https://github.com/AlNaheyan/termchat/releases/latest/download/termchat-linux-amd64 -o termchat
chmod +x termchat
sudo mv termchat /usr/local/bin/
```

**Windows:**
Download [termchat-windows-amd64.exe](https://github.com/AlNaheyan/termchat/releases/latest) and add to PATH.

## 💡 Usage

### Start Chatting

```bash
# Join a room
termchat myroom

# Or create your own room name
termchat secret-project-chat
```

### Commands

**In Chat:**
- `/upload <filepath>` - Upload a file
- `/download <filename>` - Download a file
- `/leave` - Exit the room

**Example:**
```bash
> /upload ~/Documents/report.pdf
Uploading report.pdf...
✓ Uploaded: report.pdf
📎 alice uploaded: report.pdf (2.4 MB)

> /download report.pdf
✓ Downloaded: report.pdf → /Users/you/Downloads/report.pdf
```