package api_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/api"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
)

func TestPKIRoutesIssueRevokeAndDownload(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-pki-api-*")
	if err != nil {
		t.Fatalf("create temporary directory: %v", err)
	}

	defer os.RemoveAll(dir)
	manager, err := pki.New(pki.Config{DataDir: filepath.Join(dir, "pki")})
	if err != nil {
		t.Fatalf("initialize pki: %v", err)
	}
	server, err := api.NewServer(api.Config{Port: 0, PKI: manager})
	if err != nil {
		t.Fatalf("create api server: %v", err)
	}
	echo := server.GetEcho()

	statusRec := httptest.NewRecorder()
	echo.ServeHTTP(statusRec, httptest.NewRequest(http.MethodGet, "/api/pki/status", nil))
	if statusRec.Code != http.StatusOK || !strings.Contains(statusRec.Body.String(), `"ready":false`) {
		t.Fatalf("new PKI manager unexpectedly has a CA: %d %s", statusRec.Code, statusRec.Body.String())
	}
	for _, path := range []string{"/api/pki/ca.pem", "/ca.pem"} {
		rec := httptest.NewRecorder()
		echo.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("uninitialized CA endpoint %s returned %d", path, rec.Code)
		}
	}
	createCAReq := httptest.NewRequest(http.MethodPost, "/api/pki/ca", strings.NewReader(`{"commonName":"API Test Root CA","organization":"TWSNMP NEO Test","keyType":"ecdsa-256","validYears":10}`))
	createCAReq.Header.Set("Content-Type", "application/json")
	createCARec := httptest.NewRecorder()
	echo.ServeHTTP(createCARec, createCAReq)
	if createCARec.Code != http.StatusCreated || !strings.Contains(createCARec.Body.String(), `"ready":true`) {
		t.Fatalf("initialize CA returned %d: %s", createCARec.Code, createCARec.Body.String())
	}
	settings := manager.Settings()
	settings.EnableHTTP = true
	if err := manager.UpdateSettings(settings); err != nil {
		t.Fatalf("enable PKI HTTP services for route tests: %v", err)
	}
	statusRec = httptest.NewRecorder()
	echo.ServeHTTP(statusRec, httptest.NewRequest(http.MethodGet, "/api/pki/status", nil))
	if statusRec.Code != http.StatusOK || !strings.Contains(statusRec.Body.String(), `"ready":true`) {
		t.Fatalf("unexpected PKI status response: %d %s", statusRec.Code, statusRec.Body.String())
	}
	for _, path := range []string{"/ca.pem", "/scepca.pem", "/crl", "/crl.pem"} {
		rec := httptest.NewRecorder()
		echo.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
			t.Fatalf("public PKI endpoint %s returned %d: %s", path, rec.Code, rec.Body.String())
		}
	}

	scepCACertRec := httptest.NewRecorder()
	echo.ServeHTTP(scepCACertRec, httptest.NewRequest(http.MethodGet, "/scep?operation=GetCACert", nil))
	if scepCACertRec.Code != http.StatusOK || scepCACertRec.Header().Get("Content-Type") != "application/x-x509-ca-ra-cert" {
		t.Fatalf("SCEP CA certificate endpoint returned %d: %s", scepCACertRec.Code, scepCACertRec.Body.String())
	}
	scepCapsRec := httptest.NewRecorder()
	echo.ServeHTTP(scepCapsRec, httptest.NewRequest(http.MethodGet, "/scep?operation=GetCACaps", nil))
	if scepCapsRec.Code != http.StatusOK || !strings.Contains(scepCapsRec.Body.String(), "POSTPKIOperation") {
		t.Fatalf("SCEP capabilities endpoint returned %d: %s", scepCapsRec.Code, scepCapsRec.Body.String())
	}

	body := `{"commonName":"api.example.test","dnsNames":["api.example.test"],"ipAddresses":["192.0.2.20"],"validDays":30}`
	issueReq := httptest.NewRequest(http.MethodPost, "/api/pki/certificates", strings.NewReader(body))
	issueReq.Header.Set("Content-Type", "application/json")
	issueRec := httptest.NewRecorder()
	echo.ServeHTTP(issueRec, issueReq)
	if issueRec.Code != http.StatusCreated {
		t.Fatalf("issue certificate returned %d: %s", issueRec.Code, issueRec.Body.String())
	}
	var cert pki.Certificate
	if err := json.Unmarshal(issueRec.Body.Bytes(), &cert); err != nil {
		t.Fatalf("decode issue response: %v", err)
	}
	if cert.Serial == "" || cert.KeyPEM != "" {
		t.Fatalf("invalid certificate response: serial=%q key included=%t", cert.Serial, cert.KeyPEM != "")
	}

	csrKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CSR key: %v", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: "csr-api.example.test"},
		DNSNames: []string{"csr-api.example.test"},
	}, csrKey)
	if err != nil {
		t.Fatalf("create CSR: %v", err)
	}
	csrPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}))
	issueCSRReq := httptest.NewRequest(http.MethodPost, "/api/pki/certificates/csr", strings.NewReader(`{"csrPEM":`+mustJSON(t, csrPEM)+`}`))
	issueCSRReq.Header.Set("Content-Type", "application/json")
	issueCSRRec := httptest.NewRecorder()
	echo.ServeHTTP(issueCSRRec, issueCSRReq)
	if issueCSRRec.Code != http.StatusCreated || !strings.Contains(issueCSRRec.Body.String(), `"type":"manual"`) {
		t.Fatalf("issue certificate from CSR returned %d: %s", issueCSRRec.Code, issueCSRRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	echo.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/pki/certificates", nil))
	if listRec.Code != http.StatusOK || strings.Contains(listRec.Body.String(), "keyPEM") {
		t.Fatalf("certificate listing leaked private key or failed: %d %s", listRec.Code, listRec.Body.String())
	}

	downloadRec := httptest.NewRecorder()
	echo.ServeHTTP(downloadRec, httptest.NewRequest(http.MethodGet, "/api/pki/certificates/"+cert.Serial+"/download", nil))
	if downloadRec.Code != http.StatusOK || !strings.Contains(downloadRec.Body.String(), "PRIVATE KEY") {
		t.Fatalf("certificate download did not include key pair: %d %s", downloadRec.Code, downloadRec.Body.String())
	}

	revokeRec := httptest.NewRecorder()
	echo.ServeHTTP(revokeRec, httptest.NewRequest(http.MethodDelete, "/api/pki/certificates/"+cert.Serial, nil))
	if revokeRec.Code != http.StatusNoContent {
		t.Fatalf("revoke certificate returned %d: %s", revokeRec.Code, revokeRec.Body.String())
	}

	crlRec := httptest.NewRecorder()
	echo.ServeHTTP(crlRec, httptest.NewRequest(http.MethodGet, "/api/pki/crl", nil))
	if crlRec.Code != http.StatusOK || len(crlRec.Body.Bytes()) == 0 {
		t.Fatalf("CRL endpoint returned %d: %s", crlRec.Code, crlRec.Body.String())
	}

	resetRec := httptest.NewRecorder()
	echo.ServeHTTP(resetRec, httptest.NewRequest(http.MethodDelete, "/api/pki/ca", nil))
	if resetRec.Code != http.StatusNoContent {
		t.Fatalf("reset CA returned %d: %s", resetRec.Code, resetRec.Body.String())
	}
	statusRec = httptest.NewRecorder()
	echo.ServeHTTP(statusRec, httptest.NewRequest(http.MethodGet, "/api/pki/status", nil))
	if !strings.Contains(statusRec.Body.String(), `"ready":false`) || len(manager.ListCertificates()) != 0 {
		t.Fatalf("CA reset did not clear CA and issued certificates: status=%s inventory=%d", statusRec.Body.String(), len(manager.ListCertificates()))
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode JSON test value: %v", err)
	}
	return string(encoded)
}

func TestACMEHTTPSListenerServesDirectoryAndShutsDown(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-acme-api-*")
	if err != nil {
		t.Fatalf("create temporary directory: %v", err)
	}
	defer os.RemoveAll(dir)
	manager, err := pki.New(pki.Config{DataDir: filepath.Join(dir, "pki")})
	if err != nil {
		t.Fatalf("initialize pki: %v", err)
	}
	if err := manager.InitializeCA(pki.CAOptions{CommonName: "ACME Test CA", KeyType: "ecdsa-256", ValidYears: 10}); err != nil {
		t.Fatalf("initialize ACME test CA: %v", err)
	}

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve ACME port: %v", err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	if err := probe.Close(); err != nil {
		t.Fatalf("release ACME port: %v", err)
	}
	baseURL := fmt.Sprintf("https://127.0.0.1:%d/acme", port)
	server, err := api.NewServer(api.Config{
		Port:        0,
		PKI:         manager,
		ACMEBaseURL: baseURL,
	})
	if err != nil {
		t.Fatalf("create API server: %v", err)
	}
	httpProbe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve PKI HTTP port: %v", err)
	}
	httpPort := httpProbe.Addr().(*net.TCPAddr).Port
	if err := httpProbe.Close(); err != nil {
		t.Fatalf("release PKI HTTP port: %v", err)
	}
	settings := manager.Settings()
	settings.HTTPPort = httpPort
	settings.EnableHTTP = true
	if err := manager.UpdateSettings(settings); err != nil {
		t.Fatalf("enable PKI HTTP listener: %v", err)
	}

	rootBlock, _ := pem.Decode([]byte(manager.GetCACertPEM()))
	if rootBlock == nil {
		t.Fatal("root certificate is not PEM encoded")
	}
	rootCert, err := x509.ParseCertificate(rootBlock.Bytes)
	if err != nil {
		t.Fatalf("parse root certificate: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(rootCert)
	client := &http.Client{
		Timeout: time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: roots},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Start(ctx) }()

	var response *http.Response
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err = client.Get(baseURL + "/directory")
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		cancel()
		<-done
		t.Fatalf("request ACME directory: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		cancel()
		<-done
		t.Fatalf("ACME directory returned %d", response.StatusCode)
	}
	httpURL := fmt.Sprintf("http://127.0.0.1:%d/ca.pem", httpPort)
	var httpResponse *http.Response
	for time.Now().Before(deadline) {
		httpResponse, err = http.Get(httpURL)
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		cancel()
		<-done
		t.Fatalf("request PKI HTTP listener: %v", err)
	}
	_ = httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		cancel()
		<-done
		t.Fatalf("PKI HTTP endpoint returned %d", httpResponse.StatusCode)
	}
	settings = manager.Settings()
	settings.EnableHTTP = false
	settings.EnableACME = false
	settingsBody, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("encode PKI settings: %v", err)
	}
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/pki/settings", strings.NewReader(string(settingsBody)))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateResponse := httptest.NewRecorder()
	server.GetEcho().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		cancel()
		<-done
		t.Fatalf("disable PKI service listeners returned %d: %s", updateResponse.Code, updateResponse.Body.String())
	}
	if server.GetACMEEcho() != nil {
		cancel()
		<-done
		t.Fatal("ACME listener remained active after settings disabled it")
	}
	if _, err := http.Get(httpURL); err == nil {
		cancel()
		<-done
		t.Fatal("PKI HTTP listener remained active after settings disabled it")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stop API server: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("API server did not shut down")
	}
}
