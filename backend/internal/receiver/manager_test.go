package receiver

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

func TestManager_CleanOldData(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-manager-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	bStore, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create bbolt store: %v", err)
	}
	defer bStore.Close()

	pqStore, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(dir, "logs"),
		BufferSize:     1,
		BufferInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}
	defer pqStore.Close()

	ctx := context.Background()

	// Configure different retentions: 20 days for EventLog, 5 days for Parquet
	conf := &datastore.MapConfEnt{
		EventLogDays:   20,
		ParquetLogDays: 5,
		LogDays:        14,
	}
	if err := bStore.SaveMapConf(ctx, conf); err != nil {
		t.Fatalf("save map conf: %v", err)
	}

	// 1. Add event logs to bbolt (one 30 days ago, one 2 days ago)
	oldEvent := &datastore.EventLogEnt{
		Time:     time.Now().AddDate(0, 0, -30).UnixNano(),
		Type:     "system",
		Level:    "warn",
		NodeName: "Node-Old",
		Event:    "Old event",
	}
	recentEvent := &datastore.EventLogEnt{
		Time:     time.Now().AddDate(0, 0, -2).UnixNano(),
		Type:     "system",
		Level:    "info",
		NodeName: "Node-Recent",
		Event:    "Recent event",
	}
	_ = bStore.AddEventLog(ctx, oldEvent)
	_ = bStore.AddEventLog(ctx, recentEvent)

	// 2. Add log to parquet and simulate old file
	_ = pqStore.WriteLog(&parquet.ParquetLogRecord{
		Type: "syslog",
		Src:  "192.168.1.1",
		Log:  "test syslog message",
		Time: time.Now().UnixNano(),
	})
	_ = pqStore.Flush()

	// Create an artificial old parquet file in logs/syslog
	syslogDir := filepath.Join(dir, "logs", "syslog")
	oldFile := filepath.Join(syslogDir, "syslog_old.parquet")
	if err := os.WriteFile(oldFile, []byte("fake-parquet-content"), 0644); err != nil {
		t.Fatalf("write old parquet file: %v", err)
	}
	// Set mtime to 10 days ago (older than 5 days ParquetLogDays)
	oldTime := time.Now().AddDate(0, 0, -10)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	// Create manager and execute cleanOldData
	mgr := NewManager(Config{
		Store:    bStore,
		LogStore: pqStore,
	})

	mgr.cleanOldData(ctx)

	// Verify bbolt event logs: 30-day old event should be gone, 2-day old retained
	events, err := bStore.ListEventLogs(ctx, 10)
	if err != nil {
		t.Fatalf("list event logs: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 remaining event log, got %d", len(events))
	}
	if events[0].Event != "Recent event" {
		t.Errorf("expected recent event, got: %s", events[0].Event)
	}

	// Verify old parquet file was deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old parquet file to be deleted, but it still exists")
	}
}

func TestManager_UpdateConf(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-updateconf-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	bStore, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create bbolt store: %v", err)
	}
	defer bStore.Close()

	pqStore, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(dir, "logs"),
		BufferSize:     1,
		BufferInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}
	defer pqStore.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initial configuration: Syslog and Trap enabled; NetFlow, sFlow, ARP, OTel, MQTT disabled
	mgr := NewManager(Config{
		Store:              bStore,
		LogStore:           pqStore,
		SyslogUDP:          60100,
		SyslogTCP:          0,
		TrapPort:           60101,
		NetFlowPort:        60102,
		SFlowPort:          60103,
		OTelPort:           60104,
		MQTTPort:           60105,
		EnableSyslogd:      true,
		EnableTrapd:        true,
		EnableNetflowd:     false,
		EnableSFlowd:       false,
		EnableArpWatch:     false,
		EnableOTel:         false,
		EnableMqtt:         false,
		HasExplicitEnables: true,
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- mgr.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Verify initial status
	st1 := mgr.GetReceiverStatus(false)
	syslogSt, _ := st1["syslog"].(map[string]any)
	if syslogSt["status"] != "running" {
		t.Fatalf("expected syslog running initially, got: %v", syslogSt)
	}
	trapSt, _ := st1["trap"].(map[string]any)
	if trapSt["status"] != "running" {
		t.Fatalf("expected trap running initially, got: %v", trapSt)
	}
	netflowSt, _ := st1["netflow"].(map[string]any)
	if netflowSt["status"] != "stopped" {
		t.Fatalf("expected netflow stopped initially, got: %v", netflowSt)
	}
	sflowSt, _ := st1["sflow"].(map[string]any)
	if sflowSt["status"] != "stopped" {
		t.Fatalf("expected sflow stopped initially, got: %v", sflowSt)
	}
	arpSt, _ := st1["arp"].(map[string]any)
	if arpSt["status"] != "disabled" {
		t.Fatalf("expected arp disabled initially, got: %v", arpSt)
	}

	// Dynamically update config: turn off Syslog, Trap; turn on NetFlow, sFlow
	mgr.UpdateConf(&datastore.MapConfEnt{
		EnableSyslogd:  false,
		EnableTrapd:    false,
		EnableNetflowd: true,
		EnableSFlowd:   true,
		EnableArpWatch: false,
		EnableOTel:     false,
		EnableMqtt:     false,
	})

	time.Sleep(100 * time.Millisecond)

	// Verify updated status
	st2 := mgr.GetReceiverStatus(false)
	syslogSt2, _ := st2["syslog"].(map[string]any)
	if syslogSt2["status"] != "stopped" {
		t.Errorf("expected syslog stopped after update, got: %v", syslogSt2)
	}
	trapSt2, _ := st2["trap"].(map[string]any)
	if trapSt2["status"] != "stopped" {
		t.Errorf("expected trap stopped after update, got: %v", trapSt2)
	}
	netflowSt2, _ := st2["netflow"].(map[string]any)
	if netflowSt2["status"] != "running" {
		t.Errorf("expected netflow running after update, got: %v", netflowSt2)
	}
	sflowSt2, _ := st2["sflow"].(map[string]any)
	if sflowSt2["status"] != "running" {
		t.Errorf("expected sflow running after update, got: %v", sflowSt2)
	}
	arpSt2, _ := st2["arp"].(map[string]any)
	if arpSt2["status"] != "disabled" {
		t.Errorf("expected arp disabled after update, got: %v", arpSt2)
	}

	// Stop manager
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("manager start error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("manager did not shut down in time")
	}

	// Verify event logs were recorded for the receiver stop/start events
	events, err := bStore.ListEventLogs(context.Background(), 100)
	if err != nil {
		t.Fatalf("list event logs: %v", err)
	}
	hasStopSyslog := false
	hasStartNetflow := false
	for _, e := range events {
		if e.Event == "Stopped Syslog receiver" || e.Event == "Syslog受信を停止しました" {
			hasStopSyslog = true
		}
		if e.Event == "Started NetFlow receiver" || e.Event == "NetFlow受信を開始しました" {
			hasStartNetflow = true
		}
	}
	if !hasStopSyslog {
		t.Errorf("expected Stop Syslog event log, got: %v", events)
	}
	if !hasStartNetflow {
		t.Errorf("expected Start NetFlow event log, got: %v", events)
	}
}

func TestManager_PortConflictDetection(t *testing.T) {
	ResetAllReceiverStatuses()

	tmpDir, err := os.MkdirTemp("", "twsnmpneo-portconflict-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	bStore, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("failed to open bbolt: %v", err)
	}
	defer bStore.Close()

	logDir := filepath.Join(tmpDir, "logs")
	logStore, err := parquet.New(parquet.Config{
		Dir:        logDir,
		BufferSize: 1,
	})
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}
	defer logStore.Close()

	// Pre-occupy UDP port for NetFlow and TCP port for MQTT
	conflictUDPPort := 61102
	udpConn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", conflictUDPPort))
	if err != nil {
		t.Skipf("cannot bind test udp port: %v", err)
	}
	defer udpConn.Close()

	conflictTCPPort := 61105
	tcpLn, err := net.Listen("tcp", fmt.Sprintf(":%d", conflictTCPPort))
	if err != nil {
		t.Skipf("cannot bind test tcp port: %v", err)
	}
	defer tcpLn.Close()

	normalSyslogPort := 61100
	cfg := Config{
		Store:          bStore,
		LogStore:       logStore,
		SyslogUDP:      normalSyslogPort,
		NetFlowPort:    conflictUDPPort,
		MQTTPort:       conflictTCPPort,
		EnableSyslogd:  true,
		EnableNetflowd: true,
		EnableMqtt:     true,
	}

	mgr := NewManager(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- mgr.Start(ctx)
	}()

	time.Sleep(150 * time.Millisecond)

	statuses := mgr.GetReceiverStatus(false)

	// Syslog should be running normally
	syslogSt, ok := statuses["syslog"].(map[string]any)
	if !ok || syslogSt["status"] != "running" {
		t.Errorf("expected syslog status running, got: %v", syslogSt)
	}

	// NetFlow should detect port error
	netflowSt, ok := statuses["netflow"].(map[string]any)
	if !ok || netflowSt["status"] != "error" {
		t.Errorf("expected netflow status error, got: %v", netflowSt)
	}
	if netflowSt["error"] == nil || netflowSt["error"] == "" {
		t.Errorf("expected netflow error message, got empty: %v", netflowSt)
	}

	// MQTT should detect port error
	mqttSt, ok := statuses["mqtt"].(map[string]any)
	if !ok || mqttSt["status"] != "error" {
		t.Errorf("expected mqtt status error, got: %v", mqttSt)
	}
	if mqttSt["error"] == nil || mqttSt["error"] == "" {
		t.Errorf("expected mqtt error message, got empty: %v", mqttSt)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("manager start returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("manager did not stop in time")
	}

	// Verify warning event logs were recorded for port errors
	events, err := bStore.ListEventLogs(context.Background(), 100)
	if err != nil {
		t.Fatalf("list event logs: %v", err)
	}
	hasNetFlowErrorLog := false
	hasMQTTErrorLog := false
	for _, e := range events {
		if strings.Contains(e.Event, "NetFlow") && e.Level == "warn" {
			hasNetFlowErrorLog = true
		}
		if strings.Contains(e.Event, "MQTT") && e.Level == "warn" {
			hasMQTTErrorLog = true
		}
	}
	if !hasNetFlowErrorLog {
		t.Errorf("expected warn event log for NetFlow bind error, got events: %v", events)
	}
	if !hasMQTTErrorLog {
		t.Errorf("expected warn event log for MQTT bind error, got events: %v", events)
	}
}

func TestManager_PortConflictDetection_MoreReceivers(t *testing.T) {
	ResetAllReceiverStatuses()

	tmpDir, err := os.MkdirTemp("", "twsnmpneo-portconflict-more-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	bStore, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("failed to open bbolt: %v", err)
	}
	defer bStore.Close()

	logDir := filepath.Join(tmpDir, "logs")
	logStore, err := parquet.New(parquet.Config{
		Dir:        logDir,
		BufferSize: 1,
	})
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}
	defer logStore.Close()

	// Pre-occupy UDP port for Trap & sFlow, and TCP port for OTel
	conflictTrapPort := 61201
	trapConn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", conflictTrapPort))
	if err != nil {
		t.Skipf("cannot bind test udp port for trap: %v", err)
	}
	defer trapConn.Close()

	conflictSFlowPort := 61203
	sflowConn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", conflictSFlowPort))
	if err != nil {
		t.Skipf("cannot bind test udp port for sflow: %v", err)
	}
	defer sflowConn.Close()

	conflictOTelPort := 61204
	otelLn, err := net.Listen("tcp", fmt.Sprintf(":%d", conflictOTelPort))
	if err != nil {
		t.Skipf("cannot bind test tcp port for otel: %v", err)
	}
	defer otelLn.Close()

	cfg := Config{
		Store:        bStore,
		LogStore:     logStore,
		TrapPort:     conflictTrapPort,
		SFlowPort:    conflictSFlowPort,
		OTelPort:     conflictOTelPort,
		EnableTrapd:  true,
		EnableSFlowd: true,
		EnableOTel:   true,
	}

	mgr := NewManager(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- mgr.Start(ctx)
	}()

	time.Sleep(150 * time.Millisecond)

	statuses := mgr.GetReceiverStatus(false)

	// Trap should detect port error
	trapSt, ok := statuses["trap"].(map[string]any)
	if !ok || trapSt["status"] != "error" {
		t.Errorf("expected trap status error, got: %v", trapSt)
	}
	if trapSt["error"] == nil || trapSt["error"] == "" {
		t.Errorf("expected trap error message, got empty: %v", trapSt)
	}

	// sFlow should detect port error
	sflowSt, ok := statuses["sflow"].(map[string]any)
	if !ok || sflowSt["status"] != "error" {
		t.Errorf("expected sflow status error, got: %v", sflowSt)
	}
	if sflowSt["error"] == nil || sflowSt["error"] == "" {
		t.Errorf("expected sflow error message, got empty: %v", sflowSt)
	}

	// OTel should detect port error
	otelSt, ok := statuses["otel"].(map[string]any)
	if !ok || otelSt["status"] != "error" {
		t.Errorf("expected otel status error, got: %v", otelSt)
	}
	if otelSt["error"] == nil || otelSt["error"] == "" {
		t.Errorf("expected otel error message, got empty: %v", otelSt)
	}

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
