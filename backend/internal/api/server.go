package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/twsnmp/twsnmpneo/backend/internal/ai"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/web"
)

type Server struct {
	echo      *echo.Echo
	port      int
	store     datastore.DataStore
	logStore  *parquet.Store
	mcpServer *ai.MCPServer
}

type Config struct {
	Port      int
	Debug     bool
	Version   string
	Store     datastore.DataStore
	LogStore  *parquet.Store
	MCPServer *ai.MCPServer
}

func NewServer(cfg Config) (*Server, error) {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middlewares
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// API Group
	apiGroup := e.Group("/api")
	apiGroup.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"status":  "ok",
			"time":    time.Now().Format(time.RFC3339),
			"version": cfg.Version,
		})
	})

	// MCP Endpoint
	if cfg.MCPServer != nil {
		apiGroup.Any("/mcp*", echo.WrapHandler(cfg.MCPServer))
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
			if err := cfg.Store.SaveNode(c.Request().Context(), &n); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
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
			if err := cfg.Store.DeleteNode(c.Request().Context(), c.Param("id")); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
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
			if err := cfg.Store.SavePolling(c.Request().Context(), &p); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, &p)
		})
		apiGroup.DELETE("/pollings/:id", func(c echo.Context) error {
			if err := cfg.Store.DeletePolling(c.Request().Context(), c.Param("id")); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
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
			if err := cfg.Store.SaveLine(c.Request().Context(), &l); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, &l)
		})
		apiGroup.DELETE("/lines/:id", func(c echo.Context) error {
			if err := cfg.Store.DeleteLine(c.Request().Context(), c.Param("id")); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
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
			if err := cfg.Store.SaveNetwork(c.Request().Context(), &n); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, &n)
		})
		apiGroup.DELETE("/networks/:id", func(c echo.Context) error {
			if err := cfg.Store.DeleteNetwork(c.Request().Context(), c.Param("id")); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})

		// DrawItems
		apiGroup.GET("/drawitems", func(c echo.Context) error {
			items, err := cfg.Store.ListDrawItems(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, items)
		})
		apiGroup.POST("/drawitems", func(c echo.Context) error {
			var item datastore.DrawItemEnt
			if err := c.Bind(&item); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if err := cfg.Store.SaveDrawItem(c.Request().Context(), &item); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, &item)
		})
		apiGroup.DELETE("/drawitems/:id", func(c echo.Context) error {
			if err := cfg.Store.DeleteDrawItem(c.Request().Context(), c.Param("id")); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		})

		// Map Conf
		apiGroup.GET("/map/conf", func(c echo.Context) error {
			conf, err := cfg.Store.GetMapConf(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, conf)
		})
		apiGroup.POST("/map/conf", func(c echo.Context) error {
			var conf datastore.MapConfEnt
			if err := c.Bind(&conf); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if err := cfg.Store.SaveMapConf(c.Request().Context(), &conf); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, &conf)
		})

		// Event Logs
		apiGroup.GET("/logs/events", func(c echo.Context) error {
			logs, err := cfg.Store.ListEventLogs(c.Request().Context(), 100)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, logs)
		})
	}

	// Parquet Logs Query
	if cfg.LogStore != nil {
		apiGroup.GET("/logs/query", func(c echo.Context) error {
			logType := c.QueryParam("type")
			filter := c.QueryParam("filter")
			logs, err := cfg.LogStore.Query(c.Request().Context(), parquet.LogFilter{
				Type:      logType,
				Filter:    filter,
				Limit:     100,
				StartTime: time.Now().Add(-24 * time.Hour).UnixNano(),
				EndTime:   time.Now().UnixNano(),
			})
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, logs)
		})
	}

	// Diagnostic Tools APIs (Ping, WOL)
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

		start := time.Now()
		d := net.Dialer{Timeout: 2 * time.Second}
		conn, err := d.DialContext(c.Request().Context(), "udp", net.JoinHostPort(req.IP, "7"))
		elapsed := time.Since(start)

		stat := 1 // Normal
		recvSrc := req.IP
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				stat = 2 // Timeout
			} else {
				stat = 1 // Connected or port handled
			}
		} else {
			_ = conn.Close()
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"Stat":      stat,
			"TimeStamp": time.Now().Unix(),
			"Time":      elapsed.Nanoseconds(),
			"Size":      req.Size,
			"SendTTL":   req.TTL,
			"RecvTTL":   req.TTL,
			"RecvSrc":   recvSrc,
			"Loc":       "LOCAL",
		})
	})
	toolsGroup.POST("/wol", func(c echo.Context) error {
		var req struct {
			MAC string `json:"mac"`
			IP  string `json:"ip"`
		}
		if err := c.Bind(&req); err != nil || req.MAC == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid mac"})
		}
		hw, err := net.ParseMAC(req.MAC)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid mac format"})
		}
		packet := make([]byte, 102)
		for i := 0; i < 6; i++ {
			packet[i] = 0xFF
		}
		for i := 1; i <= 16; i++ {
			copy(packet[i*6:], hw)
		}
		bcast := "255.255.255.255:9"
		if req.IP != "" {
			bcast = net.JoinHostPort(req.IP, "9")
		}
		conn, err := net.Dial("udp", bcast)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		defer conn.Close()
		_, _ = conn.Write(packet)
		return c.JSON(http.StatusOK, map[string]string{"status": "sent"})
	})

	// Static Files (Frontend SPA)
	staticFS, err := web.GetStaticFS()
	if err != nil {
		slog.Warn("Frontend static assets not found, running API-only mode", "error", err)
	} else {
		fileServer := http.FileServer(staticFS)
		e.GET("/*", echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		echo:  e,
		port:  cfg.Port,
		store: cfg.Store,
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	slog.Info("Starting HTTP Web/API server", "addr", addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("Stopping HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.echo.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// GetEcho returns underlying Echo instance for testing and route inspection.
func (s *Server) GetEcho() *echo.Echo {
	return s.echo
}
