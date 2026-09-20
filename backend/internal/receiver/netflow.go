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

// NetFlowServer ingests NetFlow v5/v9 packets over UDP.
type NetFlowServer struct {
	port     int
	logStore *parquet.Store
	mu       sync.Mutex
	running  bool
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
}

func NewNetFlowServer(cfg NetFlowConfig) *NetFlowServer {
	return &NetFlowServer{
		port:     cfg.Port,
		logStore: cfg.LogStore,
	}
}

func (s *NetFlowServer) Start(ctx context.Context) error {
	if s.port <= 0 {
		return nil
	}

	addr := fmt.Sprintf(":%d", s.port)
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("listen netflow udp: %w", err)
	}
	defer conn.Close()

	slog.Info("Starting NetFlow receiver", "addr", addr)
	buf := make([]byte, 8192)

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

	// NetFlow v5 parsing (header is 24 bytes, record is 48 bytes)
	if version == 5 && len(data) >= 24 {
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

			flow := &NetFlowRecord{
				Time:     now,
				Version:  version,
				SrcIP:    srcIP,
				DstIP:    dstIP,
				SrcPort:  srcPort,
				DstPort:  dstPort,
				Protocol: prot,
				Packets:  packets,
				Bytes:    bytes,
			}

			if s.logStore != nil {
				rawJSON, _ := json.Marshal(flow)
				_ = s.logStore.WriteLog(&parquet.ParquetLogRecord{
					Time: now,
					Type: "netflow",
					Src:  fromIP,
					Log:  string(rawJSON),
				})
			}
			offset += 48
		}
	}
}
