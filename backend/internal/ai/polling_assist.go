package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/polling/templates"
)

// AssistPollingResp is the JSON result structure for AI polling assist.
type AssistPollingResp struct {
	TemplateID int    `json:"template_id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Mode       string `json:"mode"`
	Params     string `json:"params"`
	Filter     string `json:"filter"`
	Extractor  string `json:"extractor"`
	Script     string `json:"script"`
	Level      string `json:"level"`
	PollInt    int    `json:"poll_int"`
	Timeout    int    `json:"timeout"`
	Retry      int    `json:"retry"`
	Advice     string `json:"advice"`
	Error      string `json:"error,omitempty"`
}

// PollingScriptResp is the response for generated polling scripts.
type PollingScriptResp struct {
	Script      string `json:"script"`
	Explanation string `json:"explanation"`
}

// AssistPolling recommends and configures the best polling settings based on natural language intent.
func (c *LLMClient) AssistPolling(ctx context.Context, store datastore.DataStore, nodeID, userPrompt, lang string) (*AssistPollingResp, error) {
	resp := &AssistPollingResp{
		Level:   "high",
		PollInt: 60,
		Timeout: 3,
		Retry:   1,
	}

	isJa := strings.HasPrefix(strings.ToLower(lang), "ja")

	// 1. Gather Templates
	tmplList, err := templates.LoadTemplates(lang)
	if err != nil {
		tmplList, _ = templates.LoadTemplates("ja")
	}

	var tmplSB strings.Builder
	for _, t := range tmplList {
		tmplSB.WriteString(fmt.Sprintf("- ID: %d, Name: %s, Type: %s, Mode: %s, Descr: %s, Params: %s, Filter: %s, Extractor: %s, Script: %s\n",
			t.ID, t.Name, t.Type, t.Mode, t.Descr, t.Params, t.Filter, t.Extractor, t.Script))
	}

	// 2. Gather Node Information
	var nodeSB strings.Builder
	if store != nil && nodeID != "" {
		if n, err := store.GetNode(ctx, nodeID); err == nil && n != nil {
			nodeSB.WriteString(fmt.Sprintf("Name: %s, IP: %s, Vendor: %s, Descr: %s, SnmpMode: %s\n",
				n.Name, n.IP, n.Vendor, n.Descr, n.SnmpMode))
		}
	}

	system := `You are an expert network monitoring engineer specialized in TWSNMP NEO.
Select the best polling template and native parameters based on the user's intent and target node context.

### CRITICAL TWSNMP SCRIPT & EVALUATION RULES:
1. **Script Condition Logic (CRITICAL)**:
   - In TWSNMP NEO, the 'Script' expression MUST evaluate to **true when NORMAL / OK**, and **false when ALARM / FAULT**.
   - Example (Ping RTT < 100ms): 'rtt < 100 * 1000 * 1000' (evaluates to true if normal). DO NOT write 'rtt >= 100'!
   - Example (HTTP Status 200): 'code == 200' (evaluates to true if status is 200).
   - Example (CPU < 90%): 'cpu < 90.0' or 'hrProcessorLoad < 90.0' (evaluates to true if CPU is normal).

2. **Variable Names & RTT Units (CRITICAL)**:
   - DO NOT prefix variables with 'stats.'. Use direct top-level variables: 'rtt', 'code', 'status', 'count', 'exitCode', etc.
   - 'rtt' is in **NANOSECONDS (ns)**! (1 ms = 1,000,000 ns).
   - 100ms is written as '100 * 1000 * 1000' or '100000000' ns.

3. **Native Polling Types**:
   - 'ping': Mode: "", "rtt", or "smoke" (Smokeping mode). For "smoke" mode, Params specifies 'count=N,size=BYTES' (e.g. 'count=10,size=64').
   - 'snmp': Mode: "get", "stats", etc. Params: OID or MIB name. Script: e.g. 'hrProcessorLoad < 90.0'.
   - 'http': Mode: "" or "status" or "https". Script: 'code == 200' or 'rtt < 2000 * 1000 * 1000'.
   - 'tls': Mode: "expire". Script: e.g. '30' (days remaining).
   - 'syslog' / 'trap': Filter: Regex pattern. Script: 'count == 0' (0 errors is normal).
   - 'mqtt': Mode: "subscribe". Params: MQTT Broker URL (e.g. "tcp://localhost:1883").
   - 'netflow': Mode: "traffic" or "count".
   - 'stun': Mode: "ipv4" or "ipv6".

4. **Output Format**:
You MUST return ONLY valid JSON matching this schema with NO markdown code fences or other prose:
{
  "template_id": <int template ID from list or 0 if custom>,
  "name": "<clear descriptive polling name>",
  "type": "<ping|snmp|http|tls|syslog|trap|mqtt|netflow|stun|cmd>",
  "mode": "<mode string>",
  "params": "<parameters string>",
  "filter": "<filter regex/expression or empty>",
  "extractor": "<extractor expression or empty>",
  "script": "<evaluation script where true is normal>",
  "level": "<high|low|warn|info>",
  "poll_int": <polling interval in seconds, default 60>,
  "timeout": <timeout in seconds, default 3>,
  "retry": <retry count, default 1>,
  "advice": "<concise explanation and recommendation in user's language>"
}`

	if isJa {
		system += "\n必ず日本語でadviceを記述してください。余計なマークダウンのトリプルバッククォートは含めず、純粋なJSONのみを出力してください。"
	} else {
		system += "\nPlease write the advice in English. Do not include markdown triple backticks, output strictly valid raw JSON."
	}

	userMsg := fmt.Sprintf("# Target Node Info:\n%s\n# User Intent:\n%s\n\n# Available Templates:\n%s",
		nodeSB.String(), userPrompt, tmplSB.String())

	rawAns, err := c.GenerateAnswer(ctx, system, userMsg)
	if err != nil {
		log.Printf("AssistPolling LLM error: %v", err)
		resp.Error = err.Error()
		return resp, nil
	}

	cleaned := cleanJSONOutput(rawAns)
	if err := json.Unmarshal([]byte(cleaned), resp); err != nil {
		log.Printf("AssistPolling JSON unmarshal failed: %v, raw: %s", err, rawAns)
		resp.Advice = rawAns
		resp.Name = "Custom Polling"
		resp.Type = "ping"
		resp.Script = "rtt < 300 * 1000 * 1000"
	}

	return resp, nil
}

// GeneratePollingScript creates an Otto JavaScript expression for polling evaluation.
func (c *LLMClient) GeneratePollingScript(ctx context.Context, pollingType, mode, purpose, lang string) (*PollingScriptResp, error) {
	isJa := strings.HasPrefix(strings.ToLower(lang), "ja")

	system := `You are an expert in Otto JavaScript scripting for TWSNMP NEO network monitoring pollings.
Generate a valid, robust JavaScript script expression that meets the user's requirements.

### CRITICAL EVALUATION RULES:
1. **Normal vs Alarm**: In TWSNMP NEO, the expression MUST evaluate to **true when NORMAL / OK**, and **false when ALARM / FAULT**.
2. **Units**: RTT / response_time is strictly in **NANOSECONDS (ns)**! (1 ms = 1,000,000 ns).
3. **No stats. prefix**: Use top-level variables: rtt, code, status, count, exitCode, etc.
4. Output Format: You MUST output strictly valid JSON with no markdown wrapping:
{
  "script": "<JavaScript boolean expression or script block>",
  "explanation": "<concise explanation of how the script evaluates>"
}`

	if isJa {
		system += "\n必ず日本語でexplanationを記述してください。純粋なJSONのみを出力してください。"
	}

	userMsg := fmt.Sprintf("Polling Type: %s\nMode: %s\nDesired Purpose / Condition: %s", pollingType, mode, purpose)

	rawAns, err := c.GenerateAnswer(ctx, system, userMsg)
	if err != nil {
		return nil, err
	}

	cleaned := cleanJSONOutput(rawAns)
	var resp PollingScriptResp
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return &PollingScriptResp{
			Script:      strings.TrimSpace(rawAns),
			Explanation: "Generated expression",
		}, nil
	}
	return &resp, nil
}

func cleanJSONOutput(s string) string {
	res := strings.TrimSpace(s)
	if strings.HasPrefix(res, "```json") {
		res = strings.TrimPrefix(res, "```json")
	} else if strings.HasPrefix(res, "```") {
		res = strings.TrimPrefix(res, "```")
	}
	res = strings.TrimSuffix(res, "```")
	return strings.TrimSpace(res)
}
