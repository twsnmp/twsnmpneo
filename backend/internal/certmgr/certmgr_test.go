package certmgr

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndParseSelfSigned(t *testing.T) {
	certPEM, keyPEM, err := GenerateSelfSignedCert("test.local", "192.168.1.100")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert failed: %v", err)
	}

	cert, info, err := ParseCertKeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("ParseCertKeyPair failed: %v", err)
	}

	if cert == nil || info == nil {
		t.Fatal("expected non-nil cert and info")
	}

	if info.Subject != "test.local" {
		t.Errorf("expected subject 'test.local', got %s", info.Subject)
	}
	if info.KeyType != "ECDSA" {
		t.Errorf("expected key type 'ECDSA', got %s", info.KeyType)
	}
	if info.DaysLeft < 360 {
		t.Errorf("expected ~365 days, got %d", info.DaysLeft)
	}
	if !info.IsSelfSigned {
		t.Errorf("expected isSelfSigned to be true")
	}

	foundDNS := false
	for _, dns := range info.DNSNames {
		if dns == "test.local" {
			foundDNS = true
			break
		}
	}
	if !foundDNS {
		t.Errorf("expected test.local in DNSNames: %v", info.DNSNames)
	}

	foundIP := false
	for _, ip := range info.IPAddresses {
		if ip == "192.168.1.100" {
			foundIP = true
			break
		}
	}
	if !foundIP {
		t.Errorf("expected 192.168.1.100 in IPAddresses: %v", info.IPAddresses)
	}
}

func TestManager_FileAndSelfSigned(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	mgr := New(Config{
		DataDir:   tmpDir,
		CertPath:  certPath,
		KeyPath:   keyPath,
		Host:      "myserver.local",
		EnableTLS: true,
	}, nil)

	ctx := context.Background()

	// 1. Initial resolution: no files exist -> self-signed generated
	if err := mgr.LoadOrResolve(ctx); err != nil {
		t.Fatalf("LoadOrResolve failed: %v", err)
	}

	st := mgr.GetStatus()
	if st.Mode != ModeSelfSigned {
		t.Errorf("expected mode selfsigned, got %s", st.Mode)
	}
	if st.Subject != "myserver.local" {
		t.Errorf("expected subject myserver.local, got %s", st.Subject)
	}

	// Verify files were written to disk
	if !fileExists(certPath) || !fileExists(keyPath) {
		t.Fatal("expected cert and key files to be created on disk")
	}

	// Verify GetCertificate works
	tlsCert, err := mgr.GetCertificate(&tls.ClientHelloInfo{})
	if err != nil || tlsCert == nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}

	// 2. New manager with existing files -> file mode
	mgr2 := New(Config{
		DataDir:  tmpDir,
		CertPath: certPath,
		KeyPath:  keyPath,
	}, nil)

	if err := mgr2.LoadOrResolve(ctx); err != nil {
		t.Fatalf("LoadOrResolve with existing files failed: %v", err)
	}

	st2 := mgr2.GetStatus()
	if st2.Mode != ModeFile {
		t.Errorf("expected mode file, got %s", st2.Mode)
	}

	// 3. Hot reload test
	newCertPEM, newKeyPEM, err := GenerateSelfSignedCert("reloaded.local", "")
	if err != nil {
		t.Fatalf("Generate second cert: %v", err)
	}
	if err := os.WriteFile(certPath, newCertPEM, 0600); err != nil {
		t.Fatalf("overwrite cert: %v", err)
	}
	if err := os.WriteFile(keyPath, newKeyPEM, 0600); err != nil {
		t.Fatalf("overwrite key: %v", err)
	}

	reloadedCert, reloadedInfo, err := LoadCertKeyPair(certPath, keyPath)
	if err != nil {
		t.Fatalf("load reloaded cert: %v", err)
	}
	mgr2.setCert(reloadedCert, reloadedInfo, ModeFile)

	activeCert, err := mgr2.GetCertificate(&tls.ClientHelloInfo{})
	if err != nil {
		t.Fatalf("get cert after reload: %v", err)
	}
	if activeCert.Leaf.Subject.CommonName != "reloaded.local" {
		t.Errorf("expected subject reloaded.local, got %s", activeCert.Leaf.Subject.CommonName)
	}
}

func TestManager_DisabledMode(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := New(Config{
		DataDir:   tmpDir,
		EnableTLS: false,
	}, nil)

	ctx := context.Background()
	if err := mgr.LoadOrResolve(ctx); err != nil {
		t.Fatalf("LoadOrResolve should succeed when disabled: %v", err)
	}

	st := mgr.GetStatus()
	if st.Enabled || st.Mode != ModeDisabled {
		t.Errorf("expected disabled mode, got enabled=%v mode=%s", st.Enabled, st.Mode)
	}

	_, err := mgr.GetCertificate(&tls.ClientHelloInfo{})
	if err == nil {
		t.Error("expected error from GetCertificate when disabled")
	}
}

func TestDetectHostAndIPs(t *testing.T) {
	host, ips := detectHostAndIPs()
	if host == "" {
		t.Error("expected non-empty hostname")
	}
	t.Logf("Detected host=%s, ips=%v", host, ips)
}

func TestEmbeddedHTTP01Provider(t *testing.T) {
	p := NewEmbeddedHTTP01Provider()

	if err := p.Present("example.com", "token123", "keyAuth456"); err != nil {
		t.Fatalf("Present failed: %v", err)
	}

	// Test challenge request
	req, _ := http.NewRequest("GET", "http://example.com/.well-known/acme-challenge/token123", nil)
	rr := httptest.NewRecorder()
	if !p.ServeHTTP(rr, req) {
		t.Fatal("expected ServeHTTP to handle acme challenge")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "keyAuth456" {
		t.Errorf("expected 'keyAuth456', got '%s'", rr.Body.String())
	}

	// Test non-challenge request
	reqNon, _ := http.NewRequest("GET", "http://example.com/other", nil)
	rrNon := httptest.NewRecorder()
	if p.ServeHTTP(rrNon, reqNon) {
		t.Fatal("expected ServeHTTP to NOT handle normal request")
	}

	// Clean up
	if err := p.CleanUp("example.com", "token123", "keyAuth456"); err != nil {
		t.Fatalf("CleanUp failed: %v", err)
	}

	rrAfter := httptest.NewRecorder()
	if p.ServeHTTP(rrAfter, req) {
		t.Fatal("expected ServeHTTP to return false after CleanUp")
	}
}

