package pki

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/ocsp"
)

func (m *Manager) ocspFunc(c echo.Context, b []byte) error {
	req, err := ocsp.ParseRequest(b)
	if err != nil {
		slog.Warn("PKI OCSP parse request error", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	m.mu.RLock()
	key, ok := m.rootCAPrivateKey.(crypto.Signer)
	if !ok || m.rootCACertificate == nil {
		m.mu.RUnlock()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "CA key not available"})
	}
	ca, err := x509.ParseCertificate(m.rootCACertificate)
	m.mu.RUnlock()

	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	res := ocsp.Response{
		Status:       ocsp.Unknown,
		SerialNumber: req.SerialNumber,
		ThisUpdate:   time.Now(),
		NextUpdate:   time.Now().Add(time.Second * 60 * 10),
		IssuerHash:   req.HashAlgorithm,
	}

	if bytes.Equal(req.IssuerKeyHash, ca.SubjectKeyId) && m.store != nil {
		id := fmt.Sprintf("%x", req.SerialNumber.Int64())
		cert, _ := m.store.GetPKICert(context.Background(), id)
		if cert != nil {
			if cert.Revoked != 0 {
				res.Status = ocsp.Revoked
				res.RevokedAt = time.Unix(0, cert.Revoked)
				res.RevocationReason = ocsp.Unspecified
			} else {
				res.Status = ocsp.Good
			}
		}
	}

	bres, err := ocsp.CreateResponse(ca, ca, res, key)
	if err != nil {
		slog.Warn("PKI OCSP create response error", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.Blob(http.StatusOK, "application/ocsp-response", bres)
}
