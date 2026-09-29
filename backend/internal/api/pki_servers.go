package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
)

type pkiServiceServers struct {
	manager *pki.Manager
	store   datastore.DataStore

	mu               sync.Mutex
	started          bool
	httpServer       *http.Server
	httpEcho         *echo.Echo
	httpPort         int
	acmeServer       *http.Server
	acmeEcho         *echo.Echo
	acmeRegistration *pki.ACMERegistration
	acmePort         int
	acmeBaseURL      string
}

func newPKIServiceServers(manager *pki.Manager, store datastore.DataStore) *pkiServiceServers {
	return &pkiServiceServers{manager: manager, store: store}
}

func (s *pkiServiceServers) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	s.started = true
	if err := s.applyLocked(s.manager.Settings()); err != nil {
		cleanupErr := s.stopLocked()
		s.started = false
		if cleanupErr != nil {
			return fmt.Errorf("start PKI service listeners: %w", errors.Join(err, cleanupErr))
		}
		return fmt.Errorf("start PKI service listeners: %w", err)
	}
	return nil
}

func (s *pkiServiceServers) Apply() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	return s.applyLocked(s.manager.Settings())
}

func (s *pkiServiceServers) applyLocked(settings pki.Settings) error {
	if s.httpServer != nil && (!settings.EnableHTTP || s.httpPort != settings.HTTPPort) {
		if err := s.stopHTTPServerLocked(); err != nil {
			return err
		}
	}
	if s.acmeServer != nil && (!settings.EnableACME || s.acmePort != settings.ACMEPort || s.acmeBaseURL != settings.ACMEBaseURL) {
		if err := s.stopACMEServerLocked(); err != nil {
			return err
		}
	}
	if !s.manager.Status().Ready {
		if settings.EnableHTTP || settings.EnableACME {
			return fmt.Errorf("cannot start PKI services before the CA is initialized")
		}
		return nil
	}
	if settings.EnableHTTP && s.httpServer == nil {
		if err := s.startHTTPServerLocked(settings.HTTPPort); err != nil {
			return err
		}
	}
	if settings.EnableACME && s.acmeServer == nil {
		if err := s.startACMEServerLocked(settings); err != nil {
			return err
		}
	}
	return nil
}

func (s *pkiServiceServers) startHTTPServerLocked(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("listen for PKI HTTP services on port %d: %w", port, err)
	}
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())
	registerPKIProtocolRoutes(e, s.manager, s.store)
	server := &http.Server{Handler: e, ReadHeaderTimeout: 10 * time.Second}
	s.httpServer, s.httpEcho, s.httpPort = server, e, port
	slog.Info("Starting PKI HTTP services", "addr", listener.Addr().String())
	go s.serve("PKI HTTP", server, listener)
	return nil
}

func (s *pkiServiceServers) startACMEServerLocked(settings pki.Settings) error {
	baseURL, err := url.Parse(settings.ACMEBaseURL)
	if err != nil || baseURL.Hostname() == "" {
		return fmt.Errorf("invalid ACME base URL %q", settings.ACMEBaseURL)
	}
	registrationEcho := echo.New()
	registrationEcho.HideBanner = true
	registrationEcho.HidePort = true
	registrationEcho.Use(middleware.Recover())
	registration, err := pki.RegisterACME(registrationEcho, s.manager, pki.ACMEConfig{
		BaseURL:            settings.ACMEBaseURL,
		ChallengeValidator: validateACMEChallenge,
	})
	if err != nil {
		return fmt.Errorf("register ACME service: %w", err)
	}
	host := baseURL.Hostname()
	dnsNames := make([]string, 0, len(settings.SANs))
	ipAddresses := make([]net.IP, 0, len(settings.SANs))
	for _, name := range settings.SANs {
		if ip := net.ParseIP(name); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else {
			dnsNames = append(dnsNames, name)
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		ipAddresses = appendUniqueIP(ipAddresses, ip)
	} else if !containsStringFold(dnsNames, host) {
		dnsNames = append(dnsNames, host)
	}
	validDays := (settings.CertValidityHours + 23) / 24
	if validDays < 1 {
		validDays = 1
	}
	pair, err := s.manager.IssueServerCertificate(host, dnsNames, ipAddresses, validDays)
	if err != nil {
		_ = registration.Close()
		return fmt.Errorf("create ACME TLS certificate: %w", err)
	}
	certificate, err := tls.X509KeyPair([]byte(pair.CertPEM), []byte(pair.KeyPEM))
	if err != nil {
		_ = registration.Close()
		return fmt.Errorf("parse ACME TLS certificate: %w", err)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", settings.ACMEPort))
	if err != nil {
		_ = registration.Close()
		return fmt.Errorf("listen for ACME on port %d: %w", settings.ACMEPort, err)
	}
	server := &http.Server{Handler: registrationEcho, ReadHeaderTimeout: 10 * time.Second}
	tlsListener := tls.NewListener(listener, &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	})
	s.acmeServer = server
	s.acmeEcho = registrationEcho
	s.acmeRegistration = registration
	s.acmePort = settings.ACMEPort
	s.acmeBaseURL = settings.ACMEBaseURL
	slog.Info("Starting HTTPS ACME server", "addr", listener.Addr().String(), "baseURL", settings.ACMEBaseURL)
	go s.serve("ACME HTTPS", server, tlsListener)
	return nil
}

func appendUniqueIP(ips []net.IP, ip net.IP) []net.IP {
	for _, existing := range ips {
		if existing.Equal(ip) {
			return ips
		}
	}
	return append(ips, ip)
}

func containsStringFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func (s *pkiServiceServers) serve(name string, server *http.Server, listener net.Listener) {
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		slog.Error("PKI service listener stopped", "service", name, "error", err)
	}
}

func (s *pkiServiceServers) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = false
	return s.stopLockedContext(ctx)
}

func (s *pkiServiceServers) stopLocked() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.stopLockedContext(ctx)
}

func (s *pkiServiceServers) stopLockedContext(ctx context.Context) error {
	var errs []error
	if err := s.stopACMEServerContextLocked(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := s.stopHTTPServerContextLocked(ctx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (s *pkiServiceServers) stopHTTPServerLocked() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.stopHTTPServerContextLocked(ctx)
}

func (s *pkiServiceServers) stopHTTPServerContextLocked(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	server := s.httpServer
	s.httpServer, s.httpEcho, s.httpPort = nil, nil, 0
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
		return fmt.Errorf("stop PKI HTTP services: %w", err)
	}
	return nil
}

func (s *pkiServiceServers) stopACMEServerLocked() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.stopACMEServerContextLocked(ctx)
}

func (s *pkiServiceServers) stopACMEServerContextLocked(ctx context.Context) error {
	if s.acmeRegistration != nil {
		if err := s.acmeRegistration.Close(); err != nil {
			return fmt.Errorf("close ACME registration: %w", err)
		}
	}
	if s.acmeServer == nil {
		s.acmeRegistration, s.acmeEcho = nil, nil
		return nil
	}
	server := s.acmeServer
	s.acmeServer, s.acmeEcho, s.acmeRegistration = nil, nil, nil
	s.acmePort, s.acmeBaseURL = 0, ""
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
		return fmt.Errorf("stop ACME HTTPS listener: %w", err)
	}
	return nil
}

func (s *pkiServiceServers) ACMEEcho() *echo.Echo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.acmeEcho
}
