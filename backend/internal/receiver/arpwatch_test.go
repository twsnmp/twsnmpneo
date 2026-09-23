package receiver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

func TestParseArpLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantIP  string
		wantMAC string
	}{
		{
			name:    "macOS format",
			line:    "? (192.168.1.203) at 0:11:32:2e:9d:8f on en0 ifscope [ethernet]",
			wantIP:  "192.168.1.203",
			wantMAC: "0:11:32:2e:9d:8f",
		},
		{
			name:    "Linux arp format",
			line:    "? (10.0.0.1) at 00:11:22:33:44:55 [ether] on eth0",
			wantIP:  "10.0.0.1",
			wantMAC: "00:11:22:33:44:55",
		},
		{
			name:    "Windows format",
			line:    "  192.168.1.1           00-11-22-33-44-55     dynamic",
			wantIP:  "192.168.1.1",
			wantMAC: "00-11-22-33-44-55",
		},
		{
			name:    "Incomplete line",
			line:    "? (192.168.1.123) at (incomplete) on en0 ifscope [ethernet]",
			wantIP:  "",
			wantMAC: "",
		},
		{
			name:    "Empty line",
			line:    "   ",
			wantIP:  "",
			wantMAC: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIP, gotMAC := parseArpLine(tt.line)
			if gotIP != tt.wantIP {
				t.Errorf("parseArpLine() gotIP = %v, want %v", gotIP, tt.wantIP)
			}
			if gotMAC != tt.wantMAC {
				t.Errorf("parseArpLine() gotMAC = %v, want %v", gotMAC, tt.wantMAC)
			}
		})
	}
}

func TestNormMACAddr(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0:11:32:2e:9d:8f", "00:11:32:2E:9D:8F"},
		{"00-11-22-33-44-55", "00:11:22:33:44:55"},
		{"AA:BB:CC:DD:EE:FF", "AA:BB:CC:DD:EE:FF"},
	}

	for _, tt := range tests {
		got := normMACAddr(tt.input)
		if got != tt.want {
			t.Errorf("normMACAddr(%s) = %s; want %s", tt.input, got, tt.want)
		}
	}
}

func TestArpWatchServer_DetectionAndLogging(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "twsnmpneo-arpwatch-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create bbolt store: %v", err)
	}
	defer store.Close()

	pqDir := filepath.Join(tmpDir, "logs")
	pqStore, err := parquet.New(parquet.Config{
		Dir:            pqDir,
		BufferSize:     10,
		BufferInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create parquet store: %v", err)
	}
	defer pqStore.Close()

	// Add a managed node
	_ = store.SaveNode(ctx, &datastore.NodeEnt{
		ID:   "node-1",
		Name: "TestRouter",
		IP:   "192.168.1.1",
		MAC:  "",
	})

	server := NewArpWatchServer(ArpWatchConfig{
		Store:    store,
		LogStore: pqStore,
		Enabled:  true,
		Range:    "192.168.1.0/24",
	})

	// 1. First discovery: New
	server.updateArpTable(ctx, "192.168.1.1", "00:11:22:33:44:55")
	_ = pqStore.Flush()

	logs, err := pqStore.Query(ctx, parquet.LogFilter{Type: "arplog", Limit: 10})
	if err != nil || len(logs) != 1 {
		t.Fatalf("expected 1 arplog record, got %d, err: %v", len(logs), err)
	}

	var firstLog datastore.ArpLogEnt
	if err := json.Unmarshal([]byte(logs[0].Log), &firstLog); err != nil {
		t.Fatalf("failed to unmarshal log: %v", err)
	}
	if firstLog.State != "New" || firstLog.IP != "192.168.1.1" || firstLog.NewMAC != "00:11:22:33:44:55" {
		t.Errorf("unexpected first log: %+v", firstLog)
	}

	// Verify node MAC auto-enriched
	server.checkNodeMAC(ctx)
	node, err := store.GetNode(ctx, "node-1")
	if err != nil || node.MAC != "00:11:22:33:44:55" {
		t.Errorf("expected node MAC 00:11:22:33:44:55, got %s, err: %v", node.MAC, err)
	}

	// 2. Unchanged update: Should not produce new log
	server.updateArpTable(ctx, "192.168.1.1", "00:11:22:33:44:55")
	_ = pqStore.Flush()
	logs2, _ := pqStore.Query(ctx, parquet.LogFilter{Type: "arplog", Limit: 10})
	if len(logs2) != 1 {
		t.Errorf("expected unchanged log count 1, got %d", len(logs2))
	}

	// 3. Change update: MAC changed!
	server.updateArpTable(ctx, "192.168.1.1", "00:99:88:77:66:55")
	_ = pqStore.Flush()
	logs3, _ := pqStore.Query(ctx, parquet.LogFilter{Type: "arplog", Limit: 10})
	if len(logs3) != 2 {
		t.Fatalf("expected 2 arplog records after change, got %d", len(logs3))
	}

	var changeLog datastore.ArpLogEnt
	if err := json.Unmarshal([]byte(logs3[0].Log), &changeLog); err != nil {
		t.Fatalf("failed to unmarshal change log: %v", err)
	}
	if changeLog.State != "Change" || changeLog.NewMAC != "00:99:88:77:66:55" || changeLog.OldMAC != "00:11:22:33:44:55" {
		t.Errorf("unexpected change log: %+v", changeLog)
	}
}
