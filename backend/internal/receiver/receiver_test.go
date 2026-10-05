package receiver_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/receiver"
)

func setupTestStores(t *testing.T) (datastore.DataStore, *parquet.Store, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "twsnmpneo-receiver-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	bStore, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create bbolt store: %v", err)
	}

	pqStore, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(dir, "logs"),
		BufferSize:     2,
		BufferInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}

	cleanup := func() {
		_ = bStore.Close()
		_ = pqStore.Close()
		_ = os.RemoveAll(dir)
	}
	return bStore, pqStore, cleanup
}

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
		t.Skipf("dial udp syslog skipped in sandbox: %v", err)
		return
	}
	_, _ = udpConn.Write([]byte("<14>Sep 20 12:00:00 myhost sudo: pam_unix authentication failure\n"))
	_ = udpConn.Close()

	// 2. Send TCP Syslog message
	tcpConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", tcpPort))
	if err != nil {
		t.Skipf("dial tcp syslog skipped in sandbox: %v", err)
		return
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

func TestSyslog_ReportIntegration(t *testing.T) {
	bStore, pqStore, cleanup := setupTestStores(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	udpL, _ := net.ListenPacket("udp", "127.0.0.1:0")
	udpPort := udpL.LocalAddr().(*net.UDPAddr).Port
	_ = udpL.Close()

	mgr := receiver.NewManager(receiver.Config{
		Store:     bStore,
		LogStore:  pqStore,
		SyslogUDP: udpPort,
	})

	go func() {
		_ = mgr.Start(ctx)
	}()
	time.Sleep(50 * time.Millisecond)

	// Send UDP Syslog message for twWifiScan
	udpConn, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", udpPort))
	if err != nil {
		t.Skipf("dial udp syslog skipped: %v", err)
		return
	}
	// Test standard RFC3164 packet: <134>Oct  6 05:00:00 scanner twWifiScan: type=APInfo,ssid=TestWifi,bssid=11:22:33:44:55:66,rssi=-45,Channel=36,info=ax
	msg := []byte("<134>Oct  6 05:00:00 scanner twWifiScan: type=APInfo,ssid=TestWifi,bssid=11:22:33:44:55:66,rssi=-45,Channel=36,info=ax\n")
	_, _ = udpConn.Write(msg)
	_ = udpConn.Close()

	// Wait for engine workerLoop flush
	time.Sleep(1500 * time.Millisecond)

	wifiData, err := bStore.ListLogReportData(ctx, "wifiAP")
	if err != nil {
		t.Fatalf("ListLogReportData failed: %v", err)
	}
	if len(wifiData) == 0 {
		t.Fatalf("expected at least 1 wifiAP entity in datastore, got 0")
	}
}

func TestSyslog_Parser(t *testing.T) {
	// 1. RFC3164 test
	msg := receiver.ParseSyslog("<14>Sep 20 12:00:00 myhost sudo: pam_unix authentication failure", "192.168.1.50")
	if msg.Facility != 1 || msg.Severity != 6 {
		t.Errorf("expected fac 1, sev 6, got fac %d, sev %d", msg.Facility, msg.Severity)
	}
	if msg.Host != "myhost" {
		t.Errorf("unexpected host: %s", msg.Host)
	}
	if msg.Tag != "sudo" {
		t.Errorf("unexpected tag: %s", msg.Tag)
	}
	if msg.Message != "pam_unix authentication failure" {
		t.Errorf("unexpected message: %s", msg.Message)
	}

	// 2. Realistic kernel syslog test matching user issue
	msg2 := receiver.ParseSyslog("<30>Sep 22 00:19:40 yamai-VPCF12AFJ kernel: NVRM: GPU 0000:01:00.0 is already bound to nouveau.", "192.168.1.100")
	if msg2.Host != "yamai-VPCF12AFJ" {
		t.Errorf("expected host yamai-VPCF12AFJ, got %s", msg2.Host)
	}
	if msg2.Tag != "kernel" {
		t.Errorf("expected tag kernel, got %s", msg2.Tag)
	}
	if msg2.Message != "NVRM: GPU 0000:01:00.0 is already bound to nouveau." {
		t.Errorf("expected message NVRM:..., got %s", msg2.Message)
	}

	// 3. RFC5424 test
	msg3 := receiver.ParseSyslog("<34>1 2003-10-11T22:14:15.003Z mymachine.example.com su - ID47 - 'su root' failed for lonvick on /dev/pts/8", "10.0.0.1")
	if msg3.Facility != 4 || msg3.Severity != 2 {
		t.Errorf("expected fac 4, sev 2, got fac %d, sev %d", msg3.Facility, msg3.Severity)
	}
	if msg3.Host != "mymachine.example.com" {
		t.Errorf("expected host mymachine.example.com, got %s", msg3.Host)
	}
	if msg3.Tag != "su" {
		t.Errorf("expected tag su, got %s", msg3.Tag)
	}
	if !strings.Contains(msg3.Message, "'su root' failed for lonvick on /dev/pts/8") {
		t.Errorf("unexpected message: %s", msg3.Message)
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
	conn, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", nfPort))
	if err != nil {
		t.Skipf("skipping udp dial test in restricted environment: %v", err)
		return
	}
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

	// Build NetFlow v9 packet with template (ID 256) and data
	v9Packet := make([]byte, 20+16+12)
	binary.BigEndian.PutUint16(v9Packet[0:2], 9)  // version 9
	binary.BigEndian.PutUint16(v9Packet[2:4], 2)  // count 2 flowsets
	binary.BigEndian.PutUint32(v9Packet[16:20], 1) // SourceID = 1
	// FlowSet 0: Template (offset 20)
	binary.BigEndian.PutUint16(v9Packet[20:22], 0)  // Template FlowSet ID = 0
	binary.BigEndian.PutUint16(v9Packet[22:24], 16) // FlowSet Length = 16
	binary.BigEndian.PutUint16(v9Packet[24:26], 256) // Template ID = 256
	binary.BigEndian.PutUint16(v9Packet[26:28], 2)   // Field count = 2
	binary.BigEndian.PutUint16(v9Packet[28:30], 8)   // Field 1 Type: IPV4_SRC_ADDR
	binary.BigEndian.PutUint16(v9Packet[30:32], 4)   // Field 1 Len: 4
	binary.BigEndian.PutUint16(v9Packet[32:34], 12)  // Field 2 Type: IPV4_DST_ADDR
	binary.BigEndian.PutUint16(v9Packet[34:36], 4)   // Field 2 Len: 4
	// FlowSet 1: Data (offset 36)
	binary.BigEndian.PutUint16(v9Packet[36:38], 256) // Data FlowSet ID = 256
	binary.BigEndian.PutUint16(v9Packet[38:40], 12)  // FlowSet Length = 12
	copy(v9Packet[40:44], net.ParseIP("10.0.0.1").To4()) // SrcIP
	copy(v9Packet[44:48], net.ParseIP("10.0.0.2").To4()) // DstIP

	_, _ = conn.Write(v9Packet)

	_ = conn.Close()

	time.Sleep(150 * time.Millisecond)

	// Verify Parquet logs (v5 record + v9 record = 2 records)
	logs, err := store.Query(ctx, parquet.LogFilter{
		Type:  "netflow",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query netflow failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 netflow logs (v5, v9), got %d", len(logs))
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
	if err := g.Connect(); err == nil && g.Conn != nil {
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
	}

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
		OTelPort:    0,
		MQTTPort:    0,
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

func TestOTel_Ingestion(t *testing.T) {
	store, _, cleanup := setupTestLogStore(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen in test env: %v", err)
		return
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	srv := receiver.NewOTelServer(receiver.OTelConfig{
		Port:     port,
		LogStore: store,
	})

	go func() {
		_ = srv.Start(ctx)
	}()
	time.Sleep(50 * time.Millisecond)
}

func TestMQTT_Ingestion(t *testing.T) {
	bStore, logStore, cleanup := setupTestStores(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen in test env: %v", err)
		return
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	srv := receiver.NewMQTTServer(receiver.MQTTConfig{
		Port:         port,
		Store:        bStore,
		LogStore:     logStore,
		MqttToSyslog: true,
	})

	go func() {
		_ = srv.Start(ctx)
	}()
	time.Sleep(50 * time.Millisecond)

	// Connect to broker
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("failed to connect to mqtt broker: %v", err)
	}
	defer conn.Close()

	// Send CONNECT packet (ClientID: "test-device-1")
	// Variable header (10 bytes): "MQTT"(4) + level(4) + flags(2) + keepalive(60)
	// Payload: clientID len (2) + "test-device-1" (13)
	connectPacket := []byte{
		0x10, 0x19, // remaining length 25
		0x00, 0x04, 'M', 'Q', 'T', 'T',
		0x04,       // level
		0x02,       // clean session
		0x00, 0x3c, // keepalive
		0x00, 0x0d, // clientID len 13
		't', 'e', 's', 't', '-', 'd', 'e', 'v', 'i', 'c', 'e', '-', '1',
	}
	if _, err := conn.Write(connectPacket); err != nil {
		t.Fatalf("failed to write CONNECT packet: %v", err)
	}

	// Read CONNACK
	connack := make([]byte, 4)
	if _, err := io.ReadFull(conn, connack); err != nil {
		t.Fatalf("failed to read CONNACK: %v", err)
	}
	if connack[0] != 0x20 || connack[3] != 0x00 {
		t.Fatalf("unexpected CONNACK response: %v", connack)
	}

	// Send a QoS 1 PUBLISH packet and wait for PUBACK to confirm processing.
	publishPacket := []byte{
		0x32, 0x11, // QoS 1, remaining length 17
		0x00, 0x09, // topic len 9
		'h', 'o', 'm', 'e', '/', 't', 'e', 'm', 'p',
		0x00, 0x01, // packet identifier
		'2', '2', '.', '5',
	}
	if _, err := conn.Write(publishPacket); err != nil {
		t.Fatalf("failed to write PUBLISH packet: %v", err)
	}
	puback := make([]byte, 4)
	if _, err := io.ReadFull(conn, puback); err != nil {
		t.Fatalf("failed to read PUBACK: %v", err)
	}
	if puback[0] != 0x40 || puback[1] != 0x02 || puback[2] != 0x00 || puback[3] != 0x01 {
		t.Fatalf("unexpected PUBACK response: %v", puback)
	}
	if err := logStore.Flush(); err != nil {
		t.Fatalf("failed to flush MQTT logs: %v", err)
	}

	// Verify stat was saved in bStore
	stats, err := bStore.ListMqttStats(ctx)
	if err != nil || len(stats) == 0 {
		t.Fatalf("expected mqtt stat saved, count=%d, err=%v", len(stats), err)
	}
	if stats[0].ClientID != "test-device-1" || stats[0].Topic != "home/temp" || stats[0].Value != "22.5" {
		t.Errorf("unexpected mqtt stat: %+v", stats[0])
	}

	// Verify log was saved in logStore (Parquet)
	logs, err := logStore.Query(ctx, parquet.LogFilter{Type: "mqtt", Limit: 10})
	if err != nil || len(logs) == 0 {
		t.Fatalf("expected parquet mqtt log, got count=%d, err=%v", len(logs), err)
	}
	if !strings.Contains(logs[0].Log, "test-device-1") || !strings.Contains(logs[0].Log, "home/temp") {
		t.Errorf("unexpected log content: %s", logs[0].Log)
	}
}
