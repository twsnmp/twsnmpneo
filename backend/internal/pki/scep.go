package pki

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/smallstep/scep"
)

const maxSCEPPayloadSize = 2 << 20

type scepRequest struct {
	Operation string
	Message   []byte
}

func (m *Manager) scepFunc(c echo.Context) error {
	req, err := decodeSCEPRequest(c)
	if err != nil {
		slog.Warn("PKI SCEP decode request error", "error", err)
		return c.String(http.StatusInternalServerError, err.Error())
	}
	switch req.Operation {
	case "GetCACert":
		return m.getSCEPCACert(c)
	case "GetCACaps":
		return m.getSCEPCACaps(c)
	case "PKIOperation":
		return m.scepPKIOperation(c, req)
	default:
		slog.Warn("PKI SCEP unknown operation", "operation", req.Operation)
		return c.String(http.StatusInternalServerError, fmt.Sprintf("unknown operation: %s", req.Operation))
	}
}

func decodeSCEPRequest(c echo.Context) (scepRequest, error) {
	defer func() { _ = c.Request().Body.Close() }()
	operation := c.QueryParam("operation")
	var err error
	req := scepRequest{
		Operation: operation,
	}
	if c.Request().Method == http.MethodPost {
		req.Message, err = io.ReadAll(io.LimitReader(c.Request().Body, maxSCEPPayloadSize))
		if err != nil {
			return scepRequest{}, fmt.Errorf("failed reading request body: %w", err)
		}
	} else {
		message := c.QueryParam("message")
		req.Message, err = decodeSCEPMessage(message)
		if err != nil {
			return scepRequest{}, fmt.Errorf("failed decoding message: %w", err)
		}
	}
	return req, nil
}

func decodeSCEPMessage(message string) ([]byte, error) {
	if message == "" {
		return nil, nil
	}
	decodedMessage, err := base64.StdEncoding.DecodeString(message)
	if err != nil {
		return nil, err
	}
	return decodedMessage, nil
}

func (m *Manager) getSCEPCACert(c echo.Context) error {
	m.mu.RLock()
	rootBytes := m.rootCACertificate
	scepBytes := m.scepCACertificate
	m.mu.RUnlock()

	if rootBytes == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "CA not initialized"})
	}

	ca, err := x509.ParseCertificate(rootBytes)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	certs := []*x509.Certificate{ca}

	if scepBytes != nil {
		scepCA, sErr := x509.ParseCertificate(scepBytes)
		if sErr == nil {
			certs = append(certs, scepCA)
		}
	}

	if len(certs) == 1 {
		return c.Blob(http.StatusOK, "application/x-x509-ca-cert", ca.Raw)
	}
	data, err := scep.DegenerateCertificates(certs)
	if err != nil {
		slog.Warn("PKI SCEP degenerate certificates error", "error", err)
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.Blob(http.StatusOK, "application/x-x509-ca-ra-cert", data)
}

func (m *Manager) getSCEPCACaps(c echo.Context) error {
	caps := []string{
		"Renewal",
		"SHA-1",
		"SHA-256",
		"AES",
		"DES3",
		"SCEPStandard",
		"POSTPKIOperation",
	}
	return c.String(http.StatusOK, strings.Join(caps, "\r\n"))
}

func (m *Manager) scepPKIOperation(c echo.Context, req scepRequest) error {
	msg, err := scep.ParsePKIMessage(req.Message)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	m.mu.RLock()
	key, ok := m.scepCAPrivateKey.(crypto.Signer)
	if !ok {
		key, ok = m.rootCAPrivateKey.(crypto.Signer)
	}
	certBytes := m.scepCACertificate
	if certBytes == nil {
		certBytes = m.rootCACertificate
	}
	m.mu.RUnlock()

	if !ok || certBytes == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "CA key not available"})
	}

	ca, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	err = msg.DecryptPKIEnvelope(ca, key)
	if err != nil {
		slog.Warn("PKI SCEP decrypt envelope error", "error", err)
		return c.JSON(http.StatusBadRequest, err)
	}

	csr := msg.CSR
	transactionID := string(msg.TransactionID)
	challengePassword := msg.ChallengePassword
	remoteIP := c.RealIP()

	m.mu.Lock()
	crtBytes, _, err := m.createCertificateFromCSR(csr.Raw, "scep", map[string]string{
		"RemoteAddr":        remoteIP,
		"TransactionID":     transactionID,
		"ChallengePassword": challengePassword,
	})
	m.mu.Unlock()

	if err != nil {
		slog.Warn("PKI SCEP certificate issuance rejected", "error", err)
		rsp, fErr := msg.Fail(ca, key, scep.BadRequest)
		if fErr != nil {
			return c.JSON(http.StatusInternalServerError, fErr)
		}
		return c.Blob(http.StatusOK, "application/x-pki-message", rsp.Raw)
	}

	crt, err := x509.ParseCertificate(crtBytes)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	rsp, err := msg.Success(ca, key, crt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.Blob(http.StatusOK, "application/x-pki-message", rsp.Raw)
}
