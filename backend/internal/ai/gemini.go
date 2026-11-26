package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultGeminiBaseURL is the default Gemini API endpoint base URL.
	DefaultGeminiBaseURL = "https://generativelanguage.googleapis.com"
	// DefaultGeminiModel is used when no Gemini model is explicitly configured.
	DefaultGeminiModel = "gemini-3.5-flash-lite"

	maxGeminiErrorBodyBytes     = 512
	defaultGeminiMaxRetries     = 3
	defaultGeminiRetryBaseDelay = 2 * time.Second
	maxGeminiRetryDelay         = 30 * time.Second
)

type geminiGenerateContentRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text    string `json:"text,omitempty"`
	Thought bool   `json:"thought,omitempty"`
}

type geminiGenerateContentResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Error      *geminiAPIError   `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content      *geminiContent `json:"content,omitempty"`
	FinishReason string         `json:"finishReason,omitempty"`
}

type geminiAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func (c *LLMClient) generateGemini(ctx context.Context, system, prompt string) (string, error) {
	apiKey := strings.TrimSpace(c.apiKey)
	if apiKey == "" {
		return "", errors.New("gemini API key is empty")
	}

	modelName := normalizeGeminiModelName(c.model)
	if modelName == "" {
		modelName = DefaultGeminiModel
	}

	endpoint := buildGeminiEndpoint(c.baseURL, modelName)
	reqBody, err := buildGeminiRequest(modelName, system, prompt)
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	maxRetries := c.geminiMaxRetries
	if maxRetries < 0 {
		maxRetries = defaultGeminiMaxRetries
	}
	baseDelay := c.geminiRetryBaseDelay

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		text, statusCode, retryAfter, err := c.doGeminiRequest(ctx, endpoint, apiKey, payload)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if attempt == maxRetries || !isGeminiRetryableStatus(statusCode) {
			break
		}
		delay := computeGeminiRetryDelay(baseDelay, attempt, retryAfter)
		if sleepErr := sleepWithContext(ctx, delay); sleepErr != nil {
			return "", lastErr
		}
	}
	return "", lastErr
}

func (c *LLMClient) doGeminiRequest(ctx context.Context, endpoint, apiKey string, payload []byte) (string, int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to create gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, "", fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, "", fmt.Errorf("failed to read gemini response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", resp.StatusCode, resp.Header.Get("Retry-After"), formatGeminiAPIError(resp.StatusCode, body)
	}

	var apiResp geminiGenerateContentResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", resp.StatusCode, "", fmt.Errorf("failed to decode gemini response: %w (body: %s)", err, truncateGeminiBody(body))
	}
	if apiResp.Error != nil {
		return "", apiResp.Error.Code, "", fmt.Errorf("gemini API error (%d %s): %s", apiResp.Error.Code, apiResp.Error.Status, apiResp.Error.Message)
	}
	if len(apiResp.Candidates) == 0 {
		return "", resp.StatusCode, "", errors.New("empty gemini response")
	}

	text := extractGeminiResponseText(&apiResp)
	if text == "" {
		return "", resp.StatusCode, "", errors.New("empty gemini response")
	}
	return text, resp.StatusCode, "", nil
}

func buildGeminiEndpoint(baseURL, modelName string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" || isDefaultOllamaBaseURL(trimmed) {
		trimmed = DefaultGeminiBaseURL
	}
	if strings.HasSuffix(trimmed, "/v1beta") {
		return fmt.Sprintf("%s/models/%s:generateContent", trimmed, modelName)
	}
	return fmt.Sprintf("%s/v1beta/models/%s:generateContent", trimmed, modelName)
}

func isDefaultOllamaBaseURL(baseURL string) bool {
	lower := strings.ToLower(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	return lower == "http://localhost:11434" || lower == "http://127.0.0.1:11434"
}

func normalizeGeminiModelName(model string) string {
	trimmed := strings.TrimSpace(model)
	return strings.TrimPrefix(trimmed, "models/")
}

func isGemmaModel(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(modelName), "gemma")
}

func buildGeminiRequest(modelName, system, prompt string) (*geminiGenerateContentRequest, error) {
	var parts []geminiPart
	if prompt != "" {
		parts = append(parts, geminiPart{Text: prompt})
	}

	req := &geminiGenerateContentRequest{}
	if system != "" {
		// Gemma models reject systemInstruction; prepend system text to the user message.
		if isGemmaModel(modelName) || len(parts) == 0 {
			if len(parts) == 0 {
				parts = []geminiPart{{Text: system}}
			} else {
				parts = append([]geminiPart{{Text: system + "\n\n"}}, parts...)
			}
		} else {
			req.SystemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: system}},
			}
		}
	}

	if len(parts) == 0 {
		return nil, errors.New("no messages to send to gemini")
	}
	req.Contents = []geminiContent{
		{
			Role:  "user",
			Parts: parts,
		},
	}
	return req, nil
}

func extractGeminiResponseText(apiResp *geminiGenerateContentResponse) string {
	if apiResp == nil || len(apiResp.Candidates) == 0 {
		return ""
	}
	cand := apiResp.Candidates[0]
	if cand.Content == nil {
		return ""
	}
	var answer strings.Builder
	var fallbackThought strings.Builder
	for _, p := range cand.Content.Parts {
		if p.Text == "" {
			continue
		}
		if p.Thought {
			fallbackThought.WriteString(p.Text)
			continue
		}
		answer.WriteString(p.Text)
	}
	if text := answer.String(); text != "" {
		return text
	}
	return fallbackThought.String()
}

func isGeminiRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func computeGeminiRetryDelay(baseDelay time.Duration, attempt int, retryAfter string) time.Duration {
	if baseDelay <= 0 {
		return 0
	}
	if sec, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && sec > 0 {
		d := time.Duration(sec) * time.Second
		if d > maxGeminiRetryDelay {
			return maxGeminiRetryDelay
		}
		return d
	}
	delay := baseDelay << attempt
	if delay > maxGeminiRetryDelay {
		delay = maxGeminiRetryDelay
	}
	// Add +/- 20% jitter to avoid synchronized retries.
	jitterRange := delay / 5
	if jitterRange > 0 {
		delay = delay - jitterRange + time.Duration(rand.Int64N(int64(jitterRange*2)+1))
	}
	return delay
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func formatGeminiAPIError(statusCode int, body []byte) error {
	var errResp struct {
		Error *geminiAPIError `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
		return fmt.Errorf("gemini API error %d (%s): %s", statusCode, errResp.Error.Status, errResp.Error.Message)
	}
	return fmt.Errorf("gemini API HTTP %d: %s", statusCode, truncateGeminiBody(body))
}

func truncateGeminiBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > maxGeminiErrorBodyBytes {
		return s[:maxGeminiErrorBodyBytes] + "..."
	}
	return s
}
