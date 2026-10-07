package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/api"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
)

func TestAPIServer_AuthenticationAndUserManagement(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-auth-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	srv, err := api.NewServer(api.Config{
		Port:    9099,
		Debug:   true,
		Version: "v0.1.0-test",
		Store:   store,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	e := srv.GetEcho()

	// 1. Unauthenticated request to /api/nodes should return 401
	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", rec.Code)
	}

	// 2. Login with invalid credentials should return 401
	badLoginPayload := `{"user":"twsnmp","password":"wrongpassword"}`
	req = httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(badLoginPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid login, got %d", rec.Code)
	}

	// 3. Login with default credentials twsnmp:twsnmp should return 200, JWT token, and set session cookie
	loginPayload := `{"user":"twsnmp","password":"twsnmp"}`
	req = httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(loginPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid login, got %d: %s", rec.Code, rec.Body.String())
	}

	var loginResp struct {
		Token string             `json:"token"`
		User  *datastore.UserEnt `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	if loginResp.Token == "" || loginResp.User == nil || loginResp.User.User != "twsnmp" {
		t.Fatalf("unexpected login response: %+v", loginResp)
	}

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "twsnmp_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value != loginResp.Token {
		t.Fatalf("expected session cookie with token value, got: %+v", sessionCookie)
	}

	// 4. GET /api/me with Bearer token
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/me returned %d: %s", rec.Code, rec.Body.String())
	}
	var meUser datastore.UserEnt
	if err := json.Unmarshal(rec.Body.Bytes(), &meUser); err != nil {
		t.Fatalf("unmarshal /api/me: %v", err)
	}
	if meUser.User != "twsnmp" || meUser.Role != "admin" {
		t.Errorf("unexpected me user: %+v", meUser)
	}

	// 5. GET /api/me with Cookie
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/me with cookie returned %d", rec.Code)
	}

	// 6. User Management: GET /api/users
	req = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/users returned %d: %s", rec.Code, rec.Body.String())
	}
	var users []*datastore.UserEnt
	_ = json.Unmarshal(rec.Body.Bytes(), &users)
	if len(users) != 1 || users[0].User != "twsnmp" {
		t.Errorf("expected 1 initial user, got: %d", len(users))
	}

	// 7. Create new user: POST /api/users
	newUserJSON, _ := json.Marshal(map[string]string{
		"user":     "operator1",
		"name":     "Network Operator",
		"password": "OperatorPassword123!",
		"role":     "user",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(newUserJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/users returned %d: %s", rec.Code, rec.Body.String())
	}

	// 8. Login as new user
	opLoginJSON, _ := json.Marshal(map[string]string{
		"user":     "operator1",
		"password": "OperatorPassword123!",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(opLoginJSON))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login as operator1 failed: %d", rec.Code)
	}

	// 9. Update user: PUT /api/users/operator1
	updateJSON, _ := json.Marshal(map[string]string{
		"name":     "Senior Operator",
		"password": "NewOperatorPass456!",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/users/operator1", bytes.NewReader(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/users/operator1 failed: %d", rec.Code)
	}

	// 10. Delete user: DELETE /api/users/operator1
	req = httptest.NewRequest(http.MethodDelete, "/api/users/operator1", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE /api/users/operator1 failed: %d", rec.Code)
	}

	// 11. Cannot delete the only remaining user (twsnmp)
	req = httptest.NewRequest(http.MethodDelete, "/api/users/twsnmp", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when deleting last user, got %d", rec.Code)
	}

	// 12. RBAC Tests: operator cannot delete or create other users
	newUser2JSON, _ := json.Marshal(map[string]string{
		"user":     "readonly1",
		"name":     "Auditor",
		"password": "ReadonlyPassword123!",
		"role":     "readonly",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(newUser2JSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create readonly1 user: %d", rec.Code)
	}

	// Login as readonly1
	roLoginJSON, _ := json.Marshal(map[string]string{
		"user":     "readonly1",
		"password": "ReadonlyPassword123!",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(roLoginJSON))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var roLoginResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &roLoginResp)

	// Readonly user trying to POST /api/nodes should get 403 Forbidden
	nodeJSON, _ := json.Marshal(map[string]string{"name": "test-node", "ip": "192.168.1.100"})
	req = httptest.NewRequest(http.MethodPost, "/api/nodes", bytes.NewReader(nodeJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+roLoginResp.Token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for readonly mutating request, got %d", rec.Code)
	}

	// Readonly user GET /api/nodes should succeed (200 OK)
	req = httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	req.Header.Set("Authorization", "Bearer "+roLoginResp.Token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for readonly GET request, got %d", rec.Code)
	}

	// 13. POST /api/logout clears session cookie
	req = httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("logout returned %d", rec.Code)
	}
	outCookies := rec.Result().Cookies()
	var clearedCookie *http.Cookie
	for _, c := range outCookies {
		if c.Name == "twsnmp_session" {
			clearedCookie = c
			break
		}
	}
	if clearedCookie == nil || clearedCookie.MaxAge > 0 {
		t.Errorf("expected cleared session cookie (MaxAge <= 0), got: %+v", clearedCookie)
	}
}
