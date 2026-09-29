// Package notify : 通知処理 - Webhook送信
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// WebHookTest sends test payloads to the configured webhook URLs.
func WebHookTest(store datastore.DataStore, n *datastore.NotifyConfEnt) error {
	if n.WebHookNotify != "" {
		payload := webhookNotifyPayload{}
		payload.Log = append(payload.Log, webhookNotifyLog{
			Time:      time.Now().Format(time.RFC3339),
			Type:      "test",
			NodeName:  "test node name",
			NodeID:    "test node ID",
			Event:     "test event",
			Level:     "info",
			LastLevel: "info",
		})
		payload.Count = len(payload.Log)
		j, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		err = PostTestWebhook(n.WebHookNotify, j)
		if err != nil {
			return err
		}
	}
	if n.WebHookReport != "" {
		payload := webhookReportPayload{
			Title: "Test Report",
		}
		payload.Info = append(payload.Info, webhookReportInfo{
			Name:  "test name",
			Value: "test value",
		})
		payload.AI = append(payload.AI, webhookReportAI{
			Score:   1.0,
			Polling: "test polling",
			Node:    "test node",
			Time:    time.Now().Format(time.RFC3339),
		})
		j, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		err = PostTestWebhook(n.WebHookReport, j)
		if err != nil {
			return err
		}
	}
	return nil
}

type webhookNotifyPayload struct {
	Count int                `json:"Count"`
	Log   []webhookNotifyLog `json:"Log"`
}

type webhookNotifyLog struct {
	Time      string `json:"Time"`
	Type      string `json:"Type"`
	Level     string `json:"Level"`
	NodeName  string `json:"NodeName"`
	NodeID    string `json:"NodeID"`
	Event     string `json:"Event"`
	LastLevel string `json:"LastLevel"`
	RootCause string `json:"RootCause,omitempty"`
}

func (m *Manager) webhookNotify(list []*datastore.EventLogEnt) {
	conf := m.getNotifyConf()
	if conf.WebHookNotify == "" {
		return
	}
	nl := getLevelNum(conf.Level)
	if nl == 3 {
		return
	}
	payload := webhookNotifyPayload{}
	ti := time.Now().Add(time.Duration(-conf.Interval) * time.Minute).UnixNano()
	var dep *DependencyAnalysisResult
	if conf.CheckDependency {
		dep = AnalyzeFailureDependencies(m.store, list)
	}
	for _, l := range list {
		if ti > l.Time {
			continue
		}
		np := getLevelNum(l.Level)
		if np > nl {
			continue
		}
		rootCause := ""
		if dep != nil && l.Type == "polling" && l.NodeID != "" {
			if rcid, ok := dep.ImpactedBy[l.NodeID]; ok {
				rootCause = GetNodeOrNetworkName(m.store, rcid)
			} else if len(dep.ImpactedMap[l.NodeID]) > 0 {
				rootCause = "self"
			}
		}
		payload.Log = append(payload.Log, webhookNotifyLog{
			Time:      time.Unix(0, l.Time).Format(time.RFC3339),
			Type:      l.Type,
			NodeName:  l.NodeName,
			NodeID:    l.NodeID,
			Event:     l.Event,
			Level:     l.Level,
			LastLevel: l.LastLevel,
			RootCause: rootCause,
		})
	}
	payload.Count = len(payload.Log)
	if payload.Count < 1 {
		return
	}
	j, err := json.Marshal(payload)
	if err != nil {
		slog.Error("webhookNotify error", "error", err)
		return
	}
	err = PostWebhook(conf.WebHookNotify, j)
	if err != nil {
		slog.Error("webhookNotify error", "error", err)
	}
}

type webhookReportPayload struct {
	Title string              `json:"Title"`
	Info  []webhookReportInfo `json:"Info"`
	AI    []webhookReportAI   `json:"AI"`
}

type webhookReportInfo struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

type webhookReportAI struct {
	Score   float64 `json:"Score"`
	Node    string  `json:"Node"`
	Polling string  `json:"Polling"`
	Count   int     `json:"Count"`
	Time    string  `json:"Time"`
}

func (m *Manager) webhookReport(title string, info []reportInfoEnt, ai []aiResultEnt) {
	conf := m.getNotifyConf()
	if conf.WebHookReport == "" {
		return
	}
	payload := webhookReportPayload{
		Title: title,
	}
	for _, i := range info {
		payload.Info = append(payload.Info,
			webhookReportInfo{
				Name:  i.Name,
				Value: i.Value,
			})
	}
	for _, a := range ai {
		payload.AI = append(payload.AI, webhookReportAI{
			Score:   a.LastScore,
			Node:    a.NodeName,
			Polling: a.PollingName,
			Count:   a.Count,
			Time:    time.Unix(0, a.LastTime).Format(time.RFC3339),
		})
	}
	j, err := json.Marshal(payload)
	if err != nil {
		slog.Error("webhookReport error", "error", err)
		return
	}
	err = PostWebhook(conf.WebHookReport, j)
	if err != nil {
		slog.Error("webhookReport error", "error", err)
	}
}

// PostWebhook posts JSON data to the given webhook URL.
func PostWebhook(url string, j []byte) error {
	return postWebhook(url, j, &http.Client{Timeout: 2 * time.Second})
}

// PostTestWebhook posts to a test webhook, including local/private destinations.
func PostTestWebhook(url string, j []byte) error {
	if err := validateWebhookURL(url); err != nil {
		return err
	}
	return postWebhook(url, j, newTestWebhookClient())
}

func postWebhook(url string, j []byte, client *http.Client) error {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook error %s", resp.Status)
	}
	return nil
}
