package datastore

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

//go:embed conf/nodedetect.json
var defaultNodeDetectJSON []byte

// SensorPollingDef represents sensor polling configuration (CPU, memory, temp, fan, power).
type SensorPollingDef struct {
	Name   string `json:"Name"`
	Type   string `json:"Type"`
	Mode   string `json:"Mode"`
	Params string `json:"Params"`
	Script string `json:"Script"`
	Level  string `json:"Level"`
	Descr  string `json:"Descr"`
}

// NodeDetectRule represents a device/OS detection rule.
type NodeDetectRule struct {
	ID               string             `json:"ID"`
	Name             string             `json:"Name"`
	Category         string             `json:"Category"`
	SubType          string             `json:"SubType,omitempty"`
	Icon             string             `json:"Icon"`
	SysObjectIDs     []string           `json:"SysObjectIDs,omitempty"`
	SysDescrPattern  string             `json:"SysDescrPattern,omitempty"`
	HTTPTitlePattern string             `json:"HTTPTitlePattern,omitempty"`
	VendorPattern    string             `json:"VendorPattern,omitempty"`
	SensorPollings   []SensorPollingDef `json:"SensorPollings,omitempty"`

	sysDescrReg  *regexp.Regexp
	httpTitleReg *regexp.Regexp
	vendorReg    *regexp.Regexp
}

// DetectInput holds signatures collected from a node for classification.
type DetectInput struct {
	IP          string
	Name        string
	HostName    string
	SysObjectID string
	SysDescr    string
	HTTPTitle   string
	HTTPServer  string
	HTTPBody    string
	Vendor      string
	IsGateway   bool
}

// DetectResult contains the classified node type, icon, and recommended pollings.
type DetectResult struct {
	RuleID         string             `json:"RuleID"`
	Name           string             `json:"Name"`
	Category       string             `json:"Category"`
	SubType        string             `json:"SubType"`
	Icon           string             `json:"Icon"`
	Confidence     int                `json:"Confidence"`
	SensorPollings []SensorPollingDef `json:"SensorPollings"`
}

var (
	nodeDetectRules []*NodeDetectRule
	nodeDetectOnce  sync.Once
	nodeDetectLock  sync.RWMutex
)

// InitNodeDetectRules initializes detection rules from embedded JSON.
func InitNodeDetectRules() {
	nodeDetectOnce.Do(func() {
		if len(defaultNodeDetectJSON) > 0 {
			var rules []*NodeDetectRule
			if err := json.Unmarshal(defaultNodeDetectJSON, &rules); err == nil {
				compileRules(rules)
				nodeDetectRules = rules
			}
		}
	})
}

func compileRules(rules []*NodeDetectRule) {
	for _, rule := range rules {
		if rule.SysDescrPattern != "" {
			rule.sysDescrReg, _ = regexp.Compile(rule.SysDescrPattern)
		}
		if rule.HTTPTitlePattern != "" {
			rule.httpTitleReg, _ = regexp.Compile(rule.HTTPTitlePattern)
		}
		if rule.VendorPattern != "" {
			rule.vendorReg, _ = regexp.Compile(rule.VendorPattern)
		}
	}
}

func normalizeOID(oid string) string {
	oid = strings.TrimSpace(oid)
	if oid == "" {
		return ""
	}
	if !strings.HasPrefix(oid, ".") {
		oid = "." + oid
	}
	return strings.TrimRight(oid, ".")
}

// DetectNode identifies device category, OS, icon, and recommended pollings.
func DetectNode(input *DetectInput) *DetectResult {
	InitNodeDetectRules()

	nodeDetectLock.RLock()
	rules := nodeDetectRules
	nodeDetectLock.RUnlock()

	var bestRule *NodeDetectRule
	highestScore := 0

	normInputOID := normalizeOID(input.SysObjectID)

	for _, rule := range rules {
		score := 0

		// 1. sysObjectID check
		if normInputOID != "" && len(rule.SysObjectIDs) > 0 {
			for _, target := range rule.SysObjectIDs {
				normTarget := normalizeOID(target)
				if normTarget != "" && (normInputOID == normTarget || strings.HasPrefix(normInputOID, normTarget+".")) {
					score += 100 + len(normTarget)
					break
				}
			}
		}

		// 2. sysDescr / HostName / Name check
		if rule.sysDescrReg != nil {
			if input.SysDescr != "" && rule.sysDescrReg.MatchString(input.SysDescr) {
				score += 50
			} else if input.HostName != "" && rule.sysDescrReg.MatchString(input.HostName) {
				score += 40
			} else if input.Name != "" && rule.sysDescrReg.MatchString(input.Name) {
				score += 40
			}
		}

		// 3. HTTP check
		if rule.httpTitleReg != nil {
			matchedHTTP := false
			if input.HTTPTitle != "" && rule.httpTitleReg.MatchString(input.HTTPTitle) {
				matchedHTTP = true
			} else if input.HTTPServer != "" && rule.httpTitleReg.MatchString(input.HTTPServer) {
				matchedHTTP = true
			} else if input.HTTPBody != "" && rule.httpTitleReg.MatchString(input.HTTPBody) {
				matchedHTTP = true
			}
			if matchedHTTP {
				score += 30
			}
		}

		// 4. MAC Vendor check
		if input.Vendor != "" && rule.vendorReg != nil {
			if rule.vendorReg.MatchString(input.Vendor) {
				score += 30
				if normInputOID != "" && len(rule.SysObjectIDs) > 0 {
					score += 30
				}
			}
		}

		// 5. Gateway check (bonus for router rules)
		if input.IsGateway && rule.Category == "router" && score > 0 {
			score += 40
		}

		if score > highestScore {
			highestScore = score
			bestRule = rule
		}
	}

	if bestRule != nil && highestScore >= 20 {
		return &DetectResult{
			RuleID:         bestRule.ID,
			Name:           bestRule.Name,
			Category:       bestRule.Category,
			SubType:        bestRule.SubType,
			Icon:           bestRule.Icon,
			Confidence:     highestScore,
			SensorPollings: bestRule.SensorPollings,
		}
	}

	return nil
}
