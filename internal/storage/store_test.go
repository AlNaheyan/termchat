package storage

import (
	"context"
	"testing"
	"time"
)

func TestUserLifecycle(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	id, err := store.CreateUser(ctx, "alice", []byte("hash"))
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if id == 0 {
		t.Fatalf("expected id > 0")
	}
	if _, err := store.CreateUser(ctx, "alice", []byte("hash2")); err == nil {
		t.Fatalf("expected duplicate error")
	}

	user, err := store.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if user == nil || user.Username != "alice" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestSessionLifecycle(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	userID, err := store.CreateUser(ctx, "bob", []byte("hash"))
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	exp := time.Now().Add(time.Hour)
	if err := store.CreateSession(ctx, userID, "token123", exp); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	session, err := store.GetSession(ctx, "token123")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if session == nil || session.UserID != userID {
		t.Fatalf("unexpected session: %+v", session)
	}
	if err := store.DeleteSession(ctx, "token123"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	session, err = store.GetSession(ctx, "token123")
	if err != nil {
		t.Fatalf("GetSession after delete: %v", err)
	}
	if session != nil {
		t.Fatalf("expected nil session after delete")
	}
}

func TestFriendships(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	aliceID, err := store.CreateUser(ctx, "alice", []byte("hash1"))
	if err != nil {
		t.Fatalf("CreateUser alice: %v", err)
	}
	bobID, err := store.CreateUser(ctx, "bob", []byte("hash2"))
	if err != nil {
		t.Fatalf("CreateUser bob: %v", err)
	}
	if err := store.AddFriendship(ctx, aliceID, bobID); err != nil {
		t.Fatalf("AddFriendship: %v", err)
	}
	if err := store.AddFriendship(ctx, aliceID, bobID); err != nil {
		t.Fatalf("AddFriendship idempotent: %v", err)
	}
	friends, err := store.ListFriends(ctx, aliceID)
	if err != nil {
		t.Fatalf("ListFriends: %v", err)
	}
	if len(friends) != 1 || friends[0].Username != "bob" {
		t.Fatalf("unexpected friends: %+v", friends)
	}
}

func TestFriendRequests(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	aliceID, _ := store.CreateUser(ctx, "alice", []byte("hash1"))
	bobID, _ := store.CreateUser(ctx, "bob", []byte("hash2"))
	if err := store.CreateFriendRequest(ctx, aliceID, bobID); err != nil {
		t.Fatalf("CreateFriendRequest: %v", err)
	}
	if err := store.CreateFriendRequest(ctx, aliceID, bobID); err == nil {
		t.Fatalf("expected duplicate friend request error")
	}
	incoming, err := store.ListIncomingFriendRequests(ctx, bobID)
	if err != nil {
		t.Fatalf("ListIncomingFriendRequests: %v", err)
	}
	if len(incoming) != 1 || incoming[0].Username != "alice" {
		t.Fatalf("unexpected incoming: %+v", incoming)
	}
	if err := store.AcceptFriendRequest(ctx, aliceID, bobID); err != nil {
		t.Fatalf("AcceptFriendRequest: %v", err)
	}
	friends, err := store.ListFriends(ctx, bobID)
	if err != nil || len(friends) != 1 || friends[0].Username != "alice" {
		t.Fatalf("expected alice as friend: %+v, err=%v", friends, err)
	}
}

func TestUpdatePassword(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	aliceID, _ := store.CreateUser(ctx, "alice", []byte("hash1"))
	if err := store.UpdatePassword(ctx, aliceID, []byte("hash2")); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	user, _ := store.GetUserByUsername(ctx, "alice")
	if string(user.PasswordHash) != "hash2" {
		t.Fatalf("expected updated hash, got %s", string(user.PasswordHash))
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := "sqlite://file:" + t.Name() + "?mode=memory&cache=shared"
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}
