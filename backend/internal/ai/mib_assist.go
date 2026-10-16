package ai

import (
	"context"
	"fmt"
	"strings"
)

// MIBSearchResp is the result for AI MIB / OID search.
type MIBSearchResp struct {
	ObjectName  string `json:"object_name"`
	OID         string `json:"oid"`
	Description string `json:"description,omitempty"`
}

// SearchMIB searches for standard and vendor SNMP MIB objects and OIDs from natural language requests.
func (c *LLMClient) SearchMIB(ctx context.Context, prompt, lang string) (*MIBSearchResp, error) {
	isJa := strings.HasPrefix(strings.ToLower(lang), "ja")

	system := `You are an expert on SNMP MIBs. Fulfill the request entered by the user.
Please provide the object name and OID of the SNMP MIB.
Please be sure to answer only the object name and OID in the following format. No extra explanation or markdown quotes:
Object name,OID`

	if isJa {
		system = `あなたはSNMPのMIBに関する専門家です。ユーザーの入力した要望を満たすSNMPのMIBのオブジェクト名とOIDを答えてください。
必ずオブジェクト名とOIDのみを以下の形式で回答してください。余計な解説やマークダウンのクォートは不要です。
オブジェクト名,OID`
	}

	ans, err := c.GenerateAnswer(ctx, system, prompt)
	if err != nil {
		return nil, err
	}

	res := strings.TrimSpace(ans)
	res = strings.TrimPrefix(res, "```")
	res = strings.TrimSuffix(res, "```")
	res = strings.TrimSpace(res)

	resp := &MIBSearchResp{}
	if parts := strings.SplitN(res, ",", 2); len(parts) == 2 {
		resp.ObjectName = strings.TrimSpace(parts[0])
		resp.OID = strings.TrimSpace(parts[1])
	} else if parts := strings.SplitN(res, "\n", 2); len(parts) >= 2 {
		resp.ObjectName = strings.TrimSpace(parts[0])
		resp.OID = strings.TrimSpace(parts[1])
	} else {
		resp.ObjectName = res
	}

	return resp, nil
}

// ExplainMIB provides expert analysis of SNMP acquisition results (walk, table, get).
func (c *LLMClient) ExplainMIB(ctx context.Context, queryType, oid, resultText, lang string) (string, error) {
	isJa := strings.HasPrefix(strings.ToLower(lang), "ja")

	system := "You are an expert on SNMP MIBs and network telemetry. Please explain the SNMP acquisition results entered by the user in clear markdown format with tables and bullet points."
	if isJa {
		system = "あなたはSNMPのMIBおよびネットワークテレメトリに関する専門家です。ユーザーの入力したSNMPの取得結果について、各MIB値の意味、正常性、および推奨される分析ポイントを分かりやすく日本語のマークダウン形式（表、箇条書き、太字）で解説してください。"
	}

	userMsg := fmt.Sprintf("Query Type: %s\nTarget OID: %s\n\nAcquisition Results:\n%s", queryType, oid, resultText)
	return c.GenerateAnswer(ctx, system, userMsg)
}
