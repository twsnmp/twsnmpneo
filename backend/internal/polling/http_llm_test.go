package polling

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

type mockRoundTripper func(req *http.Request) (*http.Response, error)

func (m mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func TestHTTPPoller_LLMOpenAI(t *testing.T) {
	var capturedAuth string
	var capturedAccept string

	client := &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			capturedAuth = req.Header.Get("Authorization")
			capturedAccept = req.Header.Get("Accept")

			jsonBody := `{"object":"list","data":[{"id":"gpt-4o","object":"model"},{"id":"gpt-4o-mini","object":"model"}]}`
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewBufferString(jsonBody)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	poller := NewHTTPPollerWithClient(client)
	pe := &datastore.PollingEnt{
		ID:      "test-llm-openai",
		Type:    "http",
		Mode:    "llm/openai",
		Params:  "http://localhost:8000/v1/models",
		Filter:  "Bearer test-key-12345",
		Script:  "code == 200 && models == 2 && rtt_ms >= 0",
		Level:   "warn",
		PollInt: 60,
	}

	res, err := poller.Poll(context.Background(), pe, nil)
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if res.State != StateNormal {
		t.Errorf("expected StateNormal, got %s (fields: %+v)", res.State, res.Fields)
	}
	if capturedAuth != "Bearer test-key-12345" {
		t.Errorf("expected Authorization header 'Bearer test-key-12345', got '%s'", capturedAuth)
	}
	if capturedAccept != "application/json" {
		t.Errorf("expected Accept 'application/json', got '%s'", capturedAccept)
	}
	if res.Fields["models"] != float64(2) {
		t.Errorf("expected models=2, got %v", res.Fields["models"])
	}
}

func TestHTTPPoller_LLMOllama(t *testing.T) {
	var capturedAuth string

	client := &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			capturedAuth = req.Header.Get("Authorization")
			jsonBody := `{"models":[{"name":"llama3.1:latest"},{"name":"qwen2.5:latest"},{"name":"gemma2:latest"}]}`
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewBufferString(jsonBody)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	poller := NewHTTPPollerWithClient(client)
	pe := &datastore.PollingEnt{
		ID:      "test-llm-ollama",
		Type:    "http",
		Mode:    "llm/ollama",
		Params:  "http://localhost:11434/api/tags",
		Filter:  "sk-my-ollama-token",
		Script:  "code == 200 && models >= 3",
		Level:   "high",
		PollInt: 60,
	}

	res, err := poller.Poll(context.Background(), pe, nil)
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if res.State != StateNormal {
		t.Errorf("expected StateNormal, got %s (fields: %+v)", res.State, res.Fields)
	}
	if capturedAuth != "Bearer sk-my-ollama-token" {
		t.Errorf("expected Authorization 'Bearer sk-my-ollama-token', got '%s'", capturedAuth)
	}
	if res.Fields["models"] != float64(3) {
		t.Errorf("expected models=3, got %v", res.Fields["models"])
	}
}
