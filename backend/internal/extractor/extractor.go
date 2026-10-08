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
		// Grok pattern matching
		g, err := grok.NewWithConfig(&grok.Config{NamedCapturesOnly: true})
		if err != nil {
			return fmt.Errorf("grok init error: %w", err)
		}
		pattern := extractor
		if pat, ok := defaultGrokPatterns[extractor]; ok {
			pattern = pat
		}
		if err := g.AddPattern("TWSNMP", pattern); err != nil {
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

var defaultGrokPatterns = map[string]string{
	"DEVICE":         `mac=%{MAC:mac}.+ip=%{IP:ip}`,
	"DEVICER":        `ip=%{IP:ip}.+mac=%{MAC:mac}`,
	"WELFFLOW":       `src=%{IP:src}:%{BASE10NUM:sport}:.+dst=%{IP:dst}:%{BASE10NUM:dport}:.+proto=%{WORD:prot}.+sent=%{BASE10NUM:sent}.+rcvd=%{BASE10NUM:rcvd}.+spkt=%{BASE10NUM:spkt}.+rpkt=%{BASE10NUM:rpkt}`,
	"OPENWEATHER":    `"weather":.+"main":\s*"%{WORD:weather}".+"main":.+"temp":\s*%{BASE10NUM:temp}.+"feels_like":\s*%{BASE10NUM:feels_like}.+"temp_min":\s*%{BASE10NUM:temp_min}.+"temp_max":\s*%{BASE10NUM:temp_max}.+"pressure":\s*%{BASE10NUM:pressure}.+"humidity":\s*%{BASE10NUM:humidity}.+"wind":\s*{"speed":\s*%{BASE10NUM:wind}`,
	"UPTIME":         `load average: %{BASE10NUM:load1m}, %{BASE10NUM:load5m}, %{BASE10NUM:load15m}`,
	"SSHLOGIN":       `%{NOTSPACE:stat} (password|publickey) for( invalid user | )%{USER:user} from %{IP:client}`,
	"TWPCAP_STATS":   `type=Stats,total=%{BASE10NUM:total},count=%{BASE10NUM:count},ps=%{BASE10NUM:ps}`,
	"TWPCAP_IPTOMAC": `type=IPToMAC,ip=%{IP:ip},mac=%{MAC:mac},count=%{BASE10NUM:count},change=%{BASE10NUM:chnage},dhcp=%{BASE10NUM:dhcp}`,
	"TWPCAP_DNS":     `type=DNS,sv=%{IP:sv},DNSType=%{WORD:dnsType},Name=%{IPORHOST:name},count=%{BASE10NUM:count},change=%{BASE10NUM:chnage},lcl=%{IP:lastIP},lMAC=%{MAC:lastMAC}`,
	"TWPCAP_DHCP":    `type=DHCP,sv=%{IP:sv},count=%{BASE10NUM:count},offer=%{BASE10NUM:offer},ack=%{BASE10NUM:ack},nak=%{BASE10NUM:nak}`,
	"TWPCAP_NTP":     `type=NTP,sv=%{IP:sv},count=%{BASE10NUM:count},change=%{BASE10NUM:change},lcl=%{IP:client},version=%{BASE10NUM:version},stratum=%{BASE10NUM:stratum},refid=%{WORD:refid}`,
	"TWPCAP_RADIUS":  `type=RADIUS,cl=%{IP:client},sv=%{IP:server},count=%{BASE10NUM:count},req=%{BASE10NUM:request},accept=%{BASE10NUM:accept},reject=%{BASE10NUM:reject},challenge=%{BASE10NUM:challenge}`,
	"TWPCAP_TLSFlow": `type=TLSFlow,cl=%{IP:client},sv=%{IP:server},serv=%{WORD:service},count=%{BASE10NUM:count},handshake=%{BASE10NUM:handshake},alert=%{BASE10NUM:alert},minver=%{DATA:minver},maxver=%{DATA:maxver},cipher=%{DATA:cipher},ft=`,
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
