package polling

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/ping"
)

func TestParsePingParams(t *testing.T) {
	tests := []struct {
		name              string
		params            string
		smoke             bool
		wantSize, wantTTL int
		wantCount         int
	}{
		{name: "defaults", wantSize: 64, wantCount: 10},
		{name: "legacy payload size", params: "128", wantSize: 128, wantCount: 10},
		{name: "size and ttl", params: "size=1400,ttl=32", wantSize: 1400, wantTTL: 32, wantCount: 10},
		{name: "smoke count", params: "count=5,size=256,ttl=16", smoke: true, wantSize: 256, wantTTL: 16, wantCount: 5},
		{name: "invalid bounds ignored", params: "size=3000,ttl=256,count=101", smoke: true, wantSize: 64, wantCount: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, ttl, count := parsePingParams(tt.params, tt.smoke)
			if size != tt.wantSize || ttl != tt.wantTTL || count != tt.wantCount {
				t.Fatalf("parsePingParams() = (%d, %d, %d), want (%d, %d, %d)", size, ttl, count, tt.wantSize, tt.wantTTL, tt.wantCount)
			}
		})
	}
}

func TestPingPollerSingleModeUsesFKParamsAndFailureLevel(t *testing.T) {
	var gotTarget string
	var gotTimeout, gotRetry, gotSize, gotTTL int
	poller := &PingPoller{
		doPing: func(target string, timeout, retry, size, ttl int) *ping.PingEnt {
			gotTarget, gotTimeout, gotRetry, gotSize, gotTTL = target, timeout, retry, size, ttl
			return &ping.PingEnt{Stat: ping.PingTimeout, Error: errors.New("timeout")}
		},
	}
	pe := &datastore.PollingEnt{Params: "size=128,ttl=24", Timeout: 3, Retry: 2, Level: "warn"}
	res, err := poller.Poll(context.Background(), pe, &datastore.NodeEnt{IP: "192.0.2.10"})
	if err != nil {
		t.Fatal(err)
	}
	if gotTarget != "192.0.2.10" || gotTimeout != 3 || gotRetry != 2 || gotSize != 128 || gotTTL != 24 {
		t.Fatalf("DoPing args = (%q, %d, %d, %d, %d)", gotTarget, gotTimeout, gotRetry, gotSize, gotTTL)
	}
	if res.State != "warn" || res.Fields["rtt"] != float64(0) || res.Fields["ttl"] != float64(0) || res.Fields["error"] != "timeout" {
		t.Fatalf("unexpected ping failure result: %+v", res)
	}
}

func TestPingPollerSmokeModeUsesScriptResult(t *testing.T) {
	poller := &PingPoller{
		doPing: func(_ string, _, _, _, _ int) *ping.PingEnt {
			return &ping.PingEnt{Stat: ping.PingOK, Time: 1500, RecvTTL: 62}
		},
	}
	pe := &datastore.PollingEnt{
		Mode:    "smoke",
		Params:  "count=1,size=128,ttl=32",
		Level:   "high",
		PollInt: 30,
		Script:  "loss === 0 && rtt === 1500 && ttl === 62 && interval === 30",
	}
	res, err := poller.Poll(context.Background(), pe, &datastore.NodeEnt{IP: "192.0.2.10"})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != StateNormal {
		t.Fatalf("script should set normal state, got %q", res.State)
	}
	if res.Fields["count"] != float64(1) || res.Fields["avg"] != float64(1500) || res.Fields["ttl"] != float64(62) {
		t.Fatalf("unexpected smoke results: %#v", res.Fields)
	}

	pe.Script = "false;"
	res, err = poller.Poll(context.Background(), pe, &datastore.NodeEnt{IP: "192.0.2.10"})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "high" {
		t.Fatalf("false script should set failure level, got %q", res.State)
	}

	pe.Script = "throw new Error('bad script');"
	res, err = poller.Poll(context.Background(), pe, &datastore.NodeEnt{IP: "192.0.2.10"})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != StateUnknown || !strings.Contains(res.Message, "invalid script") {
		t.Fatalf("invalid script should produce an unknown result with error details: %+v", res)
	}
}

func TestPingPollerLineModeMetrics(t *testing.T) {
	calls := 0
	poller := &PingPoller{
		doPing: func(_ string, _, _, size, _ int) *ping.PingEnt {
			calls++
			rtt := int64(1000000)
			if size == 1364 {
				rtt = 1104000
			}
			return &ping.PingEnt{Stat: ping.PingOK, Time: rtt, RecvTTL: 64}
		},
	}
	res, err := poller.Poll(context.Background(), &datastore.PollingEnt{Mode: "line"}, &datastore.NodeEnt{IP: "192.0.2.10"})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != StateNormal || calls != 10 {
		t.Fatalf("line poll state/calls = %q/%d, want normal/10", res.State, calls)
	}
	if res.Fields["speed"] != float64(100) || res.Fields["ttl"] != float64(64) {
		t.Fatalf("unexpected line metrics: %#v", res.Fields)
	}
}
