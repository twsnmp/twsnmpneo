package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newGeminiJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGenerateGemini_DefaultModelAndThoughtFiltering(t *testing.T) {
	var gotPath string
	var gotAPIKey string
	var gotQuery string
	var gotReq geminiGenerateContentRequest

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotPath = r.URL.Path
			gotQuery = r.URL.RawQuery
			gotAPIKey = r.Header.Get("x-goog-api-key")
			if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return newGeminiJSONResponse(http.StatusOK, `{
				"candidates": [{
					"content": {
						"role": "model",
						"parts": [
							{"text": "internal thinking...", "thought": true},
							{"text": "Node looks healthy."}
						]
					},
					"finishReason": "STOP"
				}]
			}`), nil
		}),
	}

	client := NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "gemini",
		LLMAPIKey:   " test-key ",
		LLMBaseURL:  "https://example.test",
	})
	client.SetHTTPClient(httpClient)

	ans, err := client.GenerateAnswer(context.Background(), "You are a network engineer.", "Diagnose router")
	if err != nil {
		t.Fatalf("GenerateAnswer: %v", err)
	}

	if gotPath != "/v1beta/models/"+DefaultGeminiModel+":generateContent" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotQuery != "" {
		t.Fatalf("expected empty query string, got: %s", gotQuery)
	}
	if gotAPIKey != "test-key" {
		t.Fatalf("unexpected api key header: %q", gotAPIKey)
	}
	if gotReq.SystemInstruction == nil || len(gotReq.SystemInstruction.Parts) != 1 {
		t.Fatalf("expected systemInstruction, got %+v", gotReq.SystemInstruction)
	}
	if ans != "Node looks healthy." {
		t.Fatalf("unexpected response: %q", ans)
	}
}

func TestGenerateGemini_GemmaSystemFallbackAndModelPrefix(t *testing.T) {
	var gotPath string
	var gotReq geminiGenerateContentRequest

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotPath = r.URL.Path
			if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return newGeminiJSONResponse(http.StatusOK, `{
				"candidates": [{
					"content": {"role": "model", "parts": [{"text": "OK"}]},
					"finishReason": "STOP"
				}]
			}`), nil
		}),
	}

	client := NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "gemini",
		LLMAPIKey:   "test-key",
		LLMBaseURL:  "https://example.test/v1beta/",
		LLMModel:    "models/gemma-4-26b-a4b-it",
	})
	client.SetHTTPClient(httpClient)

	ans, err := client.GenerateAnswer(context.Background(), "System prompt", "User prompt")
	if err != nil {
		t.Fatalf("GenerateAnswer: %v", err)
	}

	if gotPath != "/v1beta/models/gemma-4-26b-a4b-it:generateContent" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotReq.SystemInstruction != nil {
		t.Fatalf("expected systemInstruction to be nil for gemma, got %+v", gotReq.SystemInstruction)
	}
	if len(gotReq.Contents) != 1 || len(gotReq.Contents[0].Parts) != 2 {
		t.Fatalf("expected merged system+user parts, got %+v", gotReq.Contents)
	}
	if ans != "OK" {
		t.Fatalf("unexpected response: %q", ans)
	}
}

func TestGenerateGemini_APIErrorAndRetries(t *testing.T) {
	attempts := 0
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			return newGeminiJSONResponse(http.StatusServiceUnavailable, `{"error":{"code":503,"message":"model overloaded","status":"UNAVAILABLE"}}`), nil
		}),
	}

	client := NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "gemini",
		LLMAPIKey:   "test-key",
		LLMBaseURL:  "https://example.test",
	})
	client.SetHTTPClient(httpClient)
	client.SetGeminiRetryBaseDelay(0)

	_, err := client.GenerateAnswer(context.Background(), "", "hello")
	if err == nil || !strings.Contains(err.Error(), "503") || !strings.Contains(err.Error(), "model overloaded") {
		t.Fatalf("expected formatted 503 error, got: %v", err)
	}
	if attempts != defaultGeminiMaxRetries+1 {
		t.Fatalf("expected %d attempts, got %d", defaultGeminiMaxRetries+1, attempts)
	}
}

func TestGenerateGemini_RetrySucceeds(t *testing.T) {
	attempts := 0
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return newGeminiJSONResponse(http.StatusServiceUnavailable, `{"error":{"code":503,"message":"high demand","status":"UNAVAILABLE"}}`), nil
			}
			return newGeminiJSONResponse(http.StatusOK, `{
				"candidates": [{
					"content": {"role": "model", "parts": [{"text": "Recovered"}]},
					"finishReason": "STOP"
				}]
			}`), nil
		}),
	}

	client := NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "gemini",
		LLMAPIKey:   "test-key",
		LLMBaseURL:  "https://example.test",
	})
	client.SetHTTPClient(httpClient)
	client.SetGeminiRetryBaseDelay(0)

	ans, err := client.GenerateAnswer(context.Background(), "", "hello")
	if err != nil {
		t.Fatalf("GenerateAnswer: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if ans != "Recovered" {
		t.Fatalf("unexpected response: %q", ans)
	}
}

func TestGenerateGemini_NoRetryOn404(t *testing.T) {
	attempts := 0
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			return newGeminiJSONResponse(http.StatusNotFound, `{"error":{"code":404,"message":"not found","status":"NOT_FOUND"}}`), nil
		}),
	}

	client := NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "gemini",
		LLMAPIKey:   "test-key",
		LLMBaseURL:  "https://example.test",
	})
	client.SetHTTPClient(httpClient)
	client.SetGeminiRetryBaseDelay(0)

	_, err := client.GenerateAnswer(context.Background(), "", "hello")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404 error, got: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt without retry, got %d", attempts)
	}
}

func TestGenerateGemini_EmptyAPIKey(t *testing.T) {
	client := NewLLMClient(&datastore.MapConfEnt{
		LLMProvider: "gemini",
		LLMAPIKey:   "   ",
	})
	_, err := client.GenerateAnswer(context.Background(), "", "hello")
	if err == nil || !strings.Contains(err.Error(), "API key is empty") {
		t.Fatalf("expected empty API key error, got: %v", err)
	}
}

func TestBuildGeminiEndpoint_IgnoresDefaultOllamaURL(t *testing.T) {
	got := buildGeminiEndpoint("http://localhost:11434", "gemini-3.5-flash-lite")
	want := "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.5-flash-lite:generateContent"
	if got != want {
		t.Fatalf("buildGeminiEndpoint() = %q, want %q", got, want)
	}
}
