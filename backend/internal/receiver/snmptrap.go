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

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/logreport"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
)

// TrapConfig holds options for the SNMP TRAP receiver.
type TrapConfig struct {
	Port      int
	Community string
	Store     datastore.DataStore
	LogStore  *parquet.Store
	Reporter  logreport.Reporter
}

// TrapServer receives SNMP TRAP v1 and v2c/v3 packets.
type TrapServer struct {
	port      int
	community string
	store     datastore.DataStore
	logStore  *parquet.Store
	reporter  logreport.Reporter
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
		reporter:  cfg.Reporter,
	}
}

func (s *TrapServer) Start(ctx context.Context) error {
	if s.port <= 0 {
		return nil
	}

	tl := gosnmp.NewTrapListener()
	tl.Params = &gosnmp.GoSNMP{
		Port:          uint16(s.port),
		Transport:     "udp",
		Version:       gosnmp.Version3,
		SecurityModel: gosnmp.UserSecurityModel,
	}
	if usmTable := s.buildUsmTable(ctx); usmTable != nil {
		tl.Params.SetTrapSecurityParametersTable(usmTable)
	}
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
		tl.Close()
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

	if s.reporter != nil {
		s.reporter.ProcessTrap(msg)
	}

	if s.store != nil && srcIP != "" {
		if strings.Contains(strings.ToLower(trapType), "twagentneo") || strings.Contains(variables, "twagentNeo") {
			s.store.UpdateSensor(srcIP, "twagentneo", trapType, 1)
		}
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

func (s *TrapServer) buildUsmTable(ctx context.Context) *gosnmp.SnmpV3SecurityParametersTable {
	if s.store == nil {
		return nil
	}
	table := gosnmp.NewSnmpV3SecurityParametersTable(gosnmp.Logger{})
	count := 0
	if mapConf, err := s.store.GetMapConf(ctx); err == nil && mapConf != nil && mapConf.SnmpUser != "" {
		if sp := buildUsmParam(mapConf.SnmpUser, mapConf.SnmpPassword, mapConf.SnmpMode); sp != nil {
			_ = table.Add(mapConf.SnmpUser, sp)
			count++
		}
	}
	if nodes, err := s.store.ListNodes(ctx); err == nil {
		for _, n := range nodes {
			if n.User != "" {
				if sp := buildUsmParam(n.User, n.Password, n.SnmpMode); sp != nil {
					_ = table.Add(n.User, sp)
					count++
				}
			}
		}
	}
	if count == 0 {
		return nil
	}
	return table
}

func buildUsmParam(user, password, mode string) *gosnmp.UsmSecurityParameters {
	sp := &gosnmp.UsmSecurityParameters{
		UserName: user,
	}
	switch mode {
	case "v3auth":
		sp.AuthenticationProtocol = gosnmp.SHA
		sp.AuthenticationPassphrase = password
	case "v3authpriv":
		sp.AuthenticationProtocol = gosnmp.SHA
		sp.AuthenticationPassphrase = password
		sp.PrivacyProtocol = gosnmp.AES
		sp.PrivacyPassphrase = password
	case "v3authprivex", "v3authp256":
		sp.AuthenticationProtocol = gosnmp.SHA256
		sp.AuthenticationPassphrase = password
		sp.PrivacyProtocol = gosnmp.AES256
		sp.PrivacyPassphrase = password
	case "v3sha256aes128":
		sp.AuthenticationProtocol = gosnmp.SHA256
		sp.AuthenticationPassphrase = password
		sp.PrivacyProtocol = gosnmp.AES
		sp.PrivacyPassphrase = password
	case "v3sha512aes256":
		sp.AuthenticationProtocol = gosnmp.SHA512
		sp.AuthenticationPassphrase = password
		sp.PrivacyProtocol = gosnmp.AES256
		sp.PrivacyPassphrase = password
	default:
		// NoAuthNoPriv
		return sp
	}
	return sp
}
