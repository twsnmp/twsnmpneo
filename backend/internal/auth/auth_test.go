package auth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
)

func setupTestStore(t *testing.T) datastore.DataStore {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_auth.db")
	store, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func TestAuthManager(t *testing.T) {
	store := setupTestStore(t)
	mgr, err := NewManager(store)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	ctx := context.Background()

	// Verify default user was created by bbolt
	user, err := store.GetUser(ctx, "twsnmp")
	if err != nil {
		t.Fatalf("default user not found: %v", err)
	}
	if user.User != "twsnmp" || user.Role != "admin" {
		t.Errorf("unexpected default user: %+v", user)
	}

	// Test Authenticate with correct password
	authedUser, token, err := mgr.Authenticate(ctx, "twsnmp", "twsnmp")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if authedUser.User != "twsnmp" || token == "" {
		t.Errorf("unexpected auth result: user=%v, token=%v", authedUser, token)
	}

	// Validate Token
	claims, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.User != "twsnmp" || claims.Role != "admin" {
		t.Errorf("unexpected claims: %+v", claims)
	}

	// Test Authenticate with invalid password
	_, _, err = mgr.Authenticate(ctx, "twsnmp", "wrong_pass")
	if err == nil {
		t.Fatal("expected auth error for wrong password, got nil")
	}

	// Test Authenticate with nonexistent user
	_, _, err = mgr.Authenticate(ctx, "nonexistent", "pass")
	if err == nil {
		t.Fatal("expected auth error for nonexistent user, got nil")
	}

	// Test ValidateToken with invalid token string
	_, err = mgr.ValidateToken("invalid.token.string")
	if err == nil {
		t.Fatal("expected validate error for invalid token, got nil")
	}

	// Test ValidateToken with tampered signature
	tampered := token[:len(token)-5] + "AAAAA"
	_, err = mgr.ValidateToken(tampered)
	if err == nil {
		t.Fatal("expected validate error for tampered token, got nil")
	}

	// Test Token Expiration
	expToken, err := mgr.GenerateToken(user, -1*time.Minute)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	_, err = mgr.ValidateToken(expToken)
	if err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got: %v", err)
	}
}

func TestPasswordHashing(t *testing.T) {
	pass := "mySecretPassword123!"
	hash, err := HashPassword(pass)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !CheckPassword(pass, hash) {
		t.Error("CheckPassword failed on matching password")
	}
	if CheckPassword("wrongPassword", hash) {
		t.Error("CheckPassword succeeded on mismatched password")
	}
}
