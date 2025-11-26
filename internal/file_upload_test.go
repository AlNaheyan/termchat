package internal

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestFileUploadHandler verifies the basic file upload flow
func TestFileUploadHandler(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	hub := NewHub()
	handler := NewFileUploadHandler(hub, tmpDir, 10*1024*1024)

	// Create a test room
	room := hub.getOrCreateRoom("testroom")
	if room == nil {
		t.Fatal("failed to create test room")
	}

	// Create a test file in memory
	fileContent := []byte("Hello, this is a test file!")

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file field
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, bytes.NewReader(fileContent)); err != nil {
		t.Fatal(err)
	}

	// Add room_key field
	if err := writer.WriteField("room_key", "testroom"); err != nil {
		t.Fatal(err)
	}

	// Add username field
	if err := writer.WriteField("username", "testuser"); err != nil {
		t.Fatal(err)
	}

	writer.Close()

	// Create HTTP request
	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	// Call handler
	handler.HandleUpload(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify file was tracked in room
	if len(room.files) != 1 {
		t.Fatalf("expected 1 file in room, got %d", len(room.files))
	}

	uploadedFile := room.files[0]
	if uploadedFile.Filename != "test.txt" {
		t.Errorf("expected filename 'test.txt', got %s", uploadedFile.Filename)
	}
	if uploadedFile.UploadedBy != "testuser" {
		t.Errorf("expected uploader 'testuser', got %s", uploadedFile.UploadedBy)
	}
	if uploadedFile.SizeBytes != int64(len(fileContent)) {
		t.Errorf("expected size %d, got %d", len(fileContent), uploadedFile.SizeBytes)
	}

	// Verify file exists on disk
	expectedPath := filepath.Join(tmpDir, "testroom", uploadedFile.ID+"-test.txt")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("file does not exist at expected path: %s", expectedPath)
	}
}

// TestFileCleanup verifies files are deleted when room closes
func TestFileCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	hub := NewHub()

	// Create a room and add a mock file
	room := hub.getOrCreateRoom("cleanuptest")

	// Create actual file on disk
	roomDir := filepath.Join(tmpDir, "cleanuptest")
	if err := os.MkdirAll(roomDir, 0755); err != nil {
		t.Fatal(err)
	}

	testFilePath := filepath.Join(roomDir, "test-file.txt")
	if err := os.WriteFile(testFilePath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Add file metadata to room
	room.addFile(UploadedFile{
		ID:          "test-id",
		Filename:    "file.txt",
		StoragePath: "cleanuptest/test-file.txt",
		SizeBytes:   4,
	})

	// Verify file exists
	if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
		t.Fatal("test file should exist before cleanup")
	}

	// Cleanup
	room.deleteAllFiles(tmpDir)

	// Verify file and directory are deleted
	if _, err := os.Stat(roomDir); !os.IsNotExist(err) {
		t.Error("room directory should be deleted after cleanup")
	}

	// Verify files list is cleared
	if len(room.files) != 0 {
		t.Errorf("expected 0 files after cleanup, got %d", len(room.files))
	}
}

// TestFileSizeLimit verifies files exceeding size limit are rejected
func TestFileSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	hub := NewHub()
	maxSize := int64(100) // 100 bytes limit
	handler := NewFileUploadHandler(hub, tmpDir, maxSize)

	_ = hub.getOrCreateRoom("testroom")

	// Create large file (exceeds limit)
	largeContent := bytes.Repeat([]byte("a"), 200)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, _ := writer.CreateFormFile("file", "large.txt")
	io.Copy(part, bytes.NewReader(largeContent))
	writer.WriteField("room_key", "testroom")
	writer.WriteField("username", "testuser")
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handler.HandleUpload(rec, req)

	// Should be rejected
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", rec.Code)
	}
}
