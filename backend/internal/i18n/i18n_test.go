package i18n

import (
	"sync"
	"testing"
)

// TestParseLanguage tests whether various locale formats are properly parsed into language codes.
func TestParseLanguage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "hyphen separated (ja-JP)", input: "ja-JP", expected: "ja"},
		{name: "underscore separated (ja_JP)", input: "ja_JP", expected: "ja"},
		{name: "language code only (ja)", input: "ja", expected: "ja"},
		{name: "uppercase mixed (EN-US)", input: "EN-US", expected: "en"},
		{name: "multiple hyphens (zh-Hant-TW)", input: "zh-Hant-TW", expected: "zh"},
		{name: "empty string fallback", input: "", expected: "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := parseLanguage(tt.input)
			if actual != tt.expected {
				t.Errorf("parseLanguage(%q) = %q, want %q", tt.input, actual, tt.expected)
			}
		})
	}
}

// TestSetAndGetLang tests setting and getting the language, including lowercase normalization.
func TestSetAndGetLang(t *testing.T) {
	// Restore original language setting after test completes
	originalLang := GetLang()
	t.Cleanup(func() {
		SetLang(originalLang)
	})

	SetLang("FR")
	if got := GetLang(); got != "fr" {
		t.Errorf("GetLang() = %q, want %q", got, "fr")
	}

	SetLang("ja-JP")
	if got := GetLang(); got != "ja-jp" {
		t.Errorf("GetLang() = %q, want %q", got, "ja-jp")
	}
}

// TestTrans verifies translation lookups and fallback behaviors.
func TestTrans(t *testing.T) {
	originalLang := GetLang()
	t.Cleanup(func() {
		SetLang(originalLang)
	})

	tests := []struct {
		name     string
		lang     string
		input    string
		expected string
	}{
		{
			name:     "translation exists in Japanese",
			lang:     "ja",
			input:    "Add",
			expected: "追加",
		},
		{
			name:     "unsupported language (returns original)",
			lang:     "fr",
			input:    "Add",
			expected: "Add",
		},
		{
			name:     "missing translation key (returns original)",
			lang:     "ja",
			input:    "Unknown Key",
			expected: "Unknown Key",
		},
		{
			name:     "default language (English)",
			lang:     "en",
			input:    "Add",
			expected: "Add",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLang(tt.lang)
			actual := Trans(tt.input)
			if actual != tt.expected {
				t.Errorf("Trans(%q) with lang=%q = %q, want %q", tt.input, tt.lang, actual, tt.expected)
			}
		})
	}
}

// TestConcurrentAccess verifies there are no data races during concurrent reads and writes.
func TestConcurrentAccess(t *testing.T) {
	originalLang := GetLang()
	t.Cleanup(func() {
		SetLang(originalLang)
	})

	var wg sync.WaitGroup
	workers := 50
	iterations := 100

	wg.Add(workers * 2)

	// Writer goroutines
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if idx%2 == 0 {
					SetLang("ja")
				} else {
					SetLang("en")
				}
			}
		}(i)
	}

	// Reader goroutines
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = GetLang()
				_ = Trans("Downtime")
			}
		}()
	}

	wg.Wait()
}
