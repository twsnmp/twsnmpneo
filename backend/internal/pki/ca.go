package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CertificatePair holds PEM-encoded certificate and private key strings.
type CertificatePair struct {
	CertPEM string `json:"certPEM"`
	KeyPEM  string `json:"keyPEM"`
}

// Config defines options for the Private PKI Manager.
type Config struct {
	DataDir      string
	Organization string
	CommonName   string
	ValidYears   int
}

// Manager manages Private PKI root CA and certificate issuances.
type Manager struct {
	mu           sync.RWMutex
	dir          string
	caCert       *x509.Certificate
	caKey        crypto.Signer
	caCertPEM    string
	caKeyPEM     string
	organization string
	commonName   string
	validYears   int
}

// New creates and initializes a PKI Manager, loading or creating the Root CA.
func New(cfg Config) (*Manager, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("empty pki data dir")
	}
	if cfg.Organization == "" {
		cfg.Organization = "TWSNMP NEO"
	}
	if cfg.CommonName == "" {
		cfg.CommonName = "TWSNMP NEO Root CA"
	}
	if cfg.ValidYears <= 0 {
		cfg.ValidYears = 10
	}

	m := &Manager{
		dir:          cfg.DataDir,
		organization: cfg.Organization,
		commonName:   cfg.CommonName,
		validYears:   cfg.ValidYears,
	}

	if err := os.MkdirAll(m.dir, 0700); err != nil {
		return nil, fmt.Errorf("create pki dir: %w", err)
	}

	if err := m.loadOrCreateRootCA(); err != nil {
		return nil, fmt.Errorf("init root ca: %w", err)
	}

	return m, nil
}

func (m *Manager) loadOrCreateRootCA() error {
	certPath := filepath.Join(m.dir, "ca.crt")
	keyPath := filepath.Join(m.dir, "ca.key")

	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			// Load existing CA
			certPEM, err := os.ReadFile(certPath)
			if err != nil {
				return err
			}
			keyPEM, err := os.ReadFile(keyPath)
			if err != nil {
				return err
			}

			block, _ := pem.Decode(certPEM)
			if block == nil {
				return fmt.Errorf("invalid ca cert pem")
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return fmt.Errorf("parse ca cert: %w", err)
			}

			kBlock, _ := pem.Decode(keyPEM)
			if kBlock == nil {
				return fmt.Errorf("invalid ca key pem")
			}
			key, err := x509.ParseECPrivateKey(kBlock.Bytes)
			if err != nil {
				return fmt.Errorf("parse ca key: %w", err)
			}

			m.caCert = cert
			m.caKey = key
			m.caCertPEM = string(certPEM)
			m.caKeyPEM = string(keyPEM)
			slog.Info("Loaded existing Root CA", "commonName", cert.Subject.CommonName)
			return nil
		}
	}

	// Generate new Root CA
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ca key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.AddDate(m.validYears, 0, 0)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{m.organization},
			CommonName:   m.commonName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return fmt.Errorf("create ca cert: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("parse created cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("marshal ca key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return err
	}

	m.caCert = cert
	m.caKey = privKey
	m.caCertPEM = string(certPEM)
	m.caKeyPEM = string(keyPEM)

	slog.Info("Created new Private Root CA", "commonName", m.commonName, "validUntil", notAfter)
	return nil
}

// GetCACertPEM returns the PEM-encoded Root CA certificate.
func (m *Manager) GetCACertPEM() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.caCertPEM
}

// IssueServerCertificate generates a signed server certificate for TLS.
func (m *Manager) IssueServerCertificate(commonName string, dnsNames []string, ips []net.IP, validDays int) (*CertificatePair, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.caCert == nil || m.caKey == nil {
		return nil, fmt.Errorf("root ca not initialized")
	}
	if validDays <= 0 {
		validDays = 365
	}

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate server key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	notBefore := time.Now().Add(-1 * time.Minute)
	notAfter := notBefore.AddDate(0, 0, validDays)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{m.organization},
			CommonName:   commonName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ips,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, m.caCert, &privKey.PublicKey, m.caKey)
	if err != nil {
		return nil, fmt.Errorf("sign server cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("marshal server key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return &CertificatePair{
		CertPEM: string(certPEM),
		KeyPEM:  string(keyPEM),
	}, nil
}
