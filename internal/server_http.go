package internal

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"termchat/internal/storage"
)

type signupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

type friendsResponse struct {
	Friends []friendDTO `json:"friends"`
}

type friendDTO struct {
	Username string `json:"username"`
	Online   bool   `json:"online"`
}

type friendRequestsResponse struct {
	Incoming []string `json:"incoming"`
	Outgoing []string `json:"outgoing"`
}

type passwordChangeRequest struct {
	Current string `json:"current_password"`
	New     string `json:"new_password"`
}

func (s *Server) HandleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !s.authLimiter.Allow(s.clientIP(r)) {
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	var req signupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		writeError(w, http.StatusBadRequest, errors.New("username and password are required"))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if _, err := s.store.CreateUser(r.Context(), username, hash); err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			writeError(w, http.StatusConflict, errors.New("username already taken"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.metrics.IncSignup()
	writeJSON(w, http.StatusCreated, map[string]string{"username": username})
}

func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !s.authLimiter.Allow(s.clientIP(r)) {
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	var req signupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		writeError(w, http.StatusBadRequest, errors.New("username and password are required"))
		return
	}
	user, err := s.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if user == nil || bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password)) != nil {
		writeError(w, http.StatusUnauthorized, errors.New("invalid credentials"))
		return
	}

	token := uuid.NewString()
	expiresAt := time.Now().Add(s.tokenTTL)
	if err := s.store.CreateSession(r.Context(), user.ID, token, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.metrics.IncLogin()
	writeJSON(w, http.StatusOK, loginResponse{Token: token, Username: user.Username, ExpiresAt: expiresAt})
}

func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	authCtx, err := s.authenticateRequest(r)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnauthorized) {
			status = http.StatusUnauthorized
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	if err := s.store.DeleteSession(r.Context(), authCtx.Token); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandleFriends(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListFriends(w, r)
	default:
		methodNotAllowed(w, http.MethodGet)
	}
}

func (s *Server) handleListFriends(w http.ResponseWriter, r *http.Request) {
	authCtx, err := s.authenticateRequest(r)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnauthorized) {
			status = http.StatusUnauthorized
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	friends, err := s.store.ListFriends(r.Context(), authCtx.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	names := make([]friendDTO, 0, len(friends))
	for _, friend := range friends {
		names = append(names, friendDTO{
			Username: friend.Username,
			Online:   s.presence.Online(friend.ID),
		})
	}
	writeJSON(w, http.StatusOK, friendsResponse{Friends: names})
}

func (s *Server) HandleAddFriend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	authCtx, err := s.authenticateRequest(r)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnauthorized) {
			status = http.StatusUnauthorized
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	prefix := "/friends/"
	username := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, prefix))
	if username == "" {
		writeError(w, http.StatusBadRequest, errors.New("friend username required"))
		return
	}
	if strings.EqualFold(username, authCtx.Username) {
		writeError(w, http.StatusBadRequest, errors.New("cannot add yourself"))
		return
	}
	friend, err := s.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if friend == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if err := s.store.AddFriendship(r.Context(), authCtx.UserID, friend.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandleFriendRequests(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listFriendRequests(w, r)
	default:
		methodNotAllowed(w, http.MethodGet)
	}
}

func (s *Server) listFriendRequests(w http.ResponseWriter, r *http.Request) {
	authCtx, err := s.authenticateRequest(r)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnauthorized) {
			status = http.StatusUnauthorized
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	incoming, err := s.store.ListIncomingFriendRequests(r.Context(), authCtx.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	outgoing, err := s.store.ListOutgoingFriendRequests(r.Context(), authCtx.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	resp := friendRequestsResponse{
		Incoming: make([]string, 0, len(incoming)),
		Outgoing: make([]string, 0, len(outgoing)),
	}
	for _, u := range incoming {
		resp.Incoming = append(resp.Incoming, u.Username)
	}
	for _, u := range outgoing {
		resp.Outgoing = append(resp.Outgoing, u.Username)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) HandleCreateFriendRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	authCtx, err := s.authenticateRequest(r)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnauthorized) {
			status = http.StatusUnauthorized
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	username := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/friend-requests/"))
	if username == "" {
		writeError(w, http.StatusBadRequest, errors.New("username required"))
		return
	}
	friend, err := s.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if friend == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if err := s.store.CreateFriendRequest(r.Context(), authCtx.UserID, friend.ID); err != nil {
		if errors.Is(err, storage.ErrFriendRequestExists) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) HandleRespondFriendRequest(w http.ResponseWriter, r *http.Request) {
	authCtx, err := s.authenticateRequest(r)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnauthorized) {
			status = http.StatusUnauthorized
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/friend-requests/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	friend, err := s.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if friend == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	switch action {
	case "accept":
		if err := s.store.AcceptFriendRequest(r.Context(), friend.ID, authCtx.UserID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	case "decline":
		if err := s.store.DeleteFriendRequest(r.Context(), friend.ID, authCtx.UserID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	case "cancel":
		if err := s.store.DeleteFriendRequest(r.Context(), authCtx.UserID, friend.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	default:
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	authCtx, err := s.authenticateRequest(r)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnauthorized) {
			status = http.StatusUnauthorized
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	var req passwordChangeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.New) == "" || strings.TrimSpace(req.Current) == "" {
		writeError(w, http.StatusBadRequest, errors.New("both current and new passwords required"))
		return
	}
	user, err := s.store.GetUserByID(r.Context(), authCtx.UserID)
	if err != nil || user == nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.Current)) != nil {
		writeError(w, http.StatusUnauthorized, errors.New("current password incorrect"))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.New), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.UpdatePassword(r.Context(), authCtx.UserID, hash); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandleRoomExists(w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "missing room", http.StatusBadRequest)
		return
	}
	if s.hub.Exists(room) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func decodeJSON(r *http.Request, out interface{}) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}
