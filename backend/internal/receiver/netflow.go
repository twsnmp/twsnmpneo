package receiver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/tehmaze/netflow"
	"github.com/tehmaze/netflow/ipfix"
	"github.com/tehmaze/netflow/netflow5"
	"github.com/tehmaze/netflow/netflow9"
	"github.com/tehmaze/netflow/read"
	"github.com/tehmaze/netflow/session"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// NetFlowConfig holds options for NetFlow receiver.
type NetFlowConfig struct {
	Port     int
	LogStore *parquet.Store
}

// NetFlowServer ingests NetFlow v5, v9, and IPFIX (v10) packets over UDP.
type NetFlowServer struct {
	port     int
	logStore *parquet.Store
	mu       sync.Mutex
	decoders map[string]*netflow.Decoder
}

// NewNetFlowServer creates a new NetFlow receiver.
func NewNetFlowServer(cfg NetFlowConfig) *NetFlowServer {
	return &NetFlowServer{
		port:     cfg.Port,
		logStore: cfg.LogStore,
		decoders: make(map[string]*netflow.Decoder),
	}
}

// Start launches the UDP listener on the configured NetFlow port.
func (s *NetFlowServer) Start(ctx context.Context) error {
	if s.port <= 0 {
		return nil
	}

	addr := fmt.Sprintf(":%d", s.port)
	slog.Info("Starting NetFlow receiver", "addr", addr)

	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		slog.Warn("Failed to start NetFlow receiver", "addr", addr, "error", err)
		return fmt.Errorf("listen netflow udp: %w", err)
	}
	defer conn.Close()

	slog.Info("Started NetFlow receiver", "addr", addr)
	buf := make([]byte, 65535)

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	for {
		n, remoteAddr, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		fromIP := ""
		if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok {
			fromIP = udpAddr.IP.String()
		}
		s.handlePacket(buf[:n], fromIP, remoteAddr.String())
	}
}

func (s *NetFlowServer) getDecoder(remoteStr string) *netflow.Decoder {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, found := s.decoders[remoteStr]
	if !found {
		sess := session.New()
		d = netflow.NewDecoder(sess)
		s.decoders[remoteStr] = d
	}
	return d
}

func (s *NetFlowServer) handlePacket(data []byte, fromIP string, remoteStr string) {
	if len(data) < 4 {
		return
	}

	d := s.getDecoder(remoteStr)
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("Recovered from netflow decode panic", "error", r, "from", fromIP)
		}
	}()

	m, err := d.Read(bytes.NewBuffer(data))
	if err != nil {
		slog.Debug("Failed to decode netflow packet", "error", err, "from", fromIP)
		return
	}

	now := time.Now().UnixNano()

	switch p := m.(type) {
	case *netflow5.Packet:
		s.handleNetFlow5(p, fromIP, now)
	case *netflow9.Packet:
		s.handleNetFlow9(p, fromIP, now)
	case *ipfix.Message:
		s.handleIPFIX(p, fromIP, now)
	}
}

func (s *NetFlowServer) handleNetFlow5(p *netflow5.Packet, fromIP string, now int64) {
	for _, r := range p.Records {
		record := &datastore.NetFlowEnt{
			Time:     now,
			SrcAddr:  r.SrcAddr.String(),
			SrcPort:  int(r.SrcPort),
			DstAddr:  r.DstAddr.String(),
			DstPort:  int(r.DstPort),
			Bytes:    int(r.Bytes),
			Packets:  int(r.Packets),
			TCPFlags: read.TCPFlags(r.TCPFlags),
			Protocol: read.Protocol(r.Protocol),
			ToS:      int(r.ToS),
			Dur:      float64(r.Last-r.First) / 100.0,
		}
		if record.Protocol == "" {
			record.Protocol = fmt.Sprintf("%d", r.Protocol)
		}
		record.SrcLoc = datastore.GetLoc(record.SrcAddr)
		record.DstLoc = datastore.GetLoc(record.DstAddr)

		s.saveRecord(record, fromIP)
	}
}

func (s *NetFlowServer) handleNetFlow9(p *netflow9.Packet, fromIP string, now int64) {
	for _, ds := range p.DataFlowSets {
		if ds.Records == nil {
			continue
		}
		for _, dr := range ds.Records {
			record := &datastore.NetFlowEnt{
				Time: now,
			}
			first := 0
			last := 0
			icmpType := 0

			for _, f := range dr.Fields {
				if f.Translated == nil {
					continue
				}
				switch f.Translated.Name {
				case "sourceIPv4Address", "sourceIPv6Address":
					record.SrcAddr = getStringFromFieldValue(f.Translated.Value)
				case "sourceMacAddress", "postSourceMacAddress":
					record.SrcMAC = getStringFromFieldValue(f.Translated.Value)
				case "sourceTransportPort":
					record.SrcPort = getIntFromFieldValue(f.Translated.Value)
				case "destinationIPv4Address", "destinationIPv6Address":
					record.DstAddr = getStringFromFieldValue(f.Translated.Value)
				case "destinationMacAddress", "postDestinationMacAddress":
					record.DstMAC = getStringFromFieldValue(f.Translated.Value)
				case "destinationTransportPort":
					record.DstPort = getIntFromFieldValue(f.Translated.Value)
				case "octetDeltaCount":
					record.Bytes = getIntFromFieldValue(f.Translated.Value)
				case "packetDeltaCount":
					record.Packets = getIntFromFieldValue(f.Translated.Value)
				case "flowStartSysUpTime":
					first = getIntFromFieldValue(f.Translated.Value)
				case "flowEndSysUpTime":
					last = getIntFromFieldValue(f.Translated.Value)
				case "flowStartMilliseconds", "flowStartSeconds", "flowStartNanoSeconds":
					record.Start = getInt64FromFieldValue(f.Translated.Value)
				case "flowEndMilliseconds", "flowEndSeconds", "flowEndNanoSeconds":
					record.End = getInt64FromFieldValue(f.Translated.Value)
				case "tcpControlBits":
					record.TCPFlags = read.TCPFlags(uint8(getIntFromFieldValue(f.Translated.Value)))
				case "protocolIdentifier":
					record.Protocol = formatProtocol(uint8(getIntFromFieldValue(f.Translated.Value)))
				case "ipClassOfService":
					record.ToS = getIntFromFieldValue(f.Translated.Value)
				case "icmpTypeCodeIPv6", "icmpTypeCodeIPv4":
					icmpType = getIntFromFieldValue(f.Translated.Value)
				}
			}

			if last > 0 {
				record.Dur = float64(last-first) / 100.0
			} else if record.Start > 0 && record.End > record.Start {
				record.Dur = float64(record.End-record.Start) / (1000 * 1000 * 1000)
			}

			record.SrcLoc = datastore.GetLoc(record.SrcAddr)
			record.DstLoc = datastore.GetLoc(record.DstAddr)

			if icmpType > 0 && strings.Contains(record.Protocol, "icmp") {
				record.SrcPort = icmpType / 256
				record.DstPort = icmpType % 256
			}

			s.saveRecord(record, fromIP)
		}
	}
}

func (s *NetFlowServer) handleIPFIX(p *ipfix.Message, fromIP string, now int64) {
	for _, ds := range p.DataSets {
		if ds.Records == nil {
			continue
		}
		for _, dr := range ds.Records {
			record := &datastore.NetFlowEnt{
				Time: now,
			}
			first := 0
			last := 0
			icmpType := 0

			for _, f := range dr.Fields {
				if f.Translated == nil {
					continue
				}
				switch f.Translated.Name {
				case "sourceIPv4Address", "sourceIPv6Address":
					record.SrcAddr = getStringFromFieldValue(f.Translated.Value)
				case "sourceMacAddress", "postSourceMacAddress":
					record.SrcMAC = getStringFromFieldValue(f.Translated.Value)
				case "sourceTransportPort":
					record.SrcPort = getIntFromFieldValue(f.Translated.Value)
				case "destinationIPv4Address", "destinationIPv6Address":
					record.DstAddr = getStringFromFieldValue(f.Translated.Value)
				case "destinationMacAddress", "postDestinationMacAddress":
					record.DstMAC = getStringFromFieldValue(f.Translated.Value)
				case "destinationTransportPort":
					record.DstPort = getIntFromFieldValue(f.Translated.Value)
				case "octetDeltaCount":
					record.Bytes = getIntFromFieldValue(f.Translated.Value)
				case "packetDeltaCount":
					record.Packets = getIntFromFieldValue(f.Translated.Value)
				case "flowStartSysUpTime":
					first = getIntFromFieldValue(f.Translated.Value)
				case "flowEndSysUpTime":
					last = getIntFromFieldValue(f.Translated.Value)
				case "flowStartMilliseconds", "flowStartSeconds", "flowStartNanoSeconds":
					record.Start = getInt64FromFieldValue(f.Translated.Value)
				case "flowEndMilliseconds", "flowEndSeconds", "flowEndNanoSeconds":
					record.End = getInt64FromFieldValue(f.Translated.Value)
				case "tcpControlBits":
					record.TCPFlags = read.TCPFlags(uint8(getIntFromFieldValue(f.Translated.Value)))
				case "protocolIdentifier":
					record.Protocol = formatProtocol(uint8(getIntFromFieldValue(f.Translated.Value)))
				case "ipClassOfService":
					record.ToS = getIntFromFieldValue(f.Translated.Value)
				case "icmpTypeCodeIPv6", "icmpTypeCodeIPv4":
					icmpType = getIntFromFieldValue(f.Translated.Value)
				}
			}

			if last > 0 {
				record.Dur = float64(last-first) / 100.0
			} else if record.Start > 0 && record.End > record.Start {
				record.Dur = float64(record.End-record.Start) / (1000 * 1000 * 1000)
			}

			record.SrcLoc = datastore.GetLoc(record.SrcAddr)
			record.DstLoc = datastore.GetLoc(record.DstAddr)

			if icmpType > 0 && strings.Contains(record.Protocol, "icmp") {
				record.SrcPort = icmpType / 256
				record.DstPort = icmpType % 256
			}

			s.saveRecord(record, fromIP)
		}
	}
}

func formatProtocol(pi uint8) string {
	switch pi {
	case 1:
		return "icmp"
	case 2:
		return "igmp"
	case 6:
		return "tcp"
	case 8:
		return "egp"
	case 17:
		return "udp"
	case 58:
		return "ipv6-icmp"
	default:
		p := read.Protocol(pi)
		if p == "" {
			return fmt.Sprintf("%d", pi)
		}
		return p
	}
}

func (s *NetFlowServer) saveRecord(flow *datastore.NetFlowEnt, fromIP string) {
	if s.logStore == nil {
		return
	}
	rawJSON, err := json.Marshal(flow)
	if err != nil {
		return
	}

	// Use flow's actual source IP as ParquetLogRecord.Src. Fallback to fromIP if flow.SrcAddr is empty.
	logSrc := flow.SrcAddr
	if logSrc == "" {
		logSrc = fromIP
	}

	err = s.logStore.WriteLog(&parquet.ParquetLogRecord{
		Time: flow.Time,
		Type: "netflow",
		Src:  logSrc,
		Log:  string(rawJSON),
	})
	if err != nil {
		slog.Warn("Failed to write NetFlow record to store", "error", err)
	}
}

func getStringFromFieldValue(i any) string {
	switch v := i.(type) {
	case string:
		return v
	case net.IPAddr:
		return v.String()
	case net.IP:
		return v.String()
	case net.HardwareAddr:
		return v.String()
	}
	return ""
}

func getInt64FromFieldValue(i any) int64 {
	switch v := i.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case uint32:
		return int64(v)
	case int16:
		return int64(v)
	case uint16:
		return int64(v)
	case int8:
		return int64(v)
	case uint8:
		return int64(v)
	case int64:
		return v
	case uint64:
		return int64(v)
	case float64:
		return int64(v)
	case time.Time:
		return v.UnixNano()
	}
	return 0
}

func getIntFromFieldValue(i any) int {
	switch v := i.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case uint32:
		return int(v)
	case int16:
		return int(v)
	case uint16:
		return int(v)
	case int8:
		return int(v)
	case uint8:
		return int(v)
	case int64:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}
