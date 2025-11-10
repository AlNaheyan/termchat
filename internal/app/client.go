package app

import (
	"errors"

	intrnl "termchat/internal"
)

// RunClient launches the Bubble Tea TUI with the provided configuration.
func RunClient(cfg ClientConfig) error {
	if cfg.ServerURL == "" {
		return errors.New("server URL is required")
	}
	return intrnl.RunClient(cfg.ServerURL, cfg.RoomKey, cfg.Username)
}
