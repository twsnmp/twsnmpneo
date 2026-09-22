package receiver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

// SyslogConfig holds options for the Syslog receiver.
type SyslogConfig struct {
	UDPPort  int
	TCPPort  int
	LogStore *parquet.Store
}

// SyslogServer receives and parses Syslog messages over UDP and TCP using go-syslog.
type SyslogServer struct {
	udpPort  int
	tcpPort  int
	logStore *parquet.Store
	mu       sync.Mutex
	running  bool
}

// SyslogMessage holds parsed Syslog fields.
type SyslogMessage struct {
	Time     int64  `json:"time"`
	Facility int    `json:"facility"`
	Severity int    `json:"severity"`
	Host     string `json:"host"`
	Tag      string `json:"tag"`
	Message  string `json:"message"`
	Raw      string `json:"raw"`
}

func NewSyslogServer(cfg SyslogConfig) *SyslogServer {
	return &SyslogServer{
		udpPort:  cfg.UDPPort,
		tcpPort:  cfg.TCPPort,
		logStore: cfg.LogStore,
	}
}

func (s *SyslogServer) Start(ctx context.Context) error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	syslogCh := make(syslog.LogPartsChannel, 2000)
	server := syslog.NewServer()
	server.SetFormat(syslog.Automatic)
	server.SetHandler(syslog.NewChannelHandler(syslogCh))

	if s.udpPort > 0 {
		addr := fmt.Sprintf("0.0.0.0:%d", s.udpPort)
		if err := server.ListenUDP(addr); err != nil {
			slog.Error("Failed to start UDP syslog listener", "addr", addr, "error", err)
		} else {
			slog.Info("Started UDP Syslog receiver", "addr", addr)
		}
	}

	if s.tcpPort > 0 {
		addr := fmt.Sprintf("0.0.0.0:%d", s.tcpPort)
		if err := server.ListenTCP(addr); err != nil {
			slog.Error("Failed to start TCP syslog listener", "addr", addr, "error", err)
		} else {
			slog.Info("Started TCP Syslog receiver", "addr", addr)
		}
	}

	if err := server.Boot(); err != nil {
		slog.Error("Failed to boot syslog server", "error", err)
		return err
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping Syslog receiver...")
			_ = server.Kill()
			return nil
		case sl, ok := <-syslogCh:
			if !ok {
				return nil
			}
			s.handleLogParts(sl)
		}
	}
}

func (s *SyslogServer) handleLogParts(sl format.LogParts) {
	if sl == nil {
		return
	}
	host, _ := sl["hostname"].(string)
	if host == "" {
		if client, ok := sl["client"].(string); ok && client != "" {
			if h, _, err := net.SplitHostPort(client); err == nil {
				host = h
			} else {
				host = client
			}
		}
		if host != "" {
			sl["hostname"] = host
		}
	}

	timeNano := time.Now().UnixNano()
	if ts, ok := sl["timestamp"].(time.Time); ok && !ts.IsZero() {
		timeNano = ts.UnixNano()
	}

	if s.logStore != nil {
		rawJSON, err := json.Marshal(sl)
		if err == nil {
			_ = s.logStore.WriteLog(&parquet.ParquetLogRecord{
				Time: timeNano,
				Type: "syslog",
				Src:  host,
				Log:  string(rawJSON),
			})
		}
	}
}

// ParseSyslog parses a raw syslog string into structured SyslogMessage using go-syslog.
func ParseSyslog(raw, srcIP string) *SyslogMessage {
	parts := ParseSyslogParts([]byte(raw), srcIP)
	now := time.Now().UnixNano()
	if ts, ok := parts["timestamp"].(time.Time); ok && !ts.IsZero() {
		now = ts.UnixNano()
	}

	fac := 1
	if f, ok := parts["facility"].(int); ok {
		fac = f
	} else if f, ok := parts["facility"].(float64); ok {
		fac = int(f)
	}

	sev := 6
	if s, ok := parts["severity"].(int); ok {
		sev = s
	} else if s, ok := parts["severity"].(float64); ok {
		sev = int(s)
	}

	host, _ := parts["hostname"].(string)
	if host == "" {
		host = srcIP
	}

	tag, _ := parts["tag"].(string)
	if tag == "" {
		tag, _ = parts["app_name"].(string)
	}

	message, _ := parts["content"].(string)
	if message == "" {
		var partsList []string
		for _, k := range []string{"proc_id", "msg_id", "message", "structured_data"} {
			if m, ok := parts[k].(string); ok && m != "" {
				partsList = append(partsList, m)
			}
		}
		if len(partsList) > 0 {
			message = strings.Join(partsList, " ")
		} else {
			if m, ok := parts["message"].(string); ok && m != "" {
				message = m
			} else {
				message = raw
			}
		}
	}

	return &SyslogMessage{
		Time:     now,
		Facility: fac,
		Severity: sev,
		Host:     host,
		Tag:      tag,
		Message:  message,
		Raw:      raw,
	}
}

// ParseSyslogParts parses raw bytes into syslog LogParts map.
func ParseSyslogParts(raw []byte, srcIP string) format.LogParts {
	parser := (&format.Automatic{}).GetParser(raw)
	if err := parser.Parse(); err == nil {
		parts := parser.Dump()
		if parts == nil {
			parts = make(format.LogParts)
		}
		if srcIP != "" {
			if _, ok := parts["client"]; !ok {
				parts["client"] = srcIP
			}
		}
		if h, _ := parts["hostname"].(string); h == "" {
			if srcIP != "" {
				if h2, _, err := net.SplitHostPort(srcIP); err == nil {
					parts["hostname"] = h2
				} else {
					parts["hostname"] = srcIP
				}
			}
		}
		return parts
	}

	// Fallback for non-RFC lines
	parts := make(format.LogParts)
	parts["client"] = srcIP
	parts["hostname"] = srcIP
	parts["content"] = string(raw)
	parts["facility"] = 1
	parts["severity"] = 6
	parts["timestamp"] = time.Now()
	return parts
}
