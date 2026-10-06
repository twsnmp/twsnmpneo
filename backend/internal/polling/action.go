package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/notify"
	"github.com/twsnmp/twsnmpneo/backend/internal/wol"
)

// formatDowntime converts downtime seconds into a human-readable duration string.
func formatDowntime(sec int64) string {
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	m := sec / 60
	s := sec % 60
	if m < 60 {
		if s > 0 {
			return fmt.Sprintf("%dm %ds", m, s)
		}
		return fmt.Sprintf("%dm", m)
	}
	h := m / 60
	m = m % 60
	if h < 24 {
		if m > 0 {
			return fmt.Sprintf("%dh %dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}
	d := h / 24
	h = h % 24
	if h > 0 {
		return fmt.Sprintf("%dd %dh", d, h)
	}
	return fmt.Sprintf("%dd", d)
}

type webhookEnt struct {
	Time    string                 `json:"Time"`
	Node    string                 `json:"Node"`
	Polling string                 `json:"Polling"`
	State   string                 `json:"State"`
	Results map[string]interface{} `json:"Results"`
}

// doAction executes configured FailAction or RepairAction strings.
func doAction(ctx context.Context, pe *datastore.PollingEnt, store datastore.DataStore) {
	if pe == nil || pe.State == StateUnknown {
		return
	}
	action := pe.FailAction
	if pe.State == StateRepair || pe.State == StateNormal {
		action = pe.RepairAction
	}
	if strings.TrimSpace(action) == "" {
		return
	}
	for _, a := range strings.Split(action, "\n") {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		al := strings.Fields(a)
		if !doOneAction(ctx, al, pe, store) {
			break
		}
	}
}

func doOneAction(ctx context.Context, al []string, pe *datastore.PollingEnt, store datastore.DataStore) bool {
	if len(al) < 2 {
		return true
	}
	slog.Info("doOneAction", "action", al[0], "params", al[1:])
	switch al[0] {
	case "wol":
		target := al[1]
		mac := target
		if store != nil {
			nodes, err := store.ListNodes(ctx)
			if err == nil {
				for _, n := range nodes {
					if n.Name == target || n.IP == target {
						mac = n.MAC
						break
					}
				}
			}
		}
		if strings.Contains(mac, ":") {
			_ = wol.SendWakeOnLanPacket(mac)
		}
	case "mail":
		subject := al[1]
		body := subject
		if len(al) > 2 {
			body = strings.Join(al[2:], " ")
		}
		if mgr := notify.GetManager(); mgr != nil {
			_ = mgr.SendMail(subject, body)
		}
	case "wait":
		if to, err := strconv.Atoi(al[1]); err == nil && to > 0 {
			targetNode := ""
			targetState := ""
			if len(al) > 3 {
				targetNode = al[2]
				targetState = al[3]
			}
			for i := 0; i < to; i++ {
				select {
				case <-ctx.Done():
					return false
				default:
				}
				if targetNode != "" && store != nil {
					nodes, err := store.ListNodes(ctx)
					if err == nil {
						for _, n := range nodes {
							if n.Name == targetNode {
								if targetState == "up" && (n.State == StateNormal || n.State == StateRepair) {
									return false
								} else if targetState == "down" && (n.State == StateLow || n.State == StateHigh || n.State == StateWarn) {
									return false
								}
							}
						}
					}
				}
				time.Sleep(time.Second)
			}
		}
	case "cmd":
		doActionCmd(al[1:])
	case "webhook":
		nodeName := pe.NodeID
		if store != nil && pe.NodeID != "" {
			if n, err := store.GetNode(ctx, pe.NodeID); err == nil && n != nil {
				nodeName = n.Name
			}
		}
		payload := webhookEnt{
			Time:    time.Unix(0, pe.LastTime).Format(time.RFC3339),
			State:   pe.State,
			Polling: pe.Name,
			Node:    nodeName,
			Results: pe.Result,
		}
		if j, err := json.Marshal(&payload); err == nil {
			_ = notify.PostWebhook(al[1], j)
		}
	}
	return true
}

func doActionCmd(cl []string) {
	if len(cl) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmdName := cl[0]
	var cmd *exec.Cmd
	if filepath.Ext(cmdName) == ".sh" {
		cmd = exec.CommandContext(ctx, "/bin/sh", cl...)
	} else if len(cl) == 1 {
		cmd = exec.CommandContext(ctx, cmdName)
	} else {
		cmd = exec.CommandContext(ctx, cmdName, cl[1:]...)
	}
	_ = cmd.Run()
}
