package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/twsnmp/twsnmpneo/backend/internal/ai"
	"github.com/twsnmp/twsnmpneo/backend/internal/auth"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/discover"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
	"github.com/twsnmp/twsnmpneo/backend/internal/layout"
	"github.com/twsnmp/twsnmpneo/backend/internal/logreport"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
	"github.com/twsnmp/twsnmpneo/backend/internal/monitor"
	"github.com/twsnmp/twsnmpneo/backend/internal/notify"
	"github.com/twsnmp/twsnmpneo/backend/internal/ping"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
	"github.com/twsnmp/twsnmpneo/backend/internal/polling"
	"github.com/twsnmp/twsnmpneo/backend/internal/topology"
	"github.com/twsnmp/twsnmpneo/backend/internal/wol"
	"github.com/twsnmp/twsnmpneo/backend/web"
)

type Server struct {
	echo       *echo.Echo
	port       int
	store      datastore.DataStore
	logStore   *parquet.Store
	mcpServer  *ai.MCPServer
	pki        *pki.Manager
	pkiServers *pkiServiceServers
	authMgr    *auth.Manager
}

type ArpManager interface {
	DeleteArpEntries(ips []string)
	ResetArpTable()
}

type Config struct {
	Port           int
	Debug          bool
	Version        string
	Commit         string
	DataDir        string
	Store          datastore.DataStore
	LogStore       *parquet.Store
	MCPServer      *ai.MCPServer
	PKI            *pki.Manager
	ACMEBaseURL    string
	ArpManager     ArpManager
	Monitor        *monitor.Monitor
	PollingManager *polling.Manager
	Receivers      map[string]any
	AuthManager    *auth.Manager
}

func NewServer(cfg Config) (*Server, error) {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middlewares
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	authMgr := cfg.AuthManager
	if authMgr == nil && cfg.Store != nil {
		var aErr error
		authMgr, aErr = auth.NewManager(cfg.Store)
		if aErr != nil {
			return nil, fmt.Errorf("init auth manager: %w", aErr)
		}
	}

	if cfg.ACMEBaseURL != "" && cfg.PKI == nil {
		return nil, fmt.Errorf("ACME service requires a PKI manager")
	}
	var pkiServers *pkiServiceServers
	if cfg.PKI != nil {
		if cfg.ACMEBaseURL != "" {
			settings := cfg.PKI.Settings()
			settings.ACMEBaseURL = cfg.ACMEBaseURL
			settings.EnableACME = true
			acmeURL, err := url.Parse(cfg.ACMEBaseURL)
			if err != nil {
				return nil, fmt.Errorf("parse ACME base URL: %w", err)
			}
			if acmeURL.Port() != "" {
				port, err := strconv.Atoi(acmeURL.Port())
				if err != nil || port < 1 || port > 65535 {
					return nil, fmt.Errorf("invalid ACME listener port %q", acmeURL.Port())
				}
				settings.ACMEPort = port
			}
			if err := cfg.PKI.UpdateSettings(settings); err != nil {
				return nil, fmt.Errorf("apply ACME command-line settings: %w", err)
			}
		}
		pkiServers = newPKIServiceServers(cfg.PKI, cfg.Store)
	}

	// API Group
	apiGroup := e.Group("/api")

	// Authentication middleware
	apiGroup.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Path()
			reqPath := c.Request().URL.Path
			// Exclude public endpoints
			if path == "/api/login" || reqPath == "/api/login" ||
				path == "/api/logout" || reqPath == "/api/logout" ||
				path == "/api/health" || reqPath == "/api/health" ||
				(cfg.MCPServer != nil && cfg.MCPServer.Mode() != "auth" && (strings.HasPrefix(path, "/api/mcp") || strings.HasPrefix(reqPath, "/api/mcp"))) ||
				strings.HasPrefix(path, "/api/notify/oauth2") || strings.HasPrefix(reqPath, "/api/notify/oauth2") {
				return next(c)
			}
			if authMgr == nil {
				return next(c)
			}

			// 1. Check Bearer token in Authorization header
			var tokenStr string
			authHeader := c.Request().Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}

			// 2. Fallback to Cookie
			if tokenStr == "" {
				if cookie, err := c.Cookie(auth.SessionCookieName); err == nil && cookie.Value != "" {
					tokenStr = cookie.Value
				}
			}

			if tokenStr == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}

			claims, err := authMgr.ValidateToken(tokenStr)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			}

			// Retrieve user from store
			if cfg.Store != nil {
				user, err := cfg.Store.GetUser(c.Request().Context(), claims.User)
				if err != nil || user == nil {
					return c.JSON(http.StatusUnauthorized, map[string]string{"error": "user not found"})
				}
				c.Set("user", user)

				// Enforce read-only role restrictions
				if user.Role == "readonly" {
					method := c.Request().Method
					if method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete || method == http.MethodPatch {
						// Allow non-mutating diagnostics and session operations
						isAllowed := reqPath == "/api/logout" ||
							strings.HasPrefix(reqPath, "/api/tools/ping") ||
							strings.HasPrefix(reqPath, "/api/tools/snmp") ||
							strings.HasPrefix(reqPath, "/api/tools/gnmi") ||
							strings.HasPrefix(reqPath, "/api/ai/ask") ||
							strings.HasPrefix(reqPath, "/api/ai/diagnose")
						if !isAllowed {
							return c.JSON(http.StatusForbidden, map[string]string{"error": "permission denied: read-only user cannot perform write operations"})
						}
					}
				}
			}

			return next(c)
		}
	})

	// Auth routes
	apiGroup.POST("/login", func(c echo.Context) error {
		if authMgr == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "auth not initialized"})
		}
		var req struct {
			User     string `json:"user"`
			Password string `json:"password"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		user, token, err := authMgr.Authenticate(c.Request().Context(), req.User, req.Password)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		}

		c.SetCookie(&http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   c.IsTLS() || strings.EqualFold(c.Request().Header.Get("X-Forwarded-Proto"), "https"),
			MaxAge:   int(auth.DefaultTokenDuration.Seconds()),
		})

		return c.JSON(http.StatusOK, map[string]any{
			"token": token,
			"user":  user.ToPublic(),
		})
	})

	apiGroup.POST("/logout", func(c echo.Context) error {
		c.SetCookie(&http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
		})
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	apiGroup.GET("/me", func(c echo.Context) error {
		u := c.Get("user")
		if user, ok := u.(*datastore.UserEnt); ok && user != nil {
			return c.JSON(http.StatusOK, user.ToPublic())
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	})

	// User management routes
	apiGroup.GET("/users", func(c echo.Context) error {
		if cfg.Store == nil {
			return c.JSON(http.StatusOK, []*datastore.UserEnt{})
		}
		caller, _ := c.Get("user").(*datastore.UserEnt)
		if caller == nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}
		if caller.Role != "admin" {
			// Non-admin can only see their own profile in list
			return c.JSON(http.StatusOK, []*datastore.UserEnt{caller.ToPublic()})
		}
		users, err := cfg.Store.ListUsers(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		res := make([]*datastore.UserEnt, len(users))
		for i, u := range users {
			res[i] = u.ToPublic()
		}
		return c.JSON(http.StatusOK, res)
	})

	apiGroup.POST("/users", func(c echo.Context) error {
		if cfg.Store == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "store not available"})
		}
		caller, _ := c.Get("user").(*datastore.UserEnt)
		if caller == nil || caller.Role != "admin" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "admin privilege required to create users"})
		}
		var req struct {
			User     string `json:"user"`
			Name     string `json:"name"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		req.User = strings.TrimSpace(req.User)
		req.Name = strings.TrimSpace(req.Name)
		if req.User == "" || req.Password == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		}
		if req.Role == "" {
			req.Role = "user"
		}
		if req.Name == "" {
			req.Name = req.User
		}

		existing, _ := cfg.Store.GetUser(c.Request().Context(), req.User)
		if existing != nil {
			return c.JSON(http.StatusConflict, map[string]string{"error": "user already exists"})
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		}

		now := time.Now().Unix()
		newUser := &datastore.UserEnt{
			User:         req.User,
			Name:         req.Name,
			PasswordHash: hash,
			Role:         req.Role,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := cfg.Store.SaveUser(c.Request().Context(), newUser); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, newUser.ToPublic())
	})

	apiGroup.PUT("/users/:user", func(c echo.Context) error {
		if cfg.Store == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "store not available"})
		}
		caller, _ := c.Get("user").(*datastore.UserEnt)
		if caller == nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}
		username := c.Param("user")
		if caller.Role != "admin" && caller.User != username {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden: cannot modify other users"})
		}

		user, err := cfg.Store.GetUser(c.Request().Context(), username)
		if err != nil || user == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
		}

		var req struct {
			Name     string `json:"name"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}

		if req.Name != "" {
			user.Name = strings.TrimSpace(req.Name)
		}
		if req.Password != "" {
			hash, err := auth.HashPassword(req.Password)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
			}
			user.PasswordHash = hash
		}
		if req.Role != "" {
			if caller.Role != "admin" && req.Role != user.Role {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden: only administrator can change roles"})
			}
			if user.Role == "admin" && req.Role != "admin" {
				allUsers, _ := cfg.Store.ListUsers(c.Request().Context())
				adminCount := 0
				for _, u := range allUsers {
					if u.Role == "admin" {
						adminCount++
					}
				}
				if adminCount <= 1 {
					return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot demote last administrator"})
				}
			}
			user.Role = req.Role
		}

		user.UpdatedAt = time.Now().Unix()
		if err := cfg.Store.SaveUser(c.Request().Context(), user); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, user.ToPublic())
	})

	apiGroup.DELETE("/users/:user", func(c echo.Context) error {
		if cfg.Store == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "store not available"})
		}
		caller, _ := c.Get("user").(*datastore.UserEnt)
		if caller == nil || caller.Role != "admin" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "admin privilege required to delete users"})
		}
		username := c.Param("user")
		user, err := cfg.Store.GetUser(c.Request().Context(), username)
		if err != nil || user == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
		}

		totalUsers, _ := cfg.Store.CountUsers(c.Request().Context())
		if totalUsers <= 1 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot delete the only remaining user"})
		}

		if user.Role == "admin" {
			allUsers, _ := cfg.Store.ListUsers(c.Request().Context())
			adminCount := 0
			for _, u := range allUsers {
				if u.Role == "admin" {
					adminCount++
				}
			}
			if adminCount <= 1 {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot delete the last administrator"})
			}
		}

		if err := cfg.Store.DeleteUser(c.Request().Context(), username); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	if cfg.Store != nil {
		registerGNMIToolRoutes(apiGroup, cfg.Store)
	}
	if cfg.PKI != nil {
		registerPKIRoutes(apiGroup, cfg.PKI, cfg.Store, pkiServers.Apply)
		registerPKIProtocolRoutes(e, cfg.PKI, cfg.Store)
	}
	apiGroup.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"status":  "ok",
			"time":    time.Now().Format(time.RFC3339),
			"version": cfg.Version,
		})
	})


	// System Information & Resource Monitor
	apiGroup.GET("/system/info", func(c echo.Context) error {
		uptime := ""
		if cfg.Monitor != nil {
			uptime = cfg.Monitor.GetUptime().Truncate(time.Second).String()
		}
		nodeCount := 0
		pollCount := 0
		if cfg.Store != nil {
			nodes, _ := cfg.Store.ListNodes(c.Request().Context())
			nodeCount = len(nodes)
			polls, _ := cfg.Store.ListPollings(c.Request().Context())
			pollCount = len(polls)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"version":    cfg.Version,
			"commit":     cfg.Commit,
			"status":     "ok",
			"time":       time.Now().Format(time.RFC3339),
			"uptime":     uptime,
			"node_count": nodeCount,
			"poll_count": pollCount,
			"ping_mode":  ping.GetPingMode(),
			"receivers":  cfg.Receivers,
		})
	})

	apiGroup.GET("/system/monitor", func(c echo.Context) error {
		if cfg.Monitor != nil {
			return c.JSON(http.StatusOK, cfg.Monitor.GetData())
		}
		return c.JSON(http.StatusOK, []*monitor.MonitorDataEnt{})
	})

	apiGroup.POST("/system/monitor/update", func(c echo.Context) error {
		if cfg.Monitor != nil {
			data := cfg.Monitor.UpdateNow()
			return c.JSON(http.StatusOK, data)
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	apiGroup.POST("/system/backup", func(c echo.Context) error {
		if cfg.Monitor != nil {
			file, size, err := cfg.Monitor.Backup()
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]any{
				"file": file,
				"size": size,
				"time": time.Now().Format(time.RFC3339),
			})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "monitor service not available"})
	})

	// MCP Endpoint (Streamable HTTP JSON-RPC)
	if cfg.MCPServer != nil {
		mcpHandler := func(c echo.Context) error {
			if !cfg.MCPServer.IsEnabled() {
				return c.NoContent(http.StatusNotFound)
			}
			if !cfg.MCPServer.CheckFromAddress(c.Request().RemoteAddr, c.RealIP()) {
				return echo.ErrUnauthorized
			}
			cfg.MCPServer.ServeHTTP(c.Response().Writer, c.Request())
			return nil
		}
		apiGroup.Any("/mcp", mcpHandler)
		apiGroup.Any("/mcp/*", mcpHandler)
	}

	// AI Assistant Endpoints
	if cfg.Store != nil {
		apiGroup.POST("/ai/ask", func(c echo.Context) error {
			var req struct {
				System string `json:"system"`
				Prompt string `json:"prompt"`
			}
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			conf, err := cfg.Store.GetMapConf(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			client := ai.NewLLMClient(conf)
			ans, err := client.GenerateAnswer(c.Request().Context(), req.System, req.Prompt)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"answer": ans})
		})

		apiGroup.POST("/ai/diagnose", func(c echo.Context) error {
			var req struct {
				AlertEvent  string `json:"alert_event"`
				NodeContext string `json:"node_context"`
			}
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			conf, err := cfg.Store.GetMapConf(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			client := ai.NewLLMClient(conf)
			diag, err := client.DiagnoseAlert(c.Request().Context(), req.AlertEvent, req.NodeContext)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"diagnosis": diag})
		})

		// AI Anomaly Detection Endpoints
		anomalySvc := ai.NewAnomalyService(cfg.Store, cfg.LogStore)

		apiGroup.GET("/ai/list", func(c echo.Context) error {
			list, err := anomalySvc.GetAIList(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, list)
		})

		apiGroup.GET("/report/ai", func(c echo.Context) error {
			list, err := anomalySvc.GetAIList(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, list)
		})

		// Sensors report (Remote reporting agents: syslog, mqtt, sflow, netflow, twWifiScan, etc.)
		apiGroup.GET("/report/sensors", func(c echo.Context) error {
			list, err := cfg.Store.ListSensors(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if list == nil {
				list = []*datastore.SensorEnt{}
			}
			return c.JSON(http.StatusOK, list)
		})

		apiGroup.GET("/report/sensor/stats/:id", func(c echo.Context) error {
			id := c.Param("id")
			sn, err := cfg.Store.GetSensor(c.Request().Context(), id)
			if err != nil || sn == nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "sensor not found"})
			}
			stats := sn.Stats
			if stats == nil {
				stats = []datastore.SensorStatsEnt{}
			}
			return c.JSON(http.StatusOK, stats)
		})

		apiGroup.GET("/report/sensor/monitors/:id", func(c echo.Context) error {
			id := c.Param("id")
			sn, err := cfg.Store.GetSensor(c.Request().Context(), id)
			if err != nil || sn == nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "sensor not found"})
			}
			monitors := sn.Monitors
			if monitors == nil {
				monitors = []datastore.SensorMonitorEnt{}
			}
			return c.JSON(http.StatusOK, monitors)
		})

		apiGroup.DELETE("/report/sensor/:id", func(c echo.Context) error {
			id := c.Param("id")
			if id == "all" {
				if err := cfg.Store.DeleteAllSensors(c.Request().Context()); err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
				}
			} else {
				if err := cfg.Store.DeleteSensors(c.Request().Context(), []string{id}); err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
				}
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "ok", "resp": "ok"})
		})

		apiGroup.POST("/report/sensor/:id", func(c echo.Context) error {
			id := c.Param("id")
			if err := cfg.Store.ToggleSensorIgnore(c.Request().Context(), id); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "ok", "resp": "ok"})
		})

		// Log-derived reports (Wi-Fi AP, Bluetooth, packet capture, Windows event)
		// built by twwifiscan / twbluescan / twpcap / twwinlog pollings.
		apiGroup.GET("/report/log/:kind", func(c echo.Context) error {
			kind := c.Param("kind")
			if !logreport.IsKind(kind) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "unknown report kind"})
			}
			items, err := cfg.Store.ListLogReportData(c.Request().Context(), kind)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			list := make([]json.RawMessage, 0, len(items))
			for _, b := range items {
				list = append(list, json.RawMessage(b))
			}
			return c.JSON(http.StatusOK, list)
		})

		apiGroup.DELETE("/report/log/:kind", func(c echo.Context) error {
			kind := c.Param("kind")
			if !logreport.IsKind(kind) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "unknown report kind"})
			}
			if err := cfg.Store.ResetLogReportData(c.Request().Context(), kind); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		apiGroup.DELETE("/report/log/:kind/:id", func(c echo.Context) error {
			kind := c.Param("kind")
			id := c.Param("id")
			if !logreport.IsKind(kind) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "unknown report kind"})
			}
			if err := cfg.Store.DeleteLogReportData(c.Request().Context(), kind, []string{id}); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		apiGroup.POST("/report/log/name", func(c echo.Context) error {
			var req struct {
				Kind string `json:"Kind"`
				ID   string `json:"ID"`
				Name string `json:"Name"`
			}
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			kind := req.Kind
			switch kind {
			case "env":
				kind = logreport.KindEnvMonitor
			case "power":
				kind = logreport.KindPowerMonitor
			case "motion":
				kind = logreport.KindMotionSensor
			case "device":
				kind = logreport.KindBlueDevice
			}
			if !logreport.IsKind(kind) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "unknown report kind"})
			}
			raw, err := cfg.Store.GetLogReportData(c.Request().Context(), kind, req.ID)
			if err != nil || len(raw) == 0 {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "entity not found"})
			}
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			m["Name"] = req.Name
			updated, err := json.Marshal(m)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if err := cfg.Store.SaveLogReportData(c.Request().Context(), kind, map[string][]byte{req.ID: updated}); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		// Flow & Server Reports
		apiGroup.GET("/report/flow", func(c echo.Context) error {
			items, err := cfg.Store.ListLogReportData(c.Request().Context(), logreport.KindFlow)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			list := make([]json.RawMessage, 0, len(items))
			for _, b := range items {
				list = append(list, json.RawMessage(b))
			}
			return c.JSON(http.StatusOK, list)
		})
		apiGroup.GET("/report/server", func(c echo.Context) error {
			items, err := cfg.Store.ListLogReportData(c.Request().Context(), logreport.KindServer)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			list := make([]json.RawMessage, 0, len(items))
			for _, b := range items {
				list = append(list, json.RawMessage(b))
			}
			return c.JSON(http.StatusOK, list)
		})
		apiGroup.GET("/report/fumble", func(c echo.Context) error {
			items, err := cfg.Store.ListLogReportData(c.Request().Context(), logreport.KindFumble)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			list := make([]json.RawMessage, 0, len(items))
			for _, b := range items {
				list = append(list, json.RawMessage(b))
			}
			return c.JSON(http.StatusOK, list)
		})
		apiGroup.DELETE("/report/flow", func(c echo.Context) error {
			ctx := c.Request().Context()
			_ = cfg.Store.ResetLogReportData(ctx, logreport.KindFlow)
			_ = cfg.Store.ResetLogReportData(ctx, logreport.KindServer)
			_ = cfg.Store.ResetLogReportData(ctx, logreport.KindFumble)
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		// Syslog Statistics Report
		apiGroup.GET("/report/syslog/stats", func(c echo.Context) error {
			raw, err := cfg.Store.GetLogReportData(c.Request().Context(), logreport.KindSyslogStats, "summary")
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if len(raw) == 0 {
				return c.JSON(http.StatusOK, &logreport.SyslogStatsSummary{
					ID:         "summary",
					Hosts:      make(map[string]*logreport.SyslogHostStat),
					Tags:       make(map[string]*logreport.SyslogTagStat),
					Facilities: make(map[int]int64),
					Severities: make(map[int]int64),
				})
			}
			var sum logreport.SyslogStatsSummary
			if err := json.Unmarshal(raw, &sum); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, &sum)
		})
		apiGroup.DELETE("/report/syslog/stats", func(c echo.Context) error {
			_ = cfg.Store.ResetLogReportData(c.Request().Context(), logreport.KindSyslogStats)
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		// SNMP Trap Statistics Report
		apiGroup.GET("/report/trap/stats", func(c echo.Context) error {
			raw, err := cfg.Store.GetLogReportData(c.Request().Context(), logreport.KindTrapStats, "summary")
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if len(raw) == 0 {
				return c.JSON(http.StatusOK, &logreport.TrapStatsSummary{
					ID:          "summary",
					Hosts:       make(map[string]*logreport.TrapHostStat),
					Types:       make(map[string]*logreport.TrapTypeStat),
					Enterprises: make(map[string]int64),
				})
			}
			var sum logreport.TrapStatsSummary
			if err := json.Unmarshal(raw, &sum); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, &sum)
		})
		apiGroup.DELETE("/report/trap/stats", func(c echo.Context) error {
			_ = cfg.Store.ResetLogReportData(c.Request().Context(), logreport.KindTrapStats)
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		apiGroup.GET("/ai/result/:id", func(c echo.Context) error {
			id := c.Param("id")
			res, err := cfg.Store.GetAIResult(id)
			if err != nil || res == nil || len(res.ScoreData) < 1 {
				return c.JSON(http.StatusOK, datastore.AIResultEnt{PollingID: id, ScoreData: [][]float64{}})
			}
			return c.JSON(http.StatusOK, res)
		})

		apiGroup.DELETE("/ai/result/:id", func(c echo.Context) error {
			id := c.Param("id")
			if err := anomalySvc.DeleteAIResult(c.Request().Context(), id); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		apiGroup.GET("/ai/export/:id", func(c echo.Context) error {
			id := c.Param("id")
			ts := time.Now().Format("20060102150405")
			c.Response().Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
			c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=twsnmp_ai_data_%s_%s.csv", id, ts))
			return anomalySvc.ExportAIData(c.Request().Context(), id, c.Response().Writer)
		})

		apiGroup.POST("/ai/recheck", func(c echo.Context) error {
			anomalySvc.CheckAll(c.Request().Context())
			list, err := anomalySvc.GetAIList(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, list)
		})
	}

	if cfg.Store != nil {
		// Nodes
		apiGroup.GET("/nodes", func(c echo.Context) error {
			nodes, err := cfg.Store.ListNodes(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, nodes)
		})
		apiGroup.POST("/nodes", func(c echo.Context) error {
			var n datastore.NodeEnt
			if err := c.Bind(&n); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			isNew := n.ID == ""
			if isNew {
				n.ID = datastore.GenerateID()
			}

			// If address mode is host, resolve IP from node name
			if n.AddrMode == "host" {
				target := strings.TrimSpace(n.Name)
				if target == "" && n.IP != "" {
					target = strings.TrimSpace(n.IP)
				}
				if target != "" {
					if hitIP := resolveHostToIPv4(c.Request().Context(), target); hitIP != "" {
						n.IP = hitIP
					}
				}
			} else if n.IP != "" && net.ParseIP(strings.TrimSpace(n.IP)) == nil {
				// If IP field contains a hostname instead of a valid IP
				if hitIP := resolveHostToIPv4(c.Request().Context(), strings.TrimSpace(n.IP)); hitIP != "" {
					n.IP = hitIP
				}
			}

			if err := cfg.Store.SaveNode(c.Request().Context(), &n); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			var eventMsg string
			if isNew {
				eventMsg = fmt.Sprintf(i18n.Trans("Added node %s (%s)"), n.Name, n.IP)
			} else {
				eventMsg = fmt.Sprintf(i18n.Trans("Updated node %s (%s)"), n.Name, n.IP)
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:     time.Now().UnixNano(),
				Type:     "user",
				Level:    "info",
				NodeName: n.Name,
				NodeID:   n.ID,
				Event:    eventMsg,
			})
			return c.JSON(http.StatusOK, &n)
		})
		apiGroup.GET("/nodes/:id", func(c echo.Context) error {
			node, err := cfg.Store.GetNode(c.Request().Context(), c.Param("id"))
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "node not found"})
			}
			return c.JSON(http.StatusOK, node)
		})
		apiGroup.DELETE("/nodes/:id", func(c echo.Context) error {
			id := c.Param("id")
			node, _ := cfg.Store.GetNode(c.Request().Context(), id)
			name := id
			if node != nil {
				name = node.Name
			}
			if err := cfg.Store.DeleteNode(c.Request().Context(), id); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:     time.Now().UnixNano(),
				Type:     "user",
				Level:    "warn",
				NodeName: name,
				NodeID:   id,
				Event:    fmt.Sprintf(i18n.Trans("Delete node %s"), name),
			})
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})
		apiGroup.POST("/nodes/positions", func(c echo.Context) error {
			var req []struct {
				ID string `json:"ID"`
				X  int    `json:"X"`
				Y  int    `json:"Y"`
			}
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			var updated []*datastore.NodeEnt
			for _, item := range req {
				n, err := cfg.Store.GetNode(c.Request().Context(), item.ID)
				if err == nil && n != nil {
					n.X = item.X
					n.Y = item.Y
					updated = append(updated, n)
				}
			}
			if len(updated) > 0 {
				if err := cfg.Store.SaveNodes(c.Request().Context(), updated); err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
				}
			}
			return c.JSON(http.StatusOK, map[string]any{
				"status": "ok",
				"count":  len(updated),
			})
		})

		// Node SNMP Details (Host Resource, Ports, RMON)
		registerSNMPDetailEndpoints(apiGroup, cfg.Store)
		registerNodeDiagnoseEndpoints(apiGroup, cfg.Store)

		// Certificate Monitors (External TLS Monitor)
		apiGroup.GET("/cert_monitors", func(c echo.Context) error {
			list, err := cfg.Store.ListCertMonitors(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, list)
		})
		apiGroup.POST("/cert_monitors", func(c echo.Context) error {
			var ent datastore.CertMonitorEnt
			if err := c.Bind(&ent); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if ent.Target == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "target host is required"})
			}
			if ent.Port <= 0 {
				ent.Port = 443
			}
			monitor.CheckCert(&ent, 10)
			if err := cfg.Store.SaveCertMonitor(c.Request().Context(), &ent); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, ent)
		})
		apiGroup.DELETE("/cert_monitors/:id", func(c echo.Context) error {
			id := c.Param("id")
			if err := cfg.Store.DeleteCertMonitor(c.Request().Context(), id); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})
		apiGroup.POST("/cert_monitors/check", func(c echo.Context) error {
			if err := monitor.CheckAllCertMonitors(c.Request().Context(), cfg.Store); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			list, _ := cfg.Store.ListCertMonitors(c.Request().Context())
			return c.JSON(http.StatusOK, list)
		})

		// Pollings
		apiGroup.GET("/pollings", func(c echo.Context) error {
			polls, err := cfg.Store.ListPollings(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, polls)
		})
		apiGroup.POST("/pollings", func(c echo.Context) error {
			var p datastore.PollingEnt
			if err := c.Bind(&p); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			isNew := p.ID == ""
			if isNew {
				p.ID = datastore.GenerateID()
				if p.PollInt <= 0 {
					p.PollInt = 60
				}
				if p.Timeout <= 0 {
					p.Timeout = 1
				}
				if p.Retry <= 0 {
					p.Retry = 1
				}
				if p.NextTime <= 0 {
					p.NextTime = time.Now().UnixNano()
				}
				if p.State == "" {
					p.State = "unknown"
				}
			} else {
				existing, err := cfg.Store.GetPolling(c.Request().Context(), p.ID)
				if err == nil && existing != nil {
					if len(p.Result) == 0 {
						p.Result = existing.Result
					}
					if p.LastTime == 0 {
						p.LastTime = existing.LastTime
					}
					if p.PollInt <= 0 {
						p.PollInt = existing.PollInt
					}
					if p.Timeout <= 0 {
						p.Timeout = existing.Timeout
					}
					if p.Retry <= 0 {
						p.Retry = existing.Retry
					}
					if p.LogMode == 0 && existing.LogMode != 0 {
						p.LogMode = existing.LogMode
					}
					if p.State == "" {
						p.State = existing.State
					}
					if p.FailTime == 0 {
						p.FailTime = existing.FailTime
					}
					// Trigger immediate check upon modification
					p.NextTime = time.Now().UnixNano()
				}
			}
			if err := cfg.Store.SavePolling(c.Request().Context(), &p); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:   time.Now().UnixNano(),
				Type:   "user",
				Level:  "info",
				NodeID: p.NodeID,
				Event:  fmt.Sprintf(i18n.Trans("Saved polling %s"), p.Name),
			})
			return c.JSON(http.StatusOK, &p)
		})
		apiGroup.DELETE("/pollings/:id", func(c echo.Context) error {
			id := c.Param("id")
			p, _ := cfg.Store.GetPolling(c.Request().Context(), id)
			name := id
			if p != nil {
				name = p.Name
			}
			if err := cfg.Store.DeletePolling(c.Request().Context(), id); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "warn",
				Event: fmt.Sprintf(i18n.Trans("Delete polling %s"), name),
			})
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})
		apiGroup.POST("/polling/check-all", func(c echo.Context) error {
			count := 0
			if cfg.PollingManager != nil {
				count = cfg.PollingManager.CheckAll(c.Request().Context())
			}
			return c.JSON(http.StatusOK, map[string]any{
				"status": "ok",
				"count":  count,
			})
		})
		apiGroup.POST("/polling/check/:nodeID", func(c echo.Context) error {
			nodeID := c.Param("nodeID")
			if _, err := cfg.Store.GetNode(c.Request().Context(), nodeID); err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "node not found"})
			}
			count := 0
			if cfg.PollingManager != nil {
				var err error
				count, err = cfg.PollingManager.CheckNode(c.Request().Context(), nodeID)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
				}
			}
			return c.JSON(http.StatusOK, map[string]any{"status": "ok", "count": count})
		})

		// Polling Templates
		apiGroup.GET("/polling/templates", func(c echo.Context) error {
			lang := c.QueryParam("lang")
			if lang == "" {
				lang = i18n.GetLang()
			}
			templates, err := polling.LoadTemplates(lang)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, templates)
		})

		apiGroup.GET("/polling/template/:id", func(c echo.Context) error {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid template id"})
			}
			lang := c.QueryParam("lang")
			if lang == "" {
				lang = i18n.GetLang()
			}
			tmpl, err := polling.GetTemplate(lang, id)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, tmpl)
		})

		apiGroup.POST("/polling/auto", func(c echo.Context) error {
			var req struct {
				NodeID     string `json:"nodeID"`
				TemplateID int    `json:"templateID"`
				Lang       string `json:"lang"`
			}
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			lang := req.Lang
			if lang == "" {
				lang = i18n.GetLang()
			}
			tmpl, err := polling.GetTemplate(lang, req.TemplateID)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			var node *datastore.NodeEnt
			if req.NodeID != "" {
				node, _ = cfg.Store.GetNode(c.Request().Context(), req.NodeID)
			}
			pollings := polling.GenerateAutoPollings(c.Request().Context(), node, tmpl)
			return c.JSON(http.StatusOK, pollings)
		})

		apiGroup.POST("/polling/autogrok", func(c echo.Context) error {
			var req struct {
				TestData string `json:"testData"`
			}
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			pattern := polling.AutoGrok(req.TestData)
			return c.JSON(http.StatusOK, map[string]string{"pattern": pattern})
		})

		// Lines
		apiGroup.GET("/lines", func(c echo.Context) error {
			lines, err := cfg.Store.ListLines(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, lines)
		})
		apiGroup.POST("/lines", func(c echo.Context) error {
			var l datastore.LineEnt
			if err := c.Bind(&l); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			isNew := l.ID == ""
			if isNew {
				l.ID = datastore.GenerateID()
			}
			if err := cfg.Store.SaveLine(c.Request().Context(), &l); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: fmt.Sprintf(i18n.Trans("Update line %s - %s"), l.NodeID1, l.NodeID2),
			})
			return c.JSON(http.StatusOK, &l)
		})
		apiGroup.DELETE("/lines/:id", func(c echo.Context) error {
			id := c.Param("id")
			line, _ := cfg.Store.GetLine(c.Request().Context(), id)
			info := id
			if line != nil {
				info = fmt.Sprintf("%s - %s", line.NodeID1, line.NodeID2)
			}
			if err := cfg.Store.DeleteLine(c.Request().Context(), id); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: fmt.Sprintf(i18n.Trans("Delete line %s"), info),
			})
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})

		// Topology Discovery
		apiGroup.GET("/topology/neighbors/:id", func(c echo.Context) error {
			rawID := c.Param("id")
			id, err := url.PathUnescape(rawID)
			if err != nil {
				id = rawID
			}
			id = strings.ReplaceAll(id, "%3A", ":")

			// Check if discovery for ALL networks is requested
			if strings.EqualFold(id, "ALL") {
				resp, err := topology.FindAllTopology(c.Request().Context(), cfg.Store)
				if err != nil {
					return c.JSON(http.StatusOK, &topology.FindNeighborNetworksAndLinesResp{
						Networks: []*datastore.NetworkEnt{},
						Lines:    []topology.NeighborLineEnt{},
					})
				}
				return c.JSON(http.StatusOK, resp)
			}

			// 1. If ID has NET: prefix, or is found as a Network
			if strings.HasPrefix(id, "NET:") {
				netID := strings.TrimPrefix(id, "NET:")
				nw, err := cfg.Store.GetNetwork(c.Request().Context(), netID)
				if err == nil && nw != nil {
					resp, err := topology.FindTopologyForNetwork(c.Request().Context(), cfg.Store, nw)
					if err != nil {
						return c.JSON(http.StatusOK, &topology.FindNeighborNetworksAndLinesResp{
							Networks: []*datastore.NetworkEnt{},
							Lines:    []topology.NeighborLineEnt{},
						})
					}
					return c.JSON(http.StatusOK, resp)
				}
				// Fallback: check if netID is a node ID
				if node, err := cfg.Store.GetNode(c.Request().Context(), netID); err == nil && node != nil {
					lines, _ := topology.FindNodeConnection(c.Request().Context(), cfg.Store, node.ID)
					return c.JSON(http.StatusOK, &topology.FindNeighborNetworksAndLinesResp{
						Networks: []*datastore.NetworkEnt{},
						Lines:    lines,
					})
				}
				return c.JSON(http.StatusOK, &topology.FindNeighborNetworksAndLinesResp{
					Networks: []*datastore.NetworkEnt{},
					Lines:    []topology.NeighborLineEnt{},
				})
			}

			// 2. If ID has NODE: prefix, or is found as a Node
			cleanID := strings.TrimPrefix(id, "NODE:")
			node, err := cfg.Store.GetNode(c.Request().Context(), cleanID)
			if err == nil && node != nil {
				lines, err := topology.FindNodeConnection(c.Request().Context(), cfg.Store, cleanID)
				if err != nil {
					lines = []topology.NeighborLineEnt{}
				}
				return c.JSON(http.StatusOK, &topology.FindNeighborNetworksAndLinesResp{
					Networks: []*datastore.NetworkEnt{},
					Lines:    lines,
				})
			}

			// 3. Fallback: check if cleanID is actually a Network ID
			if nw, err := cfg.Store.GetNetwork(c.Request().Context(), cleanID); err == nil && nw != nil {
				resp, _ := topology.FindTopologyForNetwork(c.Request().Context(), cfg.Store, nw)
				if resp != nil {
					return c.JSON(http.StatusOK, resp)
				}
			}

			return c.JSON(http.StatusOK, &topology.FindNeighborNetworksAndLinesResp{
				Networks: []*datastore.NetworkEnt{},
				Lines:    []topology.NeighborLineEnt{},
			})
		})

		apiGroup.POST("/topology/connect-lines", func(c echo.Context) error {
			var lines []datastore.LineEnt
			if err := c.Bind(&lines); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			count, err := topology.ConnectCandidateLines(c.Request().Context(), cfg.Store, lines)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if count > 0 {
				_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
					Time:  time.Now().UnixNano(),
					Type:  "system",
					Level: "info",
					Event: fmt.Sprintf(i18n.Trans("Connected %d lines automatically"), count),
				})
			}
			return c.JSON(http.StatusOK, map[string]int{"connected": count})
		})

		// Discovery APIs
		apiGroup.GET("/discover/conf", func(c echo.Context) error {
			conf, err := cfg.Store.GetDiscoverConf(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, conf)
		})

		apiGroup.POST("/discover/conf", func(c echo.Context) error {
			var conf datastore.DiscoverConfEnt
			if err := c.Bind(&conf); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if err := cfg.Store.SaveDiscoverConf(c.Request().Context(), &conf); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, conf)
		})

		apiGroup.POST("/discover/start", func(c echo.Context) error {
			var conf datastore.DiscoverConfEnt
			if err := c.Bind(&conf); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if err := cfg.Store.SaveDiscoverConf(c.Request().Context(), &conf); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			engine := discover.GetDefaultEngine()
			if err := engine.StartDiscover(c.Request().Context(), cfg.Store, &conf); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]bool{"ok": true})
		})

		apiGroup.POST("/discover/stop", func(c echo.Context) error {
			discover.GetDefaultEngine().StopDiscover()
			return c.JSON(http.StatusOK, map[string]bool{"ok": true})
		})

		apiGroup.GET("/discover/stat", func(c echo.Context) error {
			stat := discover.GetDefaultEngine().GetDiscoverStats()
			return c.JSON(http.StatusOK, stat)
		})

		apiGroup.GET("/discover/ranges", func(c echo.Context) error {
			ranges := discover.GetDiscoverAddressRange()
			return c.JSON(http.StatusOK, ranges)
		})

		// Networks
		apiGroup.GET("/networks", func(c echo.Context) error {
			nets, err := cfg.Store.ListNetworks(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, nets)
		})
		apiGroup.POST("/networks", func(c echo.Context) error {
			var n datastore.NetworkEnt
			if err := c.Bind(&n); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			isNew := n.ID == ""
			if isNew {
				n.ID = datastore.GenerateID()
			} else {
				// Preserve existing SNMP settings and port details if omitted in update
				if old, err := cfg.Store.GetNetwork(c.Request().Context(), n.ID); err == nil && old != nil {
					if n.Community == "" && old.Community != "" {
						n.Community = old.Community
						n.SnmpMode = old.SnmpMode
						n.User = old.User
						n.Password = old.Password
					}
					if n.SystemID == "" && old.SystemID != "" {
						n.SystemID = old.SystemID
					}
					// Preserve port Index and Polling if missing
					oldPortMap := make(map[string]datastore.PortEnt)
					for _, op := range old.Ports {
						oldPortMap[op.ID] = op
					}
					for i := range n.Ports {
						if op, ok := oldPortMap[n.Ports[i].ID]; ok {
							if n.Ports[i].Index == "" {
								n.Ports[i].Index = op.Index
							}
							if n.Ports[i].Polling == "" {
								n.Ports[i].Polling = op.Polling
							}
						}
					}
				}
			}
			// If still missing SNMP settings, fallback to corresponding node with same IP
			if n.Community == "" && n.IP != "" {
				if nodes, err := cfg.Store.ListNodes(c.Request().Context()); err == nil {
					for _, nd := range nodes {
						if nd.IP == n.IP && (nd.Community != "" || strings.HasPrefix(nd.SnmpMode, "v3")) {
							n.Community = nd.Community
							n.SnmpMode = nd.SnmpMode
							n.User = nd.User
							n.Password = nd.Password
							if n.SnmpPort == 0 && nd.SnmpPort != 0 {
								n.SnmpPort = nd.SnmpPort
							}
							break
						}
					}
				}
			}
			if err := cfg.Store.SaveNetwork(c.Request().Context(), &n); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			var eventMsg string
			if isNew {
				eventMsg = fmt.Sprintf(i18n.Trans("Added network %s (%s)"), n.Name, n.IP)
			} else {
				eventMsg = fmt.Sprintf(i18n.Trans("Updated network %s (%s)"), n.Name, n.IP)
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: eventMsg,
			})
			return c.JSON(http.StatusOK, &n)
		})
		apiGroup.POST("/networks/:id/ports", func(c echo.Context) error {
			id := c.Param("id")
			nw, err := cfg.Store.GetNetwork(c.Request().Context(), id)
			if err != nil || nw == nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "network not found"})
			}
			ports, err := topology.FetchNetworkPorts(c.Request().Context(), nw, 3, 1)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			nw.Ports = ports
			nw.Error = ""
			if err := cfg.Store.SaveNetwork(c.Request().Context(), nw); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, nw)
		})
		apiGroup.POST("/networks/:id/check", func(c echo.Context) error {
			id := c.Param("id")
			nw, err := cfg.Store.GetNetwork(c.Request().Context(), id)
			if err != nil || nw == nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "network not found"})
			}
			if nw.Unmanaged {
				if nw.IP == "" {
					nw.Error = "network has no IP address"
					for i := range nw.Ports {
						nw.Ports[i].State = "unknown"
					}
				} else {
					result := ping.DoPing(nw.IP, 2, 1, 64, 64)
					if result.Stat == ping.PingOK {
						nw.Error = ""
						for i := range nw.Ports {
							nw.Ports[i].State = "up"
						}
					} else {
						nw.Error = "Ping No Response"
						for i := range nw.Ports {
							nw.Ports[i].State = "down"
						}
					}
				}
			} else {
				ports, err := topology.FetchNetworkPorts(c.Request().Context(), nw, 3, 1)
				if err != nil {
					nw.Error = err.Error()
					_ = cfg.Store.SaveNetwork(c.Request().Context(), nw)
					return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
				}
				nw.Ports = ports
				nw.Error = ""
			}
			if err := cfg.Store.SaveNetwork(c.Request().Context(), nw); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, nw)
		})
		apiGroup.DELETE("/networks/:id", func(c echo.Context) error {
			id := c.Param("id")
			nw, _ := cfg.Store.GetNetwork(c.Request().Context(), id)
			name := id
			if nw != nil {
				name = nw.Name
			}
			if err := cfg.Store.DeleteNetwork(c.Request().Context(), id); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "warn",
				Event: fmt.Sprintf(i18n.Trans("Delete network %s"), name),
			})
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})

		// DrawItems
		apiGroup.GET("/drawitems", func(c echo.Context) error {
			ctx := c.Request().Context()
			items, err := cfg.Store.ListDrawItems(ctx)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			for _, it := range items {
				checkDrawItem(ctx, cfg.Store, it)
			}
			return c.JSON(http.StatusOK, items)
		})
		apiGroup.GET("/drawitems/:id", func(c echo.Context) error {
			id := c.Param("id")
			item, err := cfg.Store.GetDrawItem(c.Request().Context(), id)
			if err != nil || item == nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "DrawItem not found"})
			}
			return c.JSON(http.StatusOK, item)
		})
		apiGroup.POST("/drawitems", func(c echo.Context) error {
			var item datastore.DrawItemEnt
			if err := c.Bind(&item); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			isNew := item.ID == ""
			if isNew {
				item.ID = datastore.GenerateID()
			}
			if item.Type == datastore.DrawItemTypePollingText && item.Text == "" {
				item.Text = "No Value"
			}
			if err := cfg.Store.SaveDrawItem(c.Request().Context(), &item); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			var eventMsg string
			if isNew {
				eventMsg = fmt.Sprintf(i18n.Trans("Added drawitem %s"), item.Text)
			} else {
				eventMsg = fmt.Sprintf(i18n.Trans("Updated drawitem %s"), item.Text)
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: eventMsg,
			})
			checkDrawItem(c.Request().Context(), cfg.Store, &item)
			return c.JSON(http.StatusOK, &item)
		})
		apiGroup.POST("/drawitems/:id/copy", func(c echo.Context) error {
			id := c.Param("id")
			ctx := c.Request().Context()
			ds, err := cfg.Store.GetDrawItem(ctx, id)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "DrawItem not found"})
			}
			di := *ds
			di.ID = datastore.GenerateID()
			di.X = ds.X + 100
			di.Y = ds.Y
			if err := cfg.Store.SaveDrawItem(ctx, &di); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			name := di.Text
			_ = cfg.Store.AddEventLog(ctx, &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: fmt.Sprintf(i18n.Trans("Copy DrawItem %s"), name),
			})
			checkDrawItem(ctx, cfg.Store, &di)
			return c.JSON(http.StatusOK, &di)
		})
		apiGroup.DELETE("/drawitems/:id", func(c echo.Context) error {
			id := c.Param("id")
			if err := cfg.Store.DeleteDrawItem(c.Request().Context(), id); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: i18n.Trans("Delete drawitem"),
			})
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})

		// Map Conf
		getMapConfHandler := func(c echo.Context) error {
			conf, err := cfg.Store.GetMapConf(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if conf != nil {
				conf.GeoIPInfo = datastore.GetGeoIPInfo()
			}
			return c.JSON(http.StatusOK, conf)
		}
		apiGroup.GET("/map/conf", getMapConfHandler)
		apiGroup.GET("/conf/map", getMapConfHandler)
		apiGroup.POST("/map/conf", func(c echo.Context) error {
			var conf datastore.MapConfEnt
			if err := c.Bind(&conf); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if err := cfg.Store.SaveMapConf(c.Request().Context(), &conf); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if cfg.MCPServer != nil {
				cfg.MCPServer.UpdateConf(&conf)
			}
			conf.GeoIPInfo = datastore.GetGeoIPInfo()
			return c.JSON(http.StatusOK, &conf)
		})

		// Custom Icons
		apiGroup.GET("/icons", func(c echo.Context) error {
			icons, err := cfg.Store.GetCustomIcons(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, icons)
		})
		apiGroup.POST("/icons", func(c echo.Context) error {
			var icon datastore.IconEnt
			if err := c.Bind(&icon); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if icon.Name == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
			}
			if icon.Type != "image" && icon.Image == "" && icon.Code == 0 {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "code or image is required"})
			}
			if err := cfg.Store.SaveCustomIcon(c.Request().Context(), &icon); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: fmt.Sprintf(i18n.Trans("Saved icon %s"), icon.Name),
			})
			return c.JSON(http.StatusOK, &icon)
		})
		apiGroup.POST("/icons/batch", func(c echo.Context) error {
			var icons []*datastore.IconEnt
			if err := c.Bind(&icons); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if err := cfg.Store.SaveCustomIcons(c.Request().Context(), icons); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: fmt.Sprintf(i18n.Trans("Updated %d icons"), len(icons)),
			})
			return c.JSON(http.StatusOK, icons)
		})
		apiGroup.DELETE("/icons/:name", func(c echo.Context) error {
			name := c.Param("name")
			if err := cfg.Store.DeleteCustomIcon(c.Request().Context(), name); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: fmt.Sprintf(i18n.Trans("Deleted icon %s"), name),
			})
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})

		// Auto Layout endpoints
		apiGroup.POST("/map/autolayout", func(c echo.Context) error {
			var req struct {
				Mode int `json:"mode"`
			}
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			count, err := layout.OptimizeLayout(c.Request().Context(), cfg.Store, req.Mode)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]any{
				"count":   count,
				"hasUndo": layout.HasUndoLayout(),
			})
		})

		apiGroup.POST("/map/autolayout/undo", func(c echo.Context) error {
			count, err := layout.UndoLayout(c.Request().Context(), cfg.Store)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]any{
				"count":   count,
				"hasUndo": layout.HasUndoLayout(),
			})
		})

		apiGroup.GET("/map/autolayout/undo", func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]any{
				"hasUndo": layout.HasUndoLayout(),
			})
		})

		// BackImage endpoints
		apiGroup.GET("/map/backimage", func(c echo.Context) error {
			bi, err := cfg.Store.GetBackImage(c.Request().Context())
			if err != nil || bi == nil {
				return c.JSON(http.StatusOK, datastore.BackImageEnt{Width: 100, Height: 100})
			}
			return c.JSON(http.StatusOK, bi)
		})

		apiGroup.POST("/map/backimage", func(c echo.Context) error {
			var bi datastore.BackImageEnt
			if err := c.Bind(&bi); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if err := cfg.Store.SaveBackImage(c.Request().Context(), &bi); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, bi)
		})

		apiGroup.DELETE("/map/backimage", func(c echo.Context) error {
			empty := &datastore.BackImageEnt{}
			if err := cfg.Store.SaveBackImage(c.Request().Context(), empty); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		apiGroup.POST("/map/backimage/upload", func(c echo.Context) error {
			file, err := c.FormFile("image")
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "no image provided"})
			}
			src, err := file.Open()
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			defer src.Close()

			targetDir := cfg.DataDir
			if targetDir == "" {
				targetDir = "./data"
			}
			imgDir := filepath.Join(targetDir, "images")
			_ = os.MkdirAll(imgDir, 0755)
			safeName := "backimage_" + filepath.Base(file.Filename)
			dstPath := filepath.Join(imgDir, safeName)
			dst, err := os.Create(dstPath)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			defer dst.Close()
			if _, err := io.Copy(dst, src); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			url := "/api/map/image/" + safeName
			return c.JSON(http.StatusOK, map[string]string{
				"path": url,
			})
		})

		apiGroup.GET("/map/image/:name", func(c echo.Context) error {
			name := filepath.Base(c.Param("name"))
			targetDir := cfg.DataDir
			if targetDir == "" {
				targetDir = "./data"
			}
			path := filepath.Join(targetDir, "images", name)
			if _, err := os.Stat(path); err != nil {
				return c.NoContent(http.StatusNotFound)
			}
			return c.File(path)
		})

		// Map Import endpoint
		apiGroup.POST("/map/import", func(c echo.Context) error {
			var req struct {
				Nodes     []*datastore.NodeEnt     `json:"nodes"`
				Lines     []*datastore.LineEnt     `json:"lines"`
				Networks  []*datastore.NetworkEnt  `json:"networks"`
				DrawItems []*datastore.DrawItemEnt `json:"drawItems"`
			}
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			ctx := c.Request().Context()
			if len(req.Nodes) > 0 {
				_ = cfg.Store.SaveNodes(ctx, req.Nodes)
			}
			for _, nw := range req.Networks {
				_ = cfg.Store.SaveNetwork(ctx, nw)
			}
			for _, l := range req.Lines {
				_ = cfg.Store.SaveLine(ctx, l)
			}
			for _, di := range req.DrawItems {
				_ = cfg.Store.SaveDrawItem(ctx, di)
			}
			_ = cfg.Store.AddEventLog(ctx, &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "system",
				Level: "info",
				Event: fmt.Sprintf("Imported map: %d nodes, %d lines, %d networks, %d draw items",
					len(req.Nodes), len(req.Lines), len(req.Networks), len(req.DrawItems)),
			})
			return c.JSON(http.StatusOK, map[string]any{
				"status":    "ok",
				"nodes":     len(req.Nodes),
				"lines":     len(req.Lines),
				"networks":  len(req.Networks),
				"drawItems": len(req.DrawItems),
			})
		})

		// GeoIP DB endpoints (TWSNMP FC / FK compatible)
		postGeoIPHandler := func(c echo.Context) error {
			f, err := c.FormFile("file")
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "file field required"})
			}
			if f.Size > 200*1024*1024 {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "file size exceeds 200MB limit"})
			}
			src, err := f.Open()
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to open uploaded file"})
			}
			defer src.Close()

			targetDir := cfg.DataDir
			if targetDir == "" {
				targetDir = "./data"
			}
			if err := datastore.UpdateGeoIP(targetDir, src); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid geoip database: %v", err)})
			}

			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: i18n.Trans("Updated GeoIP database"),
			})
			return c.JSON(http.StatusOK, map[string]string{"resp": "ok", "version": datastore.GetGeoIPInfo()})
		}

		deleteGeoIPHandler := func(c echo.Context) error {
			targetDir := cfg.DataDir
			if targetDir == "" {
				targetDir = "./data"
			}
			if err := datastore.DeleteGeoIP(targetDir); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: i18n.Trans("Delete geoip database"),
			})
			return c.JSON(http.StatusOK, map[string]string{"resp": "ok"})
		}

		apiGroup.POST("/conf/geoip", postGeoIPHandler)
		apiGroup.DELETE("/conf/geoip", deleteGeoIPHandler)
		apiGroup.POST("/map/geoip", postGeoIPHandler)
		apiGroup.DELETE("/map/geoip", deleteGeoIPHandler)

		// Notify Conf
		apiGroup.GET("/notify/conf", func(c echo.Context) error {
			conf, err := cfg.Store.GetNotifyConf(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, conf)
		})
		apiGroup.POST("/notify/conf", func(c echo.Context) error {
			var conf datastore.NotifyConfEnt
			if err := c.Bind(&conf); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if err := cfg.Store.SaveNotifyConf(c.Request().Context(), &conf); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, &conf)
		})
		apiGroup.POST("/notify/test/mail", func(c echo.Context) error {
			var conf datastore.NotifyConfEnt
			if err := c.Bind(&conf); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if err := notify.SendTestMail(cfg.Store, &conf); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})
		apiGroup.POST("/notify/test/webhook", func(c echo.Context) error {
			var conf datastore.NotifyConfEnt
			if err := c.Bind(&conf); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if err := notify.WebHookTest(cfg.Store, &conf); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})
		apiGroup.POST("/notify/oauth2/start", func(c echo.Context) error {
			redirectURL, err := notifyOAuth2RedirectURL(c)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			url, err := notify.GetNotifyOAuth2TokenStep1(cfg.Store, redirectURL)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"url": url})
		})
		apiGroup.GET("/notify/oauth2/callback", func(c echo.Context) error {
			if c.QueryParam("error") != "" {
				return c.String(http.StatusBadRequest, "OAuth2 authorization was denied. You can close this window.")
			}
			if err := notify.CompleteNotifyOAuth2Callback(cfg.Store, c.QueryParam("code"), c.QueryParam("state")); err != nil {
				return c.String(http.StatusBadRequest, "OAuth2 authorization failed. You can close this window.")
			}
			return c.HTML(http.StatusOK, `<!doctype html><html><body><p>OAuth2 authorization complete. This window will close.</p><script>
				window.opener?.postMessage({ type: "twsnmpneo-notify-oauth2-complete" }, window.location.origin);
				window.close();
			</script></body></html>`)
		})
		apiGroup.DELETE("/notify/oauth2/token", func(c echo.Context) error {
			notify.DeleteNotifyOAuth2Token(cfg.Store)
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})
		apiGroup.GET("/notify/oauth2/status", func(c echo.Context) error {
			conf, _ := cfg.Store.GetNotifyConf(c.Request().Context())
			hasToken := false
			if conf != nil {
				hasToken = cfg.Store.HasValidNotifyOAuth2Token(conf)
			}
			return c.JSON(http.StatusOK, map[string]bool{"hasToken": hasToken})
		})

		// Event Logs
		apiGroup.GET("/logs/events", func(c echo.Context) error {
			var startTime int64
			var endTime int64
			if s := c.QueryParam("start"); s != "" {
				startTime, _ = strconv.ParseInt(s, 10, 64)
			}
			if e := c.QueryParam("end"); e != "" {
				endTime, _ = strconv.ParseInt(e, 10, 64)
			}
			limit := 10000
			if l := c.QueryParam("limit"); l != "" {
				if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
					limit = parsed
				}
			}
			logs, err := cfg.Store.QueryEventLogs(c.Request().Context(), datastore.EventLogFilter{
				StartTime: startTime,
				EndTime:   endTime,
				Level:     c.QueryParam("level"),
				Type:      c.QueryParam("type"),
				NodeID:    c.QueryParam("nodeId"),
				NodeName:  c.QueryParam("nodeName"),
				Filter:    c.QueryParam("filter"),
				Limit:     limit,
			})
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, logs)
		})

		apiGroup.DELETE("/logs/events", func(c echo.Context) error {
			if err := cfg.Store.DeleteEventLogs(c.Request().Context()); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "warn",
				Event: i18n.Trans("Delete all event logs"),
			})
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})
	}

	// Parquet Logs Query
	if cfg.LogStore != nil {
		apiGroup.GET("/logs/query", func(c echo.Context) error {
			logType := c.QueryParam("type")
			filter := c.QueryParam("filter")
			src := c.QueryParam("src")
			level := c.QueryParam("level")
			tag := c.QueryParam("tag")
			var startTime int64
			var endTime int64
			if s := c.QueryParam("start"); s != "" {
				startTime, _ = strconv.ParseInt(s, 10, 64)
			}
			if e := c.QueryParam("end"); e != "" {
				endTime, _ = strconv.ParseInt(e, 10, 64)
			}
			limit := 10000
			if l := c.QueryParam("limit"); l != "" {
				if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
					limit = parsed
				}
			}
			logs, err := cfg.LogStore.Query(c.Request().Context(), parquet.LogFilter{
				Type:      logType,
				Filter:    filter,
				Src:       src,
				Limit:     limit,
				StartTime: startTime,
				EndTime:   endTime,
				Level:     level,
				Tag:       tag,
			})
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, logs)
		})

		apiGroup.DELETE("/logs/query", func(c echo.Context) error {
			logType := c.QueryParam("type")
			if err := cfg.LogStore.DeleteLogs(c.Request().Context(), logType); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if cfg.Store != nil {
				typeStr := logType
				if typeStr == "" {
					typeStr = "all"
				}
				_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
					Time:  time.Now().UnixNano(),
					Type:  "user",
					Level: "warn",
					Event: fmt.Sprintf(i18n.Trans("Delete logs %s"), typeStr),
				})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})

		apiGroup.GET("/logs/counts", func(c echo.Context) error {
			counts := make(map[string]int64)
			if cnt, err := cfg.Store.CountEventLogs(c.Request().Context()); err == nil {
				counts["event"] = cnt
			}
			if cfg.LogStore != nil {
				if pqCounts, err := cfg.LogStore.CountByType(c.Request().Context()); err == nil {
					for k, v := range pqCounts {
						counts[k] = v
					}
					if v, ok := counts["arplog"]; ok {
						counts["arp"] = v
					} else if v, ok := counts["arp"]; ok {
						counts["arplog"] = v
					}
				}
			}
			return c.JSON(http.StatusOK, counts)
		})
	}

	if cfg.Store != nil {
		// Discovered ARP Table
		apiGroup.GET("/arp", func(c echo.Context) error {
			entries, err := cfg.Store.LoadArpTable(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusOK, []*datastore.ArpEnt{})
			}
			return c.JSON(http.StatusOK, entries)
		})

		// IPAM (IP Address Management) report
		apiGroup.GET("/ipam", func(c echo.Context) error {
			resp, err := calculateIPAM(c.Request().Context(), cfg.Store)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, resp)
		})

		// Delete / Reset ARP Table entries
		apiGroup.DELETE("/arp", func(c echo.Context) error {
			all := c.QueryParam("all") == "true"
			ip := c.QueryParam("ip")
			var req struct {
				All bool     `json:"all"`
				IPs []string `json:"ips"`
			}
			_ = c.Bind(&req)

			if all || req.All {
				if err := cfg.Store.ResetArpTable(c.Request().Context()); err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
				}
				if cfg.ArpManager != nil {
					cfg.ArpManager.ResetArpTable()
				}
				if cfg.Store != nil {
					_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
						Time:  time.Now().UnixNano(),
						Type:  "user",
						Level: "warn",
						Event: i18n.Trans("Reset ARP table"),
					})
				}
				return c.JSON(http.StatusOK, map[string]string{"status": "cleared"})
			}

			var targetIPs []string
			if ip != "" {
				targetIPs = append(targetIPs, ip)
			}
			targetIPs = append(targetIPs, req.IPs...)

			if len(targetIPs) == 0 {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "no ip specified"})
			}

			if err := cfg.Store.DeleteArpEntries(c.Request().Context(), targetIPs); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if cfg.ArpManager != nil {
				cfg.ArpManager.DeleteArpEntries(targetIPs)
			}
			if cfg.Store != nil {
				_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
					Time:  time.Now().UnixNano(),
					Type:  "user",
					Level: "info",
					Event: fmt.Sprintf(i18n.Trans("Delete arp entry (IP: %s)"), strings.Join(targetIPs, ", ")),
				})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})

		// MIB Browser & Modules
		apiGroup.GET("/mib/tree", func(c echo.Context) error {
			return c.JSON(http.StatusOK, mib.GetMIBTree())
		})
		apiGroup.GET("/mib/modules", func(c echo.Context) error {
			return c.JSON(http.StatusOK, mib.GetMIBModules())
		})
		apiGroup.POST("/mib/upload", func(c echo.Context) error {
			if cfg.DataDir == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "data directory not configured"})
			}
			file, err := c.FormFile("file")
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "file field is required"})
			}
			src, err := file.Open()
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			defer src.Close()

			filename := filepath.Base(file.Filename)
			if filename == "" || filename == "." || filename == ".." {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid file name"})
			}

			extDir := filepath.Join(cfg.DataDir, "extmibs")
			if err := os.MkdirAll(extDir, 0755); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			dstPath := filepath.Join(extDir, filename)
			dst, err := os.Create(dstPath)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			mib.ReloadExtMIBs(cfg.DataDir)
			return c.JSON(http.StatusOK, mib.GetMIBModules())
		})
		apiGroup.POST("/mib/reload", func(c echo.Context) error {
			if cfg.DataDir != "" {
				mib.ReloadExtMIBs(cfg.DataDir)
			}
			return c.JSON(http.StatusOK, mib.GetMIBModules())
		})
		apiGroup.DELETE("/mib/modules", func(c echo.Context) error {
			var req struct {
				File string `json:"file"`
			}
			if err := c.Bind(&req); err != nil || req.File == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "file path is required"})
			}
			if cfg.DataDir == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "data directory not configured"})
			}
			extDir := filepath.Clean(filepath.Join(cfg.DataDir, "extmibs"))
			targetPath := filepath.Clean(req.File)
			if !strings.HasPrefix(targetPath, extDir) {
				targetPath = filepath.Clean(filepath.Join(extDir, filepath.Base(req.File)))
			}
			if !strings.HasPrefix(targetPath, extDir) {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "can only delete external MIBs in extmibs"})
			}
			if err := os.Remove(targetPath); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			mib.ReloadExtMIBs(cfg.DataDir)
			return c.JSON(http.StatusOK, mib.GetMIBModules())
		})
		apiGroup.POST("/tools/snmp", func(c echo.Context) error {
			var req struct {
				NodeID    string `json:"node_id"`
				NetworkID string `json:"network_id"`
				OID       string `json:"oid"`
				Mode      string `json:"mode"`
				Raw       bool   `json:"raw"`
			}
			if err := c.Bind(&req); err != nil || (req.NodeID == "" && req.NetworkID == "") || req.OID == "" || (req.NodeID != "" && req.NetworkID != "") {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "exactly one of node_id or network_id and an OID are required"})
			}
			if req.Mode != "" && req.Mode != "get" && req.Mode != "getnext" && req.Mode != "walk" && req.Mode != "table" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "mode must be get, getnext, walk, or table"})
			}
			var nw *datastore.NetworkEnt
			var err error
			if req.NetworkID != "" {
				nw, err = cfg.Store.GetNetwork(c.Request().Context(), req.NetworkID)
				if err != nil || nw == nil {
					return c.JSON(http.StatusNotFound, map[string]string{"error": "network not found"})
				}
			} else {
				node, err := cfg.Store.GetNode(c.Request().Context(), req.NodeID)
				if err != nil || node == nil {
					return c.JSON(http.StatusNotFound, map[string]string{"error": "node not found"})
				}
				nw = &datastore.NetworkEnt{
					Name: node.Name, IP: node.IP, SnmpMode: node.SnmpMode,
					Community: node.Community, User: node.User, Password: node.Password, SnmpPort: node.SnmpPort,
				}
			}
			agent := topology.GetSNMPAgentForNetwork(nw, 2, 1)
			if agent == nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid SNMP configuration"})
			}
			if err := agent.Connect(); err != nil {
				return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
			}
			defer func() {
				if agent != nil && agent.Conn != nil {
					_ = agent.Conn.Close()
				}
			}()

			targetOID := ResolveNameToOID(req.OID)
			if targetOID == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid or unknown MIB object identifier: %s", req.OID)})
			}

			var vars []gosnmp.SnmpPDU
			switch req.Mode {
			case "walk", "table":
				err = agent.Walk(targetOID, func(pdu gosnmp.SnmpPDU) error {
					vars = append(vars, pdu)
					return nil
				})
			case "getnext":
				var packet *gosnmp.SnmpPacket
				packet, err = agent.GetNext([]string{targetOID})
				if err == nil {
					vars = packet.Variables
				}
			default:
				var packet *gosnmp.SnmpPacket
				packet, err = agent.Get([]string{targetOID})
				if err == nil {
					vars = packet.Variables
				}
			}
			if err != nil {
				return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
			}
			results := make([]map[string]any, 0, len(vars))
			for _, pdu := range vars {
				name := mib.OIDToName(pdu.Name)
				val := mib.GetMIBValueString(name, &pdu, req.Raw)
				results = append(results, map[string]any{
					"name":  name,
					"oid":   pdu.Name,
					"type":  pdu.Type.String(),
					"value": val,
					"mib":   mib.FindMIBInfo(name),
				})
			}
			return c.JSON(http.StatusOK, results)
		})

		// MQTT Stats
		mqttGroup := apiGroup.Group("/mqtt")
		mqttGroup.GET("/stats", func(c echo.Context) error {
			stats, err := cfg.Store.ListMqttStats(c.Request().Context())
			if err != nil || stats == nil {
				return c.JSON(http.StatusOK, []*datastore.MqttStatEnt{})
			}
			return c.JSON(http.StatusOK, stats)
		})
		mqttGroup.DELETE("/stats", func(c echo.Context) error {
			id := c.QueryParam("id")
			var req struct {
				IDs []string `json:"ids"`
			}
			_ = c.Bind(&req)

			var targetIDs []string
			if id != "" {
				targetIDs = append(targetIDs, id)
			}
			targetIDs = append(targetIDs, req.IDs...)

			if len(targetIDs) == 0 {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "no id specified"})
			}
			if err := cfg.Store.DeleteMqttStats(c.Request().Context(), targetIDs); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: fmt.Sprintf(i18n.Trans("Delete mqtt stats (%d items)"), len(targetIDs)),
			})
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})
		mqttGroup.DELETE("/stats/all", func(c echo.Context) error {
			if err := cfg.Store.DeleteAllMqttStats(c.Request().Context()); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "warn",
				Event: i18n.Trans("Delete all mqtt stats"),
			})
			return c.JSON(http.StatusOK, map[string]string{"status": "cleared"})
		})

		// OpenTelemetry (OTel)
		otelGroup := apiGroup.Group("/otel")
		otelGroup.GET("/metrics", func(c echo.Context) error {
			metrics, err := cfg.Store.ListOTelMetrics(c.Request().Context())
			if err != nil || metrics == nil {
				return c.JSON(http.StatusOK, []*datastore.OTelMetricEnt{})
			}
			return c.JSON(http.StatusOK, metrics)
		})
		otelGroup.GET("/metrics/detail", func(c echo.Context) error {
			host := c.QueryParam("host")
			service := c.QueryParam("service")
			scope := c.QueryParam("scope")
			name := c.QueryParam("name")
			metric, err := cfg.Store.GetOTelMetric(c.Request().Context(), host, service, scope, name)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "metric not found"})
			}
			return c.JSON(http.StatusOK, metric)
		})
		otelGroup.DELETE("/metrics", func(c echo.Context) error {
			host := c.QueryParam("host")
			service := c.QueryParam("service")
			scope := c.QueryParam("scope")
			name := c.QueryParam("name")
			_ = cfg.Store.DeleteOTelMetric(c.Request().Context(), host, service, scope, name)
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})
		otelGroup.GET("/traces/buckets", func(c echo.Context) error {
			buckets, err := cfg.Store.GetOTelTraceBuckets(c.Request().Context())
			if err != nil || buckets == nil {
				return c.JSON(http.StatusOK, []string{})
			}
			return c.JSON(http.StatusOK, buckets)
		})
		otelGroup.GET("/traces", func(c echo.Context) error {
			bks := c.QueryParams()["bucket"]
			if len(bks) == 0 {
				if b := c.QueryParam("bucket"); b != "" {
					bks = strings.Split(b, ",")
				}
			}
			limit := 5000
			if lStr := c.QueryParam("limit"); lStr != "" {
				if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
					limit = l
				}
			}
			traces, err := cfg.Store.ListOTelTraces(c.Request().Context(), bks, limit)
			if err != nil || traces == nil {
				return c.JSON(http.StatusOK, []*datastore.OTelTraceSummaryEnt{})
			}
			return c.JSON(http.StatusOK, traces)
		})
		otelGroup.GET("/traces/detail", func(c echo.Context) error {
			bucket := c.QueryParam("bucket")
			traceID := c.QueryParam("traceId")
			trace, err := cfg.Store.GetOTelTrace(c.Request().Context(), bucket, traceID)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "trace not found"})
			}
			return c.JSON(http.StatusOK, trace)
		})
		otelGroup.POST("/traces/dag", func(c echo.Context) error {
			var req struct {
				Buckets []string `json:"buckets"`
			}
			_ = c.Bind(&req)
			dag, err := cfg.Store.GetOTelTraceDAG(c.Request().Context(), req.Buckets)
			if err != nil || dag == nil {
				return c.JSON(http.StatusOK, &datastore.OTelTraceDAGEnt{
					Nodes: []datastore.OTelTraceDAGNodeEnt{},
					Links: []datastore.OTelTraceDAGLinkEnt{},
				})
			}
			return c.JSON(http.StatusOK, dag)
		})
		otelGroup.GET("/logs", func(c echo.Context) error {
			if cfg.LogStore == nil {
				return c.JSON(http.StatusOK, []any{})
			}
			filter := c.QueryParam("filter")
			src := c.QueryParam("src")
			var startTime int64
			var endTime int64
			if s := c.QueryParam("start"); s != "" {
				startTime, _ = strconv.ParseInt(s, 10, 64)
			}
			if e := c.QueryParam("end"); e != "" {
				endTime, _ = strconv.ParseInt(e, 10, 64)
			}
			limit := 10000
			if l := c.QueryParam("limit"); l != "" {
				if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
					limit = parsed
				}
			}
			logs, err := cfg.LogStore.Query(c.Request().Context(), parquet.LogFilter{
				Type:      "otel",
				Filter:    filter,
				Src:       src,
				Limit:     limit,
				StartTime: startTime,
				EndTime:   endTime,
			})
			if err != nil || logs == nil {
				return c.JSON(http.StatusOK, []*parquet.ParquetLogRecord{})
			}
			return c.JSON(http.StatusOK, logs)
		})
		otelGroup.DELETE("/all", func(c echo.Context) error {
			_ = cfg.Store.DeleteAllOTelData(c.Request().Context())
			if cfg.LogStore != nil {
				_ = cfg.LogStore.DeleteLogs(c.Request().Context(), "otel")
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "warn",
				Event: i18n.Trans("Delete all OpenTelemetry data"),
			})
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})
	}

	toolsGroup := apiGroup.Group("/tools")
	toolsGroup.POST("/ping", func(c echo.Context) error {
		var req struct {
			IP   string `json:"ip"`
			Size int    `json:"size"`
			TTL  int    `json:"ttl"`
		}
		if err := c.Bind(&req); err != nil || req.IP == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid ip"})
		}
		if req.Size <= 0 {
			req.Size = 64
		}
		if req.TTL <= 0 {
			req.TTL = 64
		}

		res := ping.DoPing(req.IP, 2, 0, req.Size, req.TTL)

		stat := 2 // Timeout or error
		switch res.Stat {
		case ping.PingOK:
			stat = 1 // Normal
		case ping.PingTimeExceeded:
			stat = 3
		}

		recvSrc := res.RecvSrc
		if recvSrc == "" {
			recvSrc = req.IP
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"Stat":      stat,
			"TimeStamp": time.Now().Unix(),
			"Time":      res.Time,
			"Size":      req.Size,
			"SendTTL":   req.TTL,
			"RecvTTL":   res.RecvTTL,
			"RecvSrc":   recvSrc,
			"Loc":       "LOCAL",
		})
	})
	toolsGroup.POST("/wol", func(c echo.Context) error {
		var req struct {
			MAC    string `json:"mac"`
			NodeID string `json:"node_id"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid mac"})
		}
		var node *datastore.NodeEnt
		if req.NodeID != "" && cfg.Store != nil {
			var err error
			node, err = cfg.Store.GetNode(c.Request().Context(), req.NodeID)
			if err != nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "node not found"})
			}
			req.MAC = node.MAC
		}
		req.MAC = strings.TrimSpace(strings.SplitN(req.MAC, "(", 2)[0])
		if req.MAC == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid mac"})
		}

		err := wol.SendWakeOnLanPacket(req.MAC)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if node != nil {
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time: time.Now().UnixNano(), Type: "user", Level: "info",
				NodeID: node.ID, NodeName: node.Name,
				Event: fmt.Sprintf(i18n.Trans("Send Wake on LAN Packet to %s"), req.MAC),
			})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "sent"})
	})

	// Static Files (Frontend SPA)
	staticFS, err := web.GetStaticFS()
	if err != nil {
		slog.Warn("Frontend static assets not found, running API-only mode", "error", err)
	} else {
		fileServer := http.FileServer(staticFS)
		e.GET("/*", echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, `{"error":"API endpoint not found"}`, http.StatusNotFound)
				return
			}
			f, err := staticFS.Open(r.URL.Path)
			if err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})))
	}

	return &Server{
		echo:       e,
		port:       cfg.Port,
		store:      cfg.Store,
		pki:        cfg.PKI,
		pkiServers: pkiServers,
	}, nil
}

func notifyOAuth2RedirectURL(c echo.Context) (string, error) {
	req := c.Request()
	origin := req.Header.Get("Origin")
	if origin == "" {
		return "", fmt.Errorf("OAuth2 origin header is required")
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil ||
		(u.Scheme != "http" && u.Scheme != "https") ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("OAuth2 origin must match the API host")
	}
	reqHost := (&url.URL{Host: req.Host}).Hostname()
	sameHost := strings.EqualFold(u.Host, req.Host)
	originIP := net.ParseIP(u.Hostname())
	requestIP := net.ParseIP(reqHost)
	originLocal := strings.EqualFold(u.Hostname(), "localhost") || originIP != nil && originIP.IsLoopback()
	requestLocal := strings.EqualFold(reqHost, "localhost") || requestIP != nil && requestIP.IsLoopback()
	if !sameHost && !(originLocal && requestLocal) {
		return "", fmt.Errorf("OAuth2 origin must match the API host")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid OAuth2 redirect scheme")
	}
	hostname := u.Hostname()
	ip := net.ParseIP(hostname)
	if u.Scheme == "http" && !strings.EqualFold(hostname, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return "", fmt.Errorf("OAuth2 redirect requires HTTPS except on localhost")
	}
	u.Path = "/api/notify/oauth2/callback"
	u.RawPath = ""
	return u.String(), nil
}

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	slog.Info("Starting HTTP Web/API server", "addr", addr)
	if s.pkiServers != nil {
		if err := s.pkiServers.Start(); err != nil {
			return err
		}
	}

	errCh := make(chan error, 1)
	go func() {
		if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case <-ctx.Done():
		slog.Info("Stopping HTTP, API, and PKI servers...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.shutdown(shutdownCtx)
	case err := <-errCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return errors.Join(err, s.shutdown(shutdownCtx))
	}
}

func (s *Server) shutdown(ctx context.Context) error {
	var shutdownErrors []error
	if s.pkiServers != nil {
		if err := s.pkiServers.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("shutdown PKI services: %w", err))
		}
	}
	if err := s.echo.Shutdown(ctx); err != nil {
		shutdownErrors = append(shutdownErrors, fmt.Errorf("shutdown web/API listener: %w", err))
	}
	return errors.Join(shutdownErrors...)
}

// GetEcho returns underlying Echo instance for testing and route inspection.
func (s *Server) GetEcho() *echo.Echo {
	return s.echo
}

func (s *Server) GetACMEEcho() *echo.Echo {
	if s.pkiServers == nil {
		return nil
	}
	return s.pkiServers.ACMEEcho()
}

func extractNumericValue(raw interface{}) (float64, bool) {
	if raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case uint32:
		return float64(v), true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func autoGetPollingSetting(di *datastore.DrawItemEnt, p *datastore.PollingEnt) (string, string, float64) {
	varName := di.VarName
	format := di.Format
	scale := di.Scale
	if scale == 0.0 {
		scale = 1.0
	}
	if varName != "" {
		if varName == "rtt" {
			if num, ok := extractNumericValue(p.Result["rtt"]); ok {
				if num > 1000 {
					if scale == 1.0 {
						scale = 0.000001
					}
					if format == "" {
						format = "%.2f ms"
					}
				} else {
					if scale == 0.000001 {
						scale = 1.0
					}
					if format == "" {
						format = "%.2f ms"
					}
				}
			}
		}
		return varName, format, scale
	}
	if _, ok := p.Result["bps"]; ok {
		varName = "bps"
		if format == "" {
			format = "BPS"
		}
		scale = 1.0
		return varName, format, scale
	}
	if _, ok := p.Result["rtt"]; ok {
		varName = "rtt"
		if format == "" {
			format = "%.2f ms"
		}
		if num, ok := extractNumericValue(p.Result["rtt"]); ok && num > 1000 {
			scale = 0.000001
		} else {
			scale = 1.0
		}
		return varName, format, scale
	}
	if _, ok := p.Result["cpu"]; ok {
		varName = "cpu"
		if format == "" {
			format = "%.1f%%"
		}
		scale = 1.0
		return varName, format, scale
	}
	if _, ok := p.Result["state"]; ok {
		varName = "state"
		if format == "" {
			format = "%s"
		}
		scale = 1.0
		return varName, format, scale
	}
	if _, ok := p.Result["avg"]; ok {
		varName = "avg"
		if format == "" {
			format = "AVG=%.2f"
		}
		scale = 1.0
		return varName, format, scale
	}
	if _, ok := p.Result["count"]; ok {
		varName = "count"
		if format == "" {
			format = "%.0f"
		}
		scale = 1.0
		return varName, format, scale
	}
	return varName, format, scale
}

func formatDrawItemValue(val float64, format string) string {
	if format == "BPS" || strings.Contains(format, "BPS") {
		var bps string
		if val < 1000 {
			bps = fmt.Sprintf("%.1f bps", val)
		} else if val < 1000000 {
			bps = fmt.Sprintf("%.1f Kbps", val/1000)
		} else if val < 1000000000 {
			bps = fmt.Sprintf("%.1f Mbps", val/1000000)
		} else {
			bps = fmt.Sprintf("%.1f Gbps", val/1000000000)
		}
		if format == "BPS" {
			return bps
		}
		return strings.Replace(format, "BPS", bps, 1)
	}
	if format == "PPS" || strings.Contains(format, "PPS") {
		var pps string
		if val < 1000 {
			pps = fmt.Sprintf("%.0f PPS", val)
		} else if val < 1000000 {
			pps = fmt.Sprintf("%.1f kPPS", val/1000)
		} else {
			pps = fmt.Sprintf("%.1f MPPS", val/1000000)
		}
		if format == "PPS" {
			return pps
		}
		return strings.Replace(format, "PPS", pps, 1)
	}
	if format != "" {
		if strings.Contains(format, "%") {
			return fmt.Sprintf(format, val)
		}
		return fmt.Sprintf("%s %.1f", format, val)
	}
	return fmt.Sprintf("%.1f", val)
}

func checkDrawItem(ctx context.Context, store datastore.DataStore, di *datastore.DrawItemEnt) {
	if di.Type < 4 {
		return
	}
	if di.Type == datastore.DrawItemTypePollingText {
		if di.Text == "" {
			di.Text = "No Value"
		}
		di.FormattedText = di.Text
	}
	if di.Type >= 5 {
		di.Value = 0.0
	}
	if di.PollingID == "" {
		return
	}
	p, err := store.GetPolling(ctx, di.PollingID)
	if err != nil || p == nil {
		return
	}
	if di.Type == datastore.DrawItemTypePollingLine {
		switch p.State {
		case "high":
			di.Color = "#ef4444"
		case "low":
			di.Color = "#f87171"
		case "warn":
			di.Color = "#f59e0b"
		default:
			di.Color = "#00d2ff"
		}
		return
	}

	varName, format, scale := autoGetPollingSetting(di, p)

	val := 0.0
	text := ""
	if raw, ok := p.Result[varName]; ok {
		if num, isNum := extractNumericValue(raw); isNum {
			val = num * scale
			text = formatDrawItemValue(val, format)
		} else if str, isStr := raw.(string); isStr {
			if format != "" && strings.Contains(format, "%s") {
				text = fmt.Sprintf(format, str)
			} else {
				text = str
			}
		}
	}
	if text == "" {
		text = "No Value"
	}

	switch di.Type {
	case datastore.DrawItemTypePollingGauge, datastore.DrawItemTypePollingNewGauge, datastore.DrawItemTypePollingBar:
		if val > 100.0 {
			val = 100.0
		}
		if val >= 90.0 {
			di.Color = "#ef4444"
		} else if val >= 80.0 {
			di.Color = "#f59e0b"
		} else {
			di.Color = "#00d2ff"
		}
		di.Value = val
		di.FormattedText = text
	case datastore.DrawItemTypePollingText:
		di.Text = text
		di.FormattedText = text
		di.Value = val
		switch p.State {
		case "high":
			di.Color = "#e31a1c"
		case "low":
			di.Color = "#fb9a99"
		case "warn":
			di.Color = "#dfdf22"
		default:
			di.Color = "#eee"
		}
	case datastore.DrawItemTypePollingKPI:
		title := di.Text
		if strings.Contains(title, "\t") {
			title = strings.Split(title, "\t")[0]
		}
		if title == "" {
			if di.VarName != "" {
				title = fmt.Sprintf("%s (%s)", p.Name, di.VarName)
			} else {
				title = p.Name
			}
		}
		di.Text = title
		di.FormattedText = text
		di.Value = val
		switch p.State {
		case "high":
			di.Color = "#ef4444"
		case "low":
			di.Color = "#f87171"
		case "warn":
			di.Color = "#f59e0b"
		default:
			di.Color = "#00d2ff"
		}
		if val > 0 {
			if len(di.Values) == 0 {
				di.Values = []float64{val}
			} else if di.Values[len(di.Values)-1] != val {
				di.Values = append(di.Values, val)
				if len(di.Values) > 60 {
					di.Values = di.Values[len(di.Values)-60:]
				}
			}
		}
	}
}

var numericOIDPattern = regexp.MustCompile(`^\.?[0-9]+(?:\.[0-9]+)*$`)

// ResolveNameToOID converts a symbolic MIB name or numeric OID into a valid numeric OID for SNMP queries.
// It returns an empty string if the input cannot be resolved to a valid numeric OID.
func ResolveNameToOID(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return ""
	}
	oid := mib.NameToOID(clean)
	if oid == ".1" {
		oid = ".1.3"
	}
	if oid == ".0.0" || oid == "" || oid == "."+clean || oid == clean {
		if numericOIDPattern.MatchString(clean) {
			if !strings.HasPrefix(clean, ".") {
				return "." + clean
			}
			return clean
		}
		if !numericOIDPattern.MatchString(oid) {
			return ""
		}
	}
	if !strings.HasPrefix(oid, ".") {
		oid = "." + oid
	}
	if !numericOIDPattern.MatchString(oid) {
		return ""
	}
	return oid
}

func resolveHostToIPv4(ctx context.Context, host string) string {
	r := &net.Resolver{}
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ips, err := r.LookupHost(timeoutCtx, host)
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		if strings.Contains(ip, ":") {
			continue
		}
		parsed := net.ParseIP(ip)
		if parsed != nil && (parsed.IsGlobalUnicast() || parsed.IsLoopback()) {
			return ip
		}
	}
	return ""
}
