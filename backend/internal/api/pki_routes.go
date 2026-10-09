package api

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
)

// CertEnt is the API representation of an issued certificate.
type CertEnt struct {
	Status  string `json:"Status"`
	ID      string `json:"ID"`
	Subject string `json:"Subject"`
	Node    string `json:"Node"`
	Created int64  `json:"Created"`
	Revoked int64  `json:"Revoked"`
	Expire  int64  `json:"Expire"`
	Type    string `json:"Type"`
}

func registerPKIRoutes(api *echo.Group, manager *pki.Manager, store datastore.DataStore) {
	group := api.Group("/pki")

	// GET /api/pki/hasCA
	group.GET("/hasCA", func(c echo.Context) error {
		if manager == nil {
			return c.JSON(http.StatusOK, false)
		}
		return c.JSON(http.StatusOK, manager.IsCAValid())
	})

	// GET /api/pki/createCA
	group.GET("/createCA", func(c echo.Context) error {
		conf := datastore.DefaultPKIConf()
		if store != nil {
			if loaded, err := store.GetPKIConf(c.Request().Context()); err == nil && loaded != nil {
				conf = *loaded
			}
		}
		return c.JSON(http.StatusOK, &datastore.CreateCAReq{
			RootCAKeyType: conf.RootCAKeyType,
			Name:          conf.Name,
			SANs:          conf.SANs,
			AcmeBaseURL:   conf.AcmeBaseURL,
			AcmePort:      conf.AcmePort,
			HTTPBaseURL:   conf.HTTPBaseURL,
			HTTPPort:      conf.HTTPPort,
			RootCATerm:    conf.RootCATerm,
			CrlInterval:   conf.CrlInterval,
			CertTerm:      conf.CertTerm,
		})
	})

	// POST /api/pki/createCA
	group.POST("/createCA", func(c echo.Context) error {
		if manager == nil {
			return echo.ErrInternalServerError
		}
		if manager.IsCAValid() {
			return echo.NewHTTPError(http.StatusBadRequest, "CA already exists")
		}
		req := new(datastore.CreateCAReq)
		if err := c.Bind(req); err != nil {
			return echo.ErrBadRequest
		}
		if err := manager.CreateCA(req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return c.JSON(http.StatusOK, map[string]string{"resp": "ok"})
	})

	// POST /api/pki/destroyCA
	group.POST("/destroyCA", func(c echo.Context) error {
		if manager == nil {
			return echo.ErrInternalServerError
		}
		if err := manager.DestroyCA(); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, map[string]string{"resp": "ok"})
	})

	// GET /api/pki/certs
	group.GET("/certs", func(c echo.Context) error {
		if manager == nil || store == nil {
			return c.JSON(http.StatusOK, []*CertEnt{})
		}
		now := time.Now().UnixNano()
		certs, err := store.ListPKICerts(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusOK, []*CertEnt{})
		}

		ret := make([]*CertEnt, 0, len(certs))
		for _, cert := range certs {
			if cert == nil {
				continue
			}
			status := "valid"
			if cert.Revoked > 0 {
				status = "revoked"
			} else if cert.Expire < now {
				status = "expired"
			}
			nodeName := ""
			if cert.NodeID != "" {
				if n, _ := store.GetNode(c.Request().Context(), cert.NodeID); n != nil {
					nodeName = n.Name
				}
			}
			ret = append(ret, &CertEnt{
				Status:  status,
				ID:      cert.ID,
				Subject: cert.Subject,
				Node:    nodeName,
				Created: cert.Created,
				Revoked: cert.Revoked,
				Expire:  cert.Expire,
				Type:    cert.Type,
			})
		}
		return c.JSON(http.StatusOK, ret)
	})

	// POST /api/pki/createCSR
	group.POST("/createCSR", func(c echo.Context) error {
		if manager == nil {
			return echo.ErrInternalServerError
		}
		req := new(datastore.CSRReqEnt)
		if err := c.Bind(req); err != nil {
			return echo.ErrBadRequest
		}
		zipBytes, err := manager.CreateCertificateRequest(req)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="csr_%s.zip"`, time.Now().Format("200601021504")))
		return c.Blob(http.StatusOK, "application/zip", zipBytes)
	})

	// POST /api/pki/createCRT
	group.POST("/createCRT", func(c echo.Context) error {
		if manager == nil {
			return echo.ErrInternalServerError
		}
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return echo.ErrBadRequest
		}
		if fileHeader.Size > 10*1024*1024 {
			return echo.ErrBadRequest
		}
		f, err := fileHeader.Open()
		if err != nil {
			return echo.ErrBadRequest
		}
		defer f.Close()

		csrBytes, err := io.ReadAll(f)
		if err != nil {
			return echo.ErrBadRequest
		}

		crt, err := manager.CreateCertificate(csrBytes)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="crt_%s.pem"`, time.Now().Format("200601021504")))
		return c.Blob(http.StatusOK, "application/x-pem-file", crt)
	})

	// DELETE /api/pki/revoke/:id
	group.DELETE("/revoke/:id", func(c echo.Context) error {
		if manager == nil {
			return echo.ErrInternalServerError
		}
		id := c.Param("id")
		if err := manager.RevokeCert(id); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return c.JSON(http.StatusOK, map[string]string{"resp": "ok"})
	})

	// GET /api/pki/cert/:id
	group.GET("/cert/:id", func(c echo.Context) error {
		if store == nil {
			return echo.ErrNotFound
		}
		id := c.Param("id")
		cert, err := store.GetPKICert(c.Request().Context(), id)
		if err != nil || cert == nil {
			return echo.ErrNotFound
		}
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s.pem"`, id))
		return c.Blob(http.StatusOK, "application/x-pem-file", []byte(cert.Certificate))
	})

	// GET /api/pki/control
	group.GET("/control", func(c echo.Context) error {
		if manager == nil {
			return echo.ErrInternalServerError
		}
		return c.JSON(http.StatusOK, manager.GetPKIControl())
	})

	// POST /api/pki/control
	group.POST("/control", func(c echo.Context) error {
		if manager == nil {
			return echo.ErrInternalServerError
		}
		req := new(datastore.PKIControlEnt)
		if err := c.Bind(req); err != nil {
			return echo.ErrBadRequest
		}
		if err := manager.UpdatePKIControl(req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return c.JSON(http.StatusOK, map[string]string{"resp": "ok"})
	})

	// CA Cert download endpoint
	group.GET("/ca.pem", func(c echo.Context) error {
		if manager == nil || !manager.IsCAValid() {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "CA is not initialized"})
		}
		c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="ca.pem"`)
		return c.Blob(http.StatusOK, "application/x-pem-file", []byte(manager.GetCACertPEM()))
	})

	// SCEP CA Cert download endpoint
	group.GET("/scepca.pem", func(c echo.Context) error {
		if manager == nil || !manager.IsCAValid() {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "CA is not initialized"})
		}
		c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="scepca.pem"`)
		return c.Blob(http.StatusOK, "application/x-pem-file", []byte(manager.GetSCEPCACertPEM()))
	})

	// CRL download endpoint
	group.GET("/crl", func(c echo.Context) error {
		if manager == nil || !manager.IsCAValid() {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "CA is not initialized"})
		}
		c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="crl.der"`)
		return c.Blob(http.StatusOK, "application/pkix-crl", manager.GetCRL())
	})
}
