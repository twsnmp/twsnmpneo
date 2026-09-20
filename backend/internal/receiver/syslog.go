package receiver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// SyslogConfig holds options for the Syslog receiver.
type SyslogConfig struct {
	UDPPort  int
	TCPPort  int
	LogStore *parquet.Store
}

// SyslogServer receives and parses Syslog messages over UDP and TCP.
type SyslogServer struct {
	udpPort  int
	tcpPort  int
	logStore *parquet.Store
	mu       sync.Mutex
	running  bool
}

// SyslogMessage holds parsed Syslog fields.
type SyslogMessage struct {
	Time      int64  `json:"time"`
	Facility  int    `json:"facility"`
	Severity  int    `json:"severity"`
	Host      string `json:"host"`
	Tag       string `json:"tag"`
	Message   string `json:"message"`
	Raw       string `json:"raw"`
}

var syslogPattern = regexp.MustCompile(`^<(\d{1,3})>(.*)$`)

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

	var wg sync.WaitGroup

	// Start UDP listener
	if s.udpPort > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.startUDP(ctx)
		}()
	}

	// Start TCP listener
	if s.tcpPort > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.startTCP(ctx)
		}()
	}

	<-ctx.Done()
	slog.Info("Stopping Syslog receiver...")
	wg.Wait()
	return nil
}

func (s *SyslogServer) startUDP(ctx context.Context) {
	addr := fmt.Sprintf(":%d", s.udpPort)
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		slog.Error("Failed to start UDP syslog listener", "addr", addr, "error", err)
		return
	}
	defer conn.Close()

	slog.Info("Started UDP Syslog receiver", "addr", addr)
	buf := make([]byte, 8192)

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	for {
		n, remoteAddr, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		srcIP := ""
		if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok {
			srcIP = udpAddr.IP.String()
		}
		s.handleMessage(buf[:n], srcIP)
	}
}

func (s *SyslogServer) startTCP(ctx context.Context) {
	addr := fmt.Sprintf(":%d", s.tcpPort)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("Failed to start TCP syslog listener", "addr", addr, "error", err)
		return
	}
	defer l.Close()

	slog.Info("Started TCP Syslog receiver", "addr", addr)

	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		go s.handleTCPConn(ctx, conn)
	}
}

func (s *SyslogServer) handleTCPConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	srcIP, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		s.handleMessage(scanner.Bytes(), srcIP)
	}
}

func (s *SyslogServer) handleMessage(data []byte, srcIP string) {
	msgStr := string(data)
	msg := ParseSyslog(msgStr, srcIP)

	if s.logStore != nil {
		rawJSON, _ := json.Marshal(msg)
		_ = s.logStore.WriteLog(&parquet.ParquetLogRecord{
			Time: msg.Time,
			Type: "syslog",
			Src:  msg.Host,
			Log:  string(rawJSON),
		})
	}
}

// ParseSyslog parses a raw syslog string into structured fields.
func ParseSyslog(raw, srcIP string) *SyslogMessage {
	now := time.Now().UnixNano()
	msg := &SyslogMessage{
		Time:     now,
		Host:     srcIP,
		Raw:      raw,
		Facility: 1, // User level
		Severity: 6, // Info
		Message:  raw,
	}

	match := syslogPattern.FindStringSubmatch(raw)
	if len(match) == 3 {
		var pri int
		if _, err := fmt.Sscanf(match[1], "%d", &pri); err == nil {
			msg.Facility = pri / 8
			msg.Severity = pri % 8
		}
		rest := strings.TrimSpace(match[2])
		msg.Message = rest

		// Check for tag
		if idx := strings.Index(rest, ":"); idx > 0 && idx < 32 {
			msg.Tag = strings.TrimSpace(rest[:idx])
			msg.Message = strings.TrimSpace(rest[idx+1:])
		}
	}
	return msg
}
