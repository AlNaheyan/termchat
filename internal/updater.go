package internal

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// CheckForUpdate checks if a newer version is available
func CheckForUpdate() (bool, string, error) {
	latest, err := GetLatestVersion()
	if err != nil {
		return false, "", err
	}
	
	// Compare versions
	if CompareVersions(latest, Version) > 0 {
		return true, latest, nil
	}
	
	return false, latest, nil
}

// UpdateToLatest downloads and installs the latest version
func UpdateToLatest() error {
	fmt.Println("Checking for updates...")
	
	latest, err := GetLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	
	if CompareVersions(latest, Version) <= 0 {
		fmt.Printf("You're already on the latest version (v%s)\n", Version)
		return nil
	}
	
	fmt.Printf("Updating from v%s to v%s...\n", Version, latest)
	
	// Get download URL
	downloadURL := GetDownloadURL(latest)
	
	// Download new binary
	fmt.Println("Downloading...")
	tmpFile, err := downloadBinary(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tmpFile)
	
	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	
	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}
	
	// Make the new binary executable
	if err := os.Chmod(tmpFile, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}
	
	// Replace current binary
	fmt.Println("Installing...")
	if err := replaceBinary(tmpFile, execPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}
	
	fmt.Printf("✓ Successfully updated to v%s!\n", latest)
	fmt.Println("Restart termchat to use the new version.")
	
	return nil
}

// downloadBinary downloads a binary from the given URL
func downloadBinary(url string) (string, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "termchat-update-*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()
	
	// Download to temporary file
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}
	
	return tmpFile.Name(), nil
}

// replaceBinary replaces the old binary with the new one
func replaceBinary(newPath, oldPath string) error {
	// On Windows, we can't replace a running executable
	// On Unix, we can replace it while running
	if runtime.GOOS == "windows" {
		// Move old binary to .old
		oldBackup := oldPath + ".old"
		if err := os.Rename(oldPath, oldBackup); err != nil {
			return fmt.Errorf("failed to backup old binary: %w", err)
		}
		
		// Copy new binary
		if err := copyFile(newPath, oldPath); err != nil {
			// Restore backup on failure
			os.Rename(oldBackup, oldPath)
			return err
		}
		
		// Clean up backup
		os.Remove(oldBackup)
	} else {
		// Unix: can replace while running
		if err := os.Rename(newPath, oldPath); err != nil {
			// If rename fails (e.g., cross-device), try copy
			if err := copyFile(newPath, oldPath); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()
	
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	
	if _, err := io.Copy(destination, source); err != nil {
		return err
	}
	
	// Copy permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	
	return os.Chmod(dst, srcInfo.Mode())
}
