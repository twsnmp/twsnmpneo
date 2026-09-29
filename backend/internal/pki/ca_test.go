package pki_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
	"golang.org/x/crypto/ocsp"
)

func initializeTestCA(t *testing.T, manager *pki.Manager) {
	t.Helper()
	if err := manager.InitializeCA(pki.CAOptions{
		CommonName:   "Test Root CA",
		Organization: "TWSNMP NEO Test",
		KeyType:      "ecdsa-256",
		ValidYears:   10,
	}); err != nil {
		t.Fatalf("initialize test CA: %v", err)
	}
}

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
	initializeTestCA(t, mgr)

	caCertPEM := mgr.GetCACertPEM()
	if caCertPEM == "" {
		t.Fatal("expected non-empty CA certificate PEM")
	}
	scepCerts, err := mgr.SCEPCACertificates()
	if err != nil {
		t.Fatalf("load SCEP CA chain: %v", err)
	}
	if len(scepCerts) != 2 || !scepCerts[1].IsCA {
		t.Fatalf("expected root and subordinate SCEP CA certificates, got %d", len(scepCerts))
	}
	if err := scepCerts[1].CheckSignatureFrom(scepCerts[0]); err != nil {
		t.Fatalf("SCEP CA is not signed by Root CA: %v", err)
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
	reloadedSCEPCerts, err := mgr2.SCEPCACertificates()
	if err != nil {
		t.Fatalf("reload SCEP CA chain: %v", err)
	}
	if string(reloadedSCEPCerts[1].Raw) != string(scepCerts[1].Raw) {
		t.Fatal("reloaded SCEP CA certificate does not match original")
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
	initializeTestCA(t, mgr)
	if mgr.GetCACertPEM() == "" {
		t.Error("expected non-empty cert PEM with defaults")
	}

	// Issue cert with validDays <= 0 (defaults to 365)
	pair, err := mgr.IssueServerCertificate("default-days", nil, nil, 0)
	if err != nil || pair.CertPEM == "" {
		t.Fatalf("issue cert with default validDays failed: %v", err)
	}
}

func TestPKI_RejectsIncompleteRootCA(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-pki-incomplete-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)
	manager, err := pki.New(pki.Config{DataDir: dir})
	if err != nil {
		t.Fatalf("initialize pki: %v", err)
	}
	initializeTestCA(t, manager)
	if err := os.Remove(filepath.Join(dir, "ca.key")); err != nil {
		t.Fatalf("remove private key for incomplete-state test: %v", err)
	}
	if _, err := pki.New(pki.Config{DataDir: dir}); err == nil {
		t.Fatal("expected an error for incomplete root CA files")
	}
}

func TestPKI_PersistsCertificatesInBbolt(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-pki-bbolt-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)
	dbPath := filepath.Join(dir, "twsnmpneo.db")

	store, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("open datastore: %v", err)
	}
	manager, err := pki.New(pki.Config{DataDir: filepath.Join(dir, "pki"), CertStore: store})
	if err != nil {
		t.Fatalf("initialize pki: %v", err)
	}
	initializeTestCA(t, manager)
	settings := manager.Settings()
	settings.HTTPBaseURL = "https://pki.example.test:9443/pki"
	settings.CertValidityHours = 72
	settings.CRLIntervalHours = 12
	if err := manager.UpdateSettings(settings); err != nil {
		t.Fatalf("save PKI service settings: %v", err)
	}
	issued, err := manager.IssueCertificate("db.example.test", []string{"db.example.test"}, nil, 30)
	if err != nil {
		t.Fatalf("issue certificate: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close datastore: %v", err)
	}

	store, err = bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("reopen datastore: %v", err)
	}
	defer store.Close()
	manager, err = pki.New(pki.Config{DataDir: filepath.Join(dir, "pki"), CertStore: store})
	if err != nil {
		t.Fatalf("reload pki: %v", err)
	}
	reloadedSettings := manager.Settings()
	if reloadedSettings.HTTPBaseURL != settings.HTTPBaseURL ||
		reloadedSettings.CertValidityHours != settings.CertValidityHours ||
		reloadedSettings.CRLIntervalHours != settings.CRLIntervalHours {
		t.Fatalf("PKI settings were not persisted: %+v", reloadedSettings)
	}
	cert, err := manager.GetCertificate(issued.Serial)
	if err != nil {
		t.Fatalf("load issued certificate from datastore: %v", err)
	}
	if cert.KeyPEM == "" {
		t.Fatal("private key was not restored from datastore")
	}
}

func TestPKI_ResetClearsCAAndCertificateInventory(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-pki-reset-*")
	if err != nil {
		t.Fatalf("create temp directory: %v", err)
	}
	defer os.RemoveAll(dir)
	store, err := bbolt.New(filepath.Join(dir, "twsnmpneo.db"))
	if err != nil {
		t.Fatalf("open datastore: %v", err)
	}
	defer store.Close()
	manager, err := pki.New(pki.Config{DataDir: filepath.Join(dir, "pki"), CertStore: store})
	if err != nil {
		t.Fatalf("initialize PKI manager: %v", err)
	}
	initializeTestCA(t, manager)
	if _, err := manager.IssueCertificate("reset.example.test", nil, nil, 30); err != nil {
		t.Fatalf("issue certificate before reset: %v", err)
	}
	if err := manager.ResetCA(); err != nil {
		t.Fatalf("reset CA: %v", err)
	}
	if manager.Status().Ready || manager.GetCACertPEM() != "" || len(manager.ListCertificates()) != 0 {
		t.Fatal("reset did not clear in-memory CA state and inventory")
	}
	stored, err := store.ListPKICertificates()
	if err != nil {
		t.Fatalf("list persisted certificates after reset: %v", err)
	}
	if len(stored) != 0 {
		t.Fatalf("reset left %d certificate records in bbolt", len(stored))
	}
	reloaded, err := pki.New(pki.Config{DataDir: filepath.Join(dir, "pki"), CertStore: store})
	if err != nil {
		t.Fatalf("reload reset manager: %v", err)
	}
	if reloaded.Status().Ready {
		t.Fatal("CA was recreated automatically after reset")
	}
}

func TestPKI_IssuesVerifiedCSR(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-pki-csr-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)
	manager, err := pki.New(pki.Config{DataDir: dir})
	if err != nil {
		t.Fatalf("initialize pki: %v", err)
	}
	initializeTestCA(t, manager)
	settings := manager.Settings()
	settings.HTTPBaseURL = "https://pki.example.test:9443/pki"
	settings.CertValidityHours = 72
	if err := manager.UpdateSettings(settings); err != nil {
		t.Fatalf("configure CSR certificate profile: %v", err)
	}
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: "csr.example.test"},
		DNSNames: []string{"csr.example.test"},
	}, clientKey)
	if err != nil {
		t.Fatalf("create CSR: %v", err)
	}
	issued, err := manager.IssueCertificateFromCSR(csrDER, "scep")
	if err != nil {
		t.Fatalf("issue from CSR: %v", err)
	}
	if issued.KeyPEM != "" || issued.Type != "scep" {
		t.Fatalf("unexpected issued CSR record: %+v", issued)
	}
	certBlock, _ := pem.Decode([]byte(issued.CertPEM))
	if certBlock == nil {
		t.Fatal("issued certificate is not PEM encoded")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("parse issued certificate: %v", err)
	}
	if cert.Subject.CommonName != "csr.example.test" || len(cert.DNSNames) != 1 || cert.DNSNames[0] != "csr.example.test" {
		t.Fatalf("CSR identity was not preserved: %+v", cert)
	}
	if len(cert.CRLDistributionPoints) != 1 || cert.CRLDistributionPoints[0] != settings.HTTPBaseURL+"/crl" ||
		len(cert.OCSPServer) != 1 || cert.OCSPServer[0] != settings.HTTPBaseURL+"/ocsp" {
		t.Fatalf("service URLs were not added to the certificate: CDP=%v OCSP=%v", cert.CRLDistributionPoints, cert.OCSPServer)
	}
	if duration := cert.NotAfter.Sub(cert.NotBefore); duration < 71*time.Hour || duration > 73*time.Hour {
		t.Fatalf("certificate validity = %s, want about 72 hours", duration)
	}
	if err := cert.CheckSignatureFrom(managerCert(t, manager)); err != nil {
		t.Fatalf("issued certificate is not signed by CA: %v", err)
	}
}

func managerCert(t *testing.T, manager *pki.Manager) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode([]byte(manager.GetCACertPEM()))
	if block == nil {
		t.Fatal("root certificate is not PEM encoded")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse root certificate: %v", err)
	}
	return cert
}

func TestPKI_IssuanceRevocationAndReload(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-pki-inventory-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	manager, err := pki.New(pki.Config{DataDir: dir})
	if err != nil {
		t.Fatalf("initialize pki: %v", err)
	}
	initializeTestCA(t, manager)
	issued, err := manager.IssueCertificate("host.example.test", []string{"host.example.test"}, []net.IP{net.ParseIP("192.0.2.10")}, 60)
	if err != nil {
		t.Fatalf("issue certificate: %v", err)
	}
	if issued.KeyPEM != "" {
		t.Fatal("private key leaked through issuance response")
	}
	stored, err := manager.GetCertificate(issued.Serial)
	if err != nil {
		t.Fatalf("get issued certificate: %v", err)
	}
	if stored.KeyPEM == "" {
		t.Fatal("expected private key to be available for download")
	}
	certBlock, _ := pem.Decode([]byte(stored.CertPEM))
	if certBlock == nil {
		t.Fatal("issued certificate is not PEM encoded")
	}
	leaf, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("parse issued certificate: %v", err)
	}
	serial, ok := new(big.Int).SetString(issued.Serial, 16)
	if !ok || leaf.SerialNumber.Cmp(serial) != 0 {
		t.Fatalf("certificate serial %x does not match inventory serial %s", leaf.SerialNumber, issued.Serial)
	}

	if err := manager.RevokeCertificate(issued.Serial); err != nil {
		t.Fatalf("revoke certificate: %v", err)
	}
	crlDER, err := manager.GetCRL()
	if err != nil {
		t.Fatalf("create crl: %v", err)
	}
	crl, err := x509.ParseRevocationList(crlDER)
	if err != nil {
		t.Fatalf("parse crl: %v", err)
	}
	if len(crl.RevokedCertificateEntries) != 1 || crl.RevokedCertificateEntries[0].SerialNumber.Cmp(leaf.SerialNumber) != 0 {
		t.Fatalf("revoked serial not present in CRL: %+v", crl.RevokedCertificateEntries)
	}

	reloaded, err := pki.New(pki.Config{DataDir: dir})
	if err != nil {
		t.Fatalf("reload pki inventory: %v", err)
	}
	persisted, err := reloaded.GetCertificate(issued.Serial)
	if err != nil {
		t.Fatalf("load persisted certificate: %v", err)
	}
	if persisted.RevokedAt == 0 {
		t.Fatal("revocation state was not persisted")
	}

	rootBlock, _ := pem.Decode([]byte(reloaded.GetCACertPEM()))
	if rootBlock == nil {
		t.Fatal("root certificate is not PEM encoded")
	}
	rootCert, err := x509.ParseCertificate(rootBlock.Bytes)
	if err != nil {
		t.Fatalf("parse root certificate: %v", err)
	}
	request, err := ocsp.CreateRequest(leaf, rootCert, &ocsp.RequestOptions{Hash: crypto.SHA1})
	if err != nil {
		t.Fatalf("create OCSP request: %v", err)
	}
	responseDER, err := reloaded.CreateOCSPResponse(request)
	if err != nil {
		t.Fatalf("create revoked OCSP response: %v", err)
	}
	response, err := ocsp.ParseResponse(responseDER, rootCert)
	if err != nil {
		t.Fatalf("parse OCSP response: %v", err)
	}
	if response.Status != ocsp.Revoked {
		t.Fatalf("OCSP status = %d, want revoked", response.Status)
	}
}
