package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/twsnmp/twsnmpneo/backend/internal/ai"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
	"github.com/twsnmp/twsnmpneo/backend/internal/topology"
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
	DataDir   string
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
			isNew := n.ID == ""
			if isNew {
				n.ID = datastore.GenerateID()
			}
			if err := cfg.Store.SaveNode(c.Request().Context(), &n); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			action := "更新"
			if isNew {
				action = "追加"
				// Auto-create default Ping polling if node has IP
				if n.IP != "" {
					pID := datastore.GenerateID()
					_ = cfg.Store.SavePolling(c.Request().Context(), &datastore.PollingEnt{
						ID:       pID,
						Name:     "Ping",
						NodeID:   n.ID,
						Type:     "ping",
						PollInt:  60,
						Timeout:  1,
						Retry:    1,
						LogMode:  datastore.LogModeOnChange,
						State:    "unknown",
						NextTime: time.Now().UnixNano(),
					})
				}
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:     time.Now().UnixNano(),
				Type:     "user",
				Level:    "info",
				NodeName: n.Name,
				NodeID:   n.ID,
				Event:    fmt.Sprintf("ノード %s (%s) を%sしました", n.Name, n.IP, action),
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
				Event:    fmt.Sprintf("ノード %s を削除しました", name),
			})
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
			isNew := p.ID == ""
			if isNew {
				p.ID = datastore.GenerateID()
			}
			if err := cfg.Store.SavePolling(c.Request().Context(), &p); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:     time.Now().UnixNano(),
				Type:     "user",
				Level:    "info",
				NodeID:   p.NodeID,
				Event:    fmt.Sprintf("ポーリング %s を保存しました", p.Name),
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
				Event: fmt.Sprintf("ポーリング %s を削除しました", name),
			})
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
				Event: fmt.Sprintf("ラインを結線/更新しました (%s - %s)", l.NodeID1, l.NodeID2),
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
				Event: fmt.Sprintf("ラインを切断/削除しました (%s)", info),
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
					Event: fmt.Sprintf("自動トポロジー探索により %d 本のラインを結線しました", count),
				})
			}
			return c.JSON(http.StatusOK, map[string]int{"connected": count})
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
			}
			if err := cfg.Store.SaveNetwork(c.Request().Context(), &n); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			action := "更新"
			if isNew {
				action = "追加"
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: fmt.Sprintf("ネットワーク %s を%sしました", n.Name, action),
			})
			return c.JSON(http.StatusOK, &n)
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
				Event: fmt.Sprintf("ネットワーク %s を削除しました", name),
			})
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
			isNew := item.ID == ""
			if isNew {
				item.ID = datastore.GenerateID()
			}
			if err := cfg.Store.SaveDrawItem(c.Request().Context(), &item); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			action := "更新"
			if isNew {
				action = "追加"
			}
			_ = cfg.Store.AddEventLog(c.Request().Context(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "user",
				Level: "info",
				Event: fmt.Sprintf("描画アイテム %s を%sしました", item.Text, action),
			})
			return c.JSON(http.StatusOK, &item)
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
				Event: "描画アイテムを削除しました",
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
			conf.GeoIPInfo = datastore.GetGeoIPInfo()
			return c.JSON(http.StatusOK, &conf)
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
				Event: "IP位置情報DBを更新しました",
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
				Event: "IP位置情報DBを削除しました",
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
				Event: "すべてのイベントログを消去しました",
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
					Event: fmt.Sprintf("%s ログを消去しました", typeStr),
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
				}
			}
			return c.JSON(http.StatusOK, counts)
		})

		// MIB Browser & Modules
		apiGroup.GET("/mib/tree", func(c echo.Context) error {
			return c.JSON(http.StatusOK, mib.GetMIBTree())
		})
		apiGroup.GET("/mib/modules", func(c echo.Context) error {
			return c.JSON(http.StatusOK, mib.GetMIBModules())
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
