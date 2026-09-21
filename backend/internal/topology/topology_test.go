package topology

import (
	"context"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
)

func TestTopologyDiscovery(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	store, err := bbolt.New(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer store.Close()

	// Add test network
	net1 := &datastore.NetworkEnt{
		ID:        "hub1",
		Name:      "SW-HUB-1",
		IP:        "192.168.1.254",
		Unmanaged: true,
		Ports: []datastore.PortEnt{
			{ID: "p1", Name: "#1"},
			{ID: "p2", Name: "#2"},
		},
	}
	if err := store.SaveNetwork(ctx, net1); err != nil {
		t.Fatalf("failed to save network: %v", err)
	}

	// Add test node on same /24
	node1 := &datastore.NodeEnt{
		ID:   "n1",
		Name: "Server-1",
		IP:   "192.168.1.10",
	}
	if err := store.SaveNode(ctx, node1); err != nil {
		t.Fatalf("failed to save node: %v", err)
	}

	// 1. Test FindTopologyForNetwork
	resp, err := FindTopologyForNetwork(ctx, store, net1)
	if err != nil {
		t.Fatalf("FindTopologyForNetwork error: %v", err)
	}
	if len(resp.Lines) != 1 {
		t.Fatalf("expected 1 candidate line, got %d", len(resp.Lines))
	}
	if resp.Lines[0].NodeID1 != "NET:hub1" || resp.Lines[0].NodeID2 != "n1" {
		t.Fatalf("unexpected line: %+v", resp.Lines[0])
	}

	// 2. Test FindNodeConnection
	cand, err := FindNodeConnection(ctx, store, "n1")
	if err != nil {
		t.Fatalf("FindNodeConnection error: %v", err)
	}
	if len(cand) != 1 {
		t.Fatalf("expected 1 candidate line for n1, got %d", len(cand))
	}

	// 3. Test ConnectCandidateLines
	lines := []datastore.LineEnt{cand[0].LineEnt}
	connected, err := ConnectCandidateLines(ctx, store, lines)
	if err != nil || connected != 1 {
		t.Fatalf("ConnectCandidateLines failed: count=%d, err=%v", connected, err)
	}

	// 4. Test filtering existing line
	respAfter, err := FindTopologyForNetwork(ctx, store, net1)
	if err != nil {
		t.Fatalf("FindTopologyForNetwork error: %v", err)
	}
	if len(respAfter.Lines) != 0 {
		t.Fatalf("expected 0 candidate lines after connecting, got %d", len(respAfter.Lines))
	}
}
