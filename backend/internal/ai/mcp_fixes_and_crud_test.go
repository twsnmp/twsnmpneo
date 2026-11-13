package ai

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/logreport"
	"github.com/twsnmp/twsnmpneo/backend/internal/monitor"
)

func setupTestMCPClient(t *testing.T, store datastore.DataStore, pq *parquet.Store, mon *monitor.Monitor, cfgPatch func(*MCPConfig)) (*MCPServer, *mcp.ClientSession, func()) {
	t.Helper()
	ctx := context.Background()

	cfg := MCPConfig{
		Store:    store,
		LogStore: pq,
		Monitor:  mon,
		Version:  "v0.6.0-test",
		MCPMode:  "noauth",
	}
	if cfgPatch != nil {
		cfgPatch(&cfg)
	}

	mcpSrv := NewMCPServer(cfg)
	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := mcpSrv.server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0"}, nil)
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect failed: %v", err)
	}

	cleanup := func() {
		clientSession.Close()
		serverSession.Close()
	}
	return mcpSrv, clientSession, cleanup
}

func TestMCP_BugFixes(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := bbolt.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("bbolt.New failed: %v", err)
	}
	defer store.Close()

	// Seed WifiAP & Bluetooth report entries using canonical logreport constants
	wifiBytes, _ := json.Marshal(logreport.WifiAPEnt{
		ID:        "wifi-1",
		BSSID:     "AA:BB:CC:DD:EE:01",
		SSID:      "CorpWiFi",
		Channel:   "36",
		RSSI:      []logreport.RSSIEnt{{Value: -48, Time: 1700000060000000000}},
		Vendor:    "Aruba",
		Count:     5,
		FirstTime: 1700000000000000000,
		LastTime:  1700000060000000000,
	})
	if err := store.SaveLogReportData(ctx, logreport.KindWifiAP, map[string][]byte{"wifi-1": wifiBytes}); err != nil {
		t.Fatalf("SaveLogReportData wifiAP: %v", err)
	}
	blueBytes, _ := json.Marshal(logreport.BlueDeviceEnt{
		ID:        "bt-1",
		Address:   "11:22:33:44:55:66",
		Name:      "Beacon-Lab",
		Vendor:    "Nordic",
		RSSI:      []logreport.RSSIEnt{{Value: -62, Time: 1700000060000000000}},
		Count:     3,
		FirstTime: 1700000000000000000,
		LastTime:  1700000060000000000,
	})
	if err := store.SaveLogReportData(ctx, logreport.KindBlueDevice, map[string][]byte{"bt-1": blueBytes}); err != nil {
		t.Fatalf("SaveLogReportData blueDevice: %v", err)
	}

	// Seed ARP entry using Unix seconds (as produced by receiver/arpwatch.go)
	arpSec := int64(1794572230) // 2026-11-... in seconds
	if err := store.SaveArpTable(ctx, []*datastore.ArpEnt{
		{
			IP:        "192.168.10.25",
			MAC:       "02:11:22:33:44:55",
			FirstTime: arpSec,
			LastTime:  arpSec + 60,
		},
	}); err != nil {
		t.Fatalf("SaveArpTable: %v", err)
	}

	// Seed Monitor sample (UnixNano timestamp)
	mon := monitor.New(monitor.Config{Store: store, Interval: time.Minute})
	mon.UpdateNow()

	recvConfigured := map[string]any{
		"syslog":  true,
		"netflow": true,
		"mcp":     true,
	}

	mcpSrv, cs, cleanup := setupTestMCPClient(t, store, nil, mon, func(c *MCPConfig) {
		c.MCPFrom = "10.10.0.0/16, 172.20.1.5"
		c.ReceiverStatus = func(mcpEnabled bool) map[string]any {
			return recvConfigured
		}
	})
	defer cleanup()

	// 1. get_wifi_ap_list reads KindWifiAP ("wifiAP")
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_wifi_ap_list"})
	if err != nil {
		t.Fatalf("get_wifi_ap_list failed: %v", err)
	}
	if txt := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(txt, "CorpWiFi") || !strings.Contains(txt, "AA:BB:CC:DD:EE:01") {
		t.Errorf("expected CorpWiFi in get_wifi_ap_list, got: %s", txt)
	}

	// 2. get_bluetooth_device_list reads KindBlueDevice
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_bluetooth_device_list"})
	if err != nil {
		t.Fatalf("get_bluetooth_device_list failed: %v", err)
	}
	if txt := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(txt, "Beacon-Lab") {
		t.Errorf("expected Beacon-Lab in get_bluetooth_device_list, got: %s", txt)
	}

	// 3. get_ip_address_list & get_mac_address_list format Unix seconds accurately (year >= 2026, not 1970)
	wantYear := time.Unix(arpSec, 0).Format("2006-")
	for _, toolName := range []string{"get_ip_address_list", "get_mac_address_list"} {
		res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: toolName})
		if err != nil {
			t.Fatalf("%s failed: %v", toolName, err)
		}
		txt := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(txt, wantYear) || strings.Contains(txt, "1970-") {
			t.Errorf("%s formatted Unix seconds incorrectly: %s", toolName, txt)
		}
	}

	// 4. get_resource_monitor_list formats UnixNano cleanly (current year, not 56865...)
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_resource_monitor_list"})
	if err != nil {
		t.Fatalf("get_resource_monitor_list failed: %v", err)
	}
	monTxt := res.Content[0].(*mcp.TextContent).Text
	curYear := time.Now().Format("2006-")
	if !strings.Contains(monTxt, curYear) || strings.Contains(monTxt, "5686") {
		t.Errorf("get_resource_monitor_list formatted time incorrectly: %s", monTxt)
	}

	// 5. get_system_status dynamically populates receivers.configured
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_system_status"})
	if err != nil {
		t.Fatalf("get_system_status failed: %v", err)
	}
	sysTxt := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(sysTxt, `"syslog": true`) || !strings.Contains(sysTxt, `"netflow": true`) {
		t.Errorf("expected configured receivers in get_system_status, got: %s", sysTxt)
	}

	// 6. CIDR CheckFromAddress & UpdateConf clear-to-empty behavior
	if !mcpSrv.CheckFromAddress("10.10.254.9:1234", "") {
		t.Error("expected CIDR 10.10.0.0/16 to match 10.10.254.9")
	}
	if !mcpSrv.CheckFromAddress("", "172.20.1.5") {
		t.Error("expected exact IP 172.20.1.5 to match")
	}
	if mcpSrv.CheckFromAddress("10.11.0.1:1234", "") {
		t.Error("expected 10.11.0.1 outside 10.10.0.0/16 to be rejected")
	}

	// Clearing MCPFrom via UpdateConf must remove previous CIDRs and IPs
	mcpSrv.UpdateConf(&datastore.MapConfEnt{EnableMCP: true, MCPMode: "noauth", MCPFrom: ""})
	if mcpSrv.CheckFromAddress("10.10.254.9:1234", "") {
		t.Error("expected previously allowed CIDR to be rejected after clearing MCPFrom")
	}
	if mcpSrv.CheckFromAddress("172.20.1.5:1234", "") {
		t.Error("expected previously allowed IP to be rejected after clearing MCPFrom")
	}
	if !mcpSrv.CheckFromAddress("127.0.0.1:9999", "") {
		t.Error("expected loopback to remain allowed after clearing MCPFrom")
	}

	// 7. snmpset / snmpwalk validation and SNMP mode builder
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "snmpset",
		Arguments: map[string]any{
			"target":          "127.0.0.1",
			"mib_object_name": "nonExistentMibObjectXYZ.0",
			"type":            "string",
			"value":           "abc",
		},
	})
	if err == nil && (len(res.Content) == 0 || !res.IsError) {
		t.Error("expected invalid MIB object name error from snmpset")
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "snmpset",
		Arguments: map[string]any{
			"target":          "127.0.0.1",
			"mib_object_name": ".1.3.6.1.2.1.1.5.0",
			"type":            "invalidtype",
			"value":           "abc",
		},
	})
	if err == nil && (len(res.Content) == 0 || !res.IsError) {
		t.Error("expected invalid value type error from snmpset")
	}

	// Verify shared buildSNMPAgent supports v1, v2c (with "public" fallback), and v3 modes
	a1, err := mcpSrv.buildSNMPAgent(ctx, "192.0.2.10", "", "", "", "", "v1")
	if err != nil || a1.Version != gosnmp.Version1 || a1.Community != "public" {
		t.Errorf("unexpected v1 agent config: %+v, err=%v", a1, err)
	}
	a3, err := mcpSrv.buildSNMPAgent(ctx, "192.0.2.10", "", "", "snmpuser", "secretpw", "v3authprivex")
	if err != nil || a3.Version != gosnmp.Version3 || a3.MsgFlags != gosnmp.AuthPriv {
		t.Errorf("unexpected v3authprivex agent config: %+v, err=%v", a3, err)
	}
	usm, ok := a3.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	if !ok || usm.UserName != "snmpuser" || usm.AuthenticationProtocol != gosnmp.SHA256 || usm.PrivacyProtocol != gosnmp.AES256 {
		t.Errorf("unexpected v3authprivex USM parameters: %+v", usm)
	}
	if _, err := mcpSrv.buildSNMPAgent(ctx, "192.0.2.10", "", "", "", "", "bogus"); err == nil {
		t.Error("expected unsupported snmp_mode error")
	}
}

func TestMCP_AddNodeNoAutoPollingAndCRUD(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := bbolt.New(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("bbolt.New failed: %v", err)
	}
	defer store.Close()

	pq, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(tmpDir, "logs"),
		BufferSize:     1,
		BufferInterval: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("parquet.New failed: %v", err)
	}
	defer pq.Close()

	// Seed NetFlow log entry
	flowPayload, _ := json.Marshal(datastore.NetFlowEnt{
		Time:     time.Now().UnixNano(),
		SrcAddr:  "10.1.2.3",
		SrcPort:  54321,
		DstAddr:  "10.1.2.10",
		DstPort:  443,
		Protocol: "tcp",
		Bytes:    4096,
		Packets:  12,
	})
	_ = pq.WriteLog(&parquet.ParquetLogRecord{
		Time: time.Now().UnixNano(),
		Type: "netflow",
		Src:  "10.1.2.3",
		Log:  string(flowPayload),
	})
	_ = pq.Flush()

	var executedPollingID string
	var checkedNodeID string
	var checkedAll bool

	_, cs, cleanup := setupTestMCPClient(t, store, pq, nil, func(c *MCPConfig) {
		c.ExecutePolling = func(_ context.Context, p *datastore.PollingEnt) (string, string, map[string]any, error) {
			executedPollingID = p.ID
			return "normal", "ok", map[string]any{"rtt": 1.23}, nil
		}
		c.CheckNode = func(_ context.Context, nodeID string) (int, error) {
			checkedNodeID = nodeID
			return 2, nil
		}
		c.CheckAllPollings = func(_ context.Context) int {
			checkedAll = true
			return 5
		}
	})
	defer cleanup()

	// 1. Verify full tool and prompt inventory counts (42 tools, 31 prompts)
	toolsRes, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(toolsRes.Tools) != 42 {
		t.Errorf("expected 42 registered tools, got %d", len(toolsRes.Tools))
	}
	promptsRes, err := cs.ListPrompts(ctx, &mcp.ListPromptsParams{})
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}
	if len(promptsRes.Prompts) != 31 {
		t.Errorf("expected 31 registered prompts, got %d", len(promptsRes.Prompts))
	}

	// 2. add_node must NOT automatically add any PING polling
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "add_node",
		Arguments: map[string]any{
			"name": "Node-A",
			"ip":   "192.168.50.1",
		},
	})
	if err != nil {
		t.Fatalf("add_node Node-A failed: %v", err)
	}
	var nodeA mcpNodeEnt
	_ = json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &nodeA)
	if polls, _ := store.ListPollings(ctx); len(polls) != 0 {
		t.Fatalf("expected 0 auto-created pollings after add_node, got %d", len(polls))
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "add_node",
		Arguments: map[string]any{
			"name": "Node-B",
			"ip":   "192.168.50.2",
		},
	})
	if err != nil {
		t.Fatalf("add_node Node-B failed: %v", err)
	}
	var nodeB mcpNodeEnt
	_ = json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &nodeB)

	// 3. add_polling, get_polling, update_polling, check_polling
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "add_polling",
		Arguments: map[string]any{
			"node_id":  nodeA.ID,
			"name":     "Custom Ping",
			"type":     "ping",
			"params":   "64,1,1",
			"level":    "low",
			"log_mode": "on_change",
		},
	})
	if err != nil {
		t.Fatalf("add_polling failed: %v", err)
	}
	var pollA mcpPollingDetailEnt
	_ = json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &pollA)
	if pollA.ID == "" || pollA.Level != "low" || pollA.PollInt != 60 || pollA.LogMode != datastore.LogModeOnChange {
		t.Fatalf("unexpected add_polling output: %+v", pollA)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_polling",
		Arguments: map[string]any{"id": pollA.ID},
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Custom Ping") {
		t.Fatalf("get_polling failed: %v", err)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "update_polling",
		Arguments: map[string]any{
			"id":          pollA.ID,
			"name":        "Updated Ping",
			"level":       "high",
			"poll_int":    30,
			"log_mode":    "ai",
			"reset_state": true,
		},
	})
	if err != nil {
		t.Fatalf("update_polling failed: %v", err)
	}
	var pollUpdated mcpPollingDetailEnt
	_ = json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &pollUpdated)
	if pollUpdated.Name != "Updated Ping" || pollUpdated.Level != "high" || pollUpdated.PollInt != 30 || pollUpdated.LogMode != datastore.LogModeAI {
		t.Errorf("unexpected updated polling: %+v", pollUpdated)
	}

	// check_polling by polling_id, node_id, all
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "check_polling",
		Arguments: map[string]any{"polling_id": pollA.ID},
	}); err != nil || executedPollingID != pollA.ID {
		t.Errorf("check_polling by polling_id failed: err=%v executedID=%s", err, executedPollingID)
	}
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "check_polling",
		Arguments: map[string]any{"node_id": nodeA.ID},
	}); err != nil || checkedNodeID != nodeA.ID {
		t.Errorf("check_polling by node_id failed: err=%v checkedNodeID=%s", err, checkedNodeID)
	}
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "check_polling",
		Arguments: map[string]any{"all": true},
	}); err != nil || !checkedAll {
		t.Errorf("check_polling all failed: err=%v checkedAll=%v", err, checkedAll)
	}

	// 4. get_ai_anomaly_list
	_ = store.SaveAIResult(ctx, &datastore.AIResultEnt{
		PollingID: pollA.ID,
		ScoreData: [][]float64{{1, 51.0}, {2, 87.5}},
		LastTime:  time.Now().Unix(),
	})
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_ai_anomaly_list",
		Arguments: map[string]any{"min_score": 80.0},
	})
	if err != nil {
		t.Fatalf("get_ai_anomaly_list failed: %v", err)
	}
	if txt := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(txt, "Updated Ping") || !strings.Contains(txt, "87.5") {
		t.Errorf("expected AI anomaly entry in get_ai_anomaly_list, got: %s", txt)
	}

	// 5. get_node, add_line, get_line_list, delete_line, delete_polling
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_node",
		Arguments: map[string]any{"id": nodeA.ID},
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `"polling_count": 1`) {
		t.Fatalf("get_node failed: err=%v", err)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "add_line",
		Arguments: map[string]any{
			"node_id1":    nodeA.ID,
			"polling_id1": pollA.ID,
			"node_id2":    nodeB.ID,
			"width":       3,
			"port":        "eth0",
			"info":        "uplink",
		},
	})
	if err != nil {
		t.Fatalf("add_line failed: %v", err)
	}
	var createdLine mcpLineEnt
	_ = json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &createdLine)
	if createdLine.ID == "" || createdLine.NodeName1 != "Node-A" || createdLine.NodeName2 != "Node-B" {
		t.Errorf("unexpected add_line result: %+v", createdLine)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_line_list",
		Arguments: map[string]any{"node_filter": "Node-A"},
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "uplink") {
		t.Fatalf("get_line_list failed: %v", err)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "delete_line",
		Arguments: map[string]any{"id": createdLine.ID},
	})
	if err != nil {
		t.Fatalf("delete_line failed: %v", err)
	}
	if _, gerr := store.GetLine(ctx, createdLine.ID); gerr == nil {
		t.Error("expected line to be removed from datastore after delete_line")
	}

	// Re-add line bound to pollA so delete_polling verifies cascade cleanup
	res, _ = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "add_line",
		Arguments: map[string]any{
			"node_id1":    nodeA.ID,
			"polling_id1": pollA.ID,
			"node_id2":    nodeB.ID,
		},
	})
	_ = json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &createdLine)

	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "delete_polling",
		Arguments: map[string]any{"id": pollA.ID},
	}); err != nil {
		t.Fatalf("delete_polling failed: %v", err)
	}
	if _, gerr := store.GetPolling(ctx, pollA.ID); gerr == nil {
		t.Error("expected polling removed after delete_polling")
	}
	if _, gerr := store.GetLine(ctx, createdLine.ID); gerr == nil {
		t.Error("expected line cascade-deleted when bound polling was deleted")
	}

	// 6. delete_node ambiguity check + clean delete
	_ = store.SaveNode(ctx, &datastore.NodeEnt{Name: "DupNode", IP: "10.0.0.10"})
	_ = store.SaveNode(ctx, &datastore.NodeEnt{Name: "DupNode", IP: "10.0.0.11"})
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "delete_node",
		Arguments: map[string]any{"id": "DupNode"},
	})
	if err == nil && !res.IsError {
		t.Error("expected ambiguous node name delete_node error")
	}
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "delete_node",
		Arguments: map[string]any{"id": nodeB.ID},
	}); err != nil {
		t.Fatalf("delete_node failed: %v", err)
	}
	if _, gerr := store.GetNode(ctx, nodeB.ID); gerr == nil {
		t.Error("expected nodeB deleted from datastore")
	}

	// 7. search_netflow
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_netflow",
		Arguments: map[string]any{
			"protocol_filter": "tcp",
			"port_filter":     "^443$",
		},
	})
	if err != nil {
		t.Fatalf("search_netflow failed: %v", err)
	}
	if txt := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(txt, "10.1.2.3") {
		t.Errorf("expected flow record in search_netflow, got: %s", txt)
	}

	// 8. Exercise new prompt templates
	for _, tc := range []struct {
		name string
		args map[string]string
	}{
		{"get_node", map[string]string{"id": nodeA.ID}},
		{"delete_node", map[string]string{"id": nodeA.ID}},
		{"get_line_list", map[string]string{"node_filter": "Node"}},
		{"add_line", map[string]string{"node_id1": nodeA.ID, "node_id2": "Node-A"}},
		{"delete_line", map[string]string{"id": "line-1"}},
		{"get_polling", map[string]string{"id": "poll-1"}},
		{"add_polling", map[string]string{"node_id": nodeA.ID, "name": "HTTP", "type": "http"}},
		{"update_polling", map[string]string{"id": "poll-1", "poll_int": "30"}},
		{"delete_polling", map[string]string{"id": "poll-1"}},
		{"check_polling", map[string]string{"node_id": nodeA.ID}},
		{"search_netflow", map[string]string{"port_filter": "443"}},
		{"get_ai_anomaly_list", map[string]string{"min_score": "75"}},
	} {
		pRes, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: tc.name, Arguments: tc.args})
		if err != nil || len(pRes.Messages) == 0 {
			t.Errorf("GetPrompt(%q) failed: err=%v", tc.name, err)
		}
	}
}
