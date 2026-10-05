package logreport

import (
	"context"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

func TestFlowAndServerReport(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()

	// Add a dummy node to test node lookup
	_ = st.SaveNode(ctx, &datastore.NodeEnt{
		ID:   "node-1",
		Name: "WebServer01",
		IP:   "192.168.1.100",
	})

	eng := NewEngine(st, WithFlushInterval(10*time.Millisecond))
	_ = eng.Start(ctx)
	defer eng.Stop()

	now := time.Now().UnixNano()

	// 1. Ingest normal HTTP/HTTPS flows
	eng.ProcessNetFlow(&FlowRecord{
		Time:     now,
		SrcIP:    "192.168.1.50",
		SrcPort:  52345,
		DstIP:    "192.168.1.100",
		DstPort:  80,
		Protocol: "tcp",
		Prot:     6,
		Packets:  20,
		Bytes:    5000,
		Duration: 1.5,
	})

	eng.ProcessNetFlow(&FlowRecord{
		Time:     now,
		SrcIP:    "192.168.1.50",
		SrcPort:  52346,
		DstIP:    "192.168.1.100",
		DstPort:  443,
		Protocol: "tcp",
		Prot:     6,
		Packets:  15,
		Bytes:    8000,
		Duration: 2.0,
	})

	// 2. Ingest a fumble flow (TCP drop / small packets)
	eng.ProcessNetFlow(&FlowRecord{
		Time:     now,
		SrcIP:    "10.0.0.99",
		SrcPort:  60000,
		DstIP:    "192.168.1.100",
		DstPort:  23,
		Protocol: "tcp",
		Prot:     6,
		Packets:  1,
		Bytes:    60,
	})

	// Flush engine
	eng.Flush()

	// Verify FlowEnt
	flows := load[FlowEnt](t, st, KindFlow)
	if len(flows) == 0 {
		t.Fatal("expected at least 1 FlowEnt")
	}
	flowID := "192.168.1.50:192.168.1.100"
	f, ok := flows[flowID]
	if !ok {
		t.Fatalf("expected flow %s in %+v", flowID, flows)
	}
	if f.ServerName != "WebServer01" {
		t.Errorf("expected ServerName WebServer01, got %s", f.ServerName)
	}
	if f.Bytes != 13000 {
		t.Errorf("expected 13000 bytes, got %d", f.Bytes)
	}

	// Verify ServerEnt
	servers := load[ServerEnt](t, st, KindServer)
	if len(servers) == 0 {
		t.Fatal("expected at least 1 ServerEnt")
	}
	s, ok := servers["192.168.1.100"]
	if !ok {
		t.Fatalf("expected server 192.168.1.100 in %+v", servers)
	}
	if s.Services["http"] < 1 || s.Services["https"] < 1 {
		t.Errorf("expected http and https services in server, got %+v", s.Services)
	}

	// Verify FumbleEnt
	fumbles := load[FumbleEnt](t, st, KindFumble)
	if len(fumbles) == 0 {
		t.Fatal("expected at least 1 FumbleEnt")
	}
}

func TestSyslogAndTrapStats(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()

	eng := NewEngine(st, WithFlushInterval(10*time.Millisecond))
	_ = eng.Start(ctx)
	defer eng.Stop()

	now := time.Now().UnixNano()

	// 1. Ingest Syslog
	eng.ProcessSyslog(map[string]interface{}{
		"time":     now,
		"hostname": "switch-01",
		"tag":      "link",
		"message":  "Interface GigabitEthernet0/1 link down",
		"severity": 3, // Error
		"facility": 16,
	})
	eng.ProcessSyslog(map[string]interface{}{
		"time":     now,
		"hostname": "switch-01",
		"tag":      "system",
		"message":  "Configuration changed",
		"severity": 6, // Info/Normal
		"facility": 16,
	})

	// 2. Ingest SNMP Trap
	eng.ProcessTrap(&TrapRecord{
		Time:        now,
		FromAddress: "192.168.1.1",
		TrapType:    "linkDown",
		Enterprise:  "cisco",
		Level:       "error",
		Variables:   "ifIndex=1, ifAdminStatus=up, ifOperStatus=down",
	})

	eng.Flush()

	// Verify SyslogStats
	syslogSummaries := load[SyslogStatsSummary](t, st, KindSyslogStats)
	sum, ok := syslogSummaries["summary"]
	if !ok {
		t.Fatal("expected syslog summary")
	}
	if sum.Total != 2 {
		t.Errorf("expected total 2 syslog, got %d", sum.Total)
	}
	if sum.ErrorCount != 1 {
		t.Errorf("expected 1 error syslog, got %d", sum.ErrorCount)
	}
	if hs, ok := sum.Hosts["switch-01"]; !ok || hs.Count != 2 {
		t.Errorf("expected switch-01 host count 2, got %+v", hs)
	}

	// Verify TrapStats
	trapSummaries := load[TrapStatsSummary](t, st, KindTrapStats)
	tsum, ok := trapSummaries["summary"]
	if !ok {
		t.Fatal("expected trap summary")
	}
	if tsum.Total != 1 {
		t.Errorf("expected total 1 trap, got %d", tsum.Total)
	}
	if tsum.ErrorCount != 1 {
		t.Errorf("expected 1 error trap, got %d", tsum.ErrorCount)
	}
	if th, ok := tsum.Hosts["192.168.1.1"]; !ok || th.Count != 1 {
		t.Errorf("expected 192.168.1.1 host count 1, got %+v", th)
	}
}
