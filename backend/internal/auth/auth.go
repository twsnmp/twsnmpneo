package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken   = errors.New("invalid token")
	ErrTokenExpired   = errors.New("token expired")
	ErrAuthFailed     = errors.New("authentication failed")
	ErrUserNotFound   = errors.New("user not found")
	ErrInvalidPayload = errors.New("invalid token payload")
)

const (
	DefaultTokenDuration = 24 * time.Hour
	SessionCookieName    = "twsnmp_session"
)

// Claims represents the JWT payload claims.
type Claims struct {
	User string `json:"user"`
	Role string `json:"role"`
	Exp  int64  `json:"exp"`
	Iat  int64  `json:"iat"`
}

// Manager handles user authentication, password hashing, and token signing/verification.
type Manager struct {
	store    datastore.DataStore
	secret   []byte
	mu       sync.RWMutex
	duration time.Duration
}

// NewManager initializes an Auth Manager, generating or retrieving the persisted secret.
func NewManager(store datastore.DataStore) (*Manager, error) {
	if store == nil {
		return nil, errors.New("datastore cannot be nil")
	}
	ctx := context.Background()
	secret, err := store.GetAuthSecret(ctx)
	if err != nil {
		return nil, fmt.Errorf("get auth secret: %w", err)
	}
	if len(secret) < 32 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("generate random auth secret: %w", err)
		}
		if err := store.SaveAuthSecret(ctx, secret); err != nil {
			return nil, fmt.Errorf("save auth secret: %w", err)
		}
	}

	return &Manager{
		store:    store,
		secret:   secret,
		duration: DefaultTokenDuration,
	}, nil
}

// HashPassword hashes a raw password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Authenticate verifies user credentials and returns the UserEnt and a signed JWT token string.
func (m *Manager) Authenticate(ctx context.Context, username, password string) (*datastore.UserEnt, string, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, "", ErrAuthFailed
	}
	user, err := m.store.GetUser(ctx, username)
	if err != nil {
		// Fallback: case-insensitive match
		users, lErr := m.store.ListUsers(ctx)
		if lErr == nil {
			for _, u := range users {
				if strings.EqualFold(u.User, username) {
					user = u
					err = nil
					break
				}
			}
		}
		if err != nil || user == nil {
			return nil, "", ErrAuthFailed
		}
	}
	if !CheckPassword(password, user.PasswordHash) {
		return nil, "", ErrAuthFailed
	}

	token, err := m.GenerateToken(user, m.duration)
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}

	return user, token, nil
}

// GenerateToken creates a signed JWT token for the given user.
func (m *Manager) GenerateToken(user *datastore.UserEnt, duration time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		User: user.User,
		Role: user.Role,
		Iat:  now.Unix(),
		Exp:  now.Add(duration).Unix(),
	}

	headerJSON := `{"alg":"HS256","typ":"JWT"}`
	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	unsignedToken := headerB64 + "." + payloadB64
	sig := m.sign(unsignedToken)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return unsignedToken + "." + sigB64, nil
}

// ValidateToken verifies a signed JWT token and returns the parsed Claims.
func (m *Manager) ValidateToken(tokenStr string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	unsignedToken := parts[0] + "." + parts[1]
	expectedSig := m.sign(unsignedToken)

	providedSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	if !hmac.Equal(providedSig, expectedSig) {
		return nil, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidPayload
	}

	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidPayload
	}

	if claims.Exp <= time.Now().Unix() {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

func (m *Manager) sign(data string) []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(data))
	return h.Sum(nil)
}
