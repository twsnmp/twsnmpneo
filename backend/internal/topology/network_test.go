package topology

import (
	"context"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

func TestGetSNMPAgentForNetwork(t *testing.T) {
	nw := &datastore.NetworkEnt{
		ID:        "net1",
		Name:      "Switch-1",
		IP:        "192.168.1.1",
		SnmpMode:  "v2c",
		Community: "public",
		SnmpPort:  161,
	}

	agent := GetSNMPAgentForNetwork(nw, 2, 0)
	if agent == nil {
		t.Fatalf("expected agent to be non-nil")
	}
	if agent.Target != "192.168.1.1" {
		t.Errorf("expected target 192.168.1.1, got %s", agent.Target)
	}
	if agent.Community != "public" {
		t.Errorf("expected community public, got %s", agent.Community)
	}
	if agent.Port != 161 {
		t.Errorf("expected port 161, got %d", agent.Port)
	}

	// v3 agent
	nw3 := &datastore.NetworkEnt{
		ID:       "net2",
		Name:     "Switch-2",
		IP:       "192.168.1.2",
		SnmpMode: "v3authpriv",
		User:     "admin",
		Password: "password123",
	}
	agent3 := GetSNMPAgentForNetwork(nw3, 2, 0)
	if agent3 == nil {
		t.Fatalf("expected v3 agent to be non-nil")
	}
	if agent3.SecurityParameters == nil {
		t.Errorf("expected SecurityParameters for v3")
	}

	// Missing IP or missing v3 user should return nil
	if GetSNMPAgentForNetwork(&datastore.NetworkEnt{}, 2, 0) != nil {
		t.Errorf("expected nil for empty IP")
	}
	if GetSNMPAgentForNetwork(&datastore.NetworkEnt{IP: "1.1.1.1", SnmpMode: "v3auth"}, 2, 0) != nil {
		t.Errorf("expected nil for v3 without user")
	}
}

func TestFetchNetworkPorts_Invalid(t *testing.T) {
	ctx := context.Background()
	_, err := FetchNetworkPorts(ctx, nil, 1, 0)
	if err == nil {
		t.Errorf("expected error for nil network")
	}

	nw := &datastore.NetworkEnt{
		IP: "192.0.2.1", // Test-Net (unreachable)
	}
	// Connect will fail
	_, err = FetchNetworkPorts(ctx, nw, 1, 0)
	if err == nil {
		t.Errorf("expected error for unreachable network")
	}
	if nw.Error == "" {
		t.Errorf("expected nw.Error to be set on failure")
	}
}
