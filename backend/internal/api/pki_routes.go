package api

import (
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
)

var pkiSerialPattern = regexp.MustCompile(`^[0-9a-fA-F]{1,64}$`)

const maxOCSPRequestSize = 1 << 20

func registerPKIRoutes(api *echo.Group, manager *pki.Manager, store datastore.DataStore, applySettings func() error) {
	group := api.Group("/pki")
	group.POST("/ocsp", func(c echo.Context) error {
		body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxOCSPRequestSize+1))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if len(body) > maxOCSPRequestSize {
			return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "OCSP request exceeds 1 MiB"})
		}
		return writeOCSPResponse(c, manager, body)
	})
	group.GET("/ocsp/:request", func(c echo.Context) error {
		encoded := c.Param("request")
		if len(encoded) > base64.RawURLEncoding.EncodedLen(maxOCSPRequestSize)+4 {
			return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "OCSP request exceeds 1 MiB"})
		}
		request, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			request, err = base64.StdEncoding.DecodeString(encoded)
		}
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid base64 OCSP request"})
		}
		return writeOCSPResponse(c, manager, request)
	})
	group.GET("/status", func(c echo.Context) error {
		return c.JSON(http.StatusOK, manager.Status())
	})
	group.GET("/settings", func(c echo.Context) error {
		return c.JSON(http.StatusOK, manager.Settings())
	})
	group.PUT("/settings", func(c echo.Context) error {
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 1<<20)
		var settings pki.Settings
		if err := c.Bind(&settings); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		previous := manager.Settings()
		if err := manager.UpdateSettings(settings); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if applySettings != nil {
			if err := applySettings(); err != nil {
				rollbackErr := manager.UpdateSettings(previous)
				if rollbackErr == nil {
					rollbackErr = applySettings()
				}
				if rollbackErr != nil {
					err = errors.Join(err, rollbackErr)
				}
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
		}
		logPKIEvent(c, store, "info", "Updated PKI service settings")
		return c.JSON(http.StatusOK, manager.Settings())
	})
	group.POST("/ca", func(c echo.Context) error {
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 1<<20)
		var options pki.CAOptions
		if err := c.Bind(&options); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := manager.InitializeCA(options); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if applySettings != nil {
			if err := applySettings(); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
		}
		logPKIEvent(c, store, "info", fmt.Sprintf("Initialized CA commonName=%s", options.CommonName))
		return c.JSON(http.StatusCreated, manager.Status())
	})
	group.DELETE("/ca", func(c echo.Context) error {
		if err := manager.ResetCA(); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if applySettings != nil {
			if err := applySettings(); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
		}
		logPKIEvent(c, store, "warn", "Reset CA and deleted issued certificate inventory")
		return c.NoContent(http.StatusNoContent)
	})
	group.POST("/certificates/csr", func(c echo.Context) error {
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 1<<20)
		var request struct {
			CSRPEM string `json:"csrPEM"`
		}
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		block, _ := pem.Decode([]byte(request.CSRPEM))
		if block == nil || block.Type != "CERTIFICATE REQUEST" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "a PEM certificate request is required"})
		}
		cert, err := manager.IssueCertificateFromCSR(block.Bytes, "manual")
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		logPKIEvent(c, store, "info", fmt.Sprintf("Issued certificate from CSR subject=%s serial=%s", cert.Subject, cert.Serial))
		return c.JSON(http.StatusCreated, cert)
	})
	group.GET("/ca.pem", func(c echo.Context) error {
		if !manager.Status().Ready {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "CA is not initialized"})
		}
		return c.Blob(http.StatusOK, "application/x-pem-file", []byte(manager.GetCACertPEM()))
	})
	group.GET("/certificates", func(c echo.Context) error {
		return c.JSON(http.StatusOK, manager.ListCertificates())
	})
	group.POST("/certificates", func(c echo.Context) error {
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 1<<20)
		var req struct {
			CommonName  string   `json:"commonName"`
			DNSNames    []string `json:"dnsNames"`
			IPAddresses []string `json:"ipAddresses"`
			ValidDays   int      `json:"validDays"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		ips := make([]net.IP, 0, len(req.IPAddresses))
		for _, value := range req.IPAddresses {
			ip := net.ParseIP(strings.TrimSpace(value))
			if ip == nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid IP subject alternative name %q", value)})
			}
			ips = append(ips, ip)
		}
		cert, err := manager.IssueCertificate(req.CommonName, req.DNSNames, ips, req.ValidDays)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		logPKIEvent(c, store, "info", fmt.Sprintf("Issued certificate subject=%s serial=%s", cert.Subject, cert.Serial))
		return c.JSON(http.StatusCreated, cert)
	})
	group.DELETE("/certificates/:serial", func(c echo.Context) error {
		serial := c.Param("serial")
		if !pkiSerialPattern.MatchString(serial) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid certificate serial"})
		}
		cert, err := manager.GetCertificate(serial)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "certificate not found"})
		}
		if err := manager.RevokeCertificate(serial); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "certificate not found"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		logPKIEvent(c, store, "warn", fmt.Sprintf("Revoked certificate subject=%s serial=%s", cert.Subject, cert.Serial))
		return c.NoContent(http.StatusNoContent)
	})
	group.GET("/certificates/:serial/download", func(c echo.Context) error {
		serial := c.Param("serial")
		if !pkiSerialPattern.MatchString(serial) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid certificate serial"})
		}
		cert, err := manager.GetCertificate(serial)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "certificate not found"})
		}
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s.pem"`, strings.ToLower(serial)))
		return c.Blob(http.StatusOK, "application/x-pem-file", []byte(cert.CertPEM+"\n"+cert.KeyPEM))
	})
	group.GET("/crl", func(c echo.Context) error {
		crl, err := manager.GetCRL()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.Blob(http.StatusOK, "application/pkix-crl", crl)
	})
}

func logPKIEvent(c echo.Context, store datastore.DataStore, level, message string) {
	if store == nil {
		return
	}
	err := store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
		Time:  time.Now().UnixNano(),
		Type:  "ca",
		Level: level,
		Event: message,
	})
	if err != nil {
		slog.Error("Failed to persist PKI event", "error", err, "event", message)
	}
}

func writeOCSPResponse(c echo.Context, manager *pki.Manager, request []byte) error {
	response, err := manager.CreateOCSPResponse(request)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.Blob(http.StatusOK, "application/ocsp-response", response)
}
