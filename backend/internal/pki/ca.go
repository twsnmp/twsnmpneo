package pki

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ocsp"
)

// CertificatePair holds PEM-encoded certificate and private key strings.
type CertificatePair struct {
	CertPEM string `json:"certPEM"`
	KeyPEM  string `json:"keyPEM"`
}

type Certificate struct {
	Serial    string `json:"serial"`
	Subject   string `json:"subject"`
	Type      string `json:"type"`
	CertPEM   string `json:"certPEM"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"`
	RevokedAt int64  `json:"revokedAt,omitempty"`
	KeyPEM    string `json:"keyPEM,omitempty"`
}

type Status struct {
	Ready       bool   `json:"ready"`
	CommonName  string `json:"commonName,omitempty"`
	ExpiresAt   int64  `json:"expiresAt,omitempty"`
	Certificate string `json:"certificate,omitempty"`
}

type CertificateStore interface {
	ListPKICertificates() ([]*Certificate, error)
	SavePKICertificate(*Certificate) error
	DeleteAllPKICertificates() error
}

type CAOptions struct {
	CommonName        string   `json:"commonName"`
	Organization      string   `json:"organization"`
	SANs              []string `json:"sans"`
	KeyType           string   `json:"keyType"`
	ValidYears        int      `json:"validYears"`
	ACMEBaseURL       string   `json:"acmeBaseURL"`
	HTTPBaseURL       string   `json:"httpBaseURL"`
	CRLIntervalHours  int      `json:"crlIntervalHours"`
	CertValidityHours int      `json:"certValidityHours"`
	HTTPPort          int      `json:"httpPort"`
	ACMEPort          int      `json:"acmePort"`
}

type Settings struct {
	CAOptions
	EnableHTTP bool `json:"enableHTTP"`
	EnableACME bool `json:"enableACME"`
}

// Config defines options for the Private PKI Manager.
type Config struct {
	DataDir      string
	Organization string
	CommonName   string
	KeyType      string
	ValidYears   int
	CertStore    CertificateStore
}

// Manager manages Private PKI root CA and certificate issuances.
type Manager struct {
	mu           sync.RWMutex
	dir          string
	caCert       *x509.Certificate
	caKey        crypto.Signer
	caCertPEM    string
	caKeyPEM     string
	scepCert     *x509.Certificate
	scepKey      crypto.Signer
	organization string
	commonName   string
	validYears   int
	keyType      string
	settings     Settings
	certificates map[string]*Certificate
	certStore    CertificateStore
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
	if cfg.KeyType == "" {
		cfg.KeyType = "ecdsa-256"
	}

	m := &Manager{
		dir:          cfg.DataDir,
		organization: cfg.Organization,
		commonName:   cfg.CommonName,
		validYears:   cfg.ValidYears,
		keyType:      cfg.KeyType,
		settings:     defaultSettings(),
		certificates: make(map[string]*Certificate),
		certStore:    cfg.CertStore,
	}

	if err := os.MkdirAll(m.dir, 0700); err != nil {
		return nil, fmt.Errorf("create pki dir: %w", err)
	}
	if err := os.Chmod(m.dir, 0700); err != nil {
		return nil, fmt.Errorf("protect pki directory: %w", err)
	}
	if err := m.loadSettings(); err != nil {
		return nil, fmt.Errorf("load PKI settings: %w", err)
	}
	m.commonName = m.settings.CommonName
	m.organization = m.settings.Organization
	m.validYears = m.settings.ValidYears
	m.keyType = m.settings.KeyType

	if err := m.loadRootCA(); err != nil {
		return nil, fmt.Errorf("init root ca: %w", err)
	}
	if m.caCert != nil {
		if err := m.loadOrCreateSCEPCA(); err != nil {
			return nil, fmt.Errorf("init SCEP ca: %w", err)
		}
	}
	if err := m.loadCertificates(); err != nil {
		return nil, fmt.Errorf("load certificate inventory: %w", err)
	}

	return m, nil
}

func defaultSettings() Settings {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	sans := []string{host}
	if interfaces, err := net.Interfaces(); err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addresses, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, address := range addresses {
				ip, _, err := net.ParseCIDR(address.String())
				if err == nil && ip.To4() != nil {
					sans = append(sans, ip.String())
				}
			}
		}
	}
	httpBase := "http://" + net.JoinHostPort(host, "8082")
	acmeBase := "https://" + net.JoinHostPort(host, "8083")
	return Settings{
		CAOptions: CAOptions{
			CommonName:        "TWSNMP NEO Root CA",
			Organization:      "TWSNMP NEO",
			SANs:              sans,
			KeyType:           "ecdsa-256",
			ValidYears:        10,
			HTTPBaseURL:       httpBase,
			ACMEBaseURL:       acmeBase,
			CRLIntervalHours:  24,
			CertValidityHours: 8760,
			HTTPPort:          8082,
			ACMEPort:          8083,
		},
	}
}

func (m *Manager) loadSettings() error {
	data, err := os.ReadFile(filepath.Join(m.dir, "settings.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("decode settings: %w", err)
	}
	normalizeSettings(&settings)
	if err := validateSettings(settings); err != nil {
		return fmt.Errorf("invalid persisted settings: %w", err)
	}
	m.settings = settings
	return nil
}

func normalizeSettings(settings *Settings) {
	defaults := defaultSettings()
	if settings.CommonName == "" {
		settings.CommonName = defaults.CommonName
	}
	if settings.Organization == "" {
		settings.Organization = defaults.Organization
	}
	if len(settings.SANs) == 0 {
		settings.SANs = defaults.SANs
	}
	if settings.KeyType == "" {
		settings.KeyType = defaults.KeyType
	}
	if settings.ValidYears <= 0 {
		settings.ValidYears = defaults.ValidYears
	}
	if settings.HTTPBaseURL == "" {
		settings.HTTPBaseURL = defaults.HTTPBaseURL
	}
	if settings.ACMEBaseURL == "" {
		settings.ACMEBaseURL = defaults.ACMEBaseURL
	}
	if settings.CRLIntervalHours <= 0 {
		settings.CRLIntervalHours = defaults.CRLIntervalHours
	}
	if settings.CertValidityHours <= 0 {
		settings.CertValidityHours = defaults.CertValidityHours
	}
	if settings.HTTPPort <= 0 {
		settings.HTTPPort = defaults.HTTPPort
	}
	if settings.ACMEPort <= 0 {
		settings.ACMEPort = defaults.ACMEPort
	}
}

func validateSettings(settings Settings) error {
	if strings.TrimSpace(settings.CommonName) == "" || len(settings.SANs) == 0 {
		return fmt.Errorf("CA common name and at least one SAN are required")
	}
	if settings.ValidYears < 1 || settings.ValidYears > 100 {
		return fmt.Errorf("CA validity must be between 1 and 100 years")
	}
	if settings.KeyType != "ecdsa-256" && settings.KeyType != "rsa-2048" && settings.KeyType != "rsa-4096" {
		return fmt.Errorf("unsupported CA key type %q", settings.KeyType)
	}
	if settings.HTTPPort < 1 || settings.HTTPPort > 65535 || settings.ACMEPort < 1 || settings.ACMEPort > 65535 {
		return fmt.Errorf("HTTP and ACME ports must be between 1 and 65535")
	}
	if settings.HTTPPort == settings.ACMEPort {
		return fmt.Errorf("HTTP and ACME ports must be different")
	}
	if settings.CRLIntervalHours < 1 || settings.CRLIntervalHours > 8760 {
		return fmt.Errorf("CRL update interval must be between 1 and 8760 hours")
	}
	if settings.CertValidityHours < 1 || settings.CertValidityHours > 87600 {
		return fmt.Errorf("certificate validity must be between 1 and 87600 hours")
	}
	for _, san := range settings.SANs {
		san = strings.TrimSpace(san)
		if san == "" || strings.ContainsAny(san, "\r\n") || strings.ContainsAny(san, " /") {
			return fmt.Errorf("invalid CA SAN %q", san)
		}
	}
	for _, item := range []struct {
		value  string
		scheme string
	}{
		{settings.ACMEBaseURL, "https"},
		{settings.HTTPBaseURL, ""},
	} {
		u, err := url.Parse(item.value)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
			item.scheme != "" && u.Scheme != item.scheme ||
			item.scheme == "" && u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("invalid service base URL %q", item.value)
		}
	}
	return nil
}

func (m *Manager) saveSettings(settings Settings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode PKI settings: %w", err)
	}
	tmpPath := filepath.Join(m.dir, "settings.json.tmp")
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write PKI settings: %w", err)
	}
	if err := os.Chmod(tmpPath, 0600); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("protect PKI settings: %w", err)
	}
	if err := os.Rename(tmpPath, filepath.Join(m.dir, "settings.json")); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace PKI settings: %w", err)
	}
	return nil
}

func (m *Manager) Settings() Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	settings := m.settings
	settings.SANs = append([]string(nil), m.settings.SANs...)
	return settings
}

func (m *Manager) UpdateSettings(settings Settings) error {
	normalizeSettings(&settings)
	if err := validateSettings(settings); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.caCert != nil && (settings.CommonName != m.settings.CommonName ||
		settings.Organization != m.settings.Organization ||
		settings.KeyType != m.settings.KeyType ||
		settings.ValidYears != m.settings.ValidYears) {
		return fmt.Errorf("CA identity and key settings cannot be changed after initialization")
	}
	if err := m.saveSettings(settings); err != nil {
		return err
	}
	settings.SANs = append([]string(nil), settings.SANs...)
	m.settings = settings
	return nil
}

func (m *Manager) loadRootCA() error {
	certPath := filepath.Join(m.dir, "ca.crt")
	keyPath := filepath.Join(m.dir, "ca.key")
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)
	if certErr != nil && !os.IsNotExist(certErr) {
		return fmt.Errorf("stat ca certificate: %w", certErr)
	}
	if keyErr != nil && !os.IsNotExist(keyErr) {
		return fmt.Errorf("stat ca private key: %w", keyErr)
	}
	certExists, keyExists := certErr == nil, keyErr == nil
	if !certExists && !keyExists {
		return nil
	}
	if certExists != keyExists {
		return fmt.Errorf("incomplete root CA: certificate and private key must both exist")
	}
	if err := os.Chmod(keyPath, 0600); err != nil {
		return fmt.Errorf("protect ca private key: %w", err)
	}
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("invalid ca cert pem")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse ca cert: %w", err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("invalid ca key pem")
	}
	var key crypto.Signer
	switch keyBlock.Type {
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	case "PRIVATE KEY":
		var parsed any
		parsed, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err == nil {
			var ok bool
			key, ok = parsed.(crypto.Signer)
			if !ok {
				err = fmt.Errorf("unsupported private key type %T", parsed)
			}
		}
	default:
		err = fmt.Errorf("unsupported private key PEM type %q", keyBlock.Type)
	}
	if err != nil {
		return fmt.Errorf("parse ca key: %w", err)
	}
	if !publicKeysMatch(cert.PublicKey, key.Public()) {
		return fmt.Errorf("ca certificate and private key do not match")
	}
	if !cert.IsCA || cert.CheckSignatureFrom(cert) != nil {
		return fmt.Errorf("invalid self-signed root CA certificate")
	}
	m.caCert, m.caKey = cert, key
	m.caCertPEM, m.caKeyPEM = string(certPEM), string(keyPEM)
	m.commonName = cert.Subject.CommonName
	if len(cert.Subject.Organization) > 0 {
		m.organization = cert.Subject.Organization[0]
	}
	m.validYears = int(cert.NotAfter.Sub(cert.NotBefore).Hours() / (24 * 365))
	slog.Info("Loaded existing Root CA", "commonName", cert.Subject.CommonName)
	return nil
}

// InitializeCA creates the Root CA and its SCEP subordinate CA only on request.
func (m *Manager) InitializeCA(options CAOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.caCert != nil || m.caKey != nil {
		return fmt.Errorf("CA is already initialized")
	}
	settings := m.settings
	if value := strings.TrimSpace(options.CommonName); value != "" {
		settings.CommonName = value
	}
	if value := strings.TrimSpace(options.Organization); value != "" {
		settings.Organization = value
	}
	if options.SANs != nil {
		settings.SANs = append([]string(nil), options.SANs...)
	}
	if options.KeyType != "" {
		settings.KeyType = options.KeyType
	}
	if options.ValidYears > 0 {
		settings.ValidYears = options.ValidYears
	}
	if options.ACMEBaseURL != "" {
		settings.ACMEBaseURL = options.ACMEBaseURL
	}
	if options.HTTPBaseURL != "" {
		settings.HTTPBaseURL = options.HTTPBaseURL
	}
	if options.CRLIntervalHours > 0 {
		settings.CRLIntervalHours = options.CRLIntervalHours
	}
	if options.CertValidityHours > 0 {
		settings.CertValidityHours = options.CertValidityHours
	}
	if options.HTTPPort > 0 {
		settings.HTTPPort = options.HTTPPort
	}
	if options.ACMEPort > 0 {
		settings.ACMEPort = options.ACMEPort
	}
	normalizeSettings(&settings)
	if err := validateSettings(settings); err != nil {
		return err
	}
	m.settings = settings
	m.commonName, m.organization = settings.CommonName, settings.Organization
	m.validYears, m.keyType = settings.ValidYears, settings.KeyType
	if err := m.createRootCA(); err != nil {
		return err
	}
	if err := m.loadOrCreateSCEPCA(); err != nil {
		cleanupErrors := []error{err}
		for _, name := range []string{"ca.crt", "ca.key", "scep-ca.crt", "scep-ca.key"} {
			if removeErr := os.Remove(filepath.Join(m.dir, name)); removeErr != nil && !os.IsNotExist(removeErr) {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("remove incomplete %s: %w", name, removeErr))
			}
		}
		m.caCert, m.caKey = nil, nil
		m.caCertPEM, m.caKeyPEM = "", ""
		m.scepCert, m.scepKey = nil, nil
		return fmt.Errorf("create SCEP CA: %w", errors.Join(cleanupErrors...))
	}
	if err := m.saveSettings(settings); err != nil {
		cleanupErrors := []error{err}
		for _, name := range []string{"ca.crt", "ca.key", "scep-ca.crt", "scep-ca.key"} {
			if removeErr := os.Remove(filepath.Join(m.dir, name)); removeErr != nil && !os.IsNotExist(removeErr) {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("remove incomplete %s: %w", name, removeErr))
			}
		}
		m.caCert, m.caKey = nil, nil
		m.caCertPEM, m.caKeyPEM = "", ""
		m.scepCert, m.scepKey = nil, nil
		return fmt.Errorf("persist CA settings: %w", errors.Join(cleanupErrors...))
	}
	return nil
}

// ResetCA removes the CA keys and certificate inventory.
func (m *Manager) ResetCA() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, name := range []string{"ca.crt", "ca.key", "scep-ca.crt", "scep-ca.key"} {
		if err := os.Remove(filepath.Join(m.dir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	if m.certStore != nil {
		if err := m.certStore.DeleteAllPKICertificates(); err != nil {
			return fmt.Errorf("clear certificate inventory: %w", err)
		}
	} else if err := os.RemoveAll(filepath.Join(m.dir, "certificates")); err != nil {
		return fmt.Errorf("clear certificate inventory: %w", err)
	}
	m.caCert, m.caKey = nil, nil
	m.caCertPEM, m.caKeyPEM = "", ""
	m.scepCert, m.scepKey = nil, nil
	m.certificates = make(map[string]*Certificate)
	m.commonName = "TWSNMP NEO Root CA"
	m.organization = "TWSNMP NEO"
	m.validYears = 10
	m.keyType = "ecdsa-256"
	m.settings = defaultSettings()
	if err := m.saveSettings(m.settings); err != nil {
		return fmt.Errorf("reset PKI settings: %w", err)
	}
	return nil
}

func (m *Manager) createRootCA() error {
	var privKey crypto.Signer
	var err error
	switch m.keyType {
	case "rsa-2048":
		privKey, err = rsa.GenerateKey(rand.Reader, 2048)
	case "rsa-4096":
		privKey, err = rsa.GenerateKey(rand.Reader, 4096)
	default:
		privKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
	if err != nil {
		return fmt.Errorf("generate ca key: %w", err)
	}
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}
	notBefore := time.Now().UTC()
	notAfter := notBefore.AddDate(m.validYears, 0, 0)
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{Organization: []string{m.organization}, CommonName: m.commonName},
		NotBefore:    notBefore, NotAfter: notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	for _, san := range m.settings.SANs {
		if ip := net.ParseIP(san); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, san)
		}
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, privKey.Public(), privKey)
	if err != nil {
		return fmt.Errorf("create ca cert: %w", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("parse created cert: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("marshal ca key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(filepath.Join(m.dir, "ca.crt"), certPEM, 0644); err != nil {
		return fmt.Errorf("write ca certificate: %w", err)
	}
	if err := os.WriteFile(filepath.Join(m.dir, "ca.key"), keyPEM, 0600); err != nil {
		_ = os.Remove(filepath.Join(m.dir, "ca.crt"))
		_ = os.Remove(filepath.Join(m.dir, "ca.key"))
		return fmt.Errorf("write ca private key: %w", err)
	}
	m.caCert, m.caKey = cert, privKey
	m.caCertPEM, m.caKeyPEM = string(certPEM), string(keyPEM)
	slog.Info("Created new Private Root CA", "commonName", m.commonName, "validUntil", notAfter)
	return nil
}

func (m *Manager) loadOrCreateSCEPCA() error {
	if m.caCert == nil || m.caKey == nil {
		return fmt.Errorf("root CA is not initialized")
	}
	certPath := filepath.Join(m.dir, "scep-ca.crt")
	keyPath := filepath.Join(m.dir, "scep-ca.key")
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)
	if certErr != nil && !os.IsNotExist(certErr) {
		return fmt.Errorf("stat SCEP CA certificate: %w", certErr)
	}
	if keyErr != nil && !os.IsNotExist(keyErr) {
		return fmt.Errorf("stat SCEP CA private key: %w", keyErr)
	}
	certExists, keyExists := certErr == nil, keyErr == nil
	if certExists != keyExists {
		return fmt.Errorf("incomplete SCEP CA: certificate and private key must both exist")
	}
	if certExists {
		if err := os.Chmod(keyPath, 0600); err != nil {
			return fmt.Errorf("protect SCEP CA private key: %w", err)
		}
		certPEM, err := os.ReadFile(certPath)
		if err != nil {
			return fmt.Errorf("read SCEP CA certificate: %w", err)
		}
		certBlock, _ := pem.Decode(certPEM)
		if certBlock == nil || certBlock.Type != "CERTIFICATE" {
			return fmt.Errorf("invalid SCEP CA certificate PEM")
		}
		cert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return fmt.Errorf("parse SCEP CA certificate: %w", err)
		}
		keyPEM, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("read SCEP CA private key: %w", err)
		}
		keyBlock, _ := pem.Decode(keyPEM)
		if keyBlock == nil || keyBlock.Type != "RSA PRIVATE KEY" {
			return fmt.Errorf("invalid SCEP CA private key PEM")
		}
		key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			return fmt.Errorf("parse SCEP CA private key: %w", err)
		}
		if !cert.IsCA || cert.CheckSignatureFrom(m.caCert) != nil || !publicKeysMatch(cert.PublicKey, key.Public()) {
			return fmt.Errorf("SCEP CA certificate and private key do not match")
		}
		m.scepCert, m.scepKey = cert, key
		return nil
	}
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("generate SCEP CA key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate SCEP CA serial: %w", err)
	}
	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{m.organization}, CommonName: m.commonName + " SCEP CA"},
		NotBefore:    now.Add(-time.Minute), NotAfter: m.caCert.NotAfter,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:     true, MaxPathLen: 0, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, m.caCert, &key.PublicKey, m.caKey)
	if err != nil {
		return fmt.Errorf("create SCEP CA certificate: %w", err)
	}
	keyDER := x509.MarshalPKCS1PrivateKey(key)
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644); err != nil {
		return fmt.Errorf("write SCEP CA certificate: %w", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		_ = os.Remove(certPath)
		return fmt.Errorf("write SCEP CA private key: %w", err)
	}
	if err := os.Chmod(keyPath, 0600); err != nil {
		return fmt.Errorf("protect SCEP CA private key: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return fmt.Errorf("parse generated SCEP CA certificate: %w", err)
	}
	m.scepCert, m.scepKey = cert, key
	return nil
}

// GetCACertPEM returns the PEM-encoded Root CA certificate.
func (m *Manager) GetCACertPEM() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.caCertPEM
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.caCert == nil || m.caKey == nil {
		return Status{}
	}
	return Status{
		Ready:       true,
		CommonName:  m.caCert.Subject.CommonName,
		ExpiresAt:   m.caCert.NotAfter.Unix(),
		Certificate: m.caCertPEM,
	}
}

func (m *Manager) ListCertificates() []*Certificate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	certs := make([]*Certificate, 0, len(m.certificates))
	for _, cert := range m.certificates {
		copy := *cert
		copy.KeyPEM = ""
		certs = append(certs, &copy)
	}
	sort.Slice(certs, func(i, j int) bool {
		return certs[i].CreatedAt > certs[j].CreatedAt
	})
	return certs
}

func (m *Manager) IssueCertificate(commonName string, dnsNames []string, ipAddresses []net.IP, validDays int) (*Certificate, error) {
	if strings.TrimSpace(commonName) == "" {
		return nil, fmt.Errorf("common name is required")
	}
	if validDays < 1 || validDays > 36500 {
		return nil, fmt.Errorf("validity must be between 1 and 36500 days")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.caCert == nil || m.caKey == nil {
		return nil, fmt.Errorf("root ca not initialized")
	}
	if len(dnsNames)+len(ipAddresses) == 0 {
		if ip := net.ParseIP(commonName); ip != nil {
			ipAddresses = []net.IP{ip}
		} else {
			dnsNames = []string{commonName}
		}
	}
	if len(dnsNames)+len(ipAddresses) > 100 {
		return nil, fmt.Errorf("at most 100 subject alternative names are allowed")
	}
	for i, name := range dnsNames {
		name = strings.TrimSpace(name)
		if name == "" || strings.ContainsAny(name, "\r\n") {
			return nil, fmt.Errorf("invalid DNS subject alternative name")
		}
		dnsNames[i] = name
	}
	for _, ip := range ipAddresses {
		if ip == nil || ip.To16() == nil {
			return nil, fmt.Errorf("invalid IP subject alternative name")
		}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate certificate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial: %w", err)
	}
	now := time.Now().UTC()
	crlURLs, ocspURLs := m.certificateURLs()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{Organization: []string{m.organization}, CommonName: commonName},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.AddDate(0, 0, validDays),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ipAddresses,
		CRLDistributionPoints: crlURLs,
		OCSPServer:            ocspURLs,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, m.caCert, &key.PublicKey, m.caKey)
	if err != nil {
		return nil, fmt.Errorf("sign certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal certificate key: %w", err)
	}
	cert := &Certificate{
		Serial:    serial.Text(16),
		Subject:   template.Subject.String(),
		Type:      "manual",
		CertPEM:   string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		KeyPEM:    string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})),
		CreatedAt: now.Unix(),
		ExpiresAt: template.NotAfter.Unix(),
	}
	if err := m.saveCertificateLocked(cert); err != nil {
		return nil, err
	}
	copy := *cert
	copy.KeyPEM = ""
	return &copy, nil
}

func (m *Manager) IssueCertificateFromCSR(csrDER []byte, certType string) (*Certificate, error) {
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, fmt.Errorf("parse certificate request: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("verify certificate request signature: %w", err)
	}
	if strings.TrimSpace(csr.Subject.CommonName) == "" {
		return nil, fmt.Errorf("certificate request common name is required")
	}
	if len(csr.DNSNames)+len(csr.IPAddresses)+len(csr.EmailAddresses) > 100 {
		return nil, fmt.Errorf("at most 100 subject alternative names are allowed")
	}
	if certType == "" {
		certType = "csr"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.caCert == nil || m.caKey == nil {
		return nil, fmt.Errorf("root ca not initialized")
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial: %w", err)
	}
	now := time.Now().UTC()
	crlURLs, ocspURLs := m.certificateURLs()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               csr.Subject,
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Duration(m.settings.CertValidityHours) * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              append([]string(nil), csr.DNSNames...),
		EmailAddresses:        append([]string(nil), csr.EmailAddresses...),
		IPAddresses:           append([]net.IP(nil), csr.IPAddresses...),
		CRLDistributionPoints: crlURLs,
		OCSPServer:            ocspURLs,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, m.caCert, csr.PublicKey, m.caKey)
	if err != nil {
		return nil, fmt.Errorf("sign certificate request: %w", err)
	}
	cert := &Certificate{
		Serial:    serial.Text(16),
		Subject:   template.Subject.String(),
		Type:      certType,
		CertPEM:   string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		CreatedAt: now.Unix(),
		ExpiresAt: template.NotAfter.Unix(),
	}
	if err := m.saveCertificateLocked(cert); err != nil {
		return nil, err
	}
	copy := *cert
	return &copy, nil
}

func (m *Manager) certificateURLs() ([]string, []string) {
	base := strings.TrimRight(m.settings.HTTPBaseURL, "/")
	return []string{base + "/crl"}, []string{base + "/ocsp"}
}

func (m *Manager) RevokeCertificate(serial string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cert, ok := m.certificates[strings.ToLower(serial)]
	if !ok {
		return os.ErrNotExist
	}
	if cert.RevokedAt != 0 {
		return nil
	}
	cert.RevokedAt = time.Now().Unix()
	return m.saveCertificateLocked(cert)
}

func (m *Manager) CreateOCSPResponse(requestDER []byte) ([]byte, error) {
	req, err := ocsp.ParseRequest(requestDER)
	if err != nil {
		return nil, fmt.Errorf("parse OCSP request: %w", err)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.caCert == nil || m.caKey == nil {
		return nil, fmt.Errorf("root ca not initialized")
	}
	now := time.Now()
	response := ocsp.Response{
		Status:       ocsp.Unknown,
		SerialNumber: req.SerialNumber,
		ThisUpdate:   now,
		NextUpdate:   now.Add(10 * time.Minute),
		IssuerHash:   req.HashAlgorithm,
	}
	var publicKeyInfo struct {
		Algorithm pkix.AlgorithmIdentifier
		PublicKey asn1.BitString
	}
	if _, err := asn1.Unmarshal(m.caCert.RawSubjectPublicKeyInfo, &publicKeyInfo); err != nil {
		return nil, fmt.Errorf("parse CA public key for OCSP: %w", err)
	}
	if !req.HashAlgorithm.Available() {
		return nil, fmt.Errorf("unsupported OCSP issuer hash algorithm %v", req.HashAlgorithm)
	}
	nameHash := req.HashAlgorithm.New()
	_, _ = nameHash.Write(m.caCert.RawSubject)
	keyHash := req.HashAlgorithm.New()
	_, _ = keyHash.Write(publicKeyInfo.PublicKey.RightAlign())
	if bytes.Equal(req.IssuerNameHash, nameHash.Sum(nil)) && bytes.Equal(req.IssuerKeyHash, keyHash.Sum(nil)) {
		if cert, ok := m.certificates[strings.ToLower(req.SerialNumber.Text(16))]; ok {
			if cert.RevokedAt != 0 {
				response.Status = ocsp.Revoked
				response.RevokedAt = time.Unix(cert.RevokedAt, 0)
				response.RevocationReason = ocsp.Unspecified
			} else {
				response.Status = ocsp.Good
			}
		}
	}
	result, err := ocsp.CreateResponse(m.caCert, m.caCert, response, m.caKey)
	if err != nil {
		return nil, fmt.Errorf("create OCSP response: %w", err)
	}
	return result, nil
}

func (m *Manager) GetCertificate(serial string) (*Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cert, ok := m.certificates[strings.ToLower(serial)]
	if !ok {
		return nil, os.ErrNotExist
	}
	copy := *cert
	return &copy, nil
}

func (m *Manager) GetCRL() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.caCert == nil || m.caKey == nil {
		return nil, fmt.Errorf("root ca not initialized")
	}
	revoked := make([]x509.RevocationListEntry, 0)
	for _, cert := range m.certificates {
		if cert.RevokedAt == 0 {
			continue
		}
		serial, ok := new(big.Int).SetString(cert.Serial, 16)
		if !ok {
			return nil, fmt.Errorf("invalid certificate serial %q", cert.Serial)
		}
		revoked = append(revoked, x509.RevocationListEntry{
			SerialNumber:   serial,
			RevocationTime: time.Unix(cert.RevokedAt, 0),
		})
	}
	template := &x509.RevocationList{
		Number:                    big.NewInt(time.Now().Unix()),
		ThisUpdate:                time.Now().UTC(),
		NextUpdate:                time.Now().UTC().Add(time.Duration(m.settings.CRLIntervalHours) * time.Hour),
		RevokedCertificateEntries: revoked,
	}
	return x509.CreateRevocationList(rand.Reader, template, m.caCert, m.caKey)
}

func (m *Manager) loadCertificates() error {
	if m.certStore != nil {
		certs, err := m.certStore.ListPKICertificates()
		if err != nil {
			return err
		}
		for _, cert := range certs {
			if cert == nil || cert.Serial == "" {
				return fmt.Errorf("stored certificate record has no serial")
			}
			copy := *cert
			m.certificates[strings.ToLower(copy.Serial)] = &copy
		}
		return nil
	}
	dir := filepath.Join(m.dir, "certificates")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		var cert Certificate
		if err := json.Unmarshal(data, &cert); err != nil {
			return fmt.Errorf("parse certificate record %s: %w", entry.Name(), err)
		}
		if cert.Serial == "" {
			return fmt.Errorf("certificate record %s has no serial", entry.Name())
		}
		m.certificates[strings.ToLower(cert.Serial)] = &cert
	}
	return nil
}

func (m *Manager) saveCertificateLocked(cert *Certificate) error {
	if m.certStore != nil {
		if err := m.certStore.SavePKICertificate(cert); err != nil {
			return fmt.Errorf("save certificate record: %w", err)
		}
		copy := *cert
		m.certificates[strings.ToLower(cert.Serial)] = &copy
		return nil
	}
	dir := filepath.Join(m.dir, "certificates")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create certificate inventory: %w", err)
	}
	data, err := json.Marshal(cert)
	if err != nil {
		return fmt.Errorf("encode certificate record: %w", err)
	}
	path := filepath.Join(dir, strings.ToLower(cert.Serial)+".json")
	tmp, err := os.CreateTemp(dir, ".certificate-*.tmp")
	if err != nil {
		return fmt.Errorf("create certificate record: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("protect certificate record: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write certificate record: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close certificate record: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("save certificate record: %w", err)
	}
	copy := *cert
	m.certificates[strings.ToLower(cert.Serial)] = &copy
	return nil
}

func publicKeysMatch(a, b any) bool {
	aDER, err := x509.MarshalPKIXPublicKey(a)
	if err != nil {
		return false
	}
	bDER, err := x509.MarshalPKIXPublicKey(b)
	return err == nil && string(aDER) == string(bDER)
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
