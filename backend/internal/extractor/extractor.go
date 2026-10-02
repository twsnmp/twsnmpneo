package extractor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PaesslerAG/jsonpath"
	"github.com/PuerkitoBio/goquery"
	"github.com/robertkrimen/otto"
	"github.com/vjeantet/grok"
)

// ApplyExtractor sets up helper functions (getBody, jsonpath, goquery) on an Otto VM,
// or parses named grok pattern captures into fields and VM variables.
func ApplyExtractor(extractor string, text string, vm *otto.Otto, fields map[string]interface{}) error {
	if extractor == "" {
		return nil
	}

	switch extractor {
	case "getBody":
		if vm != nil {
			_ = vm.Set("getBody", func(call otto.FunctionCall) otto.Value {
				if r, err := otto.ToValue(text); err == nil {
					return r
				}
				return otto.UndefinedValue()
			})
		}
		return nil

	case "goquery":
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(text))
		if err != nil {
			return fmt.Errorf("goquery parse error: %w", err)
		}
		if vm != nil {
			_ = vm.Set("goquery", func(call otto.FunctionCall) otto.Value {
				if call.Argument(0).IsString() {
					sel := call.Argument(0).String()
					if ov, err := otto.ToValue(doc.Find(sel).Text()); err == nil {
						return ov
					}
				}
				return otto.UndefinedValue()
			})
		}
		return nil

	case "jsonpath":
		var parsed interface{}
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			return fmt.Errorf("jsonpath unmarshal error: %w", err)
		}
		if vm != nil {
			_ = vm.Set("jsonpath", func(call otto.FunctionCall) otto.Value {
				if call.Argument(0).IsString() {
					sel := call.Argument(0).String()
					if v, err := jsonpath.Get(sel, parsed); err == nil {
						if ov, err := otto.ToValue(v); err == nil {
							return ov
						}
					}
				}
				return otto.UndefinedValue()
			})
		}
		return nil

	default:
		// Custom grok pattern
		g, err := grok.NewWithConfig(&grok.Config{NamedCapturesOnly: true})
		if err != nil {
			return fmt.Errorf("grok init error: %w", err)
		}
		if err := g.AddPattern("TWSNMP", extractor); err != nil {
			return fmt.Errorf("grok pattern error: %w", err)
		}
		values, err := g.Parse("%{TWSNMP}", text)
		if err != nil {
			return fmt.Errorf("grok parse error: %w", err)
		}
		for k, v := range values {
			if fields != nil {
				fields[k] = v
			}
			if vm != nil {
				_ = vm.Set(k, v)
			}
		}
		return nil
	}
}

// RegisterBodyHelpers registers getBody, jsonpath, and goquery helpers on Otto VM for string payloads.
func RegisterBodyHelpers(text string, vm *otto.Otto) {
	if vm == nil {
		return
	}
	_ = vm.Set("getBody", func(call otto.FunctionCall) otto.Value {
		if r, err := otto.ToValue(text); err == nil {
			return r
		}
		return otto.UndefinedValue()
	})
	_ = vm.Set("jsonpath", func(call otto.FunctionCall) otto.Value {
		if call.Argument(0).IsString() {
			sel := call.Argument(0).String()
			var parsed interface{}
			if err := json.Unmarshal([]byte(text), &parsed); err == nil {
				if v, err := jsonpath.Get(sel, parsed); err == nil {
					if ov, err := otto.ToValue(v); err == nil {
						return ov
					}
				}
			}
		}
		return otto.UndefinedValue()
	})
	_ = vm.Set("goquery", func(call otto.FunctionCall) otto.Value {
		if call.Argument(0).IsString() {
			sel := call.Argument(0).String()
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(text))
			if err == nil {
				if ov, err := otto.ToValue(doc.Find(sel).Text()); err == nil {
					return ov
				}
			}
		}
		return otto.UndefinedValue()
	})
}
