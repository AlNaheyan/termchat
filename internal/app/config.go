package app

import (
	"os"
	"path/filepath"
	"runtime"
)

// ServerConfig defines how the HTTP/WebSocket backend should run.
type ServerConfig struct {
	Addr        string
	Path        string
	DBPath      string
	UploadDir   string // Base directory for file uploads (e.g., /data/uploads)
	MaxFileSize int64  // Maximum file size in bytes (default: 10MB)
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

// DefaultUploadDir returns a sensible default for file uploads
// Uses /data/uploads for production (Fly.io), local directory for dev
func DefaultUploadDir() string {
	if env := os.Getenv("TERMCHAT_UPLOAD_DIR"); env != "" {
		return env
	}
	// Check if /data is writable (production environment)
	if _, err := os.Stat("/data"); err == nil {
		// Try to create a test file to verify it's writable
		testPath := "/data/.termchat-write-test"
		if f, err := os.Create(testPath); err == nil {
			f.Close()
			os.Remove(testPath)
			return "/data/uploads"
		}
	}
	// Use local directory for development
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".termchat", "uploads")
	}
	return filepath.Join(".", ".termchat", "uploads")
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
