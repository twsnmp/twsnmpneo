package api

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/smallstep/scep"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
)

const maxSCEPRequestSize = 2 << 20

func registerPKIProtocolRoutes(e *echo.Echo, manager *pki.Manager, store datastore.DataStore) {
	protocols := e.Group("", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !manager.Settings().EnableHTTP {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "PKI HTTP services are disabled"})
			}
			return next(c)
		}
	})
	protocols.GET("/ca.pem", func(c echo.Context) error {
		if !manager.Status().Ready {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "CA is not initialized"})
		}
		c.Response().Header().Set(echo.HeaderCacheControl, "max-age=0, no-cache")
		return c.Blob(http.StatusOK, "application/x-pem-file", []byte(manager.GetCACertPEM()))
	})
	protocols.GET("/scepca.pem", func(c echo.Context) error {
		certs, err := manager.SCEPCACertificates()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		c.Response().Header().Set(echo.HeaderCacheControl, "max-age=0, no-cache")
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certs[1].Raw})
		return c.Blob(http.StatusOK, "application/x-pem-file", certPEM)
	})
	protocols.GET("/crl", func(c echo.Context) error {
		crl, err := manager.GetCRL()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		c.Response().Header().Set(echo.HeaderCacheControl, "max-age=0, no-cache")
		return c.Blob(http.StatusOK, "application/pkix-crl", crl)
	})
	protocols.GET("/crl.pem", func(c echo.Context) error {
		crl, err := manager.GetCRL()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		c.Response().Header().Set(echo.HeaderCacheControl, "max-age=0, no-cache")
		crlPEM := pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: crl})
		return c.Blob(http.StatusOK, "application/x-pem-file", crlPEM)
	})
	protocols.POST("/ocsp", func(c echo.Context) error {
		body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxOCSPRequestSize+1))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if len(body) > maxOCSPRequestSize {
			return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "OCSP request exceeds 1 MiB"})
		}
		return writeOCSPResponse(c, manager, body)
	})
	protocols.GET("/ocsp/:request", func(c echo.Context) error {
		if len(c.Param("request")) > base64.RawURLEncoding.EncodedLen(maxOCSPRequestSize)+4 {
			return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "OCSP request exceeds 1 MiB"})
		}
		request, err := decodeBase64Request(c.Param("request"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid base64 OCSP request"})
		}
		return writeOCSPResponse(c, manager, request)
	})
	protocols.GET("/scep", func(c echo.Context) error {
		return handleSCEP(c, manager, store)
	})
	protocols.POST("/scep", func(c echo.Context) error {
		return handleSCEP(c, manager, store)
	})
}

func handleSCEP(c echo.Context, manager *pki.Manager, store datastore.DataStore) error {
	operation := c.QueryParam("operation")
	var message []byte
	if c.Request().Method == http.MethodPost {
		body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxSCEPRequestSize+1))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if len(body) > maxSCEPRequestSize {
			return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "SCEP request exceeds 2 MiB"})
		}
		message = body
	} else {
		encoded := c.QueryParam("message")
		if len(encoded) > base64.RawStdEncoding.EncodedLen(maxSCEPRequestSize)+4 {
			return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "SCEP request exceeds 2 MiB"})
		}
		var err error
		message, err = decodeBase64Request(encoded)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid base64 SCEP message"})
		}
	}
	switch operation {
	case "GetCACert":
		certs, err := manager.SCEPCACertificates()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if len(certs) == 1 {
			return c.Blob(http.StatusOK, "application/x-x509-ca-cert", certs[0].Raw)
		}
		chain, err := scep.DegenerateCertificates(certs)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.Blob(http.StatusOK, "application/x-x509-ca-ra-cert", chain)
	case "GetCACaps":
		return c.String(http.StatusOK, strings.Join([]string{
			"Renewal",
			"SHA-256",
			"AES",
			"POSTPKIOperation",
		}, "\r\n"))
	case "PKIOperation":
		response, err := manager.ProcessSCEPMessage(message, func(csr *x509.CertificateRequest, challenge string) error {
			if challenge == "" || store == nil {
				return fmt.Errorf("SCEP challenge password was not accepted")
			}
			nodes, err := store.ListNodes(c.Request().Context())
			if err != nil {
				return fmt.Errorf("load managed nodes: %w", err)
			}
			remoteIP := remoteIPAddress(c.RealIP())
			for _, node := range nodes {
				if node == nil || node.Password != challenge {
					continue
				}
				if pki.MatchSCEPCertificateIdentity(csr, node.Name, node.IP) ||
					(remoteIP != "" && pki.MatchSCEPCertificateIdentity(csr, node.Name, remoteIP)) {
					return nil
				}
			}
			return fmt.Errorf("SCEP challenge password was not accepted")
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		logPKIEvent(c, store, "info", "Issued certificate through SCEP")
		return c.Blob(http.StatusOK, "application/x-pki-message", response)
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unsupported SCEP operation %q", operation)})
	}
}

func decodeBase64Request(encoded string) ([]byte, error) {
	if decoded, err := base64.RawURLEncoding.DecodeString(encoded); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(encoded)
}

func remoteIPAddress(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return strings.Trim(addr, "[]")
}
