package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/ai"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
)

func TestNormalizeMCPMode(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"auth", "auth", false},
		{"AUTH", "auth", false},
		{" noauth ", "noauth", false},
		{"invalid", "", true},
	}

	for _, tc := range tests {
		got, err := normalizeMCPMode(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeMCPMode(%q): expected error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeMCPMode(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeMCPMode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildMCPConfig_CLIOverrides(t *testing.T) {
	dir := t.TempDir()
	store, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.SaveMapConf(ctx, &datastore.MapConfEnt{
		EnableMCP: true,
		MCPMode:   "noauth",
		MCPFrom:   "192.168.1.99",
	}); err != nil {
		t.Fatalf("SaveMapConf failed: %v", err)
	}

	cfg := serverConfig{
		mcpMode: "auth",
		mcpFrom: "10.10.0.0/16",
		explicitFlags: map[string]bool{
			"mcpMode": true,
			"mcpFrom": true,
		},
	}
	mcpCfg := buildMCPConfig(cfg, store, nil, nil, nil, nil)
	if !mcpCfg.ExplicitMode || !mcpCfg.ExplicitFrom {
		t.Fatalf("expected explicit flags to propagate to MCPConfig: %+v", mcpCfg)
	}

	srv := ai.NewMCPServer(mcpCfg)
	if !srv.CheckFromAddress("10.10.5.6:1234", "") {
		t.Error("expected CLI CIDR 10.10.0.0/16 to be allowed")
	}
	if srv.CheckFromAddress("192.168.1.99:1234", "") {
		t.Error("expected DB MCPFrom 192.168.1.99 to be overridden by CLI -mcpFrom")
	}
}
