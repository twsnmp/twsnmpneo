package polling

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
	"github.com/twsnmp/twsnmpneo/backend/internal/polling/templates"
	"github.com/vjeantet/grok"
)

// PollingTemplateEnt defines a reusable monitoring template.
type PollingTemplateEnt = templates.PollingTemplateEnt

// LoadTemplates loads embedded polling templates for the specified language.
func LoadTemplates(lang string) ([]*PollingTemplateEnt, error) {
	return templates.LoadTemplates(lang)
}

// GetTemplate returns a template by ID.
func GetTemplate(lang string, id int) (*PollingTemplateEnt, error) {
	list, err := LoadTemplates(lang)
	if err != nil {
		return nil, err
	}
	if id > 0 && id <= len(list) {
		return list[id-1], nil
	}
	return nil, fmt.Errorf("template ID %d not found", id)
}

// GenerateAutoPollings creates one or multiple PollingEnts from a template.
// If AutoParam == "ifIndex", it walks the node's SNMP interfaces and creates pollings per interface.
func GenerateAutoPollings(ctx context.Context, node *datastore.NodeEnt, pt *PollingTemplateEnt) []*datastore.PollingEnt {
	if pt == nil {
		return nil
	}

	if node != nil && pt.Type == "snmp" && pt.AutoParam == "ifIndex" && node.IP != "" {
		ifPolls := getAutoSnmpIFPollings(node, pt)
		if len(ifPolls) > 0 {
			return ifPolls
		}
	}

	// Default: single polling from template
	nodeID := ""
	if node != nil {
		nodeID = node.ID
	}
	p := &datastore.PollingEnt{
		Name:      pt.Name,
		NodeID:    nodeID,
		Type:      pt.Type,
		Mode:      pt.Mode,
		Params:    pt.Params,
		Filter:    pt.Filter,
		Extractor: pt.Extractor,
		Script:    pt.Script,
		Level:     pt.Level,
		PollInt:   60,
		Timeout:   1,
		Retry:     1,
		State:     StateUnknown,
	}
	if p.Type == "" {
		p.Type = "ping"
	}
	return []*datastore.PollingEnt{p}
}

// getAutoSnmpIFPollings discovers network interfaces via SNMP walk and generates per-interface pollings.
func getAutoSnmpIFPollings(node *datastore.NodeEnt, pt *PollingTemplateEnt) []*datastore.PollingEnt {
	var ret []*datastore.PollingEnt
	agent := BuildSNMPAgent(node, 2, 1)
	if err := agent.Connect(); err != nil {
		return nil
	}
	defer agent.Conn.Close()

	ifMap := make(map[string]string)
	// Walk ifType
	_ = agent.Walk(mib.NameToOID("ifType"), func(v gosnmp.SnmpPDU) error {
		name := mib.OIDToName(v.Name)
		parts := strings.Split(name, ".")
		if len(parts) == 2 && parts[0] == "ifType" {
			val := gosnmp.ToBigInt(v.Value).Int64()
			// ifType 6 = ethernetCsmacd, 24 = softwareLoopback, etc.
			if val == 6 || val == 117 || val == 71 {
				ifMap[parts[1]] = fmt.Sprintf("#%s", parts[1])
			}
		}
		return nil
	})

	// Walk ifName / ifDescr to enrich display names
	_ = agent.Walk(mib.NameToOID("ifName"), func(v gosnmp.SnmpPDU) error {
		name := mib.OIDToName(v.Name)
		parts := strings.Split(name, ".")
		if len(parts) == 2 {
			if _, ok := ifMap[parts[1]]; ok {
				valStr := mib.GetMIBValueString(parts[0], &v, false)
				if valStr == "" {
					valStr = formatSNMPValue(v)
				}
				if valStr != "" {
					ifMap[parts[1]] = fmt.Sprintf("%s(%s)", valStr, parts[1])
				}
			}
		}
		return nil
	})

	// Fallback to ifDescr if ifName was empty
	if len(ifMap) == 0 {
		_ = agent.Walk(mib.NameToOID("ifDescr"), func(v gosnmp.SnmpPDU) error {
			name := mib.OIDToName(v.Name)
			parts := strings.Split(name, ".")
			if len(parts) == 2 {
				valStr := formatSNMPValue(v)
				if valStr != "" {
					ifMap[parts[1]] = fmt.Sprintf("%s(%s)", valStr, parts[1])
				}
			}
			return nil
		})
	}

	for idx, ifLabel := range ifMap {
		p := &datastore.PollingEnt{
			Name:      fmt.Sprintf("%s : %s", pt.Name, ifLabel),
			NodeID:    node.ID,
			Type:      pt.Type,
			Mode:      pt.Mode,
			Params:    strings.ReplaceAll(pt.Params, "$i", idx),
			Filter:    pt.Filter,
			Extractor: pt.Extractor,
			Script:    pt.Script,
			Level:     pt.Level,
			PollInt:   60,
			Timeout:   1,
			Retry:     1,
			State:     StateUnknown,
		}
		ret = append(ret, p)
	}
	return ret
}

var commonGrokList = []string{
	`Login %{NOTSPACE:stat}: \[(host/)*%{USER:user}\].+cli %{MAC:client}`,
	`Login %{NOTSPACE:stat}: \[.+\] %{USER:user}`,
	`mac=%{MAC:mac}.+ip=%{IP:ip}`,
	`ip=%{IP:ip}.+mac=%{MAC:mac}`,
	`src=%{IP:src}:%{BASE10NUM:sport}:.+dst=%{IP:dst}:%{BASE10NUM:dport}:.+proto=%{WORD:prot}.+sent=%{BASE10NUM:sent}.+rcvd=%{BASE10NUM:rcvd}.+spkt=%{BASE10NUM:spkt}.+rpkt=%{BASE10NUM:rpkt}`,
	`load average: %{BASE10NUM:load1m}, %{BASE10NUM:load5m}, %{BASE10NUM:load15m}`,
	`Mem:\s+%{BASE10NUM:total}\s+%{BASE10NUM:used}\s+%{BASE10NUM:free}\s+%{BASE10NUM:share}\s+%{BASE10NUM:buffer}\s+%{BASE10NUM:available}`,
	`%{NOTSPACE:stat} (password|publickey) for( invalid user | )%{USER:user} from %{IP:client}`,
}

// AutoGrok analyzes test log data and suggests a matching Grok pattern.
func AutoGrok(testData string) string {
	maxCount := 0
	hit := ""

	for _, pat := range commonGrokList {
		c := checkGrok(pat, testData)
		if c > maxCount {
			maxCount = c
			hit = pat
		}
	}
	if hit != "" {
		return hit
	}
	return autoGenGrok(testData)
}

func checkGrok(pat, td string) int {
	config := grok.Config{
		Patterns:          make(map[string]string),
		NamedCapturesOnly: true,
	}
	config.Patterns["TWSNMP"] = pat
	g, err := grok.NewWithConfig(&config)
	if err != nil {
		return 0
	}
	values, err := g.Parse("%{TWSNMP}", td)
	if err != nil {
		return 0
	}
	return len(values)
}

func autoGenGrok(td string) string {
	reg := regexp.MustCompile(`\s+([a-zA-Z_]+[a-zA-Z0-9_]+)=([^ ]+)`)
	regNum := regexp.MustCompile(`^\d+(\.\d+)?$`)
	td = " " + td
	matches := reg.FindAllStringSubmatch(td, -1)
	if len(matches) == 0 {
		return ""
	}

	res := td
	for _, m := range matches {
		if len(m) > 2 {
			k := m[1]
			v := m[2]
			var rep string
			if regNum.MatchString(v) {
				rep = fmt.Sprintf("%s=%%{NUMBER:%s}", k, k)
			} else {
				rep = fmt.Sprintf("%s=%%{WORD:%s}", k, k)
			}
			res = strings.Replace(res, m[0], " "+rep, 1)
		}
	}
	return strings.TrimSpace(res)
}
