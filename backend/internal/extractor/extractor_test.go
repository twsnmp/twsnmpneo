package extractor_test

import (
	"testing"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/extractor"
)

func TestApplyExtractor_GetBody(t *testing.T) {
	vm := otto.New()
	fields := make(map[string]interface{})
	err := extractor.ApplyExtractor("getBody", "hello world", vm, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, err := vm.Run("getBody() === 'hello world'")
	if err != nil {
		t.Fatalf("script run error: %v", err)
	}
	pass, _ := val.ToBoolean()
	if !pass {
		t.Fatalf("expected getBody to return 'hello world'")
	}
}

func TestApplyExtractor_JSONPath(t *testing.T) {
	vm := otto.New()
	fields := make(map[string]interface{})
	jsonStr := `{"data": {"count": 42, "status": "active"}}`
	err := extractor.ApplyExtractor("jsonpath", jsonStr, vm, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, err := vm.Run("jsonpath('$.data.count') === 42 && jsonpath('$.data.status') === 'active'")
	if err != nil {
		t.Fatalf("script run error: %v", err)
	}
	pass, _ := val.ToBoolean()
	if !pass {
		t.Fatalf("expected jsonpath match")
	}
}

func TestApplyExtractor_GoQuery(t *testing.T) {
	vm := otto.New()
	fields := make(map[string]interface{})
	htmlStr := `<html><body><h1 class="title">TWSNMP NEO</h1></body></html>`
	err := extractor.ApplyExtractor("goquery", htmlStr, vm, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, err := vm.Run("goquery('h1.title') === 'TWSNMP NEO'")
	if err != nil {
		t.Fatalf("script run error: %v", err)
	}
	pass, _ := val.ToBoolean()
	if !pass {
		t.Fatalf("expected goquery match")
	}
}

func TestApplyExtractor_Grok(t *testing.T) {
	vm := otto.New()
	fields := make(map[string]interface{})
	text := "CPU: 75% MEM: 80%"
	pattern := `CPU: %{NUMBER:cpu}% MEM: %{NUMBER:mem}%`
	err := extractor.ApplyExtractor(pattern, text, vm, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fields["cpu"] != "75" || fields["mem"] != "80" {
		t.Fatalf("unexpected extracted fields: %+v", fields)
	}

	val, err := vm.Run("cpu === '75' && mem === '80'")
	if err != nil {
		t.Fatalf("script run error: %v", err)
	}
	pass, _ := val.ToBoolean()
	if !pass {
		t.Fatalf("expected grok variables in VM")
	}
}
