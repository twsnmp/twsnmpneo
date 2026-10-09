package pki

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

var (
	acmeAccMap   = make(map[string]*account)
	acmeOrderMap = make(map[string]*order)
	acmeAuthzMap = make(map[string]*authorization)
	acmeCertMap  = make(map[string]*acmeCertificate)
	acmeMapMu    sync.RWMutex
)

// GetAcmeServerStatus returns the status of the ACME server.
func (m *Manager) GetAcmeServerStatus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.lastAcmeServerErr != nil {
		return fmt.Sprintf("error %v", m.lastAcmeServerErr)
	} else if m.acmeServerRunning {
		return fmt.Sprintf("running port=%d", m.conf.AcmePort)
	}
	return "stopped"
}

// StartAcmeServer starts the ACME HTTPS server.
func (m *Manager) StartAcmeServer() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.acmeServer != nil {
		return
	}
	if !m.IsCAValid() {
		slog.Warn("Cannot start ACME server before Root CA is initialized")
		return
	}

	if err := m.createAcmeServerCertificateLocked(); err != nil {
		slog.Error("Failed to create ACME server certificate", "error", err)
		m.lastAcmeServerErr = err
		return
	}

	m.lastAcmeServerErr = nil
	m.acmeServerRunning = true
	m.acmeServer = echo.New()
	m.acmeServer.HideBanner = true
	m.acmeServer.HidePort = true

	port := m.conf.AcmePort
	if m.store != nil {
		_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "pki",
			Level: "info",
			Event: fmt.Sprintf(i18n.Trans("Started ACME server on port %d"), port),
		})
	}
	go m.acmeServerFunc(m.acmeServer, port)
}

// StopAcmeServer stops the ACME server.
func (m *Manager) StopAcmeServer() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopAcmeServerLocked()
}

func (m *Manager) stopAcmeServerLocked() {
	if m.acmeServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer func() {
		cancel()
		m.acmeServer = nil
		m.lastAcmeServerErr = nil
		m.acmeServerRunning = false
	}()
	if err := m.acmeServer.Shutdown(ctx); err != nil {
		slog.Warn("PKI ACME server shutdown error", "error", err)
	}
	if m.store != nil {
		_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "pki",
			Level: "info",
			Event: i18n.Trans("Stopped ACME server"),
		})
	}
}

func (m *Manager) createAcmeServerCertificateLocked() error {
	if m.conf.AcmeServerCert != "" && m.conf.AcmeServerKey != "" {
		return nil
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	publicKey := &key.PublicKey
	ca, err := x509.ParseCertificate(m.rootCACertificate)
	if err != nil {
		return err
	}
	sn := m.getSerial()
	tmp := &x509.Certificate{
		SerialNumber: big.NewInt(sn),
		Subject: pkix.Name{
			CommonName: m.conf.Name + " ACME Server",
		},
		NotBefore:             time.Now().UTC(),
		NotAfter:              time.Now().AddDate(m.conf.RootCATerm, 0, 0).UTC(),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  false,
		BasicConstraintsValid: true,
		CRLDistributionPoints: []string{},
		OCSPServer:            []string{},
	}
	for _, san := range strings.Split(m.conf.SANs, ",") {
		san = strings.TrimSpace(san)
		if san == "" {
			continue
		}
		baseURL := fmt.Sprintf("http://%s:%d", san, m.conf.HTTPPort)
		if ip := net.ParseIP(san); ip == nil {
			tmp.DNSNames = append(tmp.DNSNames, san)
		} else {
			tmp.IPAddresses = append(tmp.IPAddresses, ip)
		}
		tmp.CRLDistributionPoints = append(tmp.CRLDistributionPoints, baseURL+"/crl")
		tmp.OCSPServer = append(tmp.OCSPServer, baseURL+"/ocsp")
	}
	if strings.HasPrefix(m.conf.HTTPBaseURL, "http://") {
		baseURL := strings.TrimRight(m.conf.HTTPBaseURL, "/")
		tmp.CRLDistributionPoints = append(tmp.CRLDistributionPoints, baseURL+"/crl")
		tmp.OCSPServer = append(tmp.OCSPServer, baseURL+"/ocsp")
	}
	cert, err := x509.CreateCertificate(rand.Reader, tmp, ca, publicKey, m.rootCAPrivateKey)
	if err != nil {
		return err
	}
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	m.conf.AcmeServerCert = string(makePEM(cert, "CERTIFICATE"))
	m.conf.AcmeServerKey = string(makePEM(b, "EC PRIVATE KEY"))

	if m.store != nil {
		_ = m.store.SavePKIConf(context.Background(), &m.conf)
		_ = m.store.SavePKICert(context.Background(), &datastore.PKICertEnt{
			ID:          fmt.Sprintf("%x", sn),
			Subject:     tmp.Subject.String(),
			Created:     time.Now().UnixNano(),
			Expire:      tmp.NotAfter.UnixNano(),
			Certificate: m.conf.AcmeServerCert,
			Type:        "system",
		})
	}
	return nil
}

// ACME Models

type meta struct {
	TermsOfService          string   `json:"termsOfService,omitempty"`
	Website                 string   `json:"website,omitempty"`
	CaaIdentities           []string `json:"caaIdentities,omitempty"`
	ExternalAccountRequired bool     `json:"externalAccountRequired,omitempty"`
}

type directory struct {
	NewNonce   string `json:"newNonce"`
	NewAccount string `json:"newAccount"`
	NewOrder   string `json:"newOrder"`
	RevokeCert string `json:"revokeCert"`
	KeyChange  string `json:"keyChange"`
	Meta       *meta  `json:"meta,omitempty"`
}

type externalAccountBinding struct {
	Protected string `json:"protected"`
	Payload   string `json:"payload"`
	Sig       string `json:"signature"`
}

type newAccountRequest struct {
	Contact                []string                `json:"contact"`
	OnlyReturnExisting     bool                    `json:"onlyReturnExisting"`
	TermsOfServiceAgreed   bool                    `json:"termsOfServiceAgreed"`
	ExternalAccountBinding *externalAccountBinding `json:"externalAccountBinding,omitempty"`
}

type account struct {
	ID                     string           `json:"-"`
	Key                    *jose.JSONWebKey `json:"-"`
	Contact                []string         `json:"contact,omitempty"`
	Status                 string           `json:"status"`
	OrdersURL              string           `json:"orders"`
	ExternalAccountBinding interface{}      `json:"externalAccountBinding,omitempty"`
	LocationPrefix         string           `json:"-"`
	ProvisionerID          string           `json:"-"`
	ProvisionerName        string           `json:"-"`
}

type acmeCertificate struct {
	ID          string
	AccountID   string
	OrderID     string
	Certificate []byte
}

type identifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type newOrderRequest struct {
	Identifiers []identifier `json:"identifiers"`
	NotBefore   time.Time    `json:"notBefore,omitempty"`
	NotAfter    time.Time    `json:"notAfter,omitempty"`
}

type subproblem struct {
	Type       string      `json:"type"`
	Detail     string      `json:"detail"`
	Identifier *identifier `json:"identifier,omitempty"`
}

type acmeError struct {
	Type        string       `json:"type"`
	Detail      string       `json:"detail"`
	Subproblems []subproblem `json:"subproblems,omitempty"`
	Err         error        `json:"-"`
	Status      int          `json:"-"`
}

type order struct {
	ID                string       `json:"id"`
	AccountID         string       `json:"-"`
	ProvisionerID     string       `json:"-"`
	Status            string       `json:"status"`
	ExpiresAt         time.Time    `json:"expires"`
	Identifiers       []identifier `json:"identifiers"`
	NotBefore         time.Time    `json:"notBefore"`
	NotAfter          time.Time    `json:"notAfter"`
	Error             *acmeError   `json:"error,omitempty"`
	AuthorizationIDs  []string     `json:"-"`
	AuthorizationURLs []string     `json:"authorizations"`
	FinalizeURL       string       `json:"finalize"`
	CertificateID     string       `json:"-"`
	CertificateURL    string       `json:"certificate,omitempty"`
}

type authorization struct {
	ID          string       `json:"-"`
	AccountID   string       `json:"-"`
	Token       string       `json:"-"`
	Fingerprint string       `json:"-"`
	Identifier  identifier   `json:"identifier"`
	Status      string       `json:"status"`
	Challenges  []*challenge `json:"challenges"`
	Wildcard    bool         `json:"wildcard"`
	ExpiresAt   time.Time    `json:"expires"`
	Error       *acmeError   `json:"error,omitempty"`
}

type challenge struct {
	ID              string     `json:"-"`
	AccountID       string     `json:"-"`
	AuthorizationID string     `json:"-"`
	Value           string     `json:"-"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	Token           string     `json:"token"`
	ValidatedAt     string     `json:"validated,omitempty"`
	URL             string     `json:"url"`
	Target          string     `json:"target,omitempty"`
	Error           *acmeError `json:"error,omitempty"`
	Payload         []byte     `json:"-"`
	PayloadFormat   string     `json:"-"`
}

type finalizeRequest struct {
	CSR string `json:"csr"`
	csr *x509.CertificateRequest
}

type revokeReqest struct {
	Certificate string `json:"certificate"`
	ReasonCode  *int   `json:"reason,omitempty"`
}

func (m *Manager) acmeServerFunc(e *echo.Echo, port int) {
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	e.GET("/directory", func(c echo.Context) error {
		baseURL := strings.TrimRight(m.conf.AcmeBaseURL, "/")
		d := directory{
			NewNonce:   baseURL + "/new-nonce",
			NewAccount: baseURL + "/new-account",
			NewOrder:   baseURL + "/new-order",
			RevokeCert: baseURL + "/revoke-cert",
			KeyChange:  baseURL + "/key-change",
		}
		return c.JSON(http.StatusOK, d)
	})
	e.HEAD("/new-nonce", func(c echo.Context) error {
		m.addACMEHeader(c)
		return c.NoContent(http.StatusOK)
	})
	e.GET("/new-nonce", func(c echo.Context) error {
		m.addACMEHeader(c)
		return c.NoContent(http.StatusNoContent)
	})
	e.POST("/new-account", func(c echo.Context) error {
		m.addACMEHeader(c)
		jws, err := getJWS(c, m.conf.AcmeBaseURL)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		jwk := extractJWK(jws)
		if jwk == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "jwk not found"})
		}
		payload, err := jws.Verify(jwk)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		var nar newAccountRequest
		if err := json.Unmarshal(payload, &nar); err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		baseURL := strings.TrimRight(m.conf.AcmeBaseURL, "/")
		acc := &account{
			ID:        jwk.KeyID,
			Key:       jwk,
			Status:    "valid",
			Contact:   nar.Contact,
			OrdersURL: baseURL + "/account/" + jwk.KeyID + "/orders",
		}
		acmeMapMu.Lock()
		acmeAccMap[acc.ID] = acc
		acmeMapMu.Unlock()

		c.Response().Header().Add("Location", baseURL+"/account/"+acc.ID)
		return c.JSON(http.StatusCreated, acc)
	})
	e.POST("/new-order", func(c echo.Context) error {
		baseURL := strings.TrimRight(m.conf.AcmeBaseURL, "/")
		m.addACMEHeader(c)
		jws, err := getJWS(c, m.conf.AcmeBaseURL)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		payload, err := jws.Verify(acc.Key)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		var nor newOrderRequest
		if err := json.Unmarshal(payload, &nor); err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		now := time.Now().UTC().Truncate(time.Second)
		o := &order{
			ID:                createNonce(),
			AccountID:         acc.ID,
			Status:            "pending",
			Identifiers:       nor.Identifiers,
			ExpiresAt:         now.Add(time.Hour * 24),
			AuthorizationIDs:  make([]string, len(nor.Identifiers)),
			AuthorizationURLs: make([]string, len(nor.Identifiers)),
			NotBefore:         nor.NotBefore,
			NotAfter:          nor.NotAfter,
		}
		for i, iden := range o.Identifiers {
			azID := createNonce()
			chTypes := []string{}
			switch iden.Type {
			case "ip":
				chTypes = append(chTypes, "http-01", "tls-alpn-01")
			case "dns":
				chTypes = append(chTypes, "dns-01")
				if !strings.HasPrefix(iden.Value, "*.") {
					chTypes = append(chTypes, "http-01", "tls-alpn-01")
				}
			}
			az := &authorization{
				ID:         azID,
				AccountID:  acc.ID,
				Identifier: iden,
				ExpiresAt:  o.ExpiresAt,
				Status:     "pending",
				Challenges: []*challenge{},
				Token:      createNonce(),
			}
			for _, chType := range chTypes {
				chID := createNonce()
				az.Challenges = append(az.Challenges,
					&challenge{
						ID:              chID,
						AccountID:       acc.ID,
						AuthorizationID: azID,
						Type:            chType,
						Value:           iden.Value,
						Token:           az.Token,
						Status:          "pending",
						URL:             baseURL + "/challenge/" + azID + "/" + chID,
					})
			}
			o.AuthorizationIDs[i] = az.ID
			o.AuthorizationURLs[i] = baseURL + "/authz/" + azID
			acmeMapMu.Lock()
			acmeAuthzMap[az.ID] = az
			acmeMapMu.Unlock()
		}
		o.FinalizeURL = baseURL + "/order/" + o.ID + "/finalize"
		acmeMapMu.Lock()
		acmeOrderMap[o.ID] = o
		acmeMapMu.Unlock()

		c.Response().Header().Add("Location", baseURL+"/order/"+o.ID)
		return c.JSON(http.StatusCreated, o)
	})

	e.POST("/challenge/:azID/:chID", func(c echo.Context) error {
		baseURL := strings.TrimRight(m.conf.AcmeBaseURL, "/")
		m.addACMEHeader(c)
		jws, err := getJWS(c, m.conf.AcmeBaseURL)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		azID := c.Param("azID")
		chID := c.Param("chID")
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}

		acmeMapMu.RLock()
		az, ok := acmeAuthzMap[azID]
		acmeMapMu.RUnlock()
		if !ok || az.AccountID != acc.ID {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "authorization not found"})
		}

		var ch *challenge
		for _, cand := range az.Challenges {
			if cand.ID == chID {
				ch = cand
				break
			}
		}
		if ch == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "challenge not found"})
		}

		switch ch.Type {
		case "http-01":
			if err := http01Validate(acc, ch); err != nil {
				return c.JSON(http.StatusBadRequest, err)
			}
		case "tls-alpn-01":
			if err := tlsalpn01Validate(acc, ch); err != nil {
				return c.JSON(http.StatusBadRequest, err)
			}
		case "dns-01":
			if err := dns01Validate(acc, ch); err != nil {
				return c.JSON(http.StatusBadRequest, err)
			}
		default:
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported challenge type"})
		}

		ch.Status = "valid"
		c.Response().Header().Add("Location", baseURL+"/challenge/"+az.ID+"/"+chID)
		c.Response().Header().Add("Link", fmt.Sprintf("<%s/authz/%s>;rel=up", baseURL, az.ID))
		return c.JSON(http.StatusOK, ch)
	})

	e.POST("/order/:id/finalize", func(c echo.Context) error {
		baseURL := strings.TrimRight(m.conf.AcmeBaseURL, "/")
		m.addACMEHeader(c)
		jws, err := getJWS(c, m.conf.AcmeBaseURL)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		id := c.Param("id")
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}

		acmeMapMu.RLock()
		o, ok := acmeOrderMap[id]
		acmeMapMu.RUnlock()
		if !ok || o.AccountID != acc.ID {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "order not found"})
		}

		payload, err := jws.Verify(acc.Key)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		var fr finalizeRequest
		if err := json.Unmarshal(payload, &fr); err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}

		csrBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(fr.CSR, "="))
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}

		m.mu.Lock()
		cert, certID, err := m.createCertificateFromCSR(csrBytes, "acme", map[string]string{
			"AccountID":  o.AccountID,
			"OrderID":    o.ID,
			"RemoteAddr": c.RealIP(),
		})
		rootCertPEM := m.conf.RootCACert
		m.mu.Unlock()

		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}

		pemCert := string(makePEM(cert, "CERTIFICATE")) + "\n" + rootCertPEM
		acmeMapMu.Lock()
		acmeCertMap[certID] = &acmeCertificate{
			ID:          certID,
			OrderID:     o.ID,
			AccountID:   o.AccountID,
			Certificate: []byte(pemCert),
		}
		o.Status = "valid"
		o.CertificateID = certID
		o.CertificateURL = baseURL + "/certificate/" + o.CertificateID
		acmeMapMu.Unlock()

		c.Response().Header().Add("Location", baseURL+"/order/"+id)
		return c.JSON(http.StatusOK, o)
	})

	e.POST("/certificate/:id", func(c echo.Context) error {
		m.addACMEHeader(c)
		jws, err := getJWS(c, m.conf.AcmeBaseURL)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		id := c.Param("id")
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}

		acmeMapMu.RLock()
		cert, ok := acmeCertMap[id]
		acmeMapMu.RUnlock()
		if !ok || cert.AccountID != acc.ID {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "certificate not found"})
		}
		return c.Blob(http.StatusOK, "application/pem-certificate-chain", cert.Certificate)
	})

	e.POST("/revoke-cert", func(c echo.Context) error {
		m.addACMEHeader(c)
		jws, err := getJWS(c, m.conf.AcmeBaseURL)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		var jwk *jose.JSONWebKey
		var acc *account
		if canExtractJWKFrom(jws) {
			jwk = extractJWK(jws)
			if jwk == nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing jwk"})
			}
		} else {
			acc, err = lookupJWKAndAccount(jws)
			if err != nil {
				return c.JSON(http.StatusBadRequest, err)
			}
			jwk = acc.Key
		}
		payload, err := jws.Verify(jwk)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		var rvr revokeReqest
		if err := json.Unmarshal(payload, &rvr); err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}

		certBytes, err := base64.RawURLEncoding.DecodeString(rvr.Certificate)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		certToBeRevoked, err := x509.ParseCertificate(certBytes)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		serial := certToBeRevoked.SerialNumber.Int64()
		id := fmt.Sprintf("%x", serial)

		if m.store != nil {
			cert, _ := m.store.GetPKICert(context.Background(), id)
			if cert == nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "cert not found"})
			}
			if acc != nil {
				accID, ok := cert.Info["AccountID"]
				if !ok || acc.ID != accID {
					return c.JSON(http.StatusBadRequest, map[string]string{"error": "not certificate owner"})
				}
			} else {
				if _, err := jws.Verify(certToBeRevoked.PublicKey); err != nil {
					return c.JSON(http.StatusBadRequest, err)
				}
			}
			_ = m.RevokeCert(id)
		}
		return c.NoContent(http.StatusOK)
	})

	certPair, err := tls.X509KeyPair([]byte(m.conf.AcmeServerCert), []byte(m.conf.AcmeServerKey))
	if err != nil {
		slog.Error("PKI ACME parse keypair error", "error", err)
		return
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: e,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{certPair},
			MinVersion:   tls.VersionTLS12,
		},
	}

	if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		m.mu.Lock()
		m.lastAcmeServerErr = err
		m.acmeServerRunning = false
		m.mu.Unlock()
		slog.Error("PKI ACME server failed to start", "port", port, "error", err)
	}
}

func (m *Manager) addACMEHeader(c echo.Context) {
	baseURL := strings.TrimRight(m.conf.AcmeBaseURL, "/")
	c.Response().Header().Add("Replay-Nonce", createNonce())
	c.Response().Header().Add("Link", baseURL+"/directory/index")
	c.Response().Header().Add(echo.HeaderCacheControl, "no-store")
}

func getJWS(c echo.Context, baseURL string) (*jose.JSONWebSignature, error) {
	b, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return nil, err
	}
	jws, err := jose.ParseSigned(string(b), []jose.SignatureAlgorithm{jose.ES256, jose.ES384, jose.RS256})
	if err != nil {
		return nil, err
	}
	if err := checkJWS(jws, baseURL); err != nil {
		return nil, err
	}
	return jws, nil
}

func checkJWS(jws *jose.JSONWebSignature, baseURL string) error {
	if len(jws.Signatures) == 0 {
		return fmt.Errorf("request body does not contain a signature")
	}
	if len(jws.Signatures) > 1 {
		return fmt.Errorf("request body contains more than one signature")
	}
	sig := jws.Signatures[0]
	uh := sig.Unprotected
	if uh.KeyID != "" || uh.JSONWebKey != nil || uh.Algorithm != "" || uh.Nonce != "" || len(uh.ExtraHeaders) > 0 {
		return fmt.Errorf("unprotected header must not be used")
	}
	hdr := sig.Protected
	if hdr.Nonce == "" {
		return fmt.Errorf("invalid nonce")
	}
	if jwsURL, ok := hdr.ExtraHeaders["url"].(string); !ok || !strings.HasPrefix(jwsURL, baseURL) {
		return fmt.Errorf("jws missing url protected header")
	}
	if hdr.JSONWebKey != nil && hdr.KeyID != "" {
		return fmt.Errorf("jwk and kid are mutually exclusive")
	}
	if hdr.JSONWebKey == nil && hdr.KeyID == "" {
		return fmt.Errorf("either jwk or kid must be defined in jws protected header")
	}
	return nil
}

func extractJWK(jws *jose.JSONWebSignature) *jose.JSONWebKey {
	jwk := jws.Signatures[0].Protected.JSONWebKey
	if jwk == nil || !jwk.Valid() {
		return nil
	}
	kid, err := keyToID(jwk)
	if err == nil {
		jwk.KeyID = kid
	}
	return jwk
}

func lookupJWKAndAccount(jws *jose.JSONWebSignature) (*account, error) {
	kid := jws.Signatures[0].Protected.KeyID
	if kid == "" {
		return nil, fmt.Errorf("jws missing keyid")
	}
	kid = path.Base(kid)
	acmeMapMu.RLock()
	acc, ok := acmeAccMap[kid]
	acmeMapMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no account for %s", kid)
	}
	return acc, nil
}

func keyToID(jwk *jose.JSONWebKey) (string, error) {
	kid, err := jwk.Thumbprint(crypto.SHA256)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(kid), nil
}

func canExtractJWKFrom(jws *jose.JSONWebSignature) bool {
	if jws == nil || len(jws.Signatures) == 0 {
		return false
	}
	return jws.Signatures[0].Protected.JSONWebKey != nil
}

func createNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func http01Validate(acc *account, ch *challenge) error {
	url := fmt.Sprintf("http://%s/.well-known/acme-challenge/%s", ch.Value, ch.Token)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	expected, err := keyAuthorization(ch.Token, acc.Key)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(b)) != expected {
		return fmt.Errorf("key auth mismatch")
	}
	return nil
}

func tlsalpn01Validate(acc *account, ch *challenge) error {
	config := &tls.Config{
		NextProtos:         []string{"acme-tls/1"},
		MinVersion:         tls.VersionTLS12,
		ServerName:         serverName(ch),
		InsecureSkipVerify: true,
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(ch.Value, "443"), config)
	if err != nil {
		return err
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return fmt.Errorf("peer cert not found")
	}
	leaf := certs[0]

	idPeAcmeIdentifier := asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 31}
	keyAuth, err := keyAuthorization(ch.Token, acc.Key)
	if err != nil {
		return err
	}
	hashedKeyAuth := sha256.Sum256([]byte(keyAuth))

	for _, ext := range leaf.Extensions {
		if idPeAcmeIdentifier.Equal(ext.Id) {
			var extValue []byte
			rest, err := asn1.Unmarshal(ext.Value, &extValue)
			if err != nil || len(rest) > 0 || len(hashedKeyAuth) != len(extValue) {
				return fmt.Errorf("malformed acme extension")
			}
			if subtle.ConstantTimeCompare(hashedKeyAuth[:], extValue) != 1 {
				return fmt.Errorf("key auth extension mismatch")
			}
			return nil
		}
	}
	return fmt.Errorf("acmeValidationV1 extension not found")
}

func dns01Validate(acc *account, ch *challenge) error {
	domain := strings.TrimPrefix(ch.Value, "*.")
	records, err := net.LookupTXT("_acme-challenge." + domain)
	if err != nil {
		return err
	}
	expectedKeyAuth, err := keyAuthorization(ch.Token, acc.Key)
	if err != nil {
		return err
	}
	h := sha256.Sum256([]byte(expectedKeyAuth))
	expected := base64.RawURLEncoding.EncodeToString(h[:])
	for _, r := range records {
		if r == expected {
			return nil
		}
	}
	return fmt.Errorf("TXT record mismatch")
}

func keyAuthorization(token string, jwk *jose.JSONWebKey) (string, error) {
	thumb, err := jwk.Thumbprint(crypto.SHA256)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", token, base64.RawURLEncoding.EncodeToString(thumb)), nil
}

func serverName(ch *challenge) string {
	if ip := net.ParseIP(ch.Value); ip != nil {
		return reverseIP(ch.Value)
	}
	return ch.Value
}

func reverseIP(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ipStr
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa", ipv4[3], ipv4[2], ipv4[1], ipv4[0])
	}
	return ipStr
}
