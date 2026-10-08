package tensai

import (
	"context"
	"testing"
)

func TestDetectAcceleration(t *testing.T) {
	accType, detail := DetectAcceleration()
	if accType == "" {
		t.Errorf("expected non-empty acceleration type")
	}
	t.Logf("Detected acceleration: %s (%s)", accType, detail)
}

func TestTensaiFallback(t *testing.T) {
	llm, err := New("non_existent_model.gguf")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	ans, err := llm.GenerateAnswer(context.Background(), "", "Ping failed on 192.168.1.1", 512)
	if err != nil {
		t.Fatalf("failed to generate fallback answer: %v", err)
	}
	if ans == "" {
		t.Errorf("expected non-empty answer")
	}
	t.Logf("Generated fallback response:\n%s", ans)
}
