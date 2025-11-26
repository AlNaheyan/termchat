package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// UploadedFile represents metadata for a file uploaded to a room
type UploadedFile struct {
	ID          string    // UUID
	Filename    string    // Original filename
	SizeBytes   int64     // File size in bytes
	UploadedBy  string    // Username of uploader
	StoragePath string    // Relative path from upload base dir
	UploadedAt  time.Time // Upload timestamp
	SHA256      string    // File hash for integrity
}

// FileUploadHandler manages file upload/download operations
type FileUploadHandler struct {
	hub         *Hub
	uploadDir   string // Base directory for uploads (e.g., /data/uploads)
	maxFileSize int64  // Maximum file size in bytes
}

// NewFileUploadHandler creates a new file upload handler
func NewFileUploadHandler(hub *Hub, uploadDir string, maxFileSize int64) *FileUploadHandler {
	return &FileUploadHandler{
		hub:         hub,
		uploadDir:   uploadDir,
		maxFileSize: maxFileSize,
	}
}

// HandleUpload processes multipart file uploads
func (h *FileUploadHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	// Parse multipart form with size limit
	r.Body = http.MaxBytesReader(w, r.Body, h.maxFileSize)
	if err := r.ParseMultipartForm(h.maxFileSize); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, errors.New("file too large"))
		return
	}

	// Extract room key and validate
	roomKey := r.FormValue("room_key")
	if roomKey == "" {
		writeError(w, http.StatusBadRequest, errors.New("room_key required"))
		return
	}

	// Validate room exists
	if !h.hub.Exists(roomKey) {
		writeError(w, http.StatusNotFound, errors.New("room not found"))
		return
	}

	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("no file provided"))
		return
	}
	defer file.Close()

	// Validate filename
	filename := filepath.Base(header.Filename)
	if filename == "" || filename == "." || filename == ".." {
		writeError(w, http.StatusBadRequest, errors.New("invalid filename"))
		return
	}

	// Additional size check
	if header.Size > h.maxFileSize {
		writeError(w, http.StatusRequestEntityTooLarge, errors.New("file too large"))
		return
	}

	// Get username from context (assuming authentication middleware sets this)
	username := r.FormValue("username")
	if username == "" {
		username = "anonymous"
	}

	// Generate unique file ID and storage path
	fileID := uuid.NewString()
	roomDir := filepath.Join(h.uploadDir, sanitizePathComponent(roomKey))
	storagePath := filepath.Join(roomDir, fmt.Sprintf("%s-%s", fileID, filename))

	// Ensure room directory exists
	if err := os.MkdirAll(roomDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to create upload directory: %w", err))
		return
	}

	// Create destination file
	destFile, err := os.Create(storagePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to create file: %w", err))
		return
	}
	defer destFile.Close()

	// Copy file content while computing hash
	hasher := sha256.New()
	multiWriter := io.MultiWriter(destFile, hasher)
	written, err := io.Copy(multiWriter, file)
	if err != nil {
		os.Remove(storagePath) // Cleanup on error
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to save file: %w", err))
		return
	}

	// Create file metadata
	uploadedFile := UploadedFile{
		ID:          fileID,
		Filename:    filename,
		SizeBytes:   written,
		UploadedBy:  username,
		StoragePath: filepath.Join(sanitizePathComponent(roomKey), fmt.Sprintf("%s-%s", fileID, filename)),
		UploadedAt:  time.Now(),
		SHA256:      hex.EncodeToString(hasher.Sum(nil)),
	}

	// Register file with room
	room := h.hub.getRoom(roomKey)
	if room == nil {
		os.Remove(storagePath) // Cleanup if room disappeared
		writeError(w, http.StatusNotFound, errors.New("room no longer exists"))
		return
	}
	room.addFile(uploadedFile)

	// Broadcast file upload event to room
	fileMsg := FileUploadMessage{
		Type:       "file_uploaded",
		FileID:     fileID,
		Filename:   filename,
		SizeBytes:  written,
		UploadedBy: username,
		UploadedAt: uploadedFile.UploadedAt.Unix(),
	}
	if encoded, err := marshalJSON(fileMsg); err == nil {
		room.broadcast <- encoded
	}

	// Return success response
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"file_id":  fileID,
		"filename": filename,
		"size":     written,
		"status":   "uploaded",
	})
}

// HandleDownload serves file downloads
func (h *FileUploadHandler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	// Extract file ID from URL path (e.g., /api/files/{fileId})
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/files/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "file ID required", http.StatusBadRequest)
		return
	}
	fileID := pathParts[0]

	// Get room key from query params
	roomKey := r.URL.Query().Get("room")
	if roomKey == "" {
		writeError(w, http.StatusBadRequest, errors.New("room parameter required"))
		return
	}

	// Get room and find file
	room := h.hub.getRoom(roomKey)
	if room == nil {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	fileInfo := room.getFile(fileID)
	if fileInfo == nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Construct full file path
	filePath := filepath.Join(h.uploadDir, fileInfo.StoragePath)

	// Security check: ensure path is within upload directory
	absPath, err := filepath.Abs(filePath)
	if err != nil || !strings.HasPrefix(absPath, filepath.Clean(h.uploadDir)) {
		http.Error(w, "invalid file path", http.StatusForbidden)
		return
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found on disk", http.StatusNotFound)
		} else {
			writeError(w, http.StatusInternalServerError, err)
		}
		return
	}
	defer file.Close()

	// Get file info for size
	stat, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Set headers for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileInfo.Filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))

	// Stream file to client
	http.ServeContent(w, r, fileInfo.Filename, fileInfo.UploadedAt, file)
}

// sanitizePathComponent removes dangerous characters from path components
func sanitizePathComponent(s string) string {
	// Remove any path separators and null bytes
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.TrimSpace(s)
	if s == "" || s == "." || s == ".." {
		return "unnamed"
	}
	return s
}

// marshalJSON is a helper to encode structs to JSON
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
