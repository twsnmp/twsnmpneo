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
)

func setupAITestEnv(t *testing.T) (datastore.DataStore, *parquet.Store, func()) {
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

	cleanup := func() {
		_ = bStore.Close()
		_ = pqStore.Close()
		_ = os.RemoveAll(dir)
	}
	return bStore, pqStore, cleanup
}

func TestLLMClient(t *testing.T) {
	// 1. Unconfigured provider
	client := ai.NewLLMClient(nil)
	ctx := context.Background()
	_, err := client.GenerateAnswer(ctx, "sys", "user")
	if err == nil {
		t.Fatal("expected error on unconfigured provider")
	}

	// 2. Unsupported provider
	conf := &datastore.MapConfEnt{
		LLMProvider: "unknown_provider",
	}
	c2 := ai.NewLLMClient(conf)
	_, err = c2.GenerateAnswer(ctx, "sys", "user")
	if err == nil || !strings.Contains(err.Error(), "unsupported llm provider") {
		t.Fatalf("expected unsupported provider error, got: %v", err)
	}

	// 3. Local/tensai fallback provider
	localConf := &datastore.MapConfEnt{
		LLMProvider: "local",
		LLMModel:    "test-model",
	}
	cLocal := ai.NewLLMClient(localConf)
	ans, err := cLocal.GenerateAnswer(ctx, "system prompt", "user query")
	if err != nil || !strings.Contains(ans, "Local Model Response") {
		t.Fatalf("expected local model response, got: %v (err: %v)", ans, err)
	}

	// 4. DiagnoseAlert helper
	diag, err := cLocal.DiagnoseAlert(ctx, "High CPU Utilization", "Core Switch (10.0.0.1)")
	if err != nil || !strings.Contains(diag, "High CPU") {
		t.Fatalf("expected diagnosis output, got: %s (err: %v)", diag, err)
	}
}

func TestLLMClient_ProvidersMock(t *testing.T) {
	ctx := context.Background()

	// Mock server handling Ollama, OpenAI, Gemini, Claude APIs
	mockSvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "generateContent"): // Gemini
			resp := map[string]interface{}{
				"candidates": []map[string]interface{}{
					{"content": map[string]interface{}{"parts": []map[string]string{{"text": "gemini response"}}}},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/api/generate"): // Ollama
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"response": "ollama response",
			})

		case strings.Contains(r.URL.Path, "/chat/completions"): // OpenAI
			resp := map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]string{"content": "openai response"}},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/messages"): // Claude
			resp := map[string]interface{}{
				"content": []map[string]string{
					{"text": "claude response"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		default:
			http.NotFound(w, r)
		}
	}))
	defer mockSvr.Close()

	// 1. Ollama mock test
	ollamaClient := ai.NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "ollama",
		LLMBaseURL:  mockSvr.URL,
	})
	res, err := ollamaClient.GenerateAnswer(ctx, "sys", "test prompt")
	if err != nil || res != "ollama response" {
		t.Fatalf("ollama test failed: res=%s, err=%v", res, err)
	}

	// 2. OpenAI mock test
	openAIClient := ai.NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "openai",
		LLMBaseURL:  mockSvr.URL,
		LLMAPIKey:   "sk-test",
	})
	res, err = openAIClient.GenerateAnswer(ctx, "sys", "test prompt")
	if err != nil || res != "openai response" {
		t.Fatalf("openai test failed: res=%s, err=%v", res, err)
	}

	// 3. Gemini mock test
	geminiClient := ai.NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "gemini",
		LLMBaseURL:  mockSvr.URL,
		LLMAPIKey:   "gem-test-key",
	})
	res, err = geminiClient.GenerateAnswer(ctx, "sys", "test prompt")
	if err != nil || res != "gemini response" {
		t.Fatalf("gemini test failed: res=%s, err=%v", res, err)
	}

	// 4. Claude mock test
	claudeClient := ai.NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "claude",
		LLMBaseURL:  mockSvr.URL,
		LLMAPIKey:   "ant-test",
	})
	res, err = claudeClient.GenerateAnswer(ctx, "sys", "test prompt")
	if err != nil || res != "claude response" {
		t.Fatalf("claude test failed: res=%s, err=%v", res, err)
	}

	// 5. Error handling mock tests (server returns 500)
	errSvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
	}))
	defer errSvr.Close()

	errClient := ai.NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "gemini",
		LLMBaseURL:  errSvr.URL,
	})
	_, err = errClient.GenerateAnswer(ctx, "sys", "test")
	if err == nil {
		t.Error("expected error for gemini 500 status")
	}

	errClaude := ai.NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "claude",
		LLMBaseURL:  errSvr.URL,
	})
	_, err = errClaude.GenerateAnswer(ctx, "sys", "test")
	if err == nil {
		t.Error("expected error for claude 500 status")
	}

	errOpenAI := ai.NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "openai",
		LLMBaseURL:  errSvr.URL,
	})
	_, err = errOpenAI.GenerateAnswer(ctx, "sys", "test")
	if err == nil {
		t.Error("expected error for openai 500 status")
	}

	errOllama := ai.NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "ollama",
		LLMBaseURL:  errSvr.URL,
	})
	_, err = errOllama.GenerateAnswer(ctx, "sys", "test")
	if err == nil {
		t.Error("expected error for ollama 500 status")
	}
}

func TestMCPServer_Tools(t *testing.T) {
	bStore, pqStore, cleanup := setupAITestEnv(t)
	defer cleanup()
	ctx := context.Background()

	// Populate test data
	node := &datastore.NodeEnt{
		ID:    "node-ai-1",
		Name:  "AI Core Switch",
		IP:    "10.10.10.1",
		State: "warn",
	}
	_ = bStore.SaveNode(ctx, node)

	poll := &datastore.PollingEnt{
		ID:     "poll-ai-1",
		NodeID: "node-ai-1",
		Name:   "SNMP CPU",
		Type:   "snmp",
		State:  "warn",
	}
	_ = bStore.SavePolling(ctx, poll)

	_ = bStore.AddEventLog(ctx, &datastore.EventLogEnt{
		Time:     time.Now().UnixNano(),
		Type:     "polling",
		Level:    "warn",
		NodeName: "AI Core Switch",
		NodeID:   "node-ai-1",
		Event:    "High CPU Utilization 92%",
	})

	_ = pqStore.WriteLog(&parquet.ParquetLogRecord{
		Time: time.Now().UnixNano(),
		Type: "syslog",
		Src:  "10.10.10.1",
		Log:  `{"msg":"fan failure alert"}`,
	})
	_ = pqStore.Flush()

	// Initialize MCP Server
	mcpSrv := ai.NewMCPServer(ai.MCPConfig{
		Store:    bStore,
		LogStore: pqStore,
		Version:  "v0.1.0-test",
	})

	// Setup in-memory client and server transports
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := mcpSrv.GetServer().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	defer serverSession.Wait()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect failed: %v", err)
	}
	defer clientSession.Close()

	// 1. Call get_system_status
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_system_status",
	})
	if err != nil || len(res.Content) == 0 {
		t.Fatalf("call get_system_status failed: %v", err)
	}
	var status map[string]interface{}
	_ = json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &status)
	if status["version"] != "v0.1.0-test" {
		t.Errorf("unexpected system status: %v", status)
	}

	// 2. Call list_nodes
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "list_nodes",
	})
	if err != nil || len(res.Content) == 0 {
		t.Fatalf("call list_nodes failed: %v", err)
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "AI Core Switch") {
		t.Errorf("expected node in list, got: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	// 3. Call get_node_detail
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_node_detail",
		Arguments: map[string]any{
			"node_id": "node-ai-1",
		},
	})
	if err != nil || len(res.Content) == 0 {
		t.Fatalf("call get_node_detail failed: %v", err)
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "SNMP CPU") {
		t.Errorf("expected polling in detail, got: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	// 4. Call get_active_alerts
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_active_alerts",
	})
	if err != nil || len(res.Content) == 0 {
		t.Fatalf("call get_active_alerts failed: %v", err)
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "High CPU") {
		t.Errorf("expected alert in list, got: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	// 5. Call query_logs
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "query_logs",
		Arguments: map[string]any{
			"type":   "syslog",
			"filter": "",
			"limit":  10,
		},
	})
	if err != nil || len(res.Content) == 0 {
		t.Fatalf("call query_logs failed: %v", err)
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "fan failure") {
		t.Errorf("expected log in query_logs, got: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	// 6. Test ServeHTTP endpoint
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	mcpSrv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}
