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

	"github.com/twsnmp/twsnmpneo/backend/internal/ai/tensai"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// DefaultLLMTimeout defines the default HTTP timeout for external/local LLM requests.
// Set to 15 minutes to accommodate local models (e.g. Ollama, tensai) processing long prompts or heavy inference.
const DefaultLLMTimeout = 15 * time.Minute

// LLMClient provides a unified abstraction across Gemini, OpenAI, Claude, Ollama, and local models.
type LLMClient struct {
	provider             string
	model                string
	apiKey               string
	baseURL              string
	dataDir              string
	httpClient           *http.Client
	geminiMaxRetries     int
	geminiRetryBaseDelay time.Duration
}

// NewLLMClient creates an LLM client from MapConf settings and optional dataDir.
func NewLLMClient(conf *datastore.MapConfEnt, dataDirs ...string) *LLMClient {
	dDir := "./data"
	if len(dataDirs) > 0 && dataDirs[0] != "" {
		dDir = dataDirs[0]
	}
	if conf == nil {
		return &LLMClient{
			dataDir:              dDir,
			httpClient:           &http.Client{Timeout: DefaultLLMTimeout},
			geminiMaxRetries:     defaultGeminiMaxRetries,
			geminiRetryBaseDelay: defaultGeminiRetryBaseDelay,
		}
	}
	return &LLMClient{
		provider:             conf.LLMProvider,
		model:                conf.LLMModel,
		apiKey:               conf.LLMAPIKey,
		baseURL:              conf.LLMBaseURL,
		dataDir:              dDir,
		httpClient:           &http.Client{Timeout: DefaultLLMTimeout},
		geminiMaxRetries:     defaultGeminiMaxRetries,
		geminiRetryBaseDelay: defaultGeminiRetryBaseDelay,
	}
}

// SetHTTPClient sets a custom HTTP client (primarily for testing).
func (c *LLMClient) SetHTTPClient(client *http.Client) {
	if client != nil {
		c.httpClient = client
	}
}

// SetGeminiRetryBaseDelay sets the base delay for Gemini retries (primarily for testing).
func (c *LLMClient) SetGeminiRetryBaseDelay(delay time.Duration) {
	c.geminiRetryBaseDelay = delay
}

// SetTimeout sets the HTTP client timeout for LLM requests.
func (c *LLMClient) SetTimeout(timeout time.Duration) {
	if c.httpClient != nil {
		c.httpClient.Timeout = timeout
	}
}

// GetTimeout returns the HTTP client timeout for LLM requests.
func (c *LLMClient) GetTimeout() time.Duration {
	if c.httpClient != nil {
		return c.httpClient.Timeout
	}
	return 0
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
		modelName := c.model
		if modelName == "" {
			modelName = "qwen2.5-0.5b"
		}
		dataDir := c.dataDir
		if dataDir == "" {
			dataDir = "./data"
		}
		mgr := GetDownloadManager(dataDir)
		modelPath, err := mgr.FindModel(modelName)
		if err != nil || modelPath == "" {
			return "", fmt.Errorf("local model not found (%s): %w", modelName, err)
		}
		tLLM, err := tensai.NewWithOptions(modelPath, false)
		if err != nil {
			return "", fmt.Errorf("failed to load local model (%s): %w", modelName, err)
		}
		ans, genErr := tLLM.GenerateAnswer(ctx, systemPrompt, userPrompt, 2048)
		if genErr != nil {
			return "", fmt.Errorf("local model inference failed: %w", genErr)
		}
		return ans, nil
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
		Response   string `json:"response"`
		Thinking   string `json:"thinking"`
		Error      string `json:"error"`
		DoneReason string `json:"done_reason"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.Error != "" {
		return "", fmt.Errorf("ollama error: %s", res.Error)
	}
	ans := strings.TrimSpace(res.Response)
	if ans == "" {
		if strings.TrimSpace(res.Thinking) != "" {
			return "", fmt.Errorf("ollama response is empty (thinking ended with reason: %s)", res.DoneReason)
		}
		return "", fmt.Errorf("empty response from ollama")
	}
	return ans, nil
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
		"max_tokens": 2048,
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
func (c *LLMClient) DiagnoseAlert(ctx context.Context, alertEvent, nodeContext string, lang ...string) (string, error) {
	isJa := false
	if len(lang) > 0 && strings.HasPrefix(lang[0], "ja") {
		isJa = true
	}
	system := "You are a network reliability engineer and security expert. Analyze the following network alert and provide concise root-cause inference and remediation steps in English using markdown format (with tables, bold, bullet points)."
	if isJa {
		system = "あなたはネットワーク信頼性エンジニアおよびセキュリティの専門家です。提示されたネットワークアラートを分析し、根本原因の推定と推奨される復旧・対策手順を日本語で簡潔に回答してください。必ずマークダウン形式（テーブル、太字、箇条書き）でわかりやすく回答してください。"
	}
	user := fmt.Sprintf("Alert Event: %s\nNode Details: %s", alertEvent, nodeContext)
	return c.GenerateAnswer(ctx, system, user)
}
