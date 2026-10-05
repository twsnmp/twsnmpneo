package logreport

import (
	"context"
	"testing"
	"time"
)

func TestEngine_ProcessSyslog(t *testing.T) {
	store := newStore(t)

	engine := NewEngine(store,
		WithFlushInterval(50*time.Millisecond),
		WithCleanInterval(10*time.Minute),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("engine.Start failed: %v", err)
	}

	// Ingest sample Wi-Fi, BlueScan, Pcap, WinLog syslogs
	now := time.Now()
	// 1. Standard tag
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "twWifiScan",
		"hostname":  "ap-scanner",
		"content":   "type=APInfo,ssid=TestSSID1,bssid=00:11:22:33:44:55,rssi=-50,Channel=1,info=wpa2",
		"timestamp": now,
	})

	// 2. Tag with trailing colon and PID: "twWifiScan[123]:"
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "twWifiScan[123]:",
		"hostname":  "ap-scanner",
		"content":   "type=APInfo,ssid=TestSSID2,bssid=00:11:22:33:44:56,rssi=-60,Channel=6,info=wpa3",
		"timestamp": now,
	})

	// 3. Tag empty, but content has prefix "twWifiScan: type=APInfo..."
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "",
		"client":    "192.168.1.50:514",
		"content":   "twWifiScan: type=APInfo,ssid=TestSSID3,bssid=00:11:22:33:44:57,rssi=-70,Channel=11,info=wpa2",
		"timestamp": now,
	})

	// 4. BlueScan
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "twBlueScan",
		"hostname":  "blue-scanner",
		"content":   "type=Device,address=aa:bb:cc:dd:ee:ff,name=BLEBeacon,rssi=-65",
		"timestamp": now,
	})

	// 5. Pcap
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "twpcap",
		"hostname":  "pcap-probe",
		"content":   "type=EtherType,0x0800=100",
		"timestamp": now,
	})

	// 6. WinLog
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "twwinlog",
		"hostname":  "DC01",
		"content":   "type=EventID,computer=DC01,eventID=4624,count=5",
		"timestamp": now,
	})

	// Flush and wait
	engine.Flush()

	// Verify datastore contains report entities
	wifiData, err := store.ListLogReportData(ctx, KindWifiAP)
	if err != nil {
		t.Fatalf("ListLogReportData failed: %v", err)
	}
	if len(wifiData) != 3 {
		t.Errorf("expected 3 wifiAP entities, got %d", len(wifiData))
	}

	blueData, err := store.ListLogReportData(ctx, KindBlueDevice)
	if err != nil {
		t.Fatalf("ListLogReportData failed: %v", err)
	}
	if len(blueData) != 1 {
		t.Errorf("expected 1 blueDevice entity, got %d", len(blueData))
	}

	pcapData, err := store.ListLogReportData(ctx, KindEtherType)
	if err != nil {
		t.Fatalf("ListLogReportData failed: %v", err)
	}
	if len(pcapData) != 1 {
		t.Errorf("expected 1 etherType entity, got %d", len(pcapData))
	}

	winData, err := store.ListLogReportData(ctx, KindWinEventID)
	if err != nil {
		t.Fatalf("ListLogReportData failed: %v", err)
	}
	if len(winData) != 1 {
		t.Errorf("expected 1 winEventID entity, got %d", len(winData))
	}

	engine.Stop()
}
