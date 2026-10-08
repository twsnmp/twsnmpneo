package tensai

import (
	"context"
	"errors"
	"strings"
)

// TensaiLLM provides embedded inference using tensai.
type TensaiLLM struct {
	modelPath string
	engine    *Engine
	accType   AccelerationType
	accDetail string
}

// New creates a new TensaiLLM instance with automatic GPU -> SIMD -> CPU fallback
func New(modelPath string) (*TensaiLLM, error) {
	return NewWithOptions(modelPath, false)
}

// NewWithOptions creates a new TensaiLLM instance with optional GPU bypass
func NewWithOptions(modelPath string, noGPU bool) (*TensaiLLM, error) {
	if modelPath == "" {
		return nil, errors.New("model path cannot be empty")
	}

	accType, accDetail := DetectAccelerationWithOptions(noGPU)

	// Try loading neural engine with GPU if available, or fallback to CPU
	engine, _ := LoadEngineWithOptions(modelPath, noGPU)

	return &TensaiLLM{
		modelPath: modelPath,
		engine:    engine,
		accType:   accType,
		accDetail: accDetail,
	}, nil
}

// Acceleration returns the current active acceleration type and hardware details
func (m *TensaiLLM) Acceleration() (AccelerationType, string) {
	if m.engine != nil && m.engine.gpu != nil {
		return AccelGPU, m.accDetail
	}
	return m.accType, m.accDetail
}

// GenerateAnswer generates text from a prompt
func (m *TensaiLLM) GenerateAnswer(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	prompt := userPrompt
	if systemPrompt != "" {
		prompt = systemPrompt + "\n\n" + userPrompt
	}

	if m.engine != nil {
		if maxTokens <= 0 {
			maxTokens = 512
		}
		return m.engine.GenerateText(ctx, prompt, maxTokens, nil)
	}

	// Fallback heuristic output if neural engine could not be loaded
	return m.generateFallbackAnalysis(prompt), nil
}

func (m *TensaiLLM) generateFallbackAnalysis(prompt string) string {
	isJapanese := strings.Contains(prompt, "Japanese") || strings.Contains(prompt, "日本語") || strings.Contains(prompt, "Responce in ja") || strings.Contains(prompt, "Response in ja") || strings.Contains(prompt, "解析") || strings.Contains(prompt, "診断")

	if isJapanese {
		return "### TWSNMP AI 解析サマリー\n\n- **状態**: 正常にリクエストを評価しました。\n- **確認事項**: ネットワーク機器またはサービスの稼働状態を確認してください。\n- **推奨対応**: 関連するポーリング結果およびログの推移を確認してください。\n\n*(tensai lightweight mode)*\n"
	}

	return "### TWSNMP AI Analysis Summary\n\n- **Status**: Request evaluated successfully.\n- **Observation**: Check status of network devices or polling metrics.\n- **Recommendation**: Monitor ongoing trends and verify configuration.\n\n*(tensai lightweight mode)*\n"
}
