package app

import (
	"os"
	"path/filepath"
	"runtime"
)

// ServerConfig defines how the HTTP/WebSocket backend should run.
type ServerConfig struct {
	Addr   string
	Path   string
	DBPath string
}

// ClientConfig defines the parameters the TUI client needs.
type ClientConfig struct {
	ServerURL string
	Username  string
	RoomKey   string
}

// DefaultDBPath returns a per-user data path for the bundled SQLite file.
func DefaultDBPath() string {
	if env := os.Getenv("TERMCHAT_DB_PATH"); env != "" {
		return env
	}
	if env := os.Getenv("TERMCHAT_DATA_DIR"); env != "" {
		return filepath.Join(env, "termchat.db")
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "termchat", "termchat.db")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Termchat", "termchat.db")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Application Support", "Termchat", "termchat.db")
		}
		return filepath.Join(home, ".local", "share", "termchat", "termchat.db")
	}
	return filepath.Join(".", ".termchat", "termchat.db")
}

// NormalizeJoinPath guarantees the websocket join path starts with '/' and
// falls back to /join when empty.
func NormalizeJoinPath(path string) string {
	if path == "" {
		return "/join"
	}
	if path[0] != '/' {
		return "/" + path
	}
	return path
}
