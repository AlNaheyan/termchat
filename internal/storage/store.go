package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	sqlite "modernc.org/sqlite"
)

const (
	sqliteConstraintCode = 19
	defaultBusyTimeout   = 5000
)

// Store wraps the SQLite handle and exposes helper methods used by the server.
type Store struct {
	db *sql.DB
}

// User represents a row in the users table.
type User struct {
	ID           int64
	Username     string
	PasswordHash []byte
	CreatedAt    time.Time
}

// Session captures persisted logins.
type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

// ErrUserExists is returned when attempting to insert a duplicate username.
var ErrUserExists = errors.New("user already exists")

// ErrFriendRequestExists is returned when a friend request is already pending.
var ErrFriendRequestExists = errors.New("friend request already exists")

// NewStore initializes the SQLite database at the provided path. Call Close when done.
func NewStore(path string) (*Store, error) {
	if path == "" {
		path = "termchat.db"
	}
	dsn := buildDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d;", defaultBusyTimeout)); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the underlying DB connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func buildDSN(path string) string {
	switch {
	case strings.HasPrefix(path, "sqlite://"):
		path = path[len("sqlite://"):]
	case strings.HasPrefix(path, "file:"), strings.HasPrefix(path, ":memory:"):
		// already in a form sqlite understands
	default:
		path = "file:" + path
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%s_pragma=busy_timeout=%d&_pragma=foreign_keys=ON", path, separator, defaultBusyTimeout)
}

// Migrate runs the schema creation statements.
func (s *Store) Migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash BLOB NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS friendships (
			user_id INTEGER NOT NULL,
			friend_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, friend_id),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(friend_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS friend_requests (
			requester_id INTEGER NOT NULL,
			receiver_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (requester_id, receiver_id),
			FOREIGN KEY(requester_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(receiver_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	for _, stmt := range statements {
		if _, err = tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CreateUser inserts a new user. ErrUserExists is returned on conflicts.
func (s *Store) CreateUser(ctx context.Context, username string, passwordHash []byte) (int64, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO users(username, password_hash) VALUES(?, ?)`, username, passwordHash)
	if err != nil {
		if isConstraintError(err) {
			return 0, ErrUserExists
		}
		return 0, err
	}
	return result.LastInsertId()
}

// GetUserByUsername fetches a user by username.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, created_at FROM users WHERE username = ?`, username)
	var user User
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByID fetches a user by primary key.
func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, created_at FROM users WHERE id = ?`, id)
	var user User
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// CreateSession stores a new session token for a user.
func (s *Store) CreateSession(ctx context.Context, userID int64, token string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions(token, user_id, expires_at) VALUES(?, ?, ?)`, token, userID, expiresAt.UTC())
	return err
}

// GetSession returns a session if it exists.
func (s *Store) GetSession(ctx context.Context, token string) (*Session, error) {
	row := s.db.QueryRowContext(ctx, `SELECT token, user_id, expires_at, created_at FROM sessions WHERE token = ?`, token)
	var sess Session
	if err := row.Scan(&sess.Token, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &sess, nil
}

// DeleteSession removes a session token (used for logout).
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// AddFriendship inserts symmetric rows for a friendship pair.
func (s *Store) AddFriendship(ctx context.Context, userID, friendID int64) error {
	if userID == friendID {
		return fmt.Errorf("cannot friend yourself")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO friendships(user_id, friend_id) VALUES(?, ?)`, userID, friendID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO friendships(user_id, friend_id) VALUES(?, ?)`, friendID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// ListFriends returns all friends for a given user (ordered by username).
func (s *Store) ListFriends(ctx context.Context, userID int64) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.password_hash, u.created_at
		FROM friendships f
		JOIN users u ON u.id = f.friend_id
		WHERE f.user_id = ?
		ORDER BY u.username ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var friends []User
	for rows.Next() {
		var friend User
		if err := rows.Scan(&friend.ID, &friend.Username, &friend.PasswordHash, &friend.CreatedAt); err != nil {
			return nil, err
		}
		friends = append(friends, friend)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return friends, nil
}

// AreFriends reports whether two users are already connected.
func (s *Store) AreFriends(ctx context.Context, userID, friendID int64) (bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM friendships WHERE user_id = ? AND friend_id = ?`, userID, friendID)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateFriendRequest stores a pending request if one does not already exist.
func (s *Store) CreateFriendRequest(ctx context.Context, requesterID, receiverID int64) error {
	if requesterID == receiverID {
		return fmt.Errorf("cannot send a friend request to yourself")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	// Prevent duplicates or already-friends cases.
	var existing int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM friendships WHERE user_id=? AND friend_id=?`, requesterID, receiverID).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return ErrFriendRequestExists
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM friend_requests WHERE requester_id=? AND receiver_id=?`, requesterID, receiverID).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return ErrFriendRequestExists
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM friend_requests WHERE requester_id=? AND receiver_id=?`, receiverID, requesterID).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return ErrFriendRequestExists
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO friend_requests(requester_id, receiver_id) VALUES(?, ?)`, requesterID, receiverID); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteFriendRequest removes any pending request between the two users.
func (s *Store) DeleteFriendRequest(ctx context.Context, requesterID, receiverID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM friend_requests WHERE requester_id=? AND receiver_id=?`, requesterID, receiverID)
	return err
}

// ListIncomingFriendRequests fetches usernames for users who requested the authenticated user.
func (s *Store) ListIncomingFriendRequests(ctx context.Context, userID int64) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.password_hash, u.created_at
		FROM friend_requests fr
		JOIN users u ON u.id = fr.requester_id
		WHERE fr.receiver_id = ?
		ORDER BY fr.created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ListOutgoingFriendRequests fetches pending requests sent by a user.
func (s *Store) ListOutgoingFriendRequests(ctx context.Context, userID int64) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.password_hash, u.created_at
		FROM friend_requests fr
		JOIN users u ON u.id = fr.receiver_id
		WHERE fr.requester_id = ?
		ORDER BY fr.created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// AcceptFriendRequest converts the pending request into a friendship.
func (s *Store) AcceptFriendRequest(ctx context.Context, requesterID, receiverID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	res, err := tx.ExecContext(ctx, `DELETE FROM friend_requests WHERE requester_id=? AND receiver_id=?`, requesterID, receiverID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO friendships(user_id, friend_id) VALUES(?, ?)`, requesterID, receiverID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO friendships(user_id, friend_id) VALUES(?, ?)`, receiverID, requesterID); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdatePassword replaces the stored password hash for a user.
func (s *Store) UpdatePassword(ctx context.Context, userID int64, newHash []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash=? WHERE id=?`, newHash, userID)
	return err
}

func isConstraintError(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code() == sqliteConstraintCode
	}
	return false
}
