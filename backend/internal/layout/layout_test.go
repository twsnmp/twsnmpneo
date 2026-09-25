package layout

import (
	"context"
	"os"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
)

func setupTestStore(t *testing.T) (datastore.DataStore, func()) {
	td, err := os.MkdirTemp("", "layout_test_*")
	if err != nil {
		t.Fatal(err)
	}
	store, err := bbolt.New(td + "/test.db")
	if err != nil {
		os.RemoveAll(td)
		t.Fatal(err)
	}
	cleanup := func() {
		_ = store.Close()
		os.RemoveAll(td)
	}
	return store, cleanup
}

func TestLayoutHierarchicalAndUndo(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	gw := &datastore.NodeEnt{
		ID:   "gw1",
		Name: "Gateway",
		IP:   "192.168.1.1",
		Icon: "router",
		X:    10,
		Y:    10,
	}
	_ = store.SaveNode(ctx, gw)

	sw := &datastore.NetworkEnt{
		ID:   "sw1",
		Name: "CoreSwitch",
		IP:   "192.168.1.2",
		X:    20,
		Y:    20,
		W:    420,
		H:    90,
		Ports: []datastore.PortEnt{
			{ID: "p1", Name: "port1", X: 0, Y: 0},
			{ID: "p2", Name: "port2", X: 1, Y: 0},
		},
	}
	_ = store.SaveNetwork(ctx, sw)

	n1 := &datastore.NodeEnt{
		ID:   "n1",
		Name: "Server1",
		IP:   "192.168.1.10",
		Icon: "server",
		X:    30,
		Y:    30,
	}
	_ = store.SaveNode(ctx, n1)

	n2 := &datastore.NodeEnt{
		ID:   "n2",
		Name: "PC1",
		IP:   "192.168.1.50",
		Icon: "desktop",
		X:    40,
		Y:    40,
	}
	_ = store.SaveNode(ctx, n2)

	_ = store.SaveLine(ctx, &datastore.LineEnt{
		ID:         "l1",
		NodeID1:    "NET:" + sw.ID,
		PollingID1: "p1",
		NodeID2:    n1.ID,
	})
	_ = store.SaveLine(ctx, &datastore.LineEnt{
		ID:         "l2",
		NodeID1:    "NET:" + sw.ID,
		PollingID1: "p2",
		NodeID2:    n2.ID,
	})

	count, err := OptimizeLayout(ctx, store, AutoLayoutHierarchical)
	if err != nil {
		t.Fatalf("OptimizeLayout error: %v", err)
	}
	if count < 4 {
		t.Fatalf("expected at least 4 items laid out, got %d", count)
	}

	savedGW, _ := store.GetNode(ctx, gw.ID)
	if savedGW.Y != 100 {
		t.Errorf("expected gateway Y=100, got %d", savedGW.Y)
	}

	savedSW, _ := store.GetNetwork(ctx, sw.ID)
	if savedSW.Y != 280 {
		t.Errorf("expected switch Y=280, got %d", savedSW.Y)
	}

	if !HasUndoLayout() {
		t.Error("expected HasUndoLayout() to be true")
	}

	undoCount, err := UndoLayout(ctx, store)
	if err != nil {
		t.Fatalf("UndoLayout error: %v", err)
	}
	if undoCount < 4 {
		t.Fatalf("expected at least 4 items restored, got %d", undoCount)
	}

	restoredGW, _ := store.GetNode(ctx, gw.ID)
	if restoredGW.X != 10 || restoredGW.Y != 10 {
		t.Errorf("expected gateway restored to (10, 10), got (%d, %d)", restoredGW.X, restoredGW.Y)
	}
}

func TestLayoutClusterAndCategorized(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	n1 := &datastore.NodeEnt{ID: "n1", Name: "Router1", IP: "192.168.1.1", Icon: "router"}
	n2 := &datastore.NodeEnt{ID: "n2", Name: "Server1", IP: "192.168.1.10", Icon: "server"}
	n3 := &datastore.NodeEnt{ID: "n3", Name: "PC1", IP: "192.168.1.50", Icon: "desktop"}
	_ = store.SaveNode(ctx, n1)
	_ = store.SaveNode(ctx, n2)
	_ = store.SaveNode(ctx, n3)

	countCluster, err := OptimizeLayout(ctx, store, AutoLayoutCluster)
	if err != nil {
		t.Fatalf("Cluster layout error: %v", err)
	}
	if countCluster < 3 {
		t.Errorf("expected at least 3 items in cluster, got %d", countCluster)
	}

	countCat, err := OptimizeLayout(ctx, store, AutoLayoutCategorized)
	if err != nil {
		t.Fatalf("Categorized layout error: %v", err)
	}
	if countCat < 3 {
		t.Errorf("expected at least 3 items in categorized, got %d", countCat)
	}
}
