package receiver

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// NetFlowConfig holds options for NetFlow receiver.
type NetFlowConfig struct {
	Port     int
	LogStore *parquet.Store
}

type v9TemplateField struct {
	Type   uint16
	Length uint16
}

type v9Template struct {
	fields   []v9TemplateField
	totalLen int
}

// NetFlowServer ingests NetFlow v5, v9, and IPFIX (v10) packets over UDP.
type NetFlowServer struct {
	port      int
	logStore  *parquet.Store
	mu        sync.RWMutex
	templates map[string]*v9Template
}

// NetFlowRecord holds decoded flow information.
type NetFlowRecord struct {
	Time     int64  `json:"time"`
	Version  uint16 `json:"version"`
	SrcIP    string `json:"srcIP"`
	DstIP    string `json:"dstIP"`
	SrcPort  uint16 `json:"srcPort"`
	DstPort  uint16 `json:"dstPort"`
	Protocol uint8  `json:"protocol"`
	Packets  uint32 `json:"packets"`
	Bytes    uint32 `json:"bytes"`
	Info     string `json:"info,omitempty"`
}

// NewNetFlowServer creates a new NetFlow receiver.
func NewNetFlowServer(cfg NetFlowConfig) *NetFlowServer {
	return &NetFlowServer{
		port:      cfg.Port,
		logStore:  cfg.LogStore,
		templates: make(map[string]*v9Template),
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
		s.handlePacket(buf[:n], fromIP)
	}
}

func (s *NetFlowServer) handlePacket(data []byte, fromIP string) {
	if len(data) < 4 {
		return
	}
	version := binary.BigEndian.Uint16(data[0:2])
	now := time.Now().UnixNano()

	recordsDecoded := 0

	switch version {
	case 5:
		// NetFlow v5 (Header: 24 bytes, each Record: 48 bytes)
		if len(data) >= 24 {
			count := int(binary.BigEndian.Uint16(data[2:4]))
			offset := 24
			for i := 0; i < count && offset+48 <= len(data); i++ {
				rec := data[offset : offset+48]
				srcIP := net.IP(rec[0:4]).String()
				dstIP := net.IP(rec[4:8]).String()
				packets := binary.BigEndian.Uint32(rec[16:20])
				bytes := binary.BigEndian.Uint32(rec[20:24])
				srcPort := binary.BigEndian.Uint16(rec[32:34])
				dstPort := binary.BigEndian.Uint16(rec[34:36])
				prot := rec[38]

				s.saveRecord(&NetFlowRecord{
					Time:     now,
					Version:  version,
					SrcIP:    srcIP,
					DstIP:    dstIP,
					SrcPort:  srcPort,
					DstPort:  dstPort,
					Protocol: prot,
					Packets:  packets,
					Bytes:    bytes,
				}, fromIP)
				recordsDecoded++
				offset += 48
			}
		}

	case 9:
		// NetFlow v9 (Header: 20 bytes)
		if len(data) >= 20 {
			sourceID := binary.BigEndian.Uint32(data[16:20])
			offset := 20
			for offset+4 <= len(data) {
				flowsetID := binary.BigEndian.Uint16(data[offset : offset+2])
				flowsetLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
				if flowsetLen < 4 || offset+flowsetLen > len(data) {
					break
				}
				fsData := data[offset+4 : offset+flowsetLen]

				if flowsetID == 0 {
					// Template FlowSet
					s.parseV9Templates(fromIP, sourceID, fsData)
				} else if flowsetID >= 256 {
					// Data FlowSet
					n := s.parseV9DataFlowSet(fromIP, sourceID, flowsetID, fsData, now)
					recordsDecoded += n
				}
				offset += flowsetLen
			}
		}

	case 10:
		// IPFIX (Header: 16 bytes)
		if len(data) >= 16 {
			domainID := binary.BigEndian.Uint32(data[12:16])
			offset := 16
			for offset+4 <= len(data) {
				setID := binary.BigEndian.Uint16(data[offset : offset+2])
				setLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
				if setLen < 4 || offset+setLen > len(data) {
					break
				}
				setData := data[offset+4 : offset+setLen]

				if setID == 2 {
					// IPFIX Template Set
					s.parseV9Templates(fromIP, domainID, setData)
				} else if setID >= 256 {
					// IPFIX Data Set
					n := s.parseV9DataFlowSet(fromIP, domainID, setID, setData, now)
					recordsDecoded += n
				}
				offset += setLen
			}
		}
	}

	// Fallback: If no individual records could be extracted, write packet summary
	// so the traffic is always recorded in Parquet and visible in the UI
	if recordsDecoded == 0 {
		s.saveRecord(&NetFlowRecord{
			Time:    now,
			Version: version,
			SrcIP:   fromIP,
			DstIP:   "-",
			Bytes:   uint32(len(data)),
			Info:    fmt.Sprintf("NetFlow v%d packet (%d bytes)", version, len(data)),
		}, fromIP)
	}
}

func (s *NetFlowServer) parseV9Templates(fromIP string, sourceID uint32, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	offset := 0
	for offset+4 <= len(data) {
		tmplID := binary.BigEndian.Uint16(data[offset : offset+2])
		fieldCount := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		offset += 4

		tmpl := &v9Template{
			fields: make([]v9TemplateField, 0, fieldCount),
		}

		for i := 0; i < fieldCount && offset+4 <= len(data); i++ {
			fType := binary.BigEndian.Uint16(data[offset : offset+2])
			fLen := binary.BigEndian.Uint16(data[offset+2 : offset+4])
			offset += 4

			tmpl.fields = append(tmpl.fields, v9TemplateField{
				Type:   fType,
				Length: fLen,
			})
			tmpl.totalLen += int(fLen)
		}

		key := fmt.Sprintf("%s:%d:%d", fromIP, sourceID, tmplID)
		s.templates[key] = tmpl
	}
}

func (s *NetFlowServer) parseV9DataFlowSet(fromIP string, sourceID uint32, tmplID uint16, data []byte, now int64) int {
	s.mu.RLock()
	key := fmt.Sprintf("%s:%d:%d", fromIP, sourceID, tmplID)
	tmpl, exists := s.templates[key]
	s.mu.RUnlock()

	if !exists || tmpl.totalLen <= 0 {
		// Template not known yet, save summary for flowset
		s.saveRecord(&NetFlowRecord{
			Time:    now,
			Version: 9,
			SrcIP:   fromIP,
			DstIP:   "-",
			Bytes:   uint32(len(data)),
			Info:    fmt.Sprintf("Data FlowSet %d (%d bytes)", tmplID, len(data)),
		}, fromIP)
		return 1
	}

	offset := 0
	count := 0
	for offset+tmpl.totalLen <= len(data) {
		rec := data[offset : offset+tmpl.totalLen]
		flow := &NetFlowRecord{
			Time:    now,
			Version: 9,
			SrcIP:   fromIP,
			DstIP:   "-",
		}

		fOffset := 0
		for _, f := range tmpl.fields {
			if fOffset+int(f.Length) > len(rec) {
				break
			}
			val := rec[fOffset : fOffset+int(f.Length)]
			fOffset += int(f.Length)

			switch f.Type {
			case 1: // IN_BYTES
				if len(val) == 4 {
					flow.Bytes = binary.BigEndian.Uint32(val)
				} else if len(val) == 8 {
					flow.Bytes = uint32(binary.BigEndian.Uint64(val))
				}
			case 2: // IN_PKTS
				if len(val) == 4 {
					flow.Packets = binary.BigEndian.Uint32(val)
				} else if len(val) == 8 {
					flow.Packets = uint32(binary.BigEndian.Uint64(val))
				}
			case 4: // PROTOCOL
				if len(val) > 0 {
					flow.Protocol = val[0]
				}
			case 7: // L4_SRC_PORT
				if len(val) >= 2 {
					flow.SrcPort = binary.BigEndian.Uint16(val)
				}
			case 8: // IPV4_SRC_ADDR
				if len(val) >= 4 {
					flow.SrcIP = net.IP(val[:4]).String()
				}
			case 11: // L4_DST_PORT
				if len(val) >= 2 {
					flow.DstPort = binary.BigEndian.Uint16(val)
				}
			case 12: // IPV4_DST_ADDR
				if len(val) >= 4 {
					flow.DstIP = net.IP(val[:4]).String()
				}
			case 27: // IPV6_SRC_ADDR
				if len(val) >= 16 {
					flow.SrcIP = net.IP(val[:16]).String()
				}
			case 28: // IPV6_DST_ADDR
				if len(val) >= 16 {
					flow.DstIP = net.IP(val[:16]).String()
				}
			}
		}

		s.saveRecord(flow, fromIP)
		count++
		offset += tmpl.totalLen
	}
	return count
}

func (s *NetFlowServer) saveRecord(flow *NetFlowRecord, fromIP string) {
	if s.logStore == nil {
		return
	}
	rawJSON, err := json.Marshal(flow)
	if err != nil {
		return
	}
	err = s.logStore.WriteLog(&parquet.ParquetLogRecord{
		Time: flow.Time,
		Type: "netflow",
		Src:  fromIP,
		Log:  string(rawJSON),
	})
	if err != nil {
		slog.Warn("Failed to write NetFlow record to store", "error", err)
	}
}
