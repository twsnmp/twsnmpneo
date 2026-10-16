package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/twsnmp/twsnmpneo/backend/internal/ai"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/monitor"
)

func setupAITestEnv(t *testing.T) (datastore.DataStore, *parquet.Store, *monitor.Monitor, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "twsnmpneo-ai-test-*")
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

	mon := monitor.New(monitor.Config{
		DataDir:  dir,
		Store:    bStore,
		Interval: time.Second,
	})

	cleanup := func() {
		_ = bStore.Close()
		_ = pqStore.Close()
		_ = os.RemoveAll(dir)
	}
	return bStore, pqStore, mon, cleanup
}

func TestLLMClient(t *testing.T) {
	client := ai.NewLLMClient(nil)
	ctx := context.Background()
	_, err := client.GenerateAnswer(ctx, "sys", "user")
	if err == nil {
		t.Fatal("expected error on unconfigured provider")
	}

	conf := &datastore.MapConfEnt{
		LLMProvider: "unknown_provider",
	}
	c2 := ai.NewLLMClient(conf)
	_, err = c2.GenerateAnswer(ctx, "sys", "user")
	if err == nil || !strings.Contains(err.Error(), "unsupported llm provider") {
		t.Fatalf("expected unsupported provider error, got: %v", err)
	}

	localConf := &datastore.MapConfEnt{
		LLMProvider: "local",
		LLMModel:    "test-model",
	}
	cLocal := ai.NewLLMClient(localConf)
	_, err = cLocal.GenerateAnswer(ctx, "system prompt", "user query")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected model not found error, got: %v", err)
	}

	_, err = cLocal.DiagnoseAlert(ctx, "High CPU Utilization", "Core Switch (10.0.0.1)")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected model not found error on DiagnoseAlert, got: %v", err)
	}
}

func TestMCPServer_ComprehensiveToolsAndPrompts(t *testing.T) {
	bStore, pqStore, mon, cleanup := setupAITestEnv(t)
	defer cleanup()
	ctx := context.Background()

	// 1. Setup seed data
	node := &datastore.NodeEnt{
		ID:       "node-1",
		Name:     "Core-Router",
		IP:       "192.168.1.1",
		MAC:      "00:11:22:33:44:55",
		State:    "normal",
		X:        120,
		Y:        150,
		Icon:     "router",
		Descr:    "Main Gateway Router",
		Vendor:   "Cisco",
		SnmpMode: "v2c",
	}
	_ = bStore.SaveNode(ctx, node)

	network := &datastore.NetworkEnt{
		ID:    "net-1",
		Name:  "Management Subnet",
		IP:    "192.168.1.0/24",
		Descr: "Mgmt Net",
		X:     100,
		Y:     100,
		Ports: []datastore.PortEnt{
			{Name: "Gi0/1", State: "up"},
		},
	}
	_ = bStore.SaveNetwork(ctx, network)

	poll := &datastore.PollingEnt{
		ID:       "poll-1",
		NodeID:   "node-1",
		Name:     "Ping Poll",
		Type:     "ping",
		State:    "normal",
		Level:    "info",
		LastTime: time.Now().UnixNano(),
		Result:   map[string]any{"rtt": 1.25, "loss": 0.0},
	}
	_ = bStore.SavePolling(ctx, poll)

	_ = bStore.SaveArpTable(ctx, []*datastore.ArpEnt{
		{
			IP:        "192.168.1.1",
			MAC:       "00:11:22:33:44:55",
			FirstTime: time.Now().UnixNano(),
			LastTime:  time.Now().UnixNano(),
		},
	})

	_ = bStore.SaveSensor(ctx, &datastore.SensorEnt{
		ID:        "sensor-1",
		Host:      "sensor-1.local",
		Type:      "twWifiScan",
		State:     "normal",
		Total:     100,
		Send:      95,
		FirstTime: time.Now().UnixNano(),
		LastTime:  time.Now().UnixNano(),
		Monitors: []datastore.SensorMonitorEnt{
			{CPU: 12.5, Mem: 34.0, Load: 0.5, Process: 42},
		},
	})

	_ = bStore.SaveCertMonitor(ctx, &datastore.CertMonitorEnt{
		ID:        "cert-1",
		Target:    "gateway.local",
		Port:      443,
		Subject:   "CN=gateway.local",
		Issuer:    "CN=Internal CA",
		Verify:    true,
		NotBefore: time.Now().Add(-24 * time.Hour).Unix(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour).Unix(),
	})

	_ = bStore.AddEventLog(ctx, &datastore.EventLogEnt{
		Time:     time.Now().UnixNano(),
		Type:     "system",
		Level:    "info",
		NodeName: "Core-Router",
		NodeID:   "node-1",
		Event:    "System reboot initiated",
	})

	_ = pqStore.WriteLog(&parquet.ParquetLogRecord{
		Time: time.Now().UnixNano(),
		Type: "syslog",
		Src:  "192.168.1.1",
		Log:  `{"host":"192.168.1.1","tag":"sshd","level":"info","message":"Accepted publickey for admin"}`,
	})
	_ = pqStore.WriteLog(&parquet.ParquetLogRecord{
		Time: time.Now().UnixNano(),
		Type: "trap",
		Src:  "192.168.1.1",
		Log:  `{"host":"192.168.1.1","tag":"linkUp","level":"info","variables":"Interface Gi0/1 link UP"}`,
	})
	_ = pqStore.Flush()

	// 2. Initialize MCP Server
	mcpSrv := ai.NewMCPServer(ai.MCPConfig{
		Store:     bStore,
		LogStore:  pqStore,
		Monitor:   mon,
		Version:   "v2.0.0-test",
		MCPMode:   "noauth",
		MCPFrom:   "192.168.1.50, 10.0.0.1",
		Receivers: map[string]any{"syslog": "514/udp"},
	})

	// 3. Test IP Whitelist Checking
	if !mcpSrv.CheckFromAddress("127.0.0.1:54321", "127.0.0.1") {
		t.Error("expected 127.0.0.1 to be allowed")
	}
	if !mcpSrv.CheckFromAddress("192.168.1.50:12345", "") {
		t.Error("expected 192.168.1.50 to be allowed")
	}
	if mcpSrv.CheckFromAddress("172.16.0.99:9999", "172.16.0.99") {
		t.Error("expected 172.16.0.99 to be blocked")
	}

	// 4. Connect in-memory MCP client
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := mcpSrv.GetServer().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Wait()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer clientSession.Close()

	// 5. Test Map tools: get_node_list, get_network_list, get_polling_list, add_node, update_node
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_node_list",
		Arguments: map[string]any{
			"name_filter": "Core.*",
		},
	})
	if err != nil || len(res.Content) == 0 {
		t.Fatalf("get_node_list failed: %v", err)
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Core-Router") {
		t.Errorf("expected Core-Router in result, got: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_network_list",
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Management Subnet") {
		t.Fatalf("get_network_list failed: %v", err)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_polling_list",
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Ping Poll") {
		t.Fatalf("get_polling_list failed: %v", err)
	}

	// Add node
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "add_node",
		Arguments: map[string]any{
			"name": "Branch-Switch",
			"ip":   "192.168.1.10",
			"icon": "server",
			"x":    200,
			"y":    200,
		},
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Branch-Switch") {
		t.Fatalf("add_node failed: %v", err)
	}

	// Update node
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "update_node",
		Arguments: map[string]any{
			"id":          "Branch-Switch",
			"description": "Updated Branch Switch",
		},
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Updated Branch Switch") {
		t.Fatalf("update_node failed: %v", err)
	}

	// 6. Test Report tools: get_sensor_list, get_mac_address_list, get_ip_address_list, get_server_certificate_list, get_resource_monitor_list
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "get_sensor_list"})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "sensor-1.local") {
		t.Fatalf("get_sensor_list failed: %v", err)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "get_mac_address_list"})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "00:11:22:33:44:55") {
		t.Fatalf("get_mac_address_list failed: %v", err)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "get_ip_address_list"})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "192.168.1.1") {
		t.Fatalf("get_ip_address_list failed: %v", err)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "get_server_certificate_list"})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "gateway.local") {
		t.Fatalf("get_server_certificate_list failed: %v", err)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "get_resource_monitor_list"})
	if err != nil {
		t.Fatalf("get_resource_monitor_list failed: %v", err)
	}

	// 7. Test Log & Info tools: search_event_log, add_event_log, search_syslog, get_syslog_summary, search_snmp_trap_log, get_ip_address_info, get_mac_address_info
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_event_log",
		Arguments: map[string]any{
			"event_filter": "System reboot.*",
		},
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "System reboot initiated") {
		t.Fatalf("search_event_log failed: %v", err)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "add_event_log",
		Arguments: map[string]any{
			"level": "warn",
			"event": "Custom alert from MCP",
		},
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Custom alert from MCP") {
		t.Fatalf("add_event_log failed: %v", err)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_syslog",
		Arguments: map[string]any{
			"tag_filter": "sshd",
		},
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Accepted publickey") {
		t.Fatalf("search_syslog failed: %v", err)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_syslog_summary",
		Arguments: map[string]any{
			"summary_type": "tag",
		},
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "sshd") {
		t.Fatalf("get_syslog_summary failed: %v", err)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_snmp_trap_log",
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "linkUp") {
		t.Fatalf("search_snmp_trap_log failed: %v", err)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_ip_address_info",
		Arguments: map[string]any{
			"ip": "192.168.1.1",
		},
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Core-Router") {
		t.Fatalf("get_ip_address_info failed: %v", err)
	}

	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_mac_address_info",
		Arguments: map[string]any{
			"mac": "00:11:22:33:44:55",
		},
	})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Core-Router") {
		t.Fatalf("get_mac_address_info failed: %v", err)
	}

	// 8. Test Prompts: get_node_list, add_node, search_event_log, get_mib_tree
	pRes, err := clientSession.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "get_node_list",
		Arguments: map[string]string{
			"name_filter": "Router",
		},
	})
	if err != nil || len(pRes.Messages) == 0 {
		t.Fatalf("get_node_list prompt failed: %v", err)
	}
	if !strings.Contains(pRes.Messages[0].Content.(*mcp.TextContent).Text, "Router") {
		t.Errorf("expected prompt text to contain filter, got: %s", pRes.Messages[0].Content.(*mcp.TextContent).Text)
	}

	pRes, err = clientSession.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "add_node",
		Arguments: map[string]string{
			"name": "Switch-2",
			"ip":   "192.168.1.20",
		},
	})
	if err != nil || len(pRes.Messages) == 0 {
		t.Fatalf("add_node prompt failed: %v", err)
	}

	pRes, err = clientSession.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "get_mib_tree",
	})
	if err != nil || len(pRes.Messages) == 0 {
		t.Fatalf("get_mib_tree prompt failed: %v", err)
	}

	// 9. Test Streamable HTTP Handler
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mcpSrv.ServeHTTP(rec, req)
	if rec.Code == http.StatusServiceUnavailable {
		t.Errorf("ServeHTTP returned unavailable status: %d", rec.Code)
	}
}

func TestAI_AssistAndDiagnose(t *testing.T) {
	bStore, pqStore, _, cleanup := setupAITestEnv(t)
	defer cleanup()
	ctx := context.Background()

	// Seed node and polling
	n := &datastore.NodeEnt{
		ID:       "node-ai-1",
		Name:     "Test-Server",
		IP:       "192.168.1.50",
		MAC:      "00:22:33:44:55:66",
		Vendor:   "Dell",
		State:    "normal",
		SnmpMode: "v2c",
	}
	_ = bStore.SaveNode(ctx, n)

	p := &datastore.PollingEnt{
		ID:      "poll-ai-1",
		NodeID:  "node-ai-1",
		Name:    "Ping Check",
		Type:    "ping",
		State:   "normal",
		PollInt: 60,
		Level:   "high",
		Params:  "count=1",
	}
	_ = bStore.SavePolling(ctx, p)

	// Mock Ollama server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Prompt string `json:"prompt"`
			System string `json:"system"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		response := "Mock AI diagnosis markdown with details."
		if strings.Contains(req.System, "Object name,OID") || strings.Contains(req.System, "オブジェクト名,OID") {
			response = "ifInOctets,.1.3.6.1.2.1.2.2.1.10"
		} else if strings.Contains(req.System, "template_id") {
			response = `{"template_id": 1, "name": "Ping Monitor", "type": "ping", "mode": "", "script": "rtt < 100 * 1000 * 1000", "advice": "Monitor ping latency"}`
		} else if strings.Contains(req.System, "JavaScript boolean expression") {
			response = `{"script": "code == 200", "explanation": "Checks HTTP 200 status"}`
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response": response,
		})
	}))
	defer mockServer.Close()

	conf := &datastore.MapConfEnt{
		LLMProvider: "ollama",
		LLMBaseURL:  mockServer.URL,
		LLMModel:    "mock",
	}
	client := ai.NewLLMClient(conf)

	// 1. DiagnoseAlertContext
	diag, err := client.DiagnoseAlertContext(ctx, bStore, pqStore, ai.DiagnoseAlertParams{
		AlertEvent: "High CPU on Test-Server",
		NodeID:     "node-ai-1",
		Type:       "polling",
		Level:      "high",
		Lang:       "ja",
	})
	if err != nil || !strings.Contains(diag, "Mock") {
		t.Fatalf("DiagnoseAlertContext failed: %v, resp: %s", err, diag)
	}

	// Test user and system types
	_, err = client.DiagnoseAlertContext(ctx, bStore, pqStore, ai.DiagnoseAlertParams{
		AlertEvent: "User admin deleted node",
		Type:       "user",
		Lang:       "en",
	})
	if err != nil {
		t.Errorf("Diagnose user event failed: %v", err)
	}

	_, err = client.DiagnoseAlertContext(ctx, bStore, pqStore, ai.DiagnoseAlertParams{
		AlertEvent: "Disk space low",
		Type:       "system",
		Lang:       "ja",
	})
	if err != nil {
		t.Errorf("Diagnose system event failed: %v", err)
	}

	_, err = client.DiagnoseAlertContext(ctx, bStore, pqStore, ai.DiagnoseAlertParams{
		AlertEvent: "Certificate expiring",
		Type:       "cert",
		Lang:       "ja",
	})
	if err != nil {
		t.Errorf("Diagnose cert event failed: %v", err)
	}

	// 2. DiagnoseNode
	nodeDiag, err := client.DiagnoseNode(ctx, bStore, pqStore, "node-ai-1", "ja", nil)
	if err != nil || !strings.Contains(nodeDiag, "Mock") {
		t.Fatalf("DiagnoseNode failed: %v, resp: %s", err, nodeDiag)
	}

	// 3. AssistPolling
	assistResp, err := client.AssistPolling(ctx, bStore, "node-ai-1", "Webサーバーの稼働を監視したい", "ja")
	if err != nil || assistResp.Name == "" {
		t.Fatalf("AssistPolling failed: %v, resp: %+v", err, assistResp)
	}

	// 4. GeneratePollingScript
	scriptResp, err := client.GeneratePollingScript(ctx, "http", "status", "Check status 200", "ja")
	if err != nil || scriptResp.Script == "" {
		t.Fatalf("GeneratePollingScript failed: %v, resp: %+v", err, scriptResp)
	}

	// 5. SearchMIB
	mibSearch, err := client.SearchMIB(ctx, "interface packet in", "ja")
	if err != nil || mibSearch.OID == "" {
		t.Fatalf("SearchMIB failed: %v, resp: %+v", err, mibSearch)
	}

	// 6. ExplainMIB
	mibExp, err := client.ExplainMIB(ctx, "walk", ".1.3.6.1.2.1.2.2.1.10", "ifInOctets.1 = 123456", "ja")
	if err != nil || !strings.Contains(mibExp, "Mock") {
		t.Fatalf("ExplainMIB failed: %v", err)
	}

	// 7. ExplainPing
	pingExp, err := client.ExplainPing(ctx, "192.168.1.1", []ai.PingReportItem{
		{Time: 5000000, Stat: 1, Size: 64},
		{Time: 0, Stat: 2, Size: 64},
	}, "normal", "rtt", "ja")
	if err != nil || !strings.Contains(pingExp, "Mock") {
		t.Fatalf("ExplainPing failed: %v", err)
	}

	// 8. ExplainReport
	repExp, err := client.ExplainReport(ctx, bStore, pqStore, "address", map[string]interface{}{"total": 10}, "ipam", "ja")
	if err != nil || !strings.Contains(repExp, "Mock") {
		t.Fatalf("ExplainReport address failed: %v", err)
	}
	_, err = client.ExplainReport(ctx, bStore, pqStore, "cert", map[string]interface{}{"total": 2}, "summary", "en")
	if err != nil {
		t.Errorf("ExplainReport cert failed: %v", err)
	}
	_, err = client.ExplainReport(ctx, bStore, pqStore, "syslog", map[string]interface{}{"count": 100}, "sigma", "ja")
	if err != nil {
		t.Errorf("ExplainReport syslog failed: %v", err)
	}
	_, err = client.ExplainReport(ctx, bStore, pqStore, "netflow", map[string]interface{}{"flows": 50}, "traffic", "ja")
	if err != nil {
		t.Errorf("ExplainReport netflow failed: %v", err)
	}
	_, err = client.ExplainReport(ctx, bStore, pqStore, "mqtt", map[string]interface{}{"clients": 5}, "stats", "ja")
	if err != nil {
		t.Errorf("ExplainReport mqtt failed: %v", err)
	}

	reportTypesToTest := []string{
		"device", "arp", "trap", "polling", "event", "winlog", "sensor", "wifi", "pcap", "anomaly",
	}
	for _, rt := range reportTypesToTest {
		if _, err := client.ExplainReport(ctx, bStore, pqStore, rt, map[string]interface{}{"metric": 42}, "overview", "ja"); err != nil {
			t.Errorf("ExplainReport %s (ja) failed: %v", rt, err)
		}
		if _, err := client.ExplainReport(ctx, bStore, pqStore, rt, map[string]interface{}{"metric": 42}, "overview", "en"); err != nil {
			t.Errorf("ExplainReport %s (en) failed: %v", rt, err)
		}
	}
}

