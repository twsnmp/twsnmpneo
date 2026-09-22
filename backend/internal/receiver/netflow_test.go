package receiver

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

func TestNetFlowServer_V5Parsing(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-netflow-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(dir, "logs"),
		BufferSize:     1,
		BufferInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}
	defer store.Close()

	srv := NewNetFlowServer(NetFlowConfig{
		Port:     2055,
		LogStore: store,
	})

	// Build a valid NetFlow v5 packet (24 bytes header + 48 bytes record)
	v5Packet := make([]byte, 24+48)
	binary.BigEndian.PutUint16(v5Packet[0:2], 5)       // version 5
	binary.BigEndian.PutUint16(v5Packet[2:4], 1)       // count 1
	binary.BigEndian.PutUint32(v5Packet[4:8], 1000000) // sysUptime
	binary.BigEndian.PutUint32(v5Packet[8:12], uint32(time.Now().Unix()))

	// Flow record offset: 24
	copy(v5Packet[24:28], net.ParseIP("192.168.1.100").To4()) // SrcAddr (0-3)
	copy(v5Packet[28:32], net.ParseIP("192.168.1.200").To4()) // DstAddr (4-7)
	// NextHop (8-11: 32-35), Input (12-13: 36-37), Output (14-15: 38-39)
	binary.BigEndian.PutUint32(v5Packet[40:44], 15)            // Packets (16-19: 40-43) = 15
	binary.BigEndian.PutUint32(v5Packet[44:48], 4500)          // Bytes (20-23: 44-47) = 4500
	binary.BigEndian.PutUint32(v5Packet[48:52], 1000)          // First (24-27: 48-51) = 1000
	binary.BigEndian.PutUint32(v5Packet[52:56], 1249)          // Last (28-31: 52-55) = 1249 (dur = 2.49s)
	binary.BigEndian.PutUint16(v5Packet[56:58], 8080)          // SrcPort (32-33: 56-57) = 8080
	binary.BigEndian.PutUint16(v5Packet[58:60], 54321)         // DstPort (34-35: 58-59) = 54321
	v5Packet[61] = 0x12                                        // TCPFlags (37: 61): SYN + ACK
	v5Packet[62] = 6                                           // Protocol (38: 62): TCP (6)

	exporterIP := "10.0.0.1"
	srv.handlePacket(v5Packet, exporterIP, "10.0.0.1:2055")

	// Query parquet logs
	ctx := context.Background()
	logs, err := store.Query(ctx, parquet.LogFilter{
		Type:  "netflow",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 netflow log, got %d", len(logs))
	}

	l := logs[0]
	// Verify that log.Src is the FLOW's SrcAddr, NOT the exporter IP!
	if l.Src != "192.168.1.100" {
		t.Errorf("expected flow Src 192.168.1.100, got %s", l.Src)
	}

	// Verify decoded JSON content
	var ent datastore.NetFlowEnt
	if err := json.Unmarshal([]byte(l.Log), &ent); err != nil {
		t.Fatalf("failed to unmarshal NetFlow log json: %v", err)
	}

	if ent.SrcAddr != "192.168.1.100" {
		t.Errorf("ent.SrcAddr = %s, expected 192.168.1.100", ent.SrcAddr)
	}
	if ent.DstAddr != "192.168.1.200" {
		t.Errorf("ent.DstAddr = %s, expected 192.168.1.200", ent.DstAddr)
	}
	if ent.SrcPort != 8080 {
		t.Errorf("ent.SrcPort = %d, expected 8080", ent.SrcPort)
	}
	if ent.DstPort != 54321 {
		t.Errorf("ent.DstPort = %d, expected 54321", ent.DstPort)
	}
	if ent.Protocol != "tcp" {
		t.Errorf("ent.Protocol = %s, expected tcp", ent.Protocol)
	}
	if ent.Bytes != 4500 {
		t.Errorf("ent.Bytes = %d, expected 4500", ent.Bytes)
	}
	if ent.Packets != 15 {
		t.Errorf("ent.Packets = %d, expected 15", ent.Packets)
	}
	if ent.Dur != 2.49 {
		t.Errorf("ent.Dur = %f, expected 2.49", ent.Dur)
	}
	if ent.SrcLoc != "LOCAL,0,0," {
		t.Errorf("ent.SrcLoc = %s, expected LOCAL,0,0,", ent.SrcLoc)
	}
}
