package receiver_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/receiver"
)

func setupTestLogStore(t *testing.T) (*parquet.Store, string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "twsnmpneo-receiver-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	store, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(dir, "logs"),
		BufferSize:     2,
		BufferInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}

	cleanup := func() {
		_ = store.Close()
		_ = os.RemoveAll(dir)
	}
	return store, dir, cleanup
}

func TestSyslog_UDPAndTCP(t *testing.T) {
	store, _, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get free ports for testing
	udpL, _ := net.ListenPacket("udp", "127.0.0.1:0")
	udpPort := udpL.LocalAddr().(*net.UDPAddr).Port
	_ = udpL.Close()

	tcpL, _ := net.Listen("tcp", "127.0.0.1:0")
	tcpPort := tcpL.Addr().(*net.TCPAddr).Port
	_ = tcpL.Close()

	srv := receiver.NewSyslogServer(receiver.SyslogConfig{
		UDPPort:  udpPort,
		TCPPort:  tcpPort,
		LogStore: store,
	})

	go func() {
		_ = srv.Start(ctx)
	}()
	time.Sleep(50 * time.Millisecond) // Allow listeners to bind

	// 1. Send UDP Syslog message
	udpConn, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", udpPort))
	if err != nil {
		t.Fatalf("dial udp syslog failed: %v", err)
	}
	_, _ = udpConn.Write([]byte("<14>Sep 20 12:00:00 myhost sudo: pam_unix authentication failure\n"))
	_ = udpConn.Close()

	// 2. Send TCP Syslog message
	tcpConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", tcpPort))
	if err != nil {
		t.Fatalf("dial tcp syslog failed: %v", err)
	}
	_, _ = tcpConn.Write([]byte("<134>Sep 20 12:00:01 web01 nginx: 404 GET /notfound\n"))
	_ = tcpConn.Close()

	time.Sleep(150 * time.Millisecond) // Wait for processing & flush

	// 3. Verify Parquet logs
	logs, err := store.Query(ctx, parquet.LogFilter{
		Type:  "syslog",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query syslog failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 syslog logs in parquet, got %d", len(logs))
	}
}

func TestSyslog_Parser(t *testing.T) {
	msg := receiver.ParseSyslog("<134>web01 sshd: Accepted publickey", "192.168.1.50")
	if msg.Facility != 16 || msg.Severity != 6 {
		t.Errorf("expected fac 16, sev 6, got fac %d, sev %d", msg.Facility, msg.Severity)
	}
	if msg.Tag != "web01 sshd" {
		t.Errorf("unexpected tag: %s", msg.Tag)
	}
	if msg.Message != "Accepted publickey" {
		t.Errorf("unexpected message: %s", msg.Message)
	}
}

func TestNetFlow_Ingestion(t *testing.T) {
	store, _, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get free UDP port
	udpL, _ := net.ListenPacket("udp", "127.0.0.1:0")
	nfPort := udpL.LocalAddr().(*net.UDPAddr).Port
	_ = udpL.Close()

	srv := receiver.NewNetFlowServer(receiver.NetFlowConfig{
		Port:     nfPort,
		LogStore: store,
	})

	go func() {
		_ = srv.Start(ctx)
	}()
	time.Sleep(50 * time.Millisecond)

	// Short invalid packet
	conn, _ := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", nfPort))
	_, _ = conn.Write([]byte{0x00, 0x01})

	// Build minimal NetFlow v5 packet (24 header + 48 record = 72 bytes)
	packet := make([]byte, 72)
	binary.BigEndian.PutUint16(packet[0:2], 5) // version 5
	binary.BigEndian.PutUint16(packet[2:4], 1) // count 1
	// Record offset 24
	copy(packet[24:28], net.ParseIP("192.168.1.100").To4()) // SrcIP
	copy(packet[28:32], net.ParseIP("8.8.8.8").To4())        // DstIP
	binary.BigEndian.PutUint32(packet[40:44], 10)           // packets
	binary.BigEndian.PutUint32(packet[44:48], 1500)         // bytes
	binary.BigEndian.PutUint16(packet[56:58], 443)          // srcPort
	binary.BigEndian.PutUint16(packet[58:60], 54321)        // dstPort
	packet[62] = 6                                          // TCP

	_, _ = conn.Write(packet)
	_ = conn.Close()

	time.Sleep(150 * time.Millisecond)

	// Verify Parquet log
	logs, err := store.Query(ctx, parquet.LogFilter{
		Type:  "netflow",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query netflow failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 netflow log, got %d", len(logs))
	}
}

func TestTrapServer_LifecycleAndPacket(t *testing.T) {
	store, _, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Zero port disabled case
	disabledTrap := receiver.NewTrapServer(receiver.TrapConfig{Port: 0})
	_ = disabledTrap.Start(ctx)

	// Trap server with free port
	l, _ := net.ListenPacket("udp", "127.0.0.1:0")
	trapPort := l.LocalAddr().(*net.UDPAddr).Port
	_ = l.Close()

	trapSrv := receiver.NewTrapServer(receiver.TrapConfig{
		Port:     trapPort,
		LogStore: store,
	})

	go func() {
		_ = trapSrv.Start(ctx)
	}()
	time.Sleep(50 * time.Millisecond)

	// Send a dummy SNMP trap packet using gosnmp
	g := &gosnmp.GoSNMP{
		Target:    "127.0.0.1",
		Port:      uint16(trapPort),
		Community: "public",
		Version:   gosnmp.Version2c,
		Timeout:   1 * time.Second,
	}
	_ = g.Connect()
	pdu := gosnmp.SnmpPDU{
		Name:  "1.3.6.1.2.1.1.3.0",
		Type:  gosnmp.TimeTicks,
		Value: uint32(1000),
	}
	trap := gosnmp.SnmpTrap{
		Variables: []gosnmp.SnmpPDU{pdu},
	}
	_, _ = g.SendTrap(trap)
	_ = g.Conn.Close()

	time.Sleep(100 * time.Millisecond)
	cancel()
}

func TestReceiverManager_Lifecycle(t *testing.T) {
	store, _, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := receiver.NewManager(receiver.Config{
		LogStore: store,
		// Ports <= 0 means disabled, testing clean startup & shutdown
		SyslogUDP:   0,
		SyslogTCP:   0,
		TrapPort:    0,
		NetFlowPort: 0,
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- mgr.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("manager start returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("manager did not stop in time")
	}
}
