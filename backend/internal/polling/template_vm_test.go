package polling_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/polling"
)

func TestTemplates_LoadAndGet(t *testing.T) {
	// 1. Load ja templates
	jaTpls, err := polling.LoadTemplates("ja")
	if err != nil {
		t.Fatalf("failed to load ja templates: %v", err)
	}
	if len(jaTpls) == 0 {
		t.Fatalf("expected non-empty ja templates")
	}

	// 2. Load en templates
	enTpls, err := polling.LoadTemplates("en")
	if err != nil {
		t.Fatalf("failed to load en templates: %v", err)
	}
	if len(enTpls) == 0 {
		t.Fatalf("expected non-empty en templates")
	}

	// 3. Get single template by ID
	firstID := jaTpls[0].ID
	tpl, err := polling.GetTemplate("ja", firstID)
	if err != nil || tpl == nil {
		t.Fatalf("expected to find template %d (err: %v)", firstID, err)
	}
	if tpl.ID != firstID {
		t.Fatalf("expected template ID %d, got %d", firstID, tpl.ID)
	}

	// Non-existent template
	_, err = polling.GetTemplate("ja", 999999)
	if err == nil {
		t.Fatalf("expected error for non-existent template ID")
	}
}

func TestTemplates_GenerateAutoPollings(t *testing.T) {
	node := &datastore.NodeEnt{
		ID:        "node-1",
		Name:      "test-node",
		IP:        "127.0.0.1",
		SnmpPort:  161,
		Community: "public",
	}

	// 1. Template without AutoParam
	tplNonAuto := &polling.PollingTemplateEnt{
		ID:        1,
		Name:      "Ping Check",
		Type:      "ping",
		Mode:      "",
		Params:    "",
		Filter:    "",
		Extractor: "",
		Script:    "",
		Level:     "warn",
		Descr:     "Simple ping",
	}

	pollings := polling.GenerateAutoPollings(context.Background(), node, tplNonAuto)
	if len(pollings) != 1 {
		t.Fatalf("expected 1 polling, got %d", len(pollings))
	}
	p := pollings[0]
	if p.NodeID != "node-1" || p.Type != "ping" || p.Level != "warn" {
		t.Fatalf("unexpected polling properties: %+v", p)
	}
}

func TestTemplates_AutoGrok(t *testing.T) {
	sample := "cpu=50 mem=1000"
	pat := polling.AutoGrok(sample)
	if pat == "" {
		t.Fatalf("expected non-empty grok pattern for sample text")
	}
}

func TestOttoVM_SetupAndBuiltins(t *testing.T) {
	pe := &datastore.PollingEnt{
		PollInt: 60,
		Result: map[string]interface{}{
			"rtt":       15.5,
			"loss":      0.0,
			"load":      2.0,
			"prev_text": "hello",
		},
	}
	fields := map[string]interface{}{
		"rtt":  25.0,
		"load": 4.5,
	}

	vm := otto.New()
	polling.SetupOttoVM(pe, vm, fields)

	// 1. Test interval and iterval variables
	v, err := vm.Get("interval")
	if err != nil || v.String() != "60" {
		t.Fatalf("expected interval=60, got %v (err: %v)", v, err)
	}
	v, err = vm.Get("iterval")
	if err != nil || v.String() != "60" {
		t.Fatalf("expected iterval=60, got %v (err: %v)", v, err)
	}

	// 2. Test historical *_last fields
	v, err = vm.Get("rtt_last")
	if err != nil || v.String() != "15.5" {
		t.Fatalf("expected rtt_last=15.5, got %v", v)
	}
	v, err = vm.Get("loss_last")
	if err != nil || v.String() != "0" {
		t.Fatalf("expected loss_last=0, got %v", v)
	}

	// 3. Test getResult, setResult, and setLevel built-in functions
	script := `
		var prev = getResult("prev_text");
		var rate = (load - load_last) / interval;
		setResult("rate", rate);
		setResult("checked_prev", prev);
		if (rate > 0.01) {
			setLevel("warn");
		}
		rate > 0;
	`
	resVal, err := vm.Run(script)
	if err != nil {
		t.Fatalf("script execution error: %v", err)
	}
	isSuccess, _ := resVal.ToBoolean()
	if !isSuccess {
		t.Fatalf("expected script to evaluate to true")
	}

	// Check that setResult updated pe.Result
	if pe.Result["rate"] == nil {
		t.Fatalf("expected pe.Result['rate'] to be set")
	}
	if pe.Result["checked_prev"] != "hello" {
		t.Fatalf("expected pe.Result['checked_prev'] == 'hello', got %v", pe.Result["checked_prev"])
	}

	// Check that setLevel updated pe.Result["_level"]
	if pe.Result["_level"] != "warn" {
		t.Fatalf("expected pe.Result['_level'] == 'warn', got %v", pe.Result["_level"])
	}
}

func TestSyslogPoller_SigmaAndModes(t *testing.T) {
	poller := polling.NewSyslogPoller(nil, nil)
	ctx := context.Background()

	// 1. count mode
	pe := &datastore.PollingEnt{
		Type:    "syslog",
		Mode:    "count",
		Script:  "count == 0",
		PollInt: 60,
	}
	res, err := poller.Poll(ctx, pe, nil)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on count 0 with nil store, got %s (err: %v)", res.State, err)
	}
	if res.Fields["count"] != float64(0) {
		t.Fatalf("expected count 0, got %v", res.Fields["count"])
	}

	// 2. sigma mode
	peSigma := &datastore.PollingEnt{
		Type:    "syslog",
		Mode:    "sigma",
		PollInt: 60,
	}
	res, err = poller.Poll(ctx, peSigma, nil)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on empty sigma check, got %s (err: %v)", res.State, err)
	}
}

func TestTrapArpNetflowPollers(t *testing.T) {
	ctx := context.Background()

	// SNMP TRAP
	trapPoller := polling.NewSnmpTrapPoller(nil, nil)
	peTrap := &datastore.PollingEnt{
		Type:    "trap",
		Mode:    "count",
		Script:  "count == 0",
		PollInt: 60,
	}
	resTrap, err := trapPoller.Poll(ctx, peTrap, nil)
	if err != nil || resTrap.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on trap count 0, got %s (err: %v)", resTrap.State, err)
	}

	// ARP Log
	arpPoller := polling.NewArpLogPoller(nil, nil)
	peArp := &datastore.PollingEnt{
		Type:    "arplog",
		Mode:    "count",
		Script:  "count == 0",
		PollInt: 60,
	}
	resArp, err := arpPoller.Poll(ctx, peArp, nil)
	if err != nil || resArp.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on arplog count 0, got %s (err: %v)", resArp.State, err)
	}

	// NetFlow
	nfPoller := polling.NewNetFlowPoller(nil, nil)
	peNf := &datastore.PollingEnt{
		Type:    "netflow",
		Mode:    "count",
		Script:  "count == 0",
		PollInt: 60,
	}
	resNf, err := nfPoller.Poll(ctx, peNf, nil)
	if err != nil || resNf.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on netflow count 0, got %s (err: %v)", resNf.State, err)
	}
}

func TestPiHolePoller(t *testing.T) {
	// Mock Pi-Hole summary API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"domains_being_blocked": 125000,
			"dns_queries_today":     4500,
			"ads_blocked_today":     350,
			"ads_percentage_today":  7.77,
			"status":                "enabled",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	poller := polling.NewPiHolePoller()
	ctx := context.Background()

	pe := &datastore.PollingEnt{
		Type:    "pihole",
		Params:  server.URL,
		Script:  "status === 'enabled' && ads_percentage_today > 5",
		Timeout: 2,
	}
	res, err := poller.Poll(ctx, pe, nil)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on pihole mock, got %s (err: %v, msg: %s)", res.State, err, res.Message)
	}
	if res.Fields["ads_percentage_today"] != 7.77 {
		t.Fatalf("expected ads_percentage_today == 7.77, got %v", res.Fields["ads_percentage_today"])
	}
}

func TestMQTTPoller_OfflineTarget(t *testing.T) {
	poller := polling.NewMQTTPoller()
	ctx := context.Background()

	node := &datastore.NodeEnt{
		IP: "127.0.0.1",
	}
	pe := &datastore.PollingEnt{
		Type:    "mqtt",
		Params:  "65534", // closed port
		Timeout: 1,
		Level:   "warn",
	}
	res, err := poller.Poll(ctx, pe, node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.State != polling.StateWarn {
		t.Fatalf("expected StateWarn on closed mqtt port, got %s", res.State)
	}
}

func TestEMailPoller_OfflineTarget(t *testing.T) {
	poller := polling.NewEMailPoller()
	ctx := context.Background()

	node := &datastore.NodeEnt{
		IP: "127.0.0.1",
	}
	pe := &datastore.PollingEnt{
		Type:    "email",
		Mode:    "pop3",
		Params:  "65534", // closed port
		Timeout: 1,
		Level:   "high",
	}
	res, err := poller.Poll(ctx, pe, node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on closed pop3 port, got %s", res.State)
	}
}

func TestLXIPoller_OfflineTarget(t *testing.T) {
	poller := polling.NewLXIPoller()
	ctx := context.Background()

	node := &datastore.NodeEnt{
		IP: "127.0.0.1",
	}
	pe := &datastore.PollingEnt{
		Type:    "lxi",
		Params:  "65534", // closed port
		Timeout: 1,
		Level:   "warn",
	}
	res, err := poller.Poll(ctx, pe, node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.State != polling.StateWarn {
		t.Fatalf("expected StateWarn on closed lxi port, got %s", res.State)
	}
}

func TestTwLogEyePoller_OfflineTarget(t *testing.T) {
	poller := polling.NewTwLogEyePoller()
	ctx := context.Background()

	node := &datastore.NodeEnt{
		IP: "127.0.0.1",
	}
	pe := &datastore.PollingEnt{
		Type:    "twlogeye",
		Params:  "http://127.0.0.1:65534/api/status",
		Timeout: 1,
		Level:   "high",
	}
	res, err := poller.Poll(ctx, pe, node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on offline twlogeye target, got %s", res.State)
	}
}
