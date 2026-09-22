package receiver

import (
	"context"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

func TestNetFlowServer_DirectParsing(t *testing.T) {
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

	// 1. Test NetFlow v5 packet
	v5Packet := make([]byte, 72)
	binary.BigEndian.PutUint16(v5Packet[0:2], 5) // version 5
	binary.BigEndian.PutUint16(v5Packet[2:4], 1) // count 1
	copy(v5Packet[24:28], net.ParseIP("192.168.1.50").To4())
	copy(v5Packet[28:32], net.ParseIP("10.0.0.1").To4())
	binary.BigEndian.PutUint32(v5Packet[40:44], 100)
	binary.BigEndian.PutUint32(v5Packet[44:48], 15000)
	binary.BigEndian.PutUint16(v5Packet[56:58], 80)
	binary.BigEndian.PutUint16(v5Packet[58:60], 43210)
	v5Packet[62] = 6

	srv.handlePacket(v5Packet, "192.168.1.50")

	// 2. Test NetFlow v9 template + data packet
	v9Packet := make([]byte, 20+16+12)
	binary.BigEndian.PutUint16(v9Packet[0:2], 9)   // version 9
	binary.BigEndian.PutUint16(v9Packet[2:4], 2)   // 2 flowsets
	binary.BigEndian.PutUint32(v9Packet[16:20], 1) // SourceID = 1
	// FlowSet 0: Template
	binary.BigEndian.PutUint16(v9Packet[20:22], 0)  // Template FlowSet
	binary.BigEndian.PutUint16(v9Packet[22:24], 16) // Length
	binary.BigEndian.PutUint16(v9Packet[24:26], 300) // Template ID = 300
	binary.BigEndian.PutUint16(v9Packet[26:28], 2)   // Field count = 2
	binary.BigEndian.PutUint16(v9Packet[28:30], 8)   // IPV4_SRC_ADDR
	binary.BigEndian.PutUint16(v9Packet[30:32], 4)
	binary.BigEndian.PutUint16(v9Packet[32:34], 12)  // IPV4_DST_ADDR
	binary.BigEndian.PutUint16(v9Packet[34:36], 4)
	// FlowSet 1: Data
	binary.BigEndian.PutUint16(v9Packet[36:38], 300) // Data FlowSet ID = 300
	binary.BigEndian.PutUint16(v9Packet[38:40], 12)
	copy(v9Packet[40:44], net.ParseIP("172.16.0.5").To4())
	copy(v9Packet[44:48], net.ParseIP("172.16.0.10").To4())

	srv.handlePacket(v9Packet, "192.168.1.50")

	// 3. Test IPFIX (v10) fallback packet
	ipfixPacket := make([]byte, 20)
	binary.BigEndian.PutUint16(ipfixPacket[0:2], 10) // version 10
	binary.BigEndian.PutUint16(ipfixPacket[2:4], 20)

	srv.handlePacket(ipfixPacket, "192.168.1.50")

	// Query parquet logs
	ctx := context.Background()
	logs, err := store.Query(ctx, parquet.LogFilter{
		Type:  "netflow",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(logs) != 3 {
		t.Fatalf("expected 3 netflow logs, got %d", len(logs))
	}

	for _, l := range logs {
		if l.Type != "netflow" {
			t.Errorf("expected type netflow, got %s", l.Type)
		}
		if l.Src != "192.168.1.50" {
			t.Errorf("expected src 192.168.1.50, got %s", l.Src)
		}
	}
}
