package receiver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// TrapConfig holds options for the SNMP TRAP receiver.
type TrapConfig struct {
	Port      int
	Community string
	LogStore  *parquet.Store
}

// TrapServer receives SNMP TRAP v1 and v2c/v3 packets.
type TrapServer struct {
	port      int
	community string
	logStore  *parquet.Store
	listener  *gosnmp.TrapListener
	mu        sync.Mutex
}

// TrapMessage represents a parsed SNMP trap.
type TrapMessage struct {
	Time      int64             `json:"time"`
	SrcIP     string            `json:"srcIP"`
	Enterprise string           `json:"enterprise,omitempty"`
	Generic   int               `json:"generic,omitempty"`
	Specific  int               `json:"specific,omitempty"`
	Variables map[string]string `json:"variables"`
}

func NewTrapServer(cfg TrapConfig) *TrapServer {
	return &TrapServer{
		port:      cfg.Port,
		community: cfg.Community,
		logStore:  cfg.LogStore,
	}
}

func (s *TrapServer) Start(ctx context.Context) error {
	if s.port <= 0 {
		return nil
	}

	tl := gosnmp.NewTrapListener()
	tl.OnNewTrap = func(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
		s.handleTrap(packet, addr)
	}

	listenAddr := fmt.Sprintf("0.0.0.0:%d", s.port)
	slog.Info("Starting SNMP TRAP receiver", "addr", listenAddr)

	errCh := make(chan error, 1)
	go func() {
		if err := tl.Listen(listenAddr); err != nil {
			errCh <- err
		}
	}()

	s.mu.Lock()
	s.listener = tl
	s.mu.Unlock()

	// Wait until listening socket is actually ready
	<-tl.Listening()

	select {
	case <-ctx.Done():
		slog.Info("Stopping SNMP TRAP receiver...")
		tl.Close()
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *TrapServer) handleTrap(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
	if packet == nil {
		return
	}

	srcIP := ""
	if addr != nil {
		srcIP = addr.IP.String()
	}

	now := time.Now().UnixNano()
	msg := &TrapMessage{
		Time:       now,
		SrcIP:      srcIP,
		Enterprise: packet.Enterprise,
		Generic:    packet.GenericTrap,
		Specific:   packet.SpecificTrap,
		Variables:  make(map[string]string),
	}

	for _, v := range packet.Variables {
		msg.Variables[v.Name] = fmt.Sprintf("%v", v.Value)
	}

	if s.logStore != nil {
		rawJSON, _ := json.Marshal(msg)
		_ = s.logStore.WriteLog(&parquet.ParquetLogRecord{
			Time: now,
			Type: "trap",
			Src:  srcIP,
			Log:  string(rawJSON),
		})
	}
}
