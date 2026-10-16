package pki

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
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

	// customAttestationRoots allows overriding or extending the trusted attestation Root CAs (used in tests).
	customAttestationRoots *x509.CertPool
)

// Root certificates for device attestation
const appleEnterpriseAttestationRootCA = `-----BEGIN CERTIFICATE-----
MIICJDCCAamgAwIBAgIUQsDCuyxyfFxeq/bxpm8frF15hzcwCgYIKoZIzj0EAwMw
UTEtMCsGA1UEAwwkQXBwbGUgRW50ZXJwcmlzZSBBdHRlc3RhdGlvbiBSb290IENB
MRMwEQYDVQQKDApBcHBsZSBJbmMuMQswCQYDVQQGEwJVUzAeFw0yMjAyMTYxOTAx
MjRaFw00NzAyMjAwMDAwMDBaMFExLTArBgNVBAMMJEFwcGxlIEVudGVycHJpc2Ug
QXR0ZXN0YXRpb24gUm9vdCBDQTETMBEGA1UECgwKQXBwbGUgSW5jLjELMAkGA1UE
BhMCVVMwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAAT6Jigq+Ps9Q4CoT8t8q+UnOe2p
oT9nRaUfGhBTbgvqSGXPjVkbYlIWYO+1zPk2Sz9hQ5ozzmLrPmTBgEWRcHjA2/y7
7GEicps9wn2tj+G89l3INNDKETdxSPPIZpPj8VmjQjBAMA8GA1UdEwEB/wQFMAMB
Af8wHQYDVR0OBBYEFPNqTQGd8muBpV5du+UIbVbi+d66MA4GA1UdDwEB/wQEAwIB
BjAKBggqhkjOPQQDAwNpADBmAjEA1xpWmTLSpr1VH4f8Ypk8f3jMUKYz4QPG8mL5
8m9sX/b2+eXpTv2pH4RZgJjucnbcAjEA4ZSB6S45FlPuS/u4pTnzoz632rA+xW/T
ZwFEh9bhKjJ+5VQ9/Do1os0u3LEkgN/r
-----END CERTIFICATE-----`

const yubicoPIVRootCA = `-----BEGIN CERTIFICATE-----
MIIDFzCCAf+gAwIBAgIDBAZHMA0GCSqGSIb3DQEBCwUAMCsxKTAnBgNVBAMMIFl1
YmljbyBQSVYgUm9vdCBDQSBTZXJpYWwgMjYzNzUxMCAXDTE2MDMxNDAwMDAwMFoY
DzIwNTIwNDE3MDAwMDAwWjArMSkwJwYDVQQDDCBZdWJpY28gUElWIFJvb3QgQ0Eg
U2VyaWFsIDI2Mzc1MTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMN2
cMTNR6YCdcTFRxuPy31PabRn5m6pJ+nSE0HRWpoaM8fc8wHC+Tmb98jmNvhWNE2E
ilU85uYKfEFP9d6Q2GmytqBnxZsAa3KqZiCCx2LwQ4iYEOb1llgotVr/whEpdVOq
joU0P5e1j1y7OfwOvky/+AXIN/9Xp0VFlYRk2tQ9GcdYKDmqU+db9iKwpAzid4oH
BVLIhmD3pvkWaRA2H3DA9t7H/HNq5v3OiO1jyLZeKqZoMbPObrxqDg+9fOdShzgf
wCqgT3XVmTeiwvBSTctyi9mHQfYd2DwkaqxRnLbNVyK9zl+DzjSGp9IhVPiVtGet
X02dxhQnGS7K6BO0Qe8CAwEAAaNCMEAwHQYDVR0OBBYEFMpfyvLEojGc6SJf8ez0
1d8Cv4O/MA8GA1UdEwQIMAYBAf8CAQEwDgYDVR0PAQH/BAQDAgEGMA0GCSqGSIb3
DQEBCwUAA4IBAQBc7Ih8Bc1fkC+FyN1fhjWioBCMr3vjneh7MLbA6kSoyWF70N3s
XhbXvT4eRh0hvxqvMZNjPU/VlRn6gLVtoEikDLrYFXN6Hh6Wmyy1GTnspnOvMvz2
lLKuym9KYdYLDgnj3BeAvzIhVzzYSeU77/Cupofj093OuAswW0jYvXsGTyix6B3d
bW5yWvyS9zNXaqGaUmP3U9/b6DlHdDogMLu3VLpBB9bm5bjaKWWJYgWltCVgUbFq
Fqyi4+JE014cSgR57Jcu3dZiehB6UtAPgad9L5cNvua/IWRmm+ANy3O2LH++Pyl8
SREzU8onbBsjMg9QDiSf5oJLKvd/Ren+zGY7
-----END CERTIFICATE-----`

const yubicoAttestationRootCA = `-----BEGIN CERTIFICATE-----
MIIDPjCCAiagAwIBAgIUXzeiEDJEOTt14F5n0o6Zf/bBwiUwDQYJKoZIhvcNAQEN
BQAwJDEiMCAGA1UEAwwZWXViaWNvIEF0dGVzdGF0aW9uIFJvb3QgMTAgFw0yNDEy
MDEwMDAwMDBaGA85OTk5MTIzMTIzNTk1OVowJDEiMCAGA1UEAwwZWXViaWNvIEF0
dGVzdGF0aW9uIFJvb3QgMTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AMZ6/TxM8rIT+EaoPvG81ontMOo/2mQ2RBwJHS0QZcxVaNXvl12LUhBZ5LmiBScI
Zd1Rnx1od585h+/dhK7hEm7JAALkKKts1fO53KGNLZujz5h3wGncr4hyKF0G74b/
U3K9hE5mGND6zqYchCRAHfrYMYRDF4YL0X4D5nGdxvppAy6nkEmtWmMnwO3i0TAu
csrbE485HvGM4r0VpgVdJpvgQjiTJCTIq+D35hwtT8QDIv+nGvpcyi5wcIfCkzyC
imJukhYy6KoqNMKQEdpNiSOvWyDMTMt1bwCvEzpw91u+msUt4rj0efnO9s0ZOwdw
MRDnH4xgUl5ZLwrrPkfC1/0CAwEAAaNmMGQwHQYDVR0OBBYEFNLu71oijTptXCOX
PfKF1SbxJXuSMB8GA1UdIwQYMBaAFNLu71oijTptXCOXPfKF1SbxJXuSMBIGA1Ud
EwEB/wQIMAYBAf8CAQMwDgYDVR0PAQH/BAQDAgGGMA0GCSqGSIb3DQEBDQUAA4IB
AQC3IW/sgB9pZ8apJNjxuGoX+FkILks0wMNrdXL/coUvsrhzsvl6mePMrbGJByJ1
XnquB5sgcRENFxdQFma3mio8Upf1owM1ZreXrJ0mADG2BplqbJnxiyYa+R11reIF
TWeIhMNcZKsDZrFAyPuFjCWSQvJmNWe9mFRYFgNhXJKkXIb5H1XgEDlwiedYRM7V
olBNlld6pRFKlX8ust6OTMOeADl2xNF0m1LThSdeuXvDyC1g9+ILfz3S6OIYgc3i
roRcFD354g7rKfu67qFAw9gC4yi0xBTPrY95rh4/HqaUYCA/L8ldRk6H7Xk35D+W
Vpmq2Sh/xT5HiFuhf4wJb0bK
-----END CERTIFICATE-----`

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

	if block, _ := pem.Decode([]byte(m.conf.AcmeServerCert)); block != nil {
		if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
			if cert.NotAfter.Sub(cert.NotBefore) > 825*24*time.Hour {
				if err := m.createAcmeServerCertificateLocked(); err == nil {
					if m.store != nil {
						_ = m.store.SavePKIConf(context.Background(), &m.conf)
					}
				}
			}
		}
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
	m.registerAcmeRoutes(m.acmeServer)

	port := m.conf.AcmePort
	tlsCert, err := tls.X509KeyPair([]byte(m.conf.AcmeServerCert), []byte(m.conf.AcmeServerKey))
	if err != nil {
		m.acmeServerRunning = false
		m.lastAcmeServerErr = err
		slog.Error("PKI ACME parse keypair error", "error", err)
		return
	}

	if m.store != nil {
		_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "pki",
			Level: "info",
			Event: fmt.Sprintf(i18n.Trans("Started ACME server on port %d"), port),
		})
	}
	go m.acmeServerFunc(m.acmeServer, port, tlsCert)
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
	termDays := m.conf.RootCATerm * 365
	if termDays < 1 || termDays > 820 {
		termDays = 820
	}
	tmp := &x509.Certificate{
		SerialNumber: big.NewInt(sn),
		Subject: pkix.Name{
			CommonName: m.conf.Name + " ACME Server",
		},
		NotBefore:             time.Now().UTC(),
		NotAfter:              time.Now().AddDate(0, 0, termDays).UTC(),
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
	NotBefore   string       `json:"notBefore,omitempty"`
	NotAfter    string       `json:"notAfter,omitempty"`
}

type subproblem struct {
	Type       string     `json:"type"`
	Detail     string     `json:"detail"`
	Identifier identifier `json:"identifier"`
}

type acmeError struct {
	Type        string        `json:"type"`
	Detail      string        `json:"detail"`
	Status      int           `json:"status,omitempty"`
	Subproblems []*subproblem `json:"subproblems,omitempty"`
}

func (e *acmeError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Detail)
}

func sendACMEErr(c echo.Context, status int, err any) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/problem+json")
	if aErr, ok := err.(*acmeError); ok {
		if aErr.Status == 0 {
			aErr.Status = status
		}
		slog.Debug("ACME error response", "status", status, "type", aErr.Type, "detail", aErr.Detail)
		return c.JSON(status, aErr)
	}
	if strErr, ok := err.(string); ok {
		aErr := &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: strErr,
			Status: status,
		}
		slog.Debug("ACME error response", "status", status, "type", aErr.Type, "detail", aErr.Detail)
		return c.JSON(status, aErr)
	}
	if gErr, ok := err.(error); ok {
		aErr := &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: gErr.Error(),
			Status: status,
		}
		slog.Debug("ACME error response", "status", status, "type", aErr.Type, "detail", aErr.Detail)
		return c.JSON(status, aErr)
	}
	if mErr, ok := err.(map[string]string); ok {
		detail := mErr["error"]
		if detail == "" {
			detail = mErr["message"]
		}
		aErr := &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: detail,
			Status: status,
		}
		slog.Debug("ACME error response", "status", status, "type", aErr.Type, "detail", aErr.Detail)
		return c.JSON(status, aErr)
	}
	aErr := &acmeError{
		Type:   "urn:ietf:params:acme:error:serverInternal",
		Detail: fmt.Sprintf("%v", err),
		Status: status,
	}
	slog.Debug("ACME error response", "status", status, "type", aErr.Type, "detail", aErr.Detail)
	return c.JSON(status, aErr)
}

func (m *Manager) getAcmeBaseURL(c echo.Context) string {
	if c != nil && c.Request() != nil && c.Request().Host != "" {
		scheme := "https"
		if c.Request().TLS == nil {
			scheme = "http"
		}
		return fmt.Sprintf("%s://%s", scheme, c.Request().Host)
	}
	return strings.TrimRight(m.conf.AcmeBaseURL, "/")
}

type order struct {
	ID                string       `json:"-"`
	AccountID         string       `json:"-"`
	Status            string       `json:"status"`
	ExpiresAt         time.Time    `json:"expires"`
	Identifiers       []identifier `json:"identifiers"`
	NotBefore         string       `json:"notBefore,omitempty"`
	NotAfter          string       `json:"notAfter,omitempty"`
	AuthorizationIDs  []string     `json:"-"`
	AuthorizationURLs []string     `json:"authorizations"`
	FinalizeURL       string       `json:"finalize"`
	CertificateID     string       `json:"-"`
	CertificateURL    string       `json:"certificate,omitempty"`
	Certificate       string       `json:"-"`
}

type authorization struct {
	ID          string       `json:"-"`
	AccountID   string       `json:"-"`
	Identifier  identifier   `json:"identifier"`
	Status      string       `json:"status"`
	ExpiresAt   time.Time    `json:"expires"`
	Challenges  []*challenge `json:"challenges"`
	Wildcard    bool         `json:"wildcard,omitempty"`
	Token       string       `json:"-"`
	Fingerprint string       `json:"-"`
}

type challenge struct {
	ID              string     `json:"-"`
	AccountID       string     `json:"-"`
	AuthorizationID string     `json:"-"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	Error           *acmeError `json:"error,omitempty"`
	Token           string     `json:"token"`
	ValidatedAt     string     `json:"validated,omitempty"`
	URL             string     `json:"url"`
	Value           string     `json:"-"`
}

type finalizeRequest struct {
	CSR string `json:"csr"`
}

type revokeReqest struct {
	Certificate string `json:"certificate"`
	ReasonCode  *int   `json:"reason,omitempty"`
}

func (m *Manager) registerAcmeRoutes(e *echo.Echo) {
	e.Use(middleware.Recover())

	e.GET("/directory", func(c echo.Context) error {
		baseURL := m.getAcmeBaseURL(c)
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
		baseURL := m.getAcmeBaseURL(c)
		m.addACMEHeader(c)
		jws, err := getJWS(c, baseURL)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		jwk := extractJWK(jws)
		if jwk == nil {
			return sendACMEErr(c, http.StatusBadRequest, "jwk not found")
		}
		payload, err := jws.Verify(jwk)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		var nar newAccountRequest
		if err := json.Unmarshal(payload, &nar); err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}

		acmeMapMu.Lock()
		defer acmeMapMu.Unlock()

		if nar.OnlyReturnExisting {
			if existing, ok := acmeAccMap[jwk.KeyID]; ok {
				c.Response().Header().Add("Location", baseURL+"/account/"+existing.ID)
				return c.JSON(http.StatusOK, existing)
			}
			return sendACMEErr(c, http.StatusBadRequest, &acmeError{
				Type:   "urn:ietf:params:acme:error:accountDoesNotExist",
				Detail: "account does not exist",
			})
		}
		if existing, ok := acmeAccMap[jwk.KeyID]; ok {
			if len(nar.Contact) > 0 {
				existing.Contact = nar.Contact
			}
			c.Response().Header().Add("Location", baseURL+"/account/"+existing.ID)
			return c.JSON(http.StatusOK, existing)
		}

		acc := &account{
			ID:        jwk.KeyID,
			Key:       jwk,
			Status:    "valid",
			Contact:   nar.Contact,
			OrdersURL: baseURL + "/account/" + jwk.KeyID + "/orders",
		}
		acmeAccMap[acc.ID] = acc
		c.Response().Header().Add("Location", baseURL+"/account/"+acc.ID)
		return c.JSON(http.StatusCreated, acc)
	})

	e.POST("/account/:id", func(c echo.Context) error {
		baseURL := m.getAcmeBaseURL(c)
		m.addACMEHeader(c)
		jws, err := getJWS(c, baseURL)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		id := c.Param("id")
		if acc.ID != id {
			return sendACMEErr(c, http.StatusBadRequest, fmt.Sprintf("account id mismatch %s!=%s", acc.ID, id))
		}
		payload, err := jws.Verify(acc.Key)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		if len(payload) > 0 && string(payload) != "{}" {
			var req struct {
				Status  string   `json:"status,omitempty"`
				Contact []string `json:"contact,omitempty"`
			}
			if err := json.Unmarshal(payload, &req); err == nil {
				acmeMapMu.Lock()
				if req.Status == "deactivated" {
					acc.Status = "deactivated"
				}
				if req.Contact != nil {
					acc.Contact = req.Contact
				}
				acmeMapMu.Unlock()
			}
		}
		c.Response().Header().Add("Location", baseURL+"/account/"+acc.ID)
		return c.JSON(http.StatusOK, acc)
	})

	e.POST("/account/:id/orders", func(c echo.Context) error {
		baseURL := m.getAcmeBaseURL(c)
		m.addACMEHeader(c)
		jws, err := getJWS(c, baseURL)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		id := c.Param("id")
		if acc.ID != id {
			return sendACMEErr(c, http.StatusBadRequest, fmt.Sprintf("account id mismatch %s!=%s", acc.ID, id))
		}
		if _, err := jws.Verify(acc.Key); err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		orders := []string{}
		acmeMapMu.RLock()
		for _, o := range acmeOrderMap {
			if o.AccountID == acc.ID {
				orders = append(orders, baseURL+"/order/"+o.ID)
			}
		}
		acmeMapMu.RUnlock()
		return c.JSON(http.StatusOK, map[string][]string{"orders": orders})
	})

	e.POST("/new-order", func(c echo.Context) error {
		baseURL := m.getAcmeBaseURL(c)
		m.addACMEHeader(c)
		jws, err := getJWS(c, baseURL)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		payload, err := jws.Verify(acc.Key)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		var nor newOrderRequest
		if err := json.Unmarshal(payload, &nor); err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
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
				if net.ParseIP(iden.Value) == nil {
					return sendACMEErr(c, http.StatusBadRequest, &acmeError{
						Type:   "urn:ietf:params:acme:error:malformed",
						Detail: fmt.Sprintf("invalid IP address: %s", iden.Value),
					})
				}
				chTypes = append(chTypes, "http-01", "tls-alpn-01")
			case "dns":
				chTypes = append(chTypes, "dns-01")
				if !strings.HasPrefix(iden.Value, "*.") {
					chTypes = append(chTypes, "http-01", "tls-alpn-01")
				}
			case "permanent-identifier", "hardware-module":
				chTypes = append(chTypes, "device-attest-01")
			default:
				return sendACMEErr(c, http.StatusBadRequest, &acmeError{
					Type:   "urn:ietf:params:acme:error:unsupportedIdentifier",
					Detail: fmt.Sprintf("unsupported identifier type: %s", iden.Type),
				})
			}
			az := &authorization{
				ID:         azID,
				AccountID:  acc.ID,
				Identifier: iden,
				Wildcard:   iden.Type == "dns" && strings.HasPrefix(iden.Value, "*."),
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

	e.POST("/authz/:id", func(c echo.Context) error {
		baseURL := m.getAcmeBaseURL(c)
		m.addACMEHeader(c)
		jws, err := getJWS(c, baseURL)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		id := c.Param("id")
		acmeMapMu.RLock()
		az, ok := acmeAuthzMap[id]
		acmeMapMu.RUnlock()
		if !ok || az.AccountID != acc.ID {
			return sendACMEErr(c, http.StatusBadRequest, "authorization not found")
		}
		if _, err := jws.Verify(acc.Key); err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		c.Response().Header().Add("Location", baseURL+"/authz/"+az.ID)
		return c.JSON(http.StatusOK, az)
	})

	e.POST("/challenge/:azID/:chID", func(c echo.Context) error {
		baseURL := m.getAcmeBaseURL(c)
		m.addACMEHeader(c)
		jws, err := getJWS(c, baseURL)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		azID := c.Param("azID")
		chID := c.Param("chID")
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}

		acmeMapMu.RLock()
		az, ok := acmeAuthzMap[azID]
		acmeMapMu.RUnlock()
		if !ok || az.AccountID != acc.ID {
			return sendACMEErr(c, http.StatusBadRequest, "authorization not found")
		}

		var ch *challenge
		for _, cand := range az.Challenges {
			if cand.ID == chID {
				ch = cand
				break
			}
		}
		if ch == nil {
			return sendACMEErr(c, http.StatusBadRequest, "challenge not found")
		}

		payload, err := jws.Verify(acc.Key)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}

		switch ch.Type {
		case "device-attest-01":
			if err := m.deviceAttest01Validate(acc, az, ch, payload); err != nil {
				ch.Status = "invalid"
				ch.Error = &acmeError{
					Type:   "urn:ietf:params:acme:error:unauthorized",
					Detail: err.Error(),
					Status: http.StatusBadRequest,
				}
				updateOrdersForAuthorization(az.ID)
				return sendACMEErr(c, http.StatusBadRequest, ch.Error)
			}
		case "http-01":
			if err := http01Validate(acc, ch); err != nil {
				ch.Status = "invalid"
				ch.Error = &acmeError{
					Type:   "urn:ietf:params:acme:error:connection",
					Detail: err.Error(),
					Status: http.StatusBadRequest,
				}
				updateOrdersForAuthorization(az.ID)
				return sendACMEErr(c, http.StatusBadRequest, ch.Error)
			}
			ch.Status = "valid"
			ch.Error = nil
			ch.ValidatedAt = time.Now().Truncate(time.Second).Format(time.RFC3339)
		case "tls-alpn-01":
			if err := tlsalpn01Validate(acc, ch); err != nil {
				ch.Status = "invalid"
				ch.Error = &acmeError{
					Type:   "urn:ietf:params:acme:error:connection",
					Detail: err.Error(),
					Status: http.StatusBadRequest,
				}
				updateOrdersForAuthorization(az.ID)
				return sendACMEErr(c, http.StatusBadRequest, ch.Error)
			}
			ch.Status = "valid"
			ch.Error = nil
			ch.ValidatedAt = time.Now().Truncate(time.Second).Format(time.RFC3339)
		case "dns-01":
			if err := dns01Validate(acc, ch); err != nil {
				ch.Status = "invalid"
				ch.Error = &acmeError{
					Type:   "urn:ietf:params:acme:error:dns",
					Detail: err.Error(),
					Status: http.StatusBadRequest,
				}
				updateOrdersForAuthorization(az.ID)
				return sendACMEErr(c, http.StatusBadRequest, ch.Error)
			}
			ch.Status = "valid"
			ch.Error = nil
			ch.ValidatedAt = time.Now().Truncate(time.Second).Format(time.RFC3339)
		default:
			return sendACMEErr(c, http.StatusBadRequest, "unsupported challenge type")
		}

		acmeMapMu.Lock()
		az.Status = "valid"
		updateOrdersForAuthorization(az.ID)
		acmeMapMu.Unlock()

		c.Response().Header().Add("Location", baseURL+"/challenge/"+az.ID+"/"+chID)
		c.Response().Header().Add("Link", fmt.Sprintf("<%s/authz/%s>;rel=up", baseURL, az.ID))
		return c.JSON(http.StatusOK, ch)
	})

	e.POST("/order/:id", func(c echo.Context) error {
		baseURL := m.getAcmeBaseURL(c)
		m.addACMEHeader(c)
		jws, err := getJWS(c, baseURL)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		id := c.Param("id")
		acmeMapMu.RLock()
		o, ok := acmeOrderMap[id]
		acmeMapMu.RUnlock()
		if !ok || o.AccountID != acc.ID {
			return sendACMEErr(c, http.StatusBadRequest, "order not found")
		}
		if _, err := jws.Verify(acc.Key); err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		c.Response().Header().Add("Location", baseURL+"/order/"+o.ID)
		return c.JSON(http.StatusOK, o)
	})

	e.POST("/order/:id/finalize", func(c echo.Context) error {
		baseURL := m.getAcmeBaseURL(c)
		m.addACMEHeader(c)
		jws, err := getJWS(c, baseURL)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		id := c.Param("id")
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}

		acmeMapMu.RLock()
		o, ok := acmeOrderMap[id]
		acmeMapMu.RUnlock()
		if !ok || o.AccountID != acc.ID {
			return sendACMEErr(c, http.StatusBadRequest, "order not found")
		}

		payload, err := jws.Verify(acc.Key)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		var fr finalizeRequest
		if err := json.Unmarshal(payload, &fr); err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}

		csrBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(fr.CSR, "="))
		if err != nil {
			csrBytes, err = base64.StdEncoding.DecodeString(fr.CSR)
			if err != nil {
				return sendACMEErr(c, http.StatusBadRequest, err)
			}
		}

		info := map[string]string{
			"AccountID":  o.AccountID,
			"OrderID":    o.ID,
			"RemoteAddr": c.RealIP(),
		}
		for _, iden := range o.Identifiers {
			if iden.Type == "permanent-identifier" || iden.Type == "hardware-module" {
				info["PermanentIdentifier"] = iden.Value
				break
			}
		}

		m.mu.Lock()
		cert, certID, err := m.createCertificateFromCSR(csrBytes, "acme", info)
		rootCertPEM := m.conf.RootCACert
		m.mu.Unlock()

		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
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
		baseURL := m.getAcmeBaseURL(c)
		m.addACMEHeader(c)
		jws, err := getJWS(c, baseURL)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		id := c.Param("id")
		acc, err := lookupJWKAndAccount(jws)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}

		acmeMapMu.RLock()
		cert, ok := acmeCertMap[id]
		acmeMapMu.RUnlock()
		if !ok || cert.AccountID != acc.ID {
			return sendACMEErr(c, http.StatusBadRequest, "certificate not found")
		}
		return c.Blob(http.StatusOK, "application/pem-certificate-chain", cert.Certificate)
	})

	e.POST("/revoke-cert", func(c echo.Context) error {
		baseURL := m.getAcmeBaseURL(c)
		m.addACMEHeader(c)
		jws, err := getJWS(c, baseURL)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		var jwk *jose.JSONWebKey
		var acc *account
		if canExtractJWKFrom(jws) {
			jwk = extractJWK(jws)
			if jwk == nil {
				return sendACMEErr(c, http.StatusBadRequest, "missing jwk")
			}
		} else {
			acc, err = lookupJWKAndAccount(jws)
			if err != nil {
				return sendACMEErr(c, http.StatusBadRequest, err)
			}
			jwk = acc.Key
		}
		payload, err := jws.Verify(jwk)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		var rvr revokeReqest
		if err := json.Unmarshal(payload, &rvr); err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}

		certBytes, err := base64.RawURLEncoding.DecodeString(rvr.Certificate)
		if err != nil {
			certBytes, err = base64.StdEncoding.DecodeString(rvr.Certificate)
			if err != nil {
				return sendACMEErr(c, http.StatusBadRequest, err)
			}
		}
		certToBeRevoked, err := x509.ParseCertificate(certBytes)
		if err != nil {
			return sendACMEErr(c, http.StatusBadRequest, err)
		}
		serial := certToBeRevoked.SerialNumber.Int64()
		id := fmt.Sprintf("%x", serial)

		if m.store != nil {
			cert, _ := m.store.GetPKICert(context.Background(), id)
			if cert == nil {
				return sendACMEErr(c, http.StatusBadRequest, "cert not found")
			}
			if acc != nil {
				accID, ok := cert.Info["AccountID"]
				if !ok || acc.ID != accID {
					return sendACMEErr(c, http.StatusBadRequest, "not certificate owner")
				}
			} else {
				if _, err := jws.Verify(certToBeRevoked.PublicKey); err != nil {
					return sendACMEErr(c, http.StatusBadRequest, err)
				}
			}
			_ = m.RevokeCert(id)
		}
		return c.NoContent(http.StatusOK)
	})

	m.registerESTRoutes(e)
}

func (m *Manager) acmeServerFunc(e *echo.Echo, port int, tlsCert tls.Certificate) {
	s := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			ClientAuth:   tls.RequestClientCert,
			MinVersion:   tls.VersionTLS12,
		},
	}

	if err := e.StartServer(s); err != nil && err != http.ErrServerClosed {
		m.mu.Lock()
		m.lastAcmeServerErr = err
		m.acmeServerRunning = false
		m.mu.Unlock()
		slog.Error("PKI ACME server failed to start", "port", port, "error", err)
	}
}

func updateOrderStatus(o *order) {
	allValid := true
	for _, azID := range o.AuthorizationIDs {
		az, ok := acmeAuthzMap[azID]
		if !ok || az.Status != "valid" {
			allValid = false
			break
		}
	}
	if allValid {
		o.Status = "ready"
	}
}

func updateOrdersForAuthorization(azID string) {
	for _, o := range acmeOrderMap {
		for _, id := range o.AuthorizationIDs {
			if id == azID {
				updateOrderStatus(o)
				break
			}
		}
	}
}

func (m *Manager) addACMEHeader(c echo.Context) {
	baseURL := m.getAcmeBaseURL(c)
	c.Response().Header().Add("Replay-Nonce", createNonce())
	c.Response().Header().Add("Link", fmt.Sprintf("<%s/directory>;rel=\"index\"", baseURL))
	c.Response().Header().Add(echo.HeaderCacheControl, "no-store")
}

func getJWS(c echo.Context, baseURL string) (*jose.JSONWebSignature, error) {
	b, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return nil, &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: err.Error(),
		}
	}
	jws, err := jose.ParseSigned(string(b), []jose.SignatureAlgorithm{jose.ES256, jose.ES384, jose.RS256})
	if err != nil {
		return nil, &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: fmt.Sprintf("failed to parse signed JWS: %v", err),
		}
	}
	var reqURL *url.URL
	if c.Request() != nil {
		reqURL = c.Request().URL
	}
	if err := checkJWS(jws, baseURL, reqURL); err != nil {
		return nil, err
	}
	return jws, nil
}

func createNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	var s string
	for _, v := range b {
		s += string(v%byte(94) + 33)
	}
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func http01ChallengeHost(value string) string {
	if ip := net.ParseIP(value); ip != nil {
		if ip.To4() == nil {
			return "[" + value + "]"
		}
		return value
	}
	return value
}

func http01Validate(acc *account, ch *challenge) error {
	u := &url.URL{Scheme: "http", Host: http01ChallengeHost(ch.Value), Path: fmt.Sprintf("/.well-known/acme-challenge/%s", ch.Token)}
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	r, err := client.Get(u.String())
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		return fmt.Errorf("http get %s status=%d", u.String(), r.StatusCode)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	keyAuth := strings.TrimSpace(string(body))
	expected, err := keyAuthorization(ch.Token, acc.Key)
	if err != nil {
		return err
	}
	if keyAuth != expected {
		return fmt.Errorf("http get %s != %s", keyAuth, expected)
	}
	return nil
}

func serverName(ch *challenge) string {
	if ip := net.ParseIP(ch.Value); ip != nil {
		return reverseAddr(ip)
	}
	return ch.Value
}

func reverseAddr(ip net.IP) (arpa string) {
	if ip.To4() != nil {
		return uitoa(uint(ip[15])) + "." + uitoa(uint(ip[14])) + "." + uitoa(uint(ip[13])) + "." + uitoa(uint(ip[12])) + ".in-addr.arpa."
	}
	// IPv6
	buf := make([]byte, 0, len(ip)*4+len("ip6.arpa."))
	for i := len(ip) - 1; i >= 0; i-- {
		v := ip[i]
		buf = append(buf, hexit[v&0xF], '.', hexit[v>>4], '.')
	}
	buf = append(buf, "ip6.arpa."...)
	return string(buf)
}

const hexit = "0123456789abcdef"

func uitoa(val uint) string {
	if val == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf) - 1
	for val >= 10 {
		v := val / 10
		buf[i] = byte('0' + val - v*10)
		i--
		val = v
	}
	buf[i] = byte('0' + val)
	return string(buf[i:])
}

func tlsalpn01Validate(acc *account, ch *challenge) error {
	config := &tls.Config{
		NextProtos:         []string{"acme-tls/1"},
		MinVersion:         tls.VersionTLS12,
		ServerName:         serverName(ch),
		InsecureSkipVerify: true,
	}
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}
	hostPort := net.JoinHostPort(ch.Value, "443")

	conn, err := tls.DialWithDialer(dialer, "tcp", hostPort, config)
	if err != nil {
		return err
	}
	defer conn.Close()

	cs := conn.ConnectionState()
	certs := cs.PeerCertificates

	if len(certs) == 0 {
		return fmt.Errorf("certificate not found for %s", hostPort)
	}
	if cs.NegotiatedProtocol != "acme-tls/1" {
		return fmt.Errorf("cannot negotiate ALPN acme-tls/1 protocol for tls-alpn-01 challenge")
	}

	leafCert := certs[0]
	if len(leafCert.DNSNames) == 0 {
		if len(leafCert.IPAddresses) != 1 || !leafCert.IPAddresses[0].Equal(net.ParseIP(ch.Value)) {
			return fmt.Errorf("incorrect certificate for tls-alpn-01 challenge: leaf certificate must contain a single IP address or DNS name, %v", ch.Value)
		}
	} else {
		if len(leafCert.DNSNames) != 1 || !strings.EqualFold(leafCert.DNSNames[0], ch.Value) {
			return fmt.Errorf("incorrect certificate for tls-alpn-01 challenge: leaf certificate must contain a single IP address or DNS name, %v", ch.Value)
		}
	}

	idPeAcmeIdentifier := asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 31}
	idPeAcmeIdentifierV1Obsolete := asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 30, 1}
	foundIDPeAcmeIdentifierV1Obsolete := false

	keyAuth, err := keyAuthorization(ch.Token, acc.Key)
	if err != nil {
		return err
	}
	hashedKeyAuth := sha256.Sum256([]byte(keyAuth))

	for _, ext := range leafCert.Extensions {
		if idPeAcmeIdentifier.Equal(ext.Id) {
			if !ext.Critical {
				return fmt.Errorf("incorrect certificate for tls-alpn-01 challenge: acmeValidationV1 extension not critical")
			}
			var extValue []byte
			rest, err := asn1.Unmarshal(ext.Value, &extValue)
			if err != nil || len(rest) > 0 || len(hashedKeyAuth) != len(extValue) {
				return fmt.Errorf("incorrect certificate for tls-alpn-01 challenge: malformed acmeValidationV1 extension value")
			}
			if subtle.ConstantTimeCompare(hashedKeyAuth[:], extValue) != 1 {
				return fmt.Errorf("incorrect certificate for tls-alpn-01 challenge: expected acmeValidationV1 extension value %s for this challenge but got %s",
					hex.EncodeToString(hashedKeyAuth[:]), hex.EncodeToString(extValue))
			}
			return nil
		}
		if idPeAcmeIdentifierV1Obsolete.Equal(ext.Id) {
			foundIDPeAcmeIdentifierV1Obsolete = true
		}
	}

	if foundIDPeAcmeIdentifierV1Obsolete {
		return fmt.Errorf("incorrect certificate for tls-alpn-01 challenge: obsolete id-pe-acmeIdentifier in acmeValidationV1 extension")
	}
	return fmt.Errorf("incorrect certificate for tls-alpn-01 challenge: missing acmeValidationV1 extension")
}

func dns01Validate(acc *account, ch *challenge) error {
	domain := strings.TrimPrefix(ch.Value, "*.")
	txtRecords, err := net.LookupTXT("_acme-challenge." + domain)
	if err != nil {
		return fmt.Errorf("error looking up TXT records for domain %s: %w", domain, err)
	}

	expectedKeyAuth, err := keyAuthorization(ch.Token, acc.Key)
	if err != nil {
		return err
	}
	h := sha256.Sum256([]byte(expectedKeyAuth))
	expected := base64.RawURLEncoding.EncodeToString(h[:])
	var found bool
	for _, r := range txtRecords {
		if r == expected {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("keyAuthorization does not match; expected %s, but got %s", expectedKeyAuth, txtRecords)
	}
	return nil
}

func keyAuthorization(token string, jwk *jose.JSONWebKey) (string, error) {
	thumbprint, err := jwk.Thumbprint(crypto.SHA256)
	if err != nil {
		return "", err
	}
	encPrint := base64.RawURLEncoding.EncodeToString(thumbprint)
	return fmt.Sprintf("%s.%s", token, encPrint), nil
}

func canExtractJWKFrom(jws *jose.JSONWebSignature) bool {
	if jws == nil || len(jws.Signatures) == 0 {
		return false
	}
	return jws.Signatures[0].Protected.JSONWebKey != nil
}

func checkJWS(jws *jose.JSONWebSignature, baseURL string, reqURL *url.URL) error {
	if len(jws.Signatures) == 0 {
		return &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: "request body does not contain a signature",
		}
	}
	if len(jws.Signatures) > 1 {
		return &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: "request body contains more than one signature",
		}
	}
	sig := jws.Signatures[0]
	uh := sig.Unprotected
	if uh.KeyID != "" || uh.JSONWebKey != nil || uh.Algorithm != "" || uh.Nonce != "" || len(uh.ExtraHeaders) > 0 {
		return &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: "unprotected header must not be used",
		}
	}
	hdr := sig.Protected
	if hdr.Nonce == "" {
		return &acmeError{
			Type:   "urn:ietf:params:acme:error:badNonce",
			Detail: "invalid nonce",
		}
	}
	jwsURL, ok := hdr.ExtraHeaders["url"].(string)
	if !ok || jwsURL == "" {
		return &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: "jws missing url protected header",
		}
	}
	matched := strings.HasPrefix(jwsURL, baseURL)
	if !matched && reqURL != nil {
		if parsed, err := url.Parse(jwsURL); err == nil {
			matched = (parsed.Path == reqURL.Path)
		}
	}
	if !matched {
		return &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: fmt.Sprintf("jws url header %q does not match endpoint %q", jwsURL, baseURL),
		}
	}
	if hdr.JSONWebKey != nil && hdr.KeyID != "" {
		return &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: "jwk and kid are mutually exclusive",
		}
	}
	if hdr.JSONWebKey == nil && hdr.KeyID == "" {
		return &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: "either jwk or kid must be defined in jws protected header",
		}
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
		return nil, &acmeError{
			Type:   "urn:ietf:params:acme:error:malformed",
			Detail: "jws missing keyid",
		}
	}
	kid = path.Base(kid)
	acmeMapMu.RLock()
	acc, ok := acmeAccMap[kid]
	acmeMapMu.RUnlock()
	if !ok {
		return nil, &acmeError{
			Type:   "urn:ietf:params:acme:error:accountDoesNotExist",
			Detail: fmt.Sprintf("account %s does not exist", kid),
		}
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

func publicKeyFingerprint(pub crypto.PublicKey) (string, error) {
	jwk := &jose.JSONWebKey{Key: pub}
	tp, err := jwk.Thumbprint(crypto.SHA256)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(tp), nil
}

type deviceAttestPayload struct {
	AttObj string `json:"attObj"`
}

type attestationObject struct {
	Format       string
	AttStatement map[string]any
	AuthData     []byte
}

func (m *Manager) getAttestationRootPool(defaultPEMs ...string) *x509.CertPool {
	if customAttestationRoots != nil {
		return customAttestationRoots
	}
	pool := x509.NewCertPool()
	for _, p := range defaultPEMs {
		pool.AppendCertsFromPEM([]byte(p))
	}
	m.mu.RLock()
	rootBytes := m.rootCACertificate
	m.mu.RUnlock()
	if len(rootBytes) > 0 {
		if ca, err := x509.ParseCertificate(rootBytes); err == nil {
			pool.AddCert(ca)
		}
	}
	return pool
}

func (m *Manager) deviceAttest01Validate(acc *account, az *authorization, ch *challenge, payload []byte) error {
	var p deviceAttestPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("failed to unmarshal device-attest-01 payload: %w", err)
	}
	if p.AttObj == "" {
		return fmt.Errorf("missing attObj in device-attest-01 payload")
	}
	rawAttObj, err := base64.RawURLEncoding.DecodeString(p.AttObj)
	if err != nil {
		rawAttObj, err = base64.StdEncoding.DecodeString(p.AttObj)
		if err != nil {
			return fmt.Errorf("failed to base64 decode attObj: %w", err)
		}
	}
	attObj, err := decodeCBORAttestationObject(rawAttObj)
	if err != nil {
		return fmt.Errorf("failed to decode CBOR attObj: %w", err)
	}

	var attestedPub crypto.PublicKey
	switch attObj.Format {
	case "apple":
		attestedPub, err = m.doAppleAttestationFormat(acc, az, ch, attObj)
	case "step":
		attestedPub, err = m.doStepAttestationFormat(acc, az, ch, attObj)
	default:
		return fmt.Errorf("unsupported attestation format: %s", attObj.Format)
	}
	if err != nil {
		return err
	}

	fp, err := publicKeyFingerprint(attestedPub)
	if err != nil {
		return fmt.Errorf("failed to compute attested key fingerprint: %w", err)
	}
	az.Fingerprint = fp
	ch.Status = "valid"
	ch.Error = nil
	ch.ValidatedAt = time.Now().Truncate(time.Second).Format(time.RFC3339)
	return nil
}

func extractX5CChain(attStmt map[string]any) ([]*x509.Certificate, error) {
	rawX5C, ok := attStmt["x5c"]
	if !ok {
		return nil, fmt.Errorf("missing x5c in attStmt")
	}
	list, ok := rawX5C.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("invalid or empty x5c in attStmt")
	}
	var certs []*x509.Certificate
	for i, item := range list {
		der, ok := item.([]byte)
		if !ok {
			return nil, fmt.Errorf("x5c[%d] is not a byte slice", i)
		}
		c, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("failed to parse x5c[%d]: %w", i, err)
		}
		certs = append(certs, c)
	}
	return certs, nil
}

func verifyAttestationChain(certs []*x509.Certificate, roots *x509.CertPool) error {
	if len(certs) == 0 {
		return fmt.Errorf("empty certificate chain")
	}
	intermediates := x509.NewCertPool()
	for _, ic := range certs[1:] {
		intermediates.AddCert(ic)
	}
	opts := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   time.Now(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}
	if _, err := certs[0].Verify(opts); err != nil {
		return fmt.Errorf("certificate chain verification failed: %w", err)
	}
	return nil
}

func parseASN1StringOrRaw(val []byte) string {
	var s string
	if rest, err := asn1.Unmarshal(val, &s); err == nil && len(rest) == 0 && s != "" {
		return s
	}
	var raw asn1.RawValue
	if rest, err := asn1.Unmarshal(val, &raw); err == nil && len(rest) == 0 && len(raw.Bytes) > 0 {
		var inner string
		if _, err2 := asn1.Unmarshal(raw.Bytes, &inner); err2 == nil && inner != "" {
			return inner
		}
		return string(raw.Bytes)
	}
	return string(val)
}

func parseASN1OctetBytes(val []byte) []byte {
	var b []byte
	if rest, err := asn1.Unmarshal(val, &b); err == nil && len(rest) == 0 {
		return b
	}
	var seq struct {
		Nonce []byte `asn1:"tag:1,explicit"`
	}
	if rest, err := asn1.Unmarshal(val, &seq); err == nil && len(rest) == 0 && len(seq.Nonce) > 0 {
		return seq.Nonce
	}
	return val
}

func (m *Manager) doAppleAttestationFormat(acc *account, az *authorization, ch *challenge, attObj *attestationObject) (crypto.PublicKey, error) {
	certs, err := extractX5CChain(attObj.AttStatement)
	if err != nil {
		return nil, err
	}
	roots := m.getAttestationRootPool(appleEnterpriseAttestationRootCA)
	if err := verifyAttestationChain(certs, roots); err != nil {
		return nil, err
	}
	leaf := certs[0]

	oidAppleSerial := asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 8, 9, 1}
	oidAppleUDID := asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 8, 9, 2}
	oidAppleNonce := asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 8, 11, 1}
	oidAppleAppAttestNonce := asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 8, 2}

	var serial, udid string
	var nonceBytes []byte
	for _, ext := range leaf.Extensions {
		switch {
		case ext.Id.Equal(oidAppleSerial):
			serial = strings.TrimSpace(parseASN1StringOrRaw(ext.Value))
		case ext.Id.Equal(oidAppleUDID):
			udid = strings.TrimSpace(parseASN1StringOrRaw(ext.Value))
		case ext.Id.Equal(oidAppleNonce), ext.Id.Equal(oidAppleAppAttestNonce):
			nonceBytes = parseASN1OctetBytes(ext.Value)
		}
	}

	if len(nonceBytes) == 0 {
		return nil, fmt.Errorf("missing Apple attestation nonce extension in leaf certificate")
	}

	keyAuth, err := keyAuthorization(ch.Token, acc.Key)
	if err != nil {
		return nil, err
	}
	expectedKeyAuthHash := sha256.Sum256([]byte(keyAuth))
	expectedTokenHash := sha256.Sum256([]byte(ch.Token))

	if subtle.ConstantTimeCompare(nonceBytes, expectedKeyAuthHash[:]) != 1 &&
		subtle.ConstantTimeCompare(nonceBytes, expectedTokenHash[:]) != 1 &&
		subtle.ConstantTimeCompare(nonceBytes, []byte(keyAuth)) != 1 &&
		subtle.ConstantTimeCompare(nonceBytes, []byte(ch.Token)) != 1 {
		return nil, fmt.Errorf("apple attestation nonce mismatch")
	}

	expectedID := az.Identifier.Value
	if expectedID == "" || (expectedID != serial && expectedID != udid) {
		return nil, fmt.Errorf("apple attestation device identifier %q does not match serial %q or udid %q", expectedID, serial, udid)
	}

	// Verify matching node exists
	node := m.findNodeFromPermanentIdentifier(expectedID)
	if node == nil {
		return nil, fmt.Errorf("node not found for apple attestation identifier %q", expectedID)
	}

	return leaf.PublicKey, nil
}

func (m *Manager) doStepAttestationFormat(acc *account, az *authorization, ch *challenge, attObj *attestationObject) (crypto.PublicKey, error) {
	certs, err := extractX5CChain(attObj.AttStatement)
	if err != nil {
		return nil, err
	}
	roots := m.getAttestationRootPool(yubicoPIVRootCA, yubicoAttestationRootCA)
	if err := verifyAttestationChain(certs, roots); err != nil {
		return nil, err
	}
	leaf := certs[0]

	rawSig, ok := attObj.AttStatement["sig"]
	if !ok {
		return nil, fmt.Errorf("missing sig in step attStmt")
	}
	sigBytes, ok := rawSig.([]byte)
	if !ok || len(sigBytes) == 0 {
		return nil, fmt.Errorf("invalid sig in step attStmt")
	}
	// Unwrap if sigBytes was double-CBOR-encoded as a byte string
	if decodedSig, rest, err := decodeCBORItem(sigBytes); err == nil && len(rest) == 0 {
		if innerBytes, ok := decodedSig.([]byte); ok && len(innerBytes) > 0 {
			sigBytes = innerBytes
		}
	}

	keyAuth, err := keyAuthorization(ch.Token, acc.Key)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(keyAuth))

	if err := verifyPublicKeySignature(leaf.PublicKey, digest[:], sigBytes, []byte(keyAuth)); err != nil {
		return nil, fmt.Errorf("step attestation signature verification failed: %w", err)
	}

	oidYubicoSerial := asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 41482, 3, 7}
	oidStepSerial := asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 37476, 9000, 64, 4}

	var identifiers []string
	if leaf.Subject.CommonName != "" {
		identifiers = append(identifiers, leaf.Subject.CommonName)
	}
	if leaf.Subject.SerialNumber != "" {
		identifiers = append(identifiers, leaf.Subject.SerialNumber)
	}
	for _, ext := range leaf.Extensions {
		if ext.Id.Equal(oidYubicoSerial) {
			var serialInt int64
			if _, err := asn1.Unmarshal(ext.Value, &serialInt); err == nil {
				identifiers = append(identifiers, strconv.FormatInt(serialInt, 10))
			} else {
				identifiers = append(identifiers, strings.TrimSpace(parseASN1StringOrRaw(ext.Value)))
			}
		} else if ext.Id.Equal(oidStepSerial) {
			var seq []string
			if _, err := asn1.Unmarshal(ext.Value, &seq); err == nil {
				identifiers = append(identifiers, seq...)
			} else {
				identifiers = append(identifiers, strings.TrimSpace(parseASN1StringOrRaw(ext.Value)))
			}
		}
	}

	expectedID := az.Identifier.Value
	matched := false
	for _, id := range identifiers {
		if id != "" && id == expectedID {
			matched = true
			break
		}
	}
	if !matched {
		return nil, fmt.Errorf("step attestation identifier %q does not match certificate identifiers %v", expectedID, identifiers)
	}

	// Verify matching node exists
	node := m.findNodeFromPermanentIdentifier(expectedID)
	if node == nil {
		return nil, fmt.Errorf("node not found for step attestation identifier %q", expectedID)
	}

	return leaf.PublicKey, nil
}

func verifyPublicKeySignature(pub crypto.PublicKey, digest, sig, rawMsg []byte) error {
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(k, digest, sig) {
			return fmt.Errorf("ecdsa signature invalid")
		}
		return nil
	case *rsa.PublicKey:
		if err := rsa.VerifyPKCS1v15(k, crypto.SHA256, digest, sig); err == nil {
			return nil
		}
		if err := rsa.VerifyPSS(k, crypto.SHA256, digest, sig, nil); err == nil {
			return nil
		}
		return fmt.Errorf("rsa signature invalid")
	case ed25519.PublicKey:
		if !ed25519.Verify(k, rawMsg, sig) && !ed25519.Verify(k, digest, sig) {
			return fmt.Errorf("ed25519 signature invalid")
		}
		return nil
	default:
		return fmt.Errorf("unsupported public key type %T", pub)
	}
}

// Minimal RFC 8949 CBOR decoder/encoder for WebAuthn / ACME device-attest-01 attestation objects.
func decodeCBORAttestationObject(data []byte) (*attestationObject, error) {
	val, _, err := decodeCBORItem(data)
	if err != nil {
		return nil, err
	}
	m, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("CBOR root is not a map")
	}
	out := &attestationObject{
		AttStatement: make(map[string]any),
	}
	if fmtVal, ok := m["fmt"].(string); ok {
		out.Format = fmtVal
	}
	if stmtVal, ok := m["attStmt"].(map[string]any); ok {
		out.AttStatement = stmtVal
	}
	if authVal, ok := m["authData"].([]byte); ok {
		out.AuthData = authVal
	}
	return out, nil
}

func readCBORLength(ai byte, data []byte) (uint64, []byte, error) {
	switch {
	case ai < 24:
		return uint64(ai), data, nil
	case ai == 24:
		if len(data) < 1 {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return uint64(data[0]), data[1:], nil
	case ai == 25:
		if len(data) < 2 {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return uint64(binary.BigEndian.Uint16(data[:2])), data[2:], nil
	case ai == 26:
		if len(data) < 4 {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return uint64(binary.BigEndian.Uint32(data[:4])), data[4:], nil
	case ai == 27:
		if len(data) < 8 {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return binary.BigEndian.Uint64(data[:8]), data[8:], nil
	default:
		return 0, nil, fmt.Errorf("unsupported or indefinite CBOR length: %d", ai)
	}
}

func decodeCBORItem(data []byte) (any, []byte, error) {
	if len(data) == 0 {
		return nil, nil, io.ErrUnexpectedEOF
	}
	initial := data[0]
	major := initial >> 5
	ai := initial & 0x1f
	rest := data[1:]

	switch major {
	case 0: // Unsigned integer
		n, r, err := readCBORLength(ai, rest)
		return n, r, err
	case 1: // Negative integer
		n, r, err := readCBORLength(ai, rest)
		if err != nil {
			return nil, nil, err
		}
		return -1 - int64(n), r, nil
	case 2: // Byte string
		n, r, err := readCBORLength(ai, rest)
		if err != nil {
			return nil, nil, err
		}
		if uint64(len(r)) < n {
			return nil, nil, io.ErrUnexpectedEOF
		}
		b := make([]byte, n)
		copy(b, r[:n])
		return b, r[n:], nil
	case 3: // Text string
		n, r, err := readCBORLength(ai, rest)
		if err != nil {
			return nil, nil, err
		}
		if uint64(len(r)) < n {
			return nil, nil, io.ErrUnexpectedEOF
		}
		return string(r[:n]), r[n:], nil
	case 4: // Array
		n, r, err := readCBORLength(ai, rest)
		if err != nil {
			return nil, nil, err
		}
		arr := make([]any, 0, n)
		for i := uint64(0); i < n; i++ {
			var item any
			item, r, err = decodeCBORItem(r)
			if err != nil {
				return nil, nil, err
			}
			arr = append(arr, item)
		}
		return arr, r, nil
	case 5: // Map
		n, r, err := readCBORLength(ai, rest)
		if err != nil {
			return nil, nil, err
		}
		m := make(map[string]any, n)
		for i := uint64(0); i < n; i++ {
			k, r2, err := decodeCBORItem(r)
			if err != nil {
				return nil, nil, err
			}
			v, r3, err := decodeCBORItem(r2)
			if err != nil {
				return nil, nil, err
			}
			ks, ok := k.(string)
			if !ok {
				ks = fmt.Sprintf("%v", k)
			}
			m[ks] = v
			r = r3
		}
		return m, r, nil
	case 6: // Semantic tag
		_, r, err := readCBORLength(ai, rest)
		if err != nil {
			return nil, nil, err
		}
		return decodeCBORItem(r)
	case 7: // Simple / float
		switch ai {
		case 20:
			return false, rest, nil
		case 21:
			return true, rest, nil
		case 22, 23:
			return nil, rest, nil
		default:
			_, r, err := readCBORLength(ai, rest)
			return nil, r, err
		}
	default:
		return nil, nil, fmt.Errorf("unknown CBOR major type: %d", major)
	}
}
