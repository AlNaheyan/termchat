package internal

type ChatMessage struct {
	Room string `json:"room"`
	User string `json:"user"`
	Body string `json:"body"`
	Ts   int64  `json:"ts"`
}

// FileUploadMessage is broadcast when a file is uploaded to a room
type FileUploadMessage struct {
	Type       string `json:"type"`        // "file_uploaded"
	FileID     string `json:"file_id"`     // UUID of the file
	Filename   string `json:"filename"`    // Original filename
	SizeBytes  int64  `json:"size_bytes"`  // File size
	UploadedBy string `json:"uploaded_by"` // Username
	UploadedAt int64  `json:"uploaded_at"` // Unix timestamp
}
