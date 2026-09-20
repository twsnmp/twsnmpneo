package pki_test

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
)

func TestPKI_CreateAndIssueCert(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-pki-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// 1. Initialize PKI
	mgr, err := pki.New(pki.Config{
		DataDir: dir,
	})
	if err != nil {
		t.Fatalf("init pki failed: %v", err)
	}

	caCertPEM := mgr.GetCACertPEM()
	if caCertPEM == "" {
		t.Fatal("expected non-empty CA certificate PEM")
	}

	// 2. Issue a server certificate
	pair, err := mgr.IssueServerCertificate("localhost", []string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")}, 30)
	if err != nil {
		t.Fatalf("issue server cert failed: %v", err)
	}
	if pair.CertPEM == "" || pair.KeyPEM == "" {
		t.Fatal("expected non-empty cert and key PEM")
	}

	// 3. Verify issued cert is valid against the Root CA
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM([]byte(caCertPEM)) {
		t.Fatal("failed to append CA cert to pool")
	}

	tlsCert, err := tls.X509KeyPair([]byte(pair.CertPEM), []byte(pair.KeyPEM))
	if err != nil {
		t.Fatalf("parse tls key pair failed: %v", err)
	}

	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf certificate failed: %v", err)
	}

	opts := x509.VerifyOptions{
		DNSName: "localhost",
		Roots:   caPool,
	}
	if _, err := leaf.Verify(opts); err != nil {
		t.Fatalf("certificate verification failed: %v", err)
	}

	// 4. Test reload of existing CA
	mgr2, err := pki.New(pki.Config{
		DataDir: dir,
	})
	if err != nil {
		t.Fatalf("reload pki failed: %v", err)
	}
	if mgr2.GetCACertPEM() != caCertPEM {
		t.Error("reloaded CA cert does not match original")
	}
}

func TestPKI_ErrorCases(t *testing.T) {
	// Empty data dir
	_, err := pki.New(pki.Config{DataDir: ""})
	if err == nil {
		t.Fatal("expected error on empty data dir")
	}

	// Defaults fallback
	dir, err := os.MkdirTemp("", "twsnmpneo-pki-defaults-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	mgr, err := pki.New(pki.Config{
		DataDir: dir,
		// Organization, CommonName, ValidYears omitted to test defaults
	})
	if err != nil {
		t.Fatalf("init pki with defaults failed: %v", err)
	}
	if mgr.GetCACertPEM() == "" {
		t.Error("expected non-empty cert PEM with defaults")
	}

	// Issue cert with validDays <= 0 (defaults to 365)
	pair, err := mgr.IssueServerCertificate("default-days", nil, nil, 0)
	if err != nil || pair.CertPEM == "" {
		t.Fatalf("issue cert with default validDays failed: %v", err)
	}
}
