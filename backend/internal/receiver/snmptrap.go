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
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
)

// TrapConfig holds options for the SNMP TRAP receiver.
type TrapConfig struct {
	Port      int
	Community string
	Store     datastore.DataStore
	LogStore  *parquet.Store
}

// TrapServer receives SNMP TRAP v1 and v2c/v3 packets.
type TrapServer struct {
	port      int
	community string
	store     datastore.DataStore
	logStore  *parquet.Store
	listener  *gosnmp.TrapListener
	mu        sync.Mutex
}

// TrapMessage represents a parsed SNMP trap.
type TrapMessage struct {
	Time        int64  `json:"Time"`
	FromAddress string `json:"FromAddress"`
	TrapType    string `json:"TrapType"`
	Variables   string `json:"Variables"`
	Enterprise  string `json:"Enterprise,omitempty"`
	Generic     int    `json:"GenericTrap,omitempty"`
	Specific    int    `json:"SpecificTrap,omitempty"`
}

func NewTrapServer(cfg TrapConfig) *TrapServer {
	return &TrapServer{
		port:      cfg.Port,
		community: cfg.Community,
		store:     cfg.Store,
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

	listenAddr := fmt.Sprintf(":%d", s.port)
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

	// Wait until listening socket is actually ready or error occurs
	select {
	case <-tl.Listening():
		slog.Info("Started SNMP TRAP receiver", "addr", listenAddr)
	case err := <-errCh:
		slog.Warn("Failed to start SNMP TRAP receiver", "addr", listenAddr, "error", err)
		return fmt.Errorf("listen snmptrap: %w", err)
	case <-ctx.Done():
		return nil
	}

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

	nodeName := ""
	if s.store != nil && srcIP != "" {
		if nodes, err := s.store.ListNodes(context.Background()); err == nil {
			for _, n := range nodes {
				if n.IP == srcIP {
					nodeName = n.Name
					break
				}
			}
		}
	}

	trapType, variables, fromAddress := mib.DecodeTrap(packet, srcIP, nodeName)

	now := time.Now().UnixNano()
	msg := &TrapMessage{
		Time:        now,
		FromAddress: fromAddress,
		TrapType:    trapType,
		Variables:   variables,
	}
	if packet.Enterprise != "" {
		msg.Enterprise = mib.OIDToName(packet.Enterprise)
		msg.Generic = packet.GenericTrap
		msg.Specific = packet.SpecificTrap
	}

	if s.logStore != nil {
		rawJSON, _ := json.Marshal(msg)
		_ = s.logStore.WriteLog(&parquet.ParquetLogRecord{
			Time: now,
			Type: "trap",
			Src:  fromAddress,
			Log:  string(rawJSON),
		})
	}
}
