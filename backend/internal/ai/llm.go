package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// LLMClient provides a unified abstraction across Gemini, OpenAI, Claude, Ollama, and local models.
type LLMClient struct {
	provider   string
	model      string
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewLLMClient creates an LLM client from MapConf settings.
func NewLLMClient(conf *datastore.MapConfEnt) *LLMClient {
	if conf == nil {
		return &LLMClient{httpClient: &http.Client{Timeout: 30 * time.Second}}
	}
	return &LLMClient{
		provider:   conf.LLMProvider,
		model:      conf.LLMModel,
		apiKey:     conf.LLMAPIKey,
		baseURL:    conf.LLMBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// GenerateAnswer sends a prompt with system context to the configured LLM.
func (c *LLMClient) GenerateAnswer(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.provider == "" {
		return "", fmt.Errorf("llm provider not configured")
	}

	switch strings.ToLower(c.provider) {
	case "ollama":
		return c.generateOllama(ctx, systemPrompt, userPrompt)
	case "openai":
		return c.generateOpenAI(ctx, systemPrompt, userPrompt)
	case "gemini", "googleai":
		return c.generateGemini(ctx, systemPrompt, userPrompt)
	case "claude", "anthropic":
		return c.generateClaude(ctx, systemPrompt, userPrompt)
	case "local", "tensai", "embedded":
		return fmt.Sprintf("[Local Model Response: %s] Analyzed: %s", c.model, userPrompt), nil
	default:
		return "", fmt.Errorf("unsupported llm provider: %s", c.provider)
	}
}

func (c *LLMClient) generateOllama(ctx context.Context, system, prompt string) (string, error) {
	url := c.baseURL
	if url == "" {
		url = "http://localhost:11434"
	}
	model := c.model
	if model == "" {
		model = "llama3"
	}

	payload := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"system": system,
		"stream": false,
	}
	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/api/generate", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	var res struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.Error != "" {
		return "", fmt.Errorf("ollama error: %s", res.Error)
	}
	return res.Response, nil
}

func (c *LLMClient) generateOpenAI(ctx context.Context, system, prompt string) (string, error) {
	url := c.baseURL
	if url == "" {
		url = "https://api.openai.com/v1"
	}
	model := c.model
	if model == "" {
		model = "gpt-4o-mini"
	}

	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": prompt},
		},
	}
	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request failed: %w", err)
	}
	defer resp.Body.Close()

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.Error.Message != "" {
		return "", fmt.Errorf("openai error: %s", res.Error.Message)
	}
	if len(res.Choices) == 0 {
		return "", fmt.Errorf("empty response from openai")
	}
	return res.Choices[0].Message.Content, nil
}

func (c *LLMClient) generateGemini(ctx context.Context, system, prompt string) (string, error) {
	model := c.model
	if model == "" {
		model = "gemini-1.5-flash"
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, c.apiKey)
	if c.baseURL != "" {
		url = fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, model, c.apiKey)
	}

	payload := map[string]interface{}{
		"system_instruction": map[string]interface{}{
			"parts": []map[string]string{{"text": system}},
		},
		"contents": []map[string]interface{}{
			{
				"role":  "user",
				"parts": []map[string]string{{"text": prompt}},
			},
		},
	}
	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("gemini api error (status %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}
	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty gemini response")
	}
	return res.Candidates[0].Content.Parts[0].Text, nil
}

func (c *LLMClient) generateClaude(ctx context.Context, system, prompt string) (string, error) {
	model := c.model
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}
	url := "https://api.anthropic.com/v1/messages"
	if c.baseURL != "" {
		url = c.baseURL + "/messages"
	}

	payload := map[string]interface{}{
		"model":      model,
		"max_tokens": 1024,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("claude api error: %s", string(body))
	}

	var res struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}
	if len(res.Content) == 0 {
		return "", fmt.Errorf("empty claude response")
	}
	return res.Content[0].Text, nil
}

// DiagnoseAlert provides AI-powered root-cause inference and remediation tips.
func (c *LLMClient) DiagnoseAlert(ctx context.Context, alertEvent, nodeContext string) (string, error) {
	system := "You are a network reliability engineer and security expert. Analyze the following network alert and provide concise root-cause inference and remediation steps."
	user := fmt.Sprintf("Alert Event: %s\nNode Details: %s", alertEvent, nodeContext)
	return c.GenerateAnswer(ctx, system, user)
}
