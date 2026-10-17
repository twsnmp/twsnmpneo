package certmgr

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/challenge/tlsalpn01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

// ACMEConfig configures the ACME client.
type ACMEConfig struct {
	URL        string   // ACME Directory URL (e.g. Let's Encrypt)
	Email      string   // Registration email
	Domains    []string // Domains / SANs
	Insecure   bool     // Skip ACME server cert verification
	HTTPPort     int                     // Port for http-01 challenge fallback (0 to disable)
	HTTPProvider *EmbeddedHTTP01Provider // Embedded provider if shared with existing HTTP server
	TLSPort      int                     // Port for tls-alpn-01 challenge (0 to disable)
	RenewDays    int                     // Days before expiry to renew (default 30)
	StorageDir   string                  // Directory to store account keys and certs
}

type acmeAccount struct {
	Email        string                 `json:"email"`
	Registration *registration.Resource `json:"registration"`
	key          crypto.PrivateKey
}

func (u *acmeAccount) GetEmail() string {
	return u.Email
}

func (u *acmeAccount) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u *acmeAccount) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// loadOrNewAccount loads existing ACME account or generates a new one.
func loadOrNewAccount(storageDir, email string) (*acmeAccount, error) {
	accPath := filepath.Join(storageDir, "acme_account.json")
	keyPath := filepath.Join(storageDir, "acme_account.key")

	keyBytes, err := os.ReadFile(keyPath)
	if err == nil {
		block, _ := pem.Decode(keyBytes)
		if block != nil {
			privKey, parseErr := x509.ParseECPrivateKey(block.Bytes)
			if parseErr == nil {
				accBytes, readErr := os.ReadFile(accPath)
				if readErr == nil {
					var acc acmeAccount
					if json.Unmarshal(accBytes, &acc) == nil {
						acc.key = privKey
						if email != "" {
							acc.Email = email
						}
						return &acc, nil
					}
				}
			}
		}
	}

	// Generate new account key
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate acme key: %w", err)
	}

	keyDer, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("marshal acme key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDer,
	})

	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		slog.Warn("Failed to persist acme account key", "path", keyPath, "error", err)
	}

	return &acmeAccount{
		Email: email,
		key:   privKey,
	}, nil
}

func saveAccount(storageDir string, acc *acmeAccount) {
	accPath := filepath.Join(storageDir, "acme_account.json")
	data, err := json.MarshalIndent(acc, "", "  ")
	if err == nil {
		_ = os.WriteFile(accPath, data, 0600)
	}
}

// ObtainCertificate requests and obtains a certificate via ACME lego.
func ObtainCertificate(cfg ACMEConfig) (*certificate.Resource, error) {
	if len(cfg.Domains) == 0 {
		return nil, errors.New("no domains specified for ACME")
	}

	acc, err := loadOrNewAccount(cfg.StorageDir, cfg.Email)
	if err != nil {
		return nil, fmt.Errorf("acme account: %w", err)
	}

	legoConfig := lego.NewConfig(acc)
	if cfg.URL != "" {
		legoConfig.CADirURL = cfg.URL
	} else {
		legoConfig.CADirURL = lego.LEDirectoryProduction
	}

	legoConfig.HTTPClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Insecure},
	}

	client, err := lego.NewClient(legoConfig)
	if err != nil {
		return nil, fmt.Errorf("init lego client: %w", err)
	}

	// Setup challenges
	if cfg.HTTPProvider != nil {
		if err := client.Challenge.SetHTTP01Provider(cfg.HTTPProvider); err != nil {
			slog.Warn("Failed to set embedded HTTP01 provider", "error", err)
		}
	} else if cfg.HTTPPort > 0 {
		hProvider := http01.NewProviderServer("", strconv.Itoa(cfg.HTTPPort))
		if err := client.Challenge.SetHTTP01Provider(hProvider); err != nil {
			slog.Warn("Failed to set HTTP01 provider", "port", cfg.HTTPPort, "error", err)
		}
	}

	if cfg.TLSPort > 0 {
		tProvider := tlsalpn01.NewProviderServer("", strconv.Itoa(cfg.TLSPort))
		if err := client.Challenge.SetTLSALPN01Provider(tProvider); err != nil {
			slog.Warn("Failed to set TLSALPN01 provider", "port", cfg.TLSPort, "error", err)
		}
	}

	// Register account if not registered
	if acc.Registration == nil {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, fmt.Errorf("acme register: %w", err)
		}
		acc.Registration = reg
		saveAccount(cfg.StorageDir, acc)
	}

	// Request certificate
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate cert key: %w", err)
	}

	var domains []string
	for _, d := range cfg.Domains {
		d = strings.TrimSpace(d)
		if d != "" {
			domains = append(domains, d)
		}
	}

	request := certificate.ObtainRequest{
		Domains:    domains,
		Bundle:     true,
		PrivateKey: privKey,
	}

	certs, err := client.Certificate.Obtain(request)
	if err != nil {
		return nil, fmt.Errorf("obtain cert: %w", err)
	}

	return certs, nil
}
