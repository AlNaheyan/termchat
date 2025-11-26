package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// browseDirectory reads directory contents for the file browser
func browseDirectory(path string) ([]FileItem, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	items := make([]FileItem, 0, len(entries)+1)

	// Add parent directory entry if not at root
	if path != "/" && path != "." {
		items = append(items, FileItem{
			Name:  "..",
			Path:  filepath.Dir(path),
			IsDir: true,
		})
	}

	// Add directory entries
	for _, entry := range entries {
		// Skip hidden files
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())
		item := FileItem{
			Name:  entry.Name(),
			Path:  fullPath,
			IsDir: entry.IsDir(),
		}

		if !entry.IsDir() {
			if info, err := entry.Info(); err == nil {
				item.Size = info.Size()
			}
		}

		items = append(items, item)
	}

	// Sort: directories first, then files, both alphabetically
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return items[i].Name < items[j].Name
	})

	return items, nil
}

// getDefaultBrowsePath returns a sensible starting directory for file browser
func getDefaultBrowsePath() string {
	// Try home directory first
	if home, err := os.UserHomeDir(); err == nil {
		// Check common document directories
		docsPath := filepath.Join(home, "Documents")
		if _, err := os.Stat(docsPath); err == nil {
			return docsPath
		}
		downloadsPath := filepath.Join(home, "Downloads")
		if _, err := os.Stat(downloadsPath); err == nil {
			return downloadsPath
		}
		return home
	}
	// Fallback to current directory
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// formatFileSize returns a human-readable file size
func formatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
