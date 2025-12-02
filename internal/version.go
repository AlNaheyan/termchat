package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// Version is the current version of termchat
// This should be updated with each release
const Version = "1.2.0"

const (
	GitHubOwner = "AlNaheyan"
	GitHubRepo  = "termchat"
)

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
}

// GetLatestVersion fetches the latest version from GitHub
func GetLatestVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", GitHubOwner, GitHubRepo)
	
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	
	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	
	// Remove 'v' prefix if present
	version := strings.TrimPrefix(release.TagName, "v")
	return version, nil
}

// CompareVersions compares two semantic version strings
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func CompareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")
	
	// Simple string comparison works for semantic versions in most cases
	// For production, consider using github.com/hashicorp/go-version
	if v1 == v2 {
		return 0
	}
	if v1 > v2 {
		return 1
	}
	return -1
}

// GetDownloadURL returns the download URL for the current platform
func GetDownloadURL(version string) string {
	platform := GetPlatform()
	baseURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s", GitHubOwner, GitHubRepo, version)
	return fmt.Sprintf("%s/%s", baseURL, platform)
}

// GetPlatform returns the binary name for the current platform
func GetPlatform() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	
	switch osName {
	case "darwin":
		if arch == "arm64" {
			return "termchat-macos-arm64"
		}
		return "termchat-macos-amd64"
	case "linux":
		if arch == "arm64" || arch == "aarch64" {
			return "termchat-linux-arm64"
		}
		return "termchat-linux-amd64"
	case "windows":
		return "termchat-windows-amd64.exe"
	default:
		return "termchat-unknown"
	}
}
