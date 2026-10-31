package logreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

func TestLookupNode_DNSReverseLookup(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()

	// 1. Registered node in map
	if err := st.SaveNode(ctx, &datastore.NodeEnt{ID: "node1", Name: "my-gateway", IP: "192.168.1.254"}); err != nil {
		t.Fatal(err)
	}

	s := NewSession(ctx, st)

	// Registered node lookup
	ni := s.lookupNode("192.168.1.254")
	if ni.Name != "my-gateway" || ni.ID != "node1" {
		t.Fatalf("expected registered node info, got %+v", ni)
	}

	// 2. Unregistered node with local DNS reverse lookup (127.0.0.1 -> localhost)
	niLocal := s.lookupNode("127.0.0.1")
	if !strings.Contains(strings.ToLower(niLocal.Name), "localhost") {
		t.Logf("127.0.0.1 resolved to %q", niLocal.Name)
	}
	if niLocal.ID != "" {
		t.Errorf("unregistered node should have empty ID, got %q", niLocal.ID)
	}

	// Verify it was cached
	start := time.Now()
	niCached := s.lookupNode("127.0.0.1")
	if time.Since(start) > 50*time.Millisecond {
		t.Errorf("cached lookup took too long: %v", time.Since(start))
	}
	if niCached.Name != niLocal.Name {
		t.Errorf("expected cached name %q, got %q", niLocal.Name, niCached.Name)
	}

	// 3. Unresolvable IP (e.g. documentation IP 192.0.2.100) -> returns IP itself
	niUnresolved := s.lookupNode("192.0.2.100")
	if niUnresolved.Name != "192.0.2.100" {
		t.Errorf("expected unresolved IP name to be %q, got %q", "192.0.2.100", niUnresolved.Name)
	}
	// Verify negative cache
	start = time.Now()
	niUnresolvedCached := s.lookupNode("192.0.2.100")
	if time.Since(start) > 50*time.Millisecond {
		t.Errorf("cached unresolved lookup took too long: %v", time.Since(start))
	}
	if niUnresolvedCached.Name != "192.0.2.100" {
		t.Errorf("expected cached unresolved IP, got %q", niUnresolvedCached.Name)
	}
}

func TestPcapPenaltyWithDNS(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()

	// Clear cache before test
	dnsCache.Range(func(key, value any) bool {
		dnsCache.Delete(key)
		return true
	})

	s := NewSession(ctx, st)
	lt := "2026-10-31T09:18:01+09:00"

	// Case A: 127.0.0.1 resolves to "localhost" (so ServerName != Server, avoiding penalty)
	// Mock/Pre-populate DNS cache for deterministic test of penalty calculation
	dnsCache.Store("10.10.10.10", dnsCacheEntry{
		name:      "resolved-server.internal",
		expiresAt: time.Now().Add(time.Hour),
	})
	dnsCache.Store("10.10.10.20", dnsCacheEntry{
		name:      "resolved-client.internal",
		expiresAt: time.Now().Add(time.Hour),
	})

	// TLS 1.3 (+0), resolved names (+0) -> penalty 0
	s.Process(SourcePcap, rec("twpcap", "type=TLSFlow,cl=10.10.10.20,sv=10.10.10.10,serv=HTTPS,count=1,maxver=TLS 1.3,cipher=AES,ft="+lt+",lt="+lt, 1))

	// Case B: Unresolved IP (ServerName == Server) -> penalty +1
	s.Process(SourcePcap, rec("twpcap", "type=TLSFlow,cl=10.10.10.20,sv=198.51.100.1,serv=HTTPS,count=1,maxver=TLS 1.3,cipher=AES,ft="+lt+",lt="+lt, 2))

	if err := s.Commit(); err != nil {
		t.Fatal(err)
	}

	flows := load[TLSFlowEnt](t, st, KindTLSFlow)

	// Flow A check: resolved server/client
	idA := "10.10.10.20:10.10.10.10:HTTPS"
	flowA := flows[idA]
	if flowA == nil {
		t.Fatalf("expected flowA %q", idA)
	}
	if flowA.ServerName != "resolved-server.internal" {
		t.Errorf("expected ServerName %q, got %q", "resolved-server.internal", flowA.ServerName)
	}
	if flowA.ClientName != "resolved-client.internal" {
		t.Errorf("expected ClientName %q, got %q", "resolved-client.internal", flowA.ClientName)
	}
	if flowA.Penalty != 0 {
		t.Errorf("expected Penalty 0 for resolved TLS 1.3 flow, got %d", flowA.Penalty)
	}

	// Flow B check: unresolved server -> ServerName == Server (198.51.100.1) -> penalty 1
	idB := "10.10.10.20:198.51.100.1:HTTPS"
	flowB := flows[idB]
	if flowB == nil {
		t.Fatalf("expected flowB %q", idB)
	}
	if flowB.ServerName != "198.51.100.1" {
		t.Errorf("expected ServerName %q, got %q", "198.51.100.1", flowB.ServerName)
	}
	if flowB.Penalty != 1 {
		t.Errorf("expected Penalty 1 for unresolved server in TLS 1.3 flow, got %d", flowB.Penalty)
	}
}

func TestTwpcapSyslogFormatCorrection(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()

	engine := NewEngine(st,
		WithFlushInterval(50*time.Millisecond),
		WithCleanInterval(10*time.Minute),
	)
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("engine.Start failed: %v", err)
	}
	defer engine.Stop()

	// 1. Tag empty, content starts with timestamp and host then twpcap:
	// Simulating go-syslog parser behavior for RFC3339 timestamp in RFC3164 packet:
	// content: "2026-10-31T09:18:01+09:00 probe-host twpcap: type=TLSFlow,cl=192.168.1.10,sv=1.1.1.1,serv=HTTPS,count=5,maxver=TLS 1.3,cipher=AES"
	engine.ProcessSyslog(map[string]interface{}{
		"tag":      "",
		"hostname": "",
		"client":   "192.168.1.50:514",
		"content":  "2026-10-31T09:18:01+09:00 probe-host twpcap: type=TLSFlow,cl=192.168.1.10,sv=1.1.1.1,serv=HTTPS,count=5,maxver=TLS 1.3,cipher=AES",
	})

	// 2. Tag has "host twpcap:", hostname has "probe-host"
	engine.ProcessSyslog(map[string]interface{}{
		"tag":      "host twpcap:",
		"hostname": "probe-host",
		"content":  "type=DNS,DNSType=A,Name=example.org,sv=8.8.4.4,count=12,change=1,lcl=192.168.1.10,lMAC=00:11:22:33:44:55",
	})

	// 3. Hostname has "probe-host twpcap:", tag is empty
	engine.ProcessSyslog(map[string]interface{}{
		"tag":      "",
		"hostname": "probe-host twpcap:",
		"content":  "type=RADIUS,cl=192.168.1.10,sv=192.168.1.20,count=4,req=4,accept=4,reject=0,challenge=0",
	})

	// 4. Content has "twpcap: type=EtherType,..." directly
	engine.ProcessSyslog(map[string]interface{}{
		"tag":      "",
		"hostname": "probe-host",
		"content":  "twpcap: type=EtherType,0x0800=500",
	})

	engine.Flush()

	// Verify all 4 pcap entities were correctly ingested
	tlsList, err := st.ListLogReportData(ctx, KindTLSFlow)
	if err != nil || len(tlsList) != 1 {
		t.Fatalf("expected 1 TLSFlow, got %d (err: %v)", len(tlsList), err)
	}

	dnsList, err := st.ListLogReportData(ctx, KindDNSQ)
	if err != nil || len(dnsList) != 1 {
		t.Fatalf("expected 1 DNSQ, got %d (err: %v)", len(dnsList), err)
	}

	radiusList, err := st.ListLogReportData(ctx, KindRADIUSFlow)
	if err != nil || len(radiusList) != 1 {
		t.Fatalf("expected 1 RADIUSFlow, got %d (err: %v)", len(radiusList), err)
	}

	etherList, err := st.ListLogReportData(ctx, KindEtherType)
	if err != nil || len(etherList) != 1 {
		t.Fatalf("expected 1 EtherType, got %d (err: %v)", len(etherList), err)
	}
}

func TestParseKV_TypeCut(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantSv   string
	}{
		{
			input:    "type=TLSFlow,sv=1.1.1.1,cl=10.0.0.1",
			wantType: "TLSFlow",
			wantSv:   "1.1.1.1",
		},
		{
			input:    "twpcap: type=TLSFlow,sv=1.1.1.1,cl=10.0.0.1",
			wantType: "TLSFlow",
			wantSv:   "1.1.1.1",
		},
		{
			input:    "2026-10-31T09:18:01+09:00 myhost twpcap: type=TLSFlow,sv=1.1.1.1,cl=10.0.0.1",
			wantType: "TLSFlow",
			wantSv:   "1.1.1.1",
		},
		{
			input:    "random garbage header: type=DNS,sv=8.8.8.8,Name=google.com",
			wantType: "DNS",
			wantSv:   "8.8.8.8",
		},
	}

	for _, tc := range tests {
		m := parseKV(tc.input)
		if m["type"] != tc.wantType {
			t.Errorf("input %q: expected type %q, got %q", tc.input, tc.wantType, m["type"])
		}
		if m["sv"] != tc.wantSv {
			t.Errorf("input %q: expected sv %q, got %q", tc.input, tc.wantSv, m["sv"])
		}
	}
}
