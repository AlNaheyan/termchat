package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	httpTimeout = 5 * time.Second
)

type sessionFile struct {
	Username string `json:"username"`
	Token    string `json:"token"`
}

type friendListResponse struct {
	Friends []struct {
		Username string `json:"username"`
		Online   bool   `json:"online"`
	} `json:"friends"`
}

type friendRequestsPayload struct {
	Incoming []string `json:"incoming"`
	Outgoing []string `json:"outgoing"`
}

func apiSignup(baseURL, username, password string) error {
	payload := map[string]string{"username": username, "password": password}
	return doJSONRequest(http.MethodPost, baseURL+"/signup", "", payload, nil)
}

func apiLogin(baseURL, username, password string) (*loginResponse, error) {
	payload := map[string]string{"username": username, "password": password}
	var resp loginResponse
	if err := doJSONRequest(http.MethodPost, baseURL+"/login", "", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func apiLogout(baseURL, token string) error {
	return doJSONRequest(http.MethodPost, baseURL+"/logout", token, nil, nil)
}

func apiGetFriends(baseURL, token string) ([]Friend, error) {
	var resp friendListResponse
	if err := doJSONRequest(http.MethodGet, baseURL+"/friends", token, nil, &resp); err != nil {
		return nil, err
	}
	friends := make([]Friend, 0, len(resp.Friends))
	for _, f := range resp.Friends {
		friends = append(friends, Friend{Username: f.Username, Online: f.Online})
	}
	return friends, nil
}

func apiSendFriendRequest(baseURL, token, friendUsername string) error {
	path := baseURL + "/friend-requests/" + url.PathEscape(friendUsername)
	return doJSONRequest(http.MethodPost, path, token, nil, nil)
}

func apiGetFriendRequests(baseURL, token string) (friendRequestsPayload, error) {
	var resp friendRequestsPayload
	err := doJSONRequest(http.MethodGet, baseURL+"/friend-requests", token, nil, &resp)
	return resp, err
}

func apiRespondFriendRequest(baseURL, token, friendUsername, action string) error {
	path := baseURL + "/friend-requests/" + url.PathEscape(friendUsername) + "/" + action
	return doJSONRequest(http.MethodPost, path, token, nil, nil)
}

func doJSONRequest(method, endpoint, token string, payload interface{}, out interface{}) error {
	var body io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewBuffer(buf)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, readResponseError(resp.Body))
	}
	if out != nil && resp.ContentLength != 0 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return err
		}
	} else if out != nil && resp.ContentLength == 0 {
		// Try to decode in case server sent chunked body without length header.
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, out); err != nil {
				return err
			}
		}
	}
	return nil
}

func readResponseError(body io.Reader) string {
	data, err := io.ReadAll(body)
	if err != nil || len(data) == 0 {
		return "request failed"
	}
	var parsed map[string]string
	if err := json.Unmarshal(data, &parsed); err == nil {
		if msg, ok := parsed["error"]; ok {
			return msg
		}
	}
	return strings.TrimSpace(string(data))
}

func httpBaseFromJoinURL(wsURL string) (string, error) {
	parsed, err := url.Parse(wsURL)
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "ws":
		parsed.Scheme = "http"
	case "wss":
		parsed.Scheme = "https"
	default:
		return "", fmt.Errorf("unsupported scheme %s", parsed.Scheme)
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func loadSessionFromDisk(path string) (*sessionFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session sessionFile
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	if session.Username == "" || session.Token == "" {
		return nil, errors.New("session file incomplete")
	}
	return &session, nil
}

func saveSessionToDisk(path string, session sessionFile) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func deleteSessionFile(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
