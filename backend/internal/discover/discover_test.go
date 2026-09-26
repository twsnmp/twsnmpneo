package discover

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
)

func setupTestStore(t *testing.T) (*bbolt.Store, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "twsnmpneo-discover-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, "test.db")
	store, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	cleanup := func() {
		_ = store.Close()
		_ = os.RemoveAll(dir)
	}
	return store, cleanup
}

func TestGetDiscoverAddressRange(t *testing.T) {
	ranges := GetDiscoverAddressRange()
	// Should return pairs of start/end IP or empty if no viable physical interface
	if len(ranges)%2 != 0 {
		t.Errorf("expected even number of IP range endpoints, got %d", len(ranges))
	}
}

func TestGetSnmpConfigs(t *testing.T) {
	mapConf := &datastore.MapConfEnt{
		SnmpMode:  "v2c",
		Community: "public",
	}
	discConf := &datastore.DiscoverConfEnt{
		SnmpConfigs: []datastore.SnmpConfEnt{
			{SnmpMode: "v2c", Community: "public"}, // duplicate with MapConf, should be skipped
			{SnmpMode: "v3auth", SnmpUser: "admin", SnmpPassword: "pwd"},
		},
	}

	configs := getSnmpConfigs(discConf, mapConf)
	if len(configs) != 2 {
		t.Fatalf("expected 2 unique SNMP configs, got %d", len(configs))
	}
	if configs[0].SnmpMode != "v2c" || configs[0].Community != "public" {
		t.Errorf("expected first config to be mapConf, got %+v", configs[0])
	}
	if configs[1].SnmpMode != "v3auth" || configs[1].SnmpUser != "admin" {
		t.Errorf("expected second config to be added snmpConf, got %+v", configs[1])
	}
}

func TestDiscover_Lifecycle(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	engine := &Engine{}

	// 1. Invalid IPs
	err := engine.StartDiscover(ctx, store, &datastore.DiscoverConfEnt{
		StartIP: "192.168.1.100",
		EndIP:   "192.168.1.10",
	})
	if err == nil {
		t.Fatal("expected error when startIP > endIP")
	}

	// 2. Normal Start and Stop
	conf := &datastore.DiscoverConfEnt{
		StartIP:    "127.0.0.1",
		EndIP:      "127.0.0.2",
		Timeout:    1,
		Retry:      0,
		AddPolling: false,
	}

	if err := engine.StartDiscover(ctx, store, conf); err != nil {
		t.Fatalf("start discover failed: %v", err)
	}

	stats := engine.GetDiscoverStats()
	if !stats.Running {
		t.Error("expected engine to be running")
	}
	if stats.Total != 2 {
		t.Errorf("expected total=2, got %d", stats.Total)
	}

	// Double start should fail
	if err := engine.StartDiscover(ctx, store, conf); err == nil {
		t.Fatal("expected error when starting already running discovery")
	}

	engine.StopDiscover()
	time.Sleep(100 * time.Millisecond)

	finalStats := engine.GetDiscoverStats()
	if finalStats.Running {
		t.Error("expected engine to be stopped")
	}
}

func TestDiscover_SkipNonExistentIP(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	engine := &Engine{}

	// Scan 192.0.2.1 - 192.0.2.2 (TEST-NET-1, non-existent)
	conf := &datastore.DiscoverConfEnt{
		StartIP:    "192.0.2.1",
		EndIP:      "192.0.2.2",
		Timeout:    1,
		Retry:      0,
		AddPolling: false,
	}

	if err := engine.StartDiscover(ctx, store, conf); err != nil {
		t.Fatalf("start discover failed: %v", err)
	}

	// Wait for scan to complete
	deadline := time.Now().Add(5 * time.Second)
	for engine.GetDiscoverStats().Running && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}

	stats := engine.GetDiscoverStats()
	if stats.Running {
		engine.StopDiscover()
		t.Fatal("scan did not finish in time")
	}

	if stats.Found != 0 {
		t.Errorf("expected 0 found nodes for non-existent IP range, got %d", stats.Found)
	}

	nodes, _ := store.ListNodes(ctx)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes in store, got %d", len(nodes))
	}
}

