package pki_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
)

func setupTestStore(t *testing.T) (datastore.DataStore, func()) {
	dir, err := os.MkdirTemp("", "twsnmpneo-pki-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, "test.db")
	store, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("failed to open bbolt store: %v", err)
	}
	cleanup := func() {
		_ = store.Close()
		_ = os.RemoveAll(dir)
	}
	return store, cleanup
}

func TestPKIManagerLifecycle(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	manager, err := pki.New(store)
	if err != nil {
		t.Fatalf("pki.New failed: %v", err)
	}

	if manager.IsCAValid() {
		t.Errorf("expected CA to be invalid initially")
	}

	// 1. Create CA
	err = manager.CreateCA(&datastore.CreateCAReq{
		Name:          "TestLab",
		RootCAKeyType: "ecdsa-256",
		RootCATerm:    5,
		CertTerm:      30 * 24,
		HTTPPort:      18082,
		AcmePort:      18083,
		SANs:          "127.0.0.1,localhost",
	})
	if err != nil {
		t.Fatalf("CreateCA failed: %v", err)
	}

	if !manager.IsCAValid() {
		t.Errorf("expected CA to be valid after CreateCA")
	}

	caPEM := manager.GetCACertPEM()
	if caPEM == "" {
		t.Errorf("expected non-empty Root CA PEM")
	}

	// 2. Generate CSR (ZIP)
	zipBytes, err := manager.CreateCertificateRequest(&datastore.CSRReqEnt{
		CommonName:   "node1.example.com",
		KeyType:      "rsa-2048",
		Organization: "Test Corp",
		Country:      "JP",
		Sans:         "node1.example.com,192.168.1.10",
	})
	if err != nil {
		t.Fatalf("CreateCertificateRequest failed: %v", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("invalid zip archive: %v", err)
	}

	var csrPEM []byte
	var keyPEM []byte
	for _, f := range zipReader.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip file %s: %v", f.Name, err)
		}
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(rc)
		_ = rc.Close()
		if f.Name == "csr.pem" {
			csrPEM = buf.Bytes()
		} else if f.Name == "key.pem" {
			keyPEM = buf.Bytes()
		}
	}

	if len(csrPEM) == 0 || len(keyPEM) == 0 {
		t.Fatalf("missing csr.pem or key.pem in generated zip")
	}

	// 3. Issue Certificate from CSR
	crtPEM, err := manager.CreateCertificate(csrPEM)
	if err != nil {
		t.Fatalf("CreateCertificate failed: %v", err)
	}

	block, _ := pem.Decode(crtPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("invalid certificate PEM output")
	}

	parsedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse issued certificate: %v", err)
	}
	if parsedCert.Subject.CommonName != "node1.example.com" {
		t.Errorf("expected CommonName node1.example.com, got %s", parsedCert.Subject.CommonName)
	}

	// 4. Verify certificate appears in datastore
	certs, err := store.ListPKICerts(context.Background())
	if err != nil {
		t.Fatalf("ListPKICerts failed: %v", err)
	}
	if len(certs) < 2 { // Root CA + SCEP CA + issued cert
		t.Errorf("expected at least 2 certs, got %d", len(certs))
	}

	// 5. Revoke Certificate
	serialHex := fmt.Sprintf("%x", parsedCert.SerialNumber)
	err = manager.RevokeCert(serialHex)
	if err != nil {
		t.Fatalf("RevokeCert failed: %v", err)
	}

	revokedCert, err := store.GetPKICert(context.Background(), serialHex)
	if err != nil {
		t.Fatalf("GetPKICert failed: %v", err)
	}
	if revokedCert.Revoked == 0 {
		t.Errorf("expected certificate to be marked revoked")
	}

	// 6. Check CRL
	crl := manager.GetCRL()
	if len(crl) == 0 {
		t.Errorf("expected non-empty CRL")
	}

	// 7. Start / Stop Services
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = manager.Start(ctx, nil)
	manager.Stop()

	// 8. Destroy CA
	err = manager.DestroyCA()
	if err != nil {
		t.Fatalf("DestroyCA failed: %v", err)
	}
	if manager.IsCAValid() {
		t.Errorf("expected CA to be invalid after DestroyCA")
	}
}
