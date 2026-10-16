package pki

import (
	"bytes"
	"context"
	"crypto/subtle"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/smallstep/scep"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

const (
	estPKCS7MimeType  = "application/pkcs7-mime; smime-type=certs-only"
	maxESTPayloadSize = 2 << 20
)

// registerESTRoutes registers RFC 7030 EST endpoints (with and without an optional label) on the Echo instance.
func (m *Manager) registerESTRoutes(e *echo.Echo) {
	e.GET("/.well-known/est/cacerts", m.estCACerts)
	e.GET("/.well-known/est/:label/cacerts", m.estCACerts)
	e.POST("/.well-known/est/simpleenroll", m.estSimpleEnroll)
	e.POST("/.well-known/est/:label/simpleenroll", m.estSimpleEnroll)
	e.POST("/.well-known/est/simplereenroll", m.estSimpleReenroll)
	e.POST("/.well-known/est/:label/simplereenroll", m.estSimpleReenroll)
}

// estCACerts handles GET /.well-known/est{/label}/cacerts (RFC 7030 Section 4.1).
func (m *Manager) estCACerts(c echo.Context) error {
	m.mu.RLock()
	valid := m.IsCAValid()
	rootBytes := m.rootCACertificate
	m.mu.RUnlock()

	if !valid || rootBytes == nil {
		return c.String(http.StatusServiceUnavailable, "CA is not ready")
	}
	ca, err := x509.ParseCertificate(rootBytes)
	if err != nil {
		slog.Warn("est cacerts parse ca error", "error", err)
		return c.String(http.StatusInternalServerError, err.Error())
	}
	p7, err := scep.DegenerateCertificates([]*x509.Certificate{ca})
	if err != nil {
		slog.Warn("est cacerts degenerate pkcs7 error", "error", err)
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return sendESTPKCS7Response(c, p7)
}

// estSimpleEnroll handles POST /.well-known/est{/label}/simpleenroll (RFC 7030 Section 4.2.1).
func (m *Manager) estSimpleEnroll(c echo.Context) error {
	return m.handleESTEnroll(c, false)
}

// estSimpleReenroll handles POST /.well-known/est{/label}/simplereenroll (RFC 7030 Section 4.2.2).
func (m *Manager) estSimpleReenroll(c echo.Context) error {
	return m.handleESTEnroll(c, true)
}

func (m *Manager) handleESTEnroll(c echo.Context, isReenroll bool) error {
	m.mu.RLock()
	valid := m.IsCAValid()
	m.mu.RUnlock()

	if !valid {
		return c.String(http.StatusServiceUnavailable, "CA is not ready")
	}
	defer func() { _ = c.Request().Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxESTPayloadSize))
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("failed reading request body: %v", err))
	}
	csrBytes, err := decodeESTCSR(body)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("invalid CSR format: %v", err))
	}
	csr, err := x509.ParseCertificateRequest(csrBytes)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("failed parsing CSR: %v", err))
	}
	if err := csr.CheckSignature(); err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("invalid CSR signature: %v", err))
	}

	info := map[string]string{
		"RemoteAddr": c.RealIP(),
	}
	if label := c.Param("label"); label != "" {
		info["ESTLabel"] = label
	}
	if isReenroll {
		info["Operation"] = "simplereenroll"
	} else {
		info["Operation"] = "simpleenroll"
	}
	if pid := extractPermanentIdentifierFromCSR(csr); pid != "" {
		info["PermanentIdentifier"] = pid
	}

	// Find target node from CSR and request info
	node := m.findNodeForCSR(csr, info)
	if node == nil {
		if m.store != nil {
			_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "ca",
				Level: "low",
				Event: fmt.Sprintf(i18n.Trans("Reject CSR subject=%s info=%+v err=%v"), csr.Subject.String(), info, fmt.Errorf("node not found")),
			})
		}
		return c.String(http.StatusBadRequest, "node not found")
	}

	// Authenticate client (mTLS or HTTP Basic Auth is mandatory)
	authMethod, err := m.authenticateESTRequest(c, node, isReenroll)
	if err != nil {
		if m.store != nil {
			_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
				Time:     time.Now().UnixNano(),
				Type:     "ca",
				Level:    "low",
				NodeID:   node.ID,
				NodeName: node.Name,
				Event:    fmt.Sprintf(i18n.Trans("Reject CSR subject=%s info=%+v err=%v"), csr.Subject.String(), info, err),
			})
		}
		c.Response().Header().Set(echo.HeaderWWWAuthenticate, `Basic realm="EST"`)
		// Direct map assignment preserves exact casing "WWW-Authenticate" for strict clients like libest (strncmp)
		c.Response().Header()["WWW-Authenticate"] = []string{`Basic realm="EST"`}
		return c.String(http.StatusUnauthorized, err.Error())
	}
	info["Auth"] = authMethod

	m.mu.Lock()
	crtBytes, _, err := m.createCertificateFromCSR(csr.Raw, "est", info)
	m.mu.Unlock()
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	crt, err := x509.ParseCertificate(crtBytes)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	p7, err := scep.DegenerateCertificates([]*x509.Certificate{crt})
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return sendESTPKCS7Response(c, p7)
}

// authenticateESTRequest verifies either a valid client certificate (mTLS) or HTTP Basic Authentication against the matched node.
func (m *Manager) authenticateESTRequest(c echo.Context, node *datastore.NodeEnt, isReenroll bool) (string, error) {
	// 1. Check mTLS client certificate if presented
	if reqTLS := c.Request().TLS; reqTLS != nil && len(reqTLS.PeerCertificates) > 0 {
		clientCert := reqTLS.PeerCertificates[0]
		if err := m.verifyESTClientCert(clientCert, node, isReenroll); err != nil {
			return "", fmt.Errorf("client certificate authentication failed: %w", err)
		}
		return "mtls", nil
	}

	// 2. Check HTTP Basic Authentication
	user, pass, hasBasic := c.Request().BasicAuth()
	if !hasBasic {
		return "", fmt.Errorf("authentication required (mTLS or HTTP Basic)")
	}
	if node.Password == "" {
		return "", fmt.Errorf("node password is not configured for Basic authentication")
	}
	if node.User != "" {
		if subtle.ConstantTimeCompare([]byte(user), []byte(node.User)) != 1 {
			return "", fmt.Errorf("basic auth user mismatch")
		}
	}
	if subtle.ConstantTimeCompare([]byte(pass), []byte(node.Password)) != 1 {
		return "", fmt.Errorf("basic auth password mismatch")
	}
	return "basic", nil
}

// verifyESTClientCert verifies that the presented client certificate is signed by our Root CA,
// is within its validity period, is not revoked, and (if tracked in datastore) belongs to the target node.
func (m *Manager) verifyESTClientCert(clientCert *x509.Certificate, node *datastore.NodeEnt, isReenroll bool) error {
	m.mu.RLock()
	rootBytes := m.rootCACertificate
	m.mu.RUnlock()

	if rootBytes == nil {
		return fmt.Errorf("CA root certificate not available")
	}
	ca, err := x509.ParseCertificate(rootBytes)
	if err != nil {
		return err
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	opts := x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageAny},
	}
	if _, err := clientCert.Verify(opts); err != nil {
		return err
	}
	if clientCert.SerialNumber == nil {
		return fmt.Errorf("missing serial number")
	}
	certID := fmt.Sprintf("%x", clientCert.SerialNumber)
	if m.store == nil {
		return fmt.Errorf("datastore not available")
	}
	stored, err := m.store.GetPKICert(context.Background(), certID)
	if err != nil || stored == nil {
		return fmt.Errorf("certificate serial %s not found in CA database", certID)
	}
	if stored.Revoked > 0 {
		return fmt.Errorf("certificate serial %s is revoked", certID)
	}
	if isReenroll && stored.NodeID != "" && stored.NodeID != node.ID {
		return fmt.Errorf("reenroll certificate node mismatch")
	}
	return nil
}

// decodeESTCSR accepts PEM, base64-encoded DER (with or without whitespace), or raw DER CSR bytes.
func decodeESTCSR(body []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty request body")
	}
	if bytes.HasPrefix(trimmed, []byte("-----BEGIN")) {
		block, _ := pem.Decode(trimmed)
		if block == nil || !strings.Contains(block.Type, "CERTIFICATE REQUEST") {
			return nil, fmt.Errorf("invalid PEM CSR block")
		}
		return block.Bytes, nil
	}
	// Strip all whitespace/newlines and try Base64 decoding
	cleaned := strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, string(trimmed))
	if der, err := base64.StdEncoding.DecodeString(cleaned); err == nil && len(der) > 0 {
		if _, err := x509.ParseCertificateRequest(der); err == nil {
			return der, nil
		}
	}
	if der, err := base64.RawStdEncoding.DecodeString(cleaned); err == nil && len(der) > 0 {
		if _, err := x509.ParseCertificateRequest(der); err == nil {
			return der, nil
		}
	}
	// Fallback: raw DER
	if _, err := x509.ParseCertificateRequest(trimmed); err == nil {
		return trimmed, nil
	}
	return nil, fmt.Errorf("unable to decode PKCS#10 CSR")
}

// extractPermanentIdentifierFromCSR extracts RFC 4043 PermanentIdentifier (OID 1.3.6.1.5.5.7.8.3) from CSR SAN extension if present.
func extractPermanentIdentifierFromCSR(csr *x509.CertificateRequest) string {
	oidSAN := asn1.ObjectIdentifier{2, 5, 29, 17}
	oidPermanentIdentifier := asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 8, 3}
	for _, ext := range csr.Extensions {
		if !ext.Id.Equal(oidSAN) {
			continue
		}
		var seq asn1.RawValue
		if _, err := asn1.Unmarshal(ext.Value, &seq); err != nil || seq.Tag != asn1.TagSequence {
			continue
		}
		rest := seq.Bytes
		for len(rest) > 0 {
			var gn asn1.RawValue
			var err error
			rest, err = asn1.Unmarshal(rest, &gn)
			if err != nil {
				break
			}
			// otherName is [0] IMPLICIT
			if gn.Class == asn1.ClassContextSpecific && gn.Tag == 0 {
				var typeID asn1.ObjectIdentifier
				innerRest, err := asn1.Unmarshal(gn.Bytes, &typeID)
				if err == nil && typeID.Equal(oidPermanentIdentifier) && len(innerRest) > 0 {
					var expVal asn1.RawValue
					if _, err := asn1.Unmarshal(innerRest, &expVal); err == nil {
						var pidSeq struct {
							IdentifierValue string                `asn1:"utf8,optional"`
							Assigner        asn1.ObjectIdentifier `asn1:"optional"`
						}
						if _, err := asn1.Unmarshal(expVal.Bytes, &pidSeq); err == nil && pidSeq.IdentifierValue != "" {
							return pidSeq.IdentifierValue
						}
					}
				}
			}
		}
	}
	return ""
}

func sendESTPKCS7Response(c echo.Context, derBytes []byte) error {
	b64 := []byte(formatBase64Lines(derBytes, 64))
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	c.Response().Header().Set("Content-Transfer-Encoding", "base64")
	c.Response().Header().Set(echo.HeaderContentLength, strconv.Itoa(len(b64)))
	return c.Blob(http.StatusOK, estPKCS7MimeType, b64)
}

func formatBase64Lines(data []byte, lineLen int) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	if lineLen <= 0 || len(encoded) <= lineLen {
		return encoded + "\n"
	}
	var sb strings.Builder
	for i := 0; i < len(encoded); i += lineLen {
		end := i + lineLen
		if end > len(encoded) {
			end = len(encoded)
		}
		sb.WriteString(encoded[i:end])
		sb.WriteByte('\n')
	}
	return sb.String()
}
