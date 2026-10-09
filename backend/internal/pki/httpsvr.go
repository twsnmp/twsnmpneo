package pki

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

// GetHTTPServerStatus returns the current status of the PKI HTTP server.
func (m *Manager) GetHTTPServerStatus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.lastHTTPServerErr != nil {
		return fmt.Sprintf("error %v", m.lastHTTPServerErr)
	} else if m.httpServerRunning {
		return fmt.Sprintf("running port=%d", m.conf.HTTPPort)
	}
	return "stopped"
}

// StartHTTPServer starts the plain HTTP server for CRL/OCSP/SCEP.
func (m *Manager) StartHTTPServer() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.httpServer != nil {
		return
	}
	m.lastHTTPServerErr = nil
	m.httpServerRunning = true
	m.httpServer = echo.New()
	m.httpServer.HideBanner = true
	m.httpServer.HidePort = true

	port := m.conf.HTTPPort
	if m.store != nil {
		_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "pki",
			Level: "info",
			Event: fmt.Sprintf(i18n.Trans("Started CRL/OCSP/SCEP HTTP server on port %d"), port),
		})
	}
	go m.httpServerFunc(m.httpServer, port)
}

// StopHTTPServer stops the plain HTTP server.
func (m *Manager) StopHTTPServer() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopHTTPServerLocked()
}

func (m *Manager) stopHTTPServerLocked() {
	if m.httpServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer func() {
		cancel()
		m.httpServer = nil
		m.lastHTTPServerErr = nil
		m.httpServerRunning = false
	}()
	if err := m.httpServer.Shutdown(ctx); err != nil {
		slog.Warn("PKI HTTP server shutdown error", "error", err)
	}
	if m.store != nil {
		_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "pki",
			Level: "info",
			Event: i18n.Trans("Stopped CRL/OCSP/SCEP HTTP server"),
		})
	}
}

func (m *Manager) httpServerFunc(e *echo.Echo, port int) {
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	e.GET("/ca.pem", func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderCacheControl, "max-age=0, no-cache")
		return c.Blob(http.StatusOK, "application/x-pem-file", []byte(m.GetCACertPEM()))
	})
	e.GET("/scepca.pem", func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderCacheControl, "max-age=0, no-cache")
		return c.Blob(http.StatusOK, "application/x-pem-file", []byte(m.GetSCEPCACertPEM()))
	})
	e.GET("/crl", func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderCacheControl, "max-age=0, no-cache")
		return c.Blob(http.StatusOK, "application/pkix-crl", m.GetCRL())
	})
	e.GET("/crl.pem", func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderCacheControl, "max-age=0, no-cache")
		return c.Blob(http.StatusOK, "application/x-pem-file", makePEM(m.GetCRL(), "X509 CRL"))
	})
	e.GET("/ocsp/:req", func(c echo.Context) error {
		req := c.Param("req")
		b, err := base64.StdEncoding.DecodeString(req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		return m.ocspFunc(c, b)
	})
	e.POST("/ocsp", func(c echo.Context) error {
		b, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		return m.ocspFunc(c, b)
	})
	e.GET("/scep", func(c echo.Context) error {
		return m.scepFunc(c)
	})
	e.POST("/scep", func(c echo.Context) error {
		return m.scepFunc(c)
	})

	if err := e.Start(fmt.Sprintf(":%d", port)); err != nil && err != http.ErrServerClosed {
		m.mu.Lock()
		m.lastHTTPServerErr = err
		m.httpServerRunning = false
		m.mu.Unlock()
		slog.Error("PKI HTTP server failed to start", "port", port, "error", err)
	}
}
