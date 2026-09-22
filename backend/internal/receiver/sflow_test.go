package receiver

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Cistern/sflow"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

func TestSFlowServer_SaveRawPacketFlow(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-sflow-test-*")
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

	srv := NewSFlowServer(SFlowConfig{
		Port:     6343,
		LogStore: store,
	})

	// Create an Ethernet + IPv4 + UDP packet header
	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		DstMAC:       net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		Length:   128,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    net.ParseIP("192.168.1.10"),
		DstIP:    net.ParseIP("192.168.1.20"),
	}
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(12345),
		DstPort: layers.UDPPort(80),
	}
	_ = udp.SetNetworkLayerForChecksum(ip)

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}
	if err := gopacket.SerializeLayers(buf, opts, eth, ip, udp, gopacket.Payload([]byte("hello sflow"))); err != nil {
		t.Fatalf("serialize packet: %v", err)
	}

	rawFlow := &sflow.RawPacketFlow{
		Header: buf.Bytes(),
	}

	srv.saveRawPacketFlow(rawFlow, 0)

	// Query parquet logs
	ctx := context.Background()
	logs, err := store.Query(ctx, parquet.LogFilter{
		Type:  "sflow",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query sflow logs failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 sflow log, got %d", len(logs))
	}

	var ent datastore.SFlowEnt
	if err := json.Unmarshal([]byte(logs[0].Log), &ent); err != nil {
		t.Fatalf("failed to unmarshal sflow json: %v", err)
	}

	if ent.SrcAddr != "192.168.1.10" {
		t.Errorf("ent.SrcAddr = %s, expected 192.168.1.10", ent.SrcAddr)
	}
	if ent.DstAddr != "192.168.1.20" {
		t.Errorf("ent.DstAddr = %s, expected 192.168.1.20", ent.DstAddr)
	}
	if ent.SrcPort != 12345 {
		t.Errorf("ent.SrcPort = %d, expected 12345", ent.SrcPort)
	}
	if ent.DstPort != 80 {
		t.Errorf("ent.DstPort = %d, expected 80", ent.DstPort)
	}
	if ent.Protocol != "udp" {
		t.Errorf("ent.Protocol = %s, expected udp", ent.Protocol)
	}
	if ent.SrcMAC != "00:11:22:33:44:55" {
		t.Errorf("ent.SrcMAC = %s, expected 00:11:22:33:44:55", ent.SrcMAC)
	}
	if ent.DstMAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("ent.DstMAC = %s, expected aa:bb:cc:dd:ee:ff", ent.DstMAC)
	}
	if ent.Bytes != 39 {
		t.Errorf("ent.Bytes = %d, expected 39", ent.Bytes)
	}
}

func TestSFlowServer_SaveCounter(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-sflow-counter-test-*")
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

	srv := NewSFlowServer(SFlowConfig{
		Port:     6343,
		LogStore: store,
	})

	srv.saveCounter("HostCPUCounter", "10.0.0.1", map[string]any{
		"loadOne": 0.5,
	})

	ctx := context.Background()
	logs, err := store.Query(ctx, parquet.LogFilter{
		Type:  "sflowCounter",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query sflowCounter logs failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 sflowCounter log, got %d", len(logs))
	}

	var ent datastore.SFlowCounterEnt
	if err := json.Unmarshal([]byte(logs[0].Log), &ent); err != nil {
		t.Fatalf("failed to unmarshal sflowCounter json: %v", err)
	}

	if ent.Type != "HostCPUCounter" {
		t.Errorf("ent.Type = %s, expected HostCPUCounter", ent.Type)
	}
	if ent.Remote != "10.0.0.1" {
		t.Errorf("ent.Remote = %s, expected 10.0.0.1", ent.Remote)
	}
}

func TestSFlowServer_Lifecycle(t *testing.T) {
	srv := NewSFlowServer(SFlowConfig{
		Port: 0, // Disabled
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("disabled server Start() returned error: %v", err)
	}
}
