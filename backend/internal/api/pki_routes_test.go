package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/api"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
)

func setupAPITest(t *testing.T) (datastore.DataStore, *pki.Manager, *api.Server, func()) {
	dir, err := os.MkdirTemp("", "twsnmpneo-api-pki-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, "test.db")
	store, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("open bbolt: %v", err)
	}
	manager, err := pki.New(store)
	if err != nil {
		t.Fatalf("pki.New: %v", err)
	}
	server, err := api.NewServer(api.Config{Port: 0, PKI: manager, Store: store})
	if err != nil {
		t.Fatalf("create api server: %v", err)
	}
	cleanup := func() {
		_ = store.Close()
		_ = os.RemoveAll(dir)
	}
	return store, manager, server, cleanup
}

func loginAdmin(t *testing.T, e http.Handler) string {
	loginPayload := `{"user":"twsnmp","password":"twsnmp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(loginPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}
	var loginResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("unmarshal login resp: %v", err)
	}
	return loginResp.Token
}

func TestPKIAPIRoutes(t *testing.T) {
	store, manager, server, cleanup := setupAPITest(t)
	defer cleanup()
	echo := server.GetEcho()
	token := loginAdmin(t, echo)

	authedReq := func(method, target string, body []byte, contentType ...string) *http.Request {
		var req *http.Request
		if len(body) > 0 {
			req = httptest.NewRequest(method, target, bytes.NewReader(body))
		} else {
			req = httptest.NewRequest(method, target, nil)
		}
		if len(contentType) > 0 {
			req.Header.Set("Content-Type", contentType[0])
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return req
	}

	// 1. Check hasCA initially false
	rec := httptest.NewRecorder()
	echo.ServeHTTP(rec, authedReq(http.MethodGet, "/api/pki/hasCA", nil))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "false" {
		t.Fatalf("expected hasCA false, got %s", rec.Body.String())
	}

	// 2. Get default createCA request
	rec = httptest.NewRecorder()
	echo.ServeHTTP(rec, authedReq(http.MethodGet, "/api/pki/createCA", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "RootCAKeyType") {
		t.Fatalf("expected createCA default req, got %d %s", rec.Code, rec.Body.String())
	}

	// 3. Create CA
	createJSON := `{"Name":"APITestCA","RootCAKeyType":"ecdsa-256","SANs":"127.0.0.1,localhost","AcmePort":18083,"HttpPort":18082,"RootCATerm":5,"CrlInterval":24,"CertTerm":720}`
	req := authedReq(http.MethodPost, "/api/pki/createCA", []byte(createJSON), "application/json")
	rec = httptest.NewRecorder()
	echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"resp":"ok"`) {
		t.Fatalf("createCA failed: %d %s", rec.Code, rec.Body.String())
	}

	// 4. Verify hasCA is now true
	rec = httptest.NewRecorder()
	echo.ServeHTTP(rec, authedReq(http.MethodGet, "/api/pki/hasCA", nil))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "true" {
		t.Fatalf("expected hasCA true, got %s", rec.Body.String())
	}

	// 5. Create CSR (ZIP download)
	csrReqJSON := `{"KeyType":"rsa-2048","CommonName":"node1.local","Organization":"Test Org","Country":"JP","Sans":"node1.local,192.168.1.50"}`
	req = authedReq(http.MethodPost, "/api/pki/createCSR", []byte(csrReqJSON), "application/json")
	rec = httptest.NewRecorder()
	echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("createCSR failed: %d", rec.Code)
	}

	zipBytes := rec.Body.Bytes()
	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}

	var csrPEM []byte
	for _, f := range zipReader.File {
		if f.Name == "csr.pem" {
			rc, _ := f.Open()
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(rc)
			_ = rc.Close()
			csrPEM = buf.Bytes()
		}
	}
	if len(csrPEM) == 0 {
		t.Fatalf("csr.pem missing in zip")
	}

	// Add node to store for SCEP / node matching
	_ = store.SaveNode(context.Background(), &datastore.NodeEnt{
		ID:   "test-node-1",
		Name: "node1.local",
		IP:   "192.168.1.50",
	})

	// 6. Create CRT from uploaded CSR (multipart)
	body := new(bytes.Buffer)
	mpWriter := multipart.NewWriter(body)
	part, _ := mpWriter.CreateFormFile("file", "csr.pem")
	_, _ = part.Write(csrPEM)
	_ = mpWriter.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/pki/createCRT", body)
	req.Header.Set("Content-Type", mpWriter.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/x-pem-file" {
		t.Fatalf("createCRT failed: %d %s", rec.Code, rec.Body.String())
	}
	crtPEM := rec.Body.String()
	if !strings.Contains(crtPEM, "BEGIN CERTIFICATE") {
		t.Fatalf("invalid crt output: %s", crtPEM)
	}

	// 7. List certs
	rec = httptest.NewRecorder()
	echo.ServeHTTP(rec, authedReq(http.MethodGet, "/api/pki/certs", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "node1.local") {
		t.Fatalf("expected cert in list: %d %s", rec.Code, rec.Body.String())
	}

	// 8. Get PKI control
	rec = httptest.NewRecorder()
	echo.ServeHTTP(rec, authedReq(http.MethodGet, "/api/pki/control", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "HttpStatus") {
		t.Fatalf("get control failed: %d %s", rec.Code, rec.Body.String())
	}

	// 9. Update PKI control
	controlJSON := `{"EnableHttp":true,"EnableAcme":false,"AcmeBaseURL":"https://localhost:18083","CertTerm":720,"CrlInterval":24}`
	req = authedReq(http.MethodPost, "/api/pki/control", []byte(controlJSON), "application/json")
	rec = httptest.NewRecorder()
	echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update control failed: %d %s", rec.Code, rec.Body.String())
	}

	// 10. Destroy CA
	rec = httptest.NewRecorder()
	echo.ServeHTTP(rec, authedReq(http.MethodPost, "/api/pki/destroyCA", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"resp":"ok"`) {
		t.Fatalf("destroyCA failed: %d %s", rec.Code, rec.Body.String())
	}

	if manager.IsCAValid() {
		t.Errorf("expected CA to be destroyed")
	}
}
