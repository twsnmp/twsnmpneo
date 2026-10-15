package tensai

import (
	"context"
	"errors"
	"fmt"
	"log"
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

	// Load neural engine with GPU if available, or fallback to CPU
	engine, err := LoadEngineWithOptions(modelPath, noGPU)
	if err != nil {
		log.Printf("TensaiLLM: LoadEngineWithOptions failed (model=%s): %v", modelPath, err)
		return nil, fmt.Errorf("failed to load local LLM model %q: %w", modelPath, err)
	}

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
	if m.engine == nil {
		err := errors.New("tensai LLM engine is not initialized")
		log.Printf("TensaiLLM: GenerateAnswer error: %v", err)
		return "", err
	}

	prompt := userPrompt
	if systemPrompt != "" {
		prompt = systemPrompt + "\n\n" + userPrompt
	}

	if maxTokens <= 0 {
		maxTokens = 512
	}
	responseContent, err := m.engine.GenerateText(ctx, prompt, maxTokens, nil)
	if err != nil {
		log.Printf("TensaiLLM: GenerateText error: %v", err)
		return "", fmt.Errorf("tensai inference error: %w", err)
	}

	return responseContent, nil
}
