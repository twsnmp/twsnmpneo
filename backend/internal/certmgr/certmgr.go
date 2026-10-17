package certmgr

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

// Mode represents how TLS certificate is obtained.
type Mode string

const (
	ModeDisabled   Mode = "disabled"
	ModeFile       Mode = "file"
	ModeACME       Mode = "acme"
	ModeSelfSigned Mode = "selfsigned"
)

// Config represents certificate manager configuration.
type Config struct {
	DataDir      string
	CertPath     string
	KeyPath      string
	Host         string
	IP           string
	ACMEDomains  string
	ACMEURL      string
	ACMEEmail    string
	ACMEInsecure bool
	ACMERenewDays int
	HTTPPort     int
	TLSPort      int
	EnableTLS    bool
}

// CertStatus represents status exposed to UI/API.
type CertStatus struct {
	Mode         Mode      `json:"mode"`
	Enabled      bool      `json:"enabled"`
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	NotBefore    time.Time `json:"notBefore"`
	NotAfter     time.Time `json:"notAfter"`
	DaysLeft     int       `json:"daysLeft"`
	DNSNames     []string  `json:"dnsNames"`
	IPAddresses  []string  `json:"ipAddresses"`
	KeyType      string    `json:"keyType"`
	IsSelfSigned bool      `json:"isSelfSigned"`
	ACMEDomains  []string  `json:"acmeDomains,omitempty"`
	NextCheck    time.Time `json:"nextCheck,omitempty"`
}

// Manager manages TLS server certificate lifecycle and auto-renewal.
type Manager struct {
	cfg          Config
	store        datastore.DataStore
	cert         atomic.Pointer[tls.Certificate]
	info         atomic.Pointer[CertInfo]
	httpProvider *EmbeddedHTTP01Provider
	activeMode   Mode
	mu           sync.Mutex
}

// New creates a new certificate manager.
func New(cfg Config, store datastore.DataStore) *Manager {
	if cfg.CertPath == "" {
		cfg.CertPath = filepath.Join(cfg.DataDir, "cert.pem")
	}
	if cfg.KeyPath == "" {
		cfg.KeyPath = filepath.Join(cfg.DataDir, "key.pem")
	}
	if cfg.ACMERenewDays <= 0 {
		cfg.ACMERenewDays = 30
	}

	return &Manager{
		cfg:          cfg,
		store:        store,
		httpProvider: NewEmbeddedHTTP01Provider(),
		activeMode:   ModeDisabled,
	}
}

// GetHTTP01Provider returns the embedded HTTP-01 challenge provider.
func (m *Manager) GetHTTP01Provider() *EmbeddedHTTP01Provider {
	return m.httpProvider
}

// GetCertificate returns active certificate for tls.Config.GetCertificate.
func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	c := m.cert.Load()
	if c == nil {
		return nil, errors.New("no certificate configured")
	}
	return c, nil
}

// GetTLSConfig returns standard tls.Config configured with this manager.
func (m *Manager) GetTLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: m.GetCertificate,
		MinVersion:     tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	}
}

// GetStatus returns the current certificate status.
func (m *Manager) GetStatus() CertStatus {
	st := CertStatus{
		Mode:    m.activeMode,
		Enabled: m.activeMode != ModeDisabled,
	}
	if m.cfg.ACMEDomains != "" {
		st.ACMEDomains = strings.Split(m.cfg.ACMEDomains, ",")
	}
	if info := m.info.Load(); info != nil {
		st.Subject = info.Subject
		st.Issuer = info.Issuer
		st.NotBefore = info.NotBefore
		st.NotAfter = info.NotAfter
		st.DaysLeft = info.DaysLeft
		st.DNSNames = info.DNSNames
		st.IPAddresses = info.IPAddresses
		st.KeyType = info.KeyType
		st.IsSelfSigned = info.IsSelfSigned
	}
	return st
}

// LoadOrResolve loads existing certificate or generates/obtains a new one.
func (m *Manager) LoadOrResolve(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hasCertFile := fileExists(m.cfg.CertPath) && fileExists(m.cfg.KeyPath)

	// 1. If valid certificate files already exist on disk:
	if hasCertFile {
		cert, info, err := LoadCertKeyPair(m.cfg.CertPath, m.cfg.KeyPath)
		if err == nil {
			mode := ModeFile
			if m.cfg.ACMEDomains != "" {
				mode = ModeACME
			}
			m.setCert(cert, info, mode)
			slog.Info("Loaded existing TLS certificate",
				"cert", m.cfg.CertPath,
				"subject", info.Subject,
				"daysLeft", info.DaysLeft,
				"mode", mode,
			)
			return nil
		}
		slog.Warn("Failed to parse existing certificate files, will generate temporary certificate", "error", err)
	}

	// 2. If TLS enabled or ACME configured, generate initial self-signed certificate so
	// HTTPS and HTTP-01 challenge listeners can immediately bind and accept connections.
	if m.cfg.EnableTLS || m.cfg.ACMEDomains != "" {
		return m.generateSelfSignedLocked(ctx)
	}

	m.activeMode = ModeDisabled
	return nil
}

// NeedsACME returns true if ACME is configured and current certificate needs issuance or renewal.
func (m *Manager) NeedsACME() bool {
	if m.cfg.ACMEDomains == "" {
		return false
	}
	info := m.info.Load()
	if info == nil || info.IsSelfSigned || info.DaysLeft <= m.cfg.ACMERenewDays {
		return true
	}
	return false
}

// InitialACME performs ACME certificate obtaining right after listeners start.
func (m *Manager) InitialACME(ctx context.Context) {
	if !m.NeedsACME() {
		return
	}
	go func() {
		// Wait a moment for HTTP listener (port 80) to be fully ready to accept challenge requests
		time.Sleep(500 * time.Millisecond)
		slog.Info("Initiating initial ACME certificate request...")
		if err := m.Renew(ctx); err != nil {
			slog.Error("Initial ACME certificate request failed", "error", err)
		}
	}()
}

func (m *Manager) generateSelfSignedLocked(ctx context.Context) error {
	slog.Info("Generating self-signed server certificate...")
	certPEM, keyPEM, err := GenerateSelfSignedCert(m.cfg.Host, m.cfg.IP)
	if err != nil {
		return fmt.Errorf("generate self-signed cert: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.cfg.CertPath), 0755); err != nil {
		return fmt.Errorf("create cert directory: %w", err)
	}
	if err := os.WriteFile(m.cfg.CertPath, certPEM, 0600); err != nil {
		return fmt.Errorf("write cert file: %w", err)
	}
	if err := os.WriteFile(m.cfg.KeyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}

	cert, info, err := ParseCertKeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("parse self-signed cert: %w", err)
	}

	m.setCert(cert, info, ModeSelfSigned)
	slog.Info("Self-signed TLS certificate generated and loaded",
		"subject", info.Subject,
		"daysLeft", info.DaysLeft,
	)

	m.recordEvent(ctx, "info", fmt.Sprintf(i18n.Trans("Generated self-signed TLS certificate for %s"), info.Subject))
	return nil
}

func (m *Manager) obtainACMELocked(ctx context.Context) error {
	domains := strings.Split(m.cfg.ACMEDomains, ",")
	slog.Info("Requesting certificate via ACME lego",
		"domains", domains,
		"url", m.cfg.ACMEURL,
	)

	res, err := ObtainCertificate(ACMEConfig{
		URL:          m.cfg.ACMEURL,
		Email:        m.cfg.ACMEEmail,
		Domains:      domains,
		Insecure:     m.cfg.ACMEInsecure,
		HTTPPort:     m.cfg.HTTPPort,
		HTTPProvider: m.httpProvider,
		TLSPort:      m.cfg.TLSPort,
		StorageDir:   m.cfg.DataDir,
	})
	if err != nil {
		m.recordEvent(ctx, "warn", fmt.Sprintf(i18n.Trans("ACME certificate renewal failed: %v"), err))
		return err
	}

	if err := os.MkdirAll(filepath.Dir(m.cfg.CertPath), 0755); err != nil {
		return fmt.Errorf("create cert directory: %w", err)
	}
	if err := os.WriteFile(m.cfg.CertPath, res.Certificate, 0600); err != nil {
		return fmt.Errorf("save acme cert: %w", err)
	}
	if err := os.WriteFile(m.cfg.KeyPath, res.PrivateKey, 0600); err != nil {
		return fmt.Errorf("save acme key: %w", err)
	}

	cert, info, err := ParseCertKeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		return fmt.Errorf("parse acme cert: %w", err)
	}

	m.setCert(cert, info, ModeACME)
	slog.Info("Successfully obtained and reloaded ACME certificate",
		"subject", info.Subject,
		"daysLeft", info.DaysLeft,
	)

	m.recordEvent(ctx, "info", fmt.Sprintf(i18n.Trans("ACME server certificate renewed for %s (days left: %d)"), info.Subject, info.DaysLeft))
	return nil
}

// Renew triggers ACME renewal immediately.
func (m *Manager) Renew(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.obtainACMELocked(ctx)
}

// StartAutoRenewal starts background loop checking certificate validity.
func (m *Manager) StartAutoRenewal(ctx context.Context, checkInterval time.Duration) {
	if m.cfg.ACMEDomains == "" {
		return
	}
	if checkInterval <= 0 {
		checkInterval = 12 * time.Hour
	}

	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				info := m.info.Load()
				if info == nil || info.DaysLeft <= m.cfg.ACMERenewDays {
					slog.Info("Checking and renewing ACME certificate...")
					if err := m.Renew(ctx); err != nil {
						slog.Error("Background ACME renewal failed", "error", err)
					}
				}
			}
		}
	}()
}

func (m *Manager) setCert(cert *tls.Certificate, info *CertInfo, mode Mode) {
	m.cert.Store(cert)
	m.info.Store(info)
	m.activeMode = mode
}

func (m *Manager) recordEvent(ctx context.Context, level, msg string) {
	if m.store != nil {
		_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "system",
			Level: level,
			Event: msg,
		})
	}
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
