package polling_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/logreport"
	"github.com/twsnmp/twsnmpneo/backend/internal/polling"
)

func writeSyslog(t *testing.T, ls *parquet.Store, host, tag, content string, ts int64) {
	t.Helper()
	b, _ := json.Marshal(map[string]interface{}{
		"hostname": host, "tag": tag, "content": content, "severity": 6,
	})
	if err := ls.WriteLog(&parquet.ParquetLogRecord{Time: ts, Type: "syslog", Src: host, Log: string(b)}); err != nil {
		t.Fatal(err)
	}
}

func TestLogReportPoller(t *testing.T) {
	store, ls, cleanup := setupTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Now()

	// BSSID-keyed AP: two reports of the same AP, one unrelated syslog, one other tag.
	writeSyslog(t, ls, "scanner", "twWifiScan", "type=APInfo,ssid=a,bssid=aa:bb,rssi=-50,Channel=1,info=x", now.Add(-3*time.Minute).UnixNano())
	writeSyslog(t, ls, "scanner", "twWifiScan", "type=APInfo,ssid=a,bssid=aa:bb,rssi=-55,Channel=1,info=x", now.Add(-2*time.Minute).UnixNano())
	writeSyslog(t, ls, "scanner", "sshd", "Accepted password", now.Add(-2*time.Minute).UnixNano())
	writeSyslog(t, ls, "scanner", "twpcap", "type=EtherType,0x0800=1", now.Add(-2*time.Minute).UnixNano())
	writeSyslog(t, ls, "other", "twWifiScan", "type=APInfo,ssid=b,bssid=cc:dd,rssi=-70,Channel=6,info=y", now.Add(-time.Minute).UnixNano())

	p := polling.NewLogReportPoller(store, ls, logreport.SourceWifiScan)
	pe := &datastore.PollingEnt{Type: logreport.SourceWifiScan, PollInt: 60}
	res, err := p.Poll(ctx, pe, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.State != polling.StateNormal {
		t.Fatalf("state=%s msg=%s", res.State, res.Message)
	}
	if res.Fields["count"].(float64) != 3 || res.Fields["scanned"].(float64) != 3 {
		t.Fatalf("fields=%v", res.Fields)
	}
	items, _ := store.ListLogReportData(ctx, logreport.KindWifiAP)
	if len(items) != 2 {
		t.Fatalf("aps=%d", len(items))
	}
	// Other tags must not create other reports.
	if e, _ := store.ListLogReportData(ctx, logreport.KindEtherType); len(e) != 0 {
		t.Fatal("twpcap record must not be processed by twwifiscan polling")
	}

	// Second run continues from lastTime: nothing new, nothing double counted.
	pe.Result = res.Fields
	res, _ = p.Poll(ctx, pe, nil)
	if res.Fields["count"].(float64) != 0 {
		t.Fatalf("second run processed %v", res.Fields["count"])
	}
	var ap logreport.WifiAPEnt
	b, _ := store.GetLogReportData(ctx, logreport.KindWifiAP, "scanner:aa:bb")
	_ = json.Unmarshal(b, &ap)
	if ap.Count != 2 || len(ap.RSSI) != 2 || ap.RSSI[0].Value != -50 {
		t.Fatalf("ap=%+v", ap)
	}

	// Sender filter (Params).
	if err := store.ResetLogReportData(ctx, ""); err != nil {
		t.Fatal(err)
	}
	pe2 := &datastore.PollingEnt{Type: logreport.SourceWifiScan, PollInt: 60, Params: "other"}
	res, _ = p.Poll(ctx, pe2, nil)
	if res.Fields["count"].(float64) != 1 {
		t.Fatalf("filtered count=%v", res.Fields["count"])
	}
}

func TestLogReportPollerPaging(t *testing.T) {
	store, ls, cleanup := setupTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	base := time.Now().Add(-time.Hour).UnixNano()
	const n = 5200 // more than one page of 5000
	for i := 0; i < n; i++ {
		writeSyslog(t, ls, "pc", "twpcap", fmt.Sprintf("type=EtherType,0x0800=1"), base+int64(i)*1000)
	}
	p := polling.NewLogReportPoller(store, ls, logreport.SourcePcap)
	res, err := p.Poll(ctx, &datastore.PollingEnt{Type: logreport.SourcePcap, PollInt: 60}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Fields["scanned"].(float64) != n || res.Fields["truncated"].(bool) {
		t.Fatalf("fields=%v", res.Fields)
	}
	b, _ := store.GetLogReportData(ctx, logreport.KindEtherType, "pc:0x0800")
	var e logreport.EtherTypeEnt
	_ = json.Unmarshal(b, &e)
	if e.Count != n {
		t.Fatalf("count=%d", e.Count)
	}
}

func TestLogReportPollerNoStore(t *testing.T) {
	p := polling.NewLogReportPoller(nil, nil, logreport.SourceWinLog)
	res, err := p.Poll(context.Background(), &datastore.PollingEnt{}, nil)
	if err != nil || res.State != polling.StateUnknown {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestSyslogPollerWithReportModes(t *testing.T) {
	store, ls, cleanup := setupTestEnv(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Now()

	writeSyslog(t, ls, "scanner", "twWifiScan", "type=APInfo,ssid=test-wifi,bssid=11:22:33:44:55:66,rssi=-45,Channel=36,info=ax", now.Add(-time.Minute).UnixNano())
	writeSyslog(t, ls, "scanner", "twBlueScan", "type=Device,address=aa:bb:cc:dd:ee:ff,name=Sensor1,rssi=-60", now.Add(-time.Minute).UnixNano())
	writeSyslog(t, ls, "scanner", "twpcap", "type=DNS,DNSType=A,Name=google.com,sv=8.8.8.8,count=1,change=0,lcl=10.0.0.1,ft=2026-10-04T00:00:00Z,lt=2026-10-04T00:00:00Z", now.Add(-time.Minute).UnixNano())
	writeSyslog(t, ls, "scanner", "twwinlog", "type=EventID,computer=Server01,eventID=4624,count=10", now.Add(-time.Minute).UnixNano())

	sp := polling.NewSyslogPoller(store, ls)

	// 1. Wifi Scan mode
	resWifi, err := sp.Poll(ctx, &datastore.PollingEnt{Type: "syslog", Mode: "twwifiscan", PollInt: 60}, nil)
	if err != nil || resWifi.State != polling.StateNormal {
		t.Fatalf("resWifi=%+v err=%v", resWifi, err)
	}
	if aps, _ := store.ListLogReportData(ctx, logreport.KindWifiAP); len(aps) != 1 {
		t.Fatalf("expected 1 wifi AP, got %d", len(aps))
	}

	// 2. Bluetooth Scan mode (case-insensitive test)
	resBlue, err := sp.Poll(ctx, &datastore.PollingEnt{Type: "syslog", Mode: "twBlueScan", PollInt: 60}, nil)
	if err != nil || resBlue.State != polling.StateNormal {
		t.Fatalf("resBlue=%+v err=%v", resBlue, err)
	}
	if devs, _ := store.ListLogReportData(ctx, logreport.KindBlueDevice); len(devs) != 1 {
		t.Fatalf("expected 1 BLE device, got %d", len(devs))
	}

	// 3. Pcap mode
	resPcap, err := sp.Poll(ctx, &datastore.PollingEnt{Type: "syslog", Mode: "twpcap", PollInt: 60}, nil)
	if err != nil || resPcap.State != polling.StateNormal {
		t.Fatalf("resPcap=%+v err=%v", resPcap, err)
	}
	if dnsq, _ := store.ListLogReportData(ctx, logreport.KindDNSQ); len(dnsq) != 1 {
		t.Fatalf("expected 1 DNS query record, got %d", len(dnsq))
	}

	// 4. WinLog mode
	resWin, err := sp.Poll(ctx, &datastore.PollingEnt{Type: "syslog", Mode: "twwinlog", PollInt: 60}, nil)
	if err != nil || resWin.State != polling.StateNormal {
		t.Fatalf("resWin=%+v err=%v", resWin, err)
	}
	if evs, _ := store.ListLogReportData(ctx, logreport.KindWinEventID); len(evs) != 1 {
		t.Fatalf("expected 1 Win EventID, got %d", len(evs))
	}
}
