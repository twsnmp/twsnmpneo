// Package notify : 通知処理
package notify

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
	"github.com/twsnmp/twsnmpneo/backend/internal/monitor"
)

var (
	lastExecLevel int
)

// Manager holds the DataStore reference for all notify operations.
type Manager struct {
	store   datastore.DataStore
	monitor *monitor.Monitor
}

var defaultManager *Manager

// Start initializes the notify background manager and starts the goroutine.
func Start(ctx context.Context, store datastore.DataStore, mon *monitor.Monitor) {
	lastExecLevel = -1
	defaultManager = &Manager{store: store, monitor: mon}
	go defaultManager.notifyBackend(ctx)
}

func (m *Manager) notifyBackend(ctx context.Context) {
	log.Println("start notify")
	conf := m.getNotifyConf()
	lastSendReport := time.Now().Add(time.Hour * time.Duration(-24))
	lastLog := time.Now().Add(time.Hour * time.Duration(-1)).UnixNano()
	lastLog = m.checkNotify(lastLog)
	timer := time.NewTicker(time.Second * 60)
	i := 0
	for {
		select {
		case <-ctx.Done():
			log.Println("stop notify")
			timer.Stop()
			return
		case <-timer.C:
			conf = m.getNotifyConf()
			i++
			if i >= conf.Interval {
				i = 0
				lastLog = m.checkNotify(lastLog)
			}
			m.checkExecCmd()
			monData := m.getMonitorData()
			if conf.Report &&
				lastSendReport.Day() != time.Now().Day() &&
				len(monData) > 1 {
				lastSendReport = time.Now()
				go m.sendReport(ctx)
			}
		}
	}
}

func (m *Manager) getNotifyConf() datastore.NotifyConfEnt {
	conf, err := m.store.GetNotifyConf(context.Background())
	if err != nil || conf == nil {
		return datastore.NotifyConfEnt{Interval: 60}
	}
	return *conf
}

func (m *Manager) getMonitorData() []*monitor.MonitorDataEnt {
	if m.monitor == nil {
		return nil
	}
	return m.monitor.GetData()
}

func getLevelNum(l string) int {
	switch l {
	case "high":
		return 0
	case "low":
		return 1
	case "warn":
		return 2
	case "none":
		return -1
	}
	return 3
}

func (m *Manager) checkNotify(last int64) int64 {
	list := []*datastore.EventLogEnt{}
	lastLogTime := int64(0)
	m.store.ForEachLastEventLog(func(l *datastore.EventLogEnt) bool {
		if lastLogTime < l.Time {
			lastLogTime = l.Time
		}
		if last >= l.Time {
			return false
		}
		list = append(list, l)
		return true
	})
	log.Printf("check notify last=%v next=%v len=%d", time.Unix(0, last), time.Unix(0, lastLogTime), len(list))
	if len(list) > 0 {
		m.sendNotifyMail(list)
		m.webhookNotify(list)
	}
	if lastLogTime > 0 {
		return lastLogTime
	}
	return time.Now().UnixNano()
}

type notifyData struct {
	failureSubject string
	failureBody    string
	repairSubject  string
	repairBody     string
}

// getNotifyData : 通知メールの本文と件名を作成する
func (m *Manager) getNotifyData(list []*datastore.EventLogEnt, nl int) notifyData {
	conf := m.getNotifyConf()
	fNodeMap := make(map[string]int)
	rNodeMap := make(map[string]int)
	failure := []*datastore.EventLogEnt{}
	repair := []*datastore.EventLogEnt{}
	ti := time.Now().Add(time.Duration(-conf.Interval) * time.Minute).UnixNano()
	for _, l := range list {
		if ti > l.Time {
			continue
		}
		if conf.NotifyRepair && l.Level == "repair" {
			// 復帰前の状態を確認する
			np := getLevelNum(l.LastLevel)
			if np > nl {
				continue
			}
			rNodeMap[l.NodeName] = np
			repair = append(repair, l)
			continue
		}
		np := getLevelNum(l.Level)
		if np > nl {
			continue
		}
		fNodeMap[l.NodeName] = np
		failure = append(failure, l)
	}
	f, r := "", ""
	fs, rs := "", ""
	if len(failure) > 0 {
		var failureLogs []*datastore.EventLogEnt = failure
		subjectNodes := getNodes(fNodeMap)

		if conf.CheckDependency {
			dep := AnalyzeFailureDependencies(m.store, failure)
			modifiedLogs := make([]*datastore.EventLogEnt, 0, len(failure))

			// 1. Root causes (polling only) first
			for _, l := range failure {
				if l.Type != "polling" || l.NodeID == "" {
					continue
				}
				if _, isImpacted := dep.ImpactedBy[l.NodeID]; !isImpacted {
					copyLog := *l
					if impactedCount := len(dep.ImpactedMap[l.NodeID]); impactedCount > 0 {
						copyLog.Event = fmt.Sprintf("[%s (%s)] %s",
							i18n.Trans("Root Cause"),
							fmt.Sprintf(i18n.Trans("other %d impacted"), impactedCount),
							l.Event)
					} else {
						copyLog.Event = fmt.Sprintf("[%s] %s", i18n.Trans("Root Cause"), l.Event)
					}
					modifiedLogs = append(modifiedLogs, &copyLog)
				}
			}

			// 2. Impacted nodes (polling only) with annotations
			for _, l := range failure {
				if l.Type != "polling" || l.NodeID == "" {
					continue
				}
				if rootCauseID, isImpacted := dep.ImpactedBy[l.NodeID]; isImpacted {
					rcName := GetNodeOrNetworkName(m.store, rootCauseID)
					copyLog := *l
					copyLog.Event = fmt.Sprintf("[%s: %s] %s", i18n.Trans("Impacted by"), rcName, l.Event)
					modifiedLogs = append(modifiedLogs, &copyLog)
				}
			}

			// 3. Non-polling events (system, syslog, trap, etc.) kept as-is
			for _, l := range failure {
				if l.Type != "polling" || l.NodeID == "" {
					modifiedLogs = append(modifiedLogs, l)
				}
			}
			failureLogs = modifiedLogs

			// Format subject with root causes
			var parts []string
			for _, rcid := range dep.RootCauses {
				name := GetNodeOrNetworkName(m.store, rcid)
				if impactedCount := len(dep.ImpactedMap[rcid]); impactedCount > 0 {
					parts = append(parts, fmt.Sprintf("%s (%s)", name, fmt.Sprintf(i18n.Trans("other %d impacted"), impactedCount)))
				} else {
					parts = append(parts, name)
				}
			}
			if len(parts) == 0 && len(dep.ImpactedBy) > 0 {
				for nid, rcid := range dep.ImpactedBy {
					parts = append(parts, fmt.Sprintf("%s (%s: %s)",
						GetNodeOrNetworkName(m.store, nid),
						i18n.Trans("Cause"),
						GetNodeOrNetworkName(m.store, rcid)))
				}
			}
			if len(parts) > 0 {
				subjectNodes = fmt.Sprintf("[%s: %s]", i18n.Trans("Root Cause"), strings.Join(parts, ", "))
			}
		}

		f = m.eventLogListToString(false, failureLogs)
		fs = conf.Subject + i18n.Trans("(Failure)")
		fs += ":" + subjectNodes
	}
	if len(repair) > 0 {
		r = m.eventLogListToString(true, repair)
		rs = conf.Subject + i18n.Trans("(Repair)")
		rs += ":" + getNodes(rNodeMap)
	}
	return notifyData{
		failureSubject: fs,
		failureBody:    f,
		repairSubject:  rs,
		repairBody:     r,
	}
}

func getNodes(m map[string]int) string {
	nodes := []string{}
	l := 0
	for n := range m {
		nodes = append(nodes, n)
		l += len(n)
		if l > 1000 {
			break
		}
	}
	return strings.Join(nodes, ",")
}

// eventLogListToString : イベントログを通知メールの本文に変換する
func (m *Manager) eventLogListToString(repair bool, list []*datastore.EventLogEnt) string {
	conf := m.getNotifyConf()
	title := conf.Subject + i18n.Trans("(Failure)")
	if repair {
		title = conf.Subject + i18n.Trans("(Repair)")
	}
	f := template.FuncMap{
		"levelName":     levelName,
		"formatLogTime": formatLogTime,
	}
	t, err := template.New("notify").Funcs(f).Parse(m.store.LoadMailTemplate("notify"))
	if err != nil {
		return fmt.Sprintf("make mail err=%v", err)
	}
	buffer := new(bytes.Buffer)
	if err = t.Execute(buffer, map[string]interface{}{
		"Title": title,
		"Logs":  list,
	}); err != nil {
		return fmt.Sprintf("make mail err=%v", err)
	}
	return buffer.String()
}

func levelName(s string) string {
	switch s {
	case "high":
		return i18n.Trans("High")
	case "low":
		return i18n.Trans("Low")
	case "warn":
		return i18n.Trans("Warning")
	case "normal", "up":
		return i18n.Trans("Normal")
	case "repair":
		return i18n.Trans("Repair")
	}
	return i18n.Trans("Unknown")
}

func formatLogTime(t int64) string {
	return time.Unix(0, t).Local().Format("2006/01/02 15:04:05")
}
func formatAITime(t int64) string {
	return time.Unix(t, 0).Local().Format("2006/01/02 15:04:05")
}

func scoreClass(s float64) string {
	if s >= 50 {
		return "info"
	} else if s > 42 {
		return "warn"
	} else if s > 33 {
		return "low"
	}
	return "high"
}

func aiScoreClass(s float64) string {
	if s > 100.0 {
		s = 1.0
	} else {
		s = 100.0 - s
	}
	return scoreClass(s)
}

func formatScore(s float64) string {
	return fmt.Sprintf("%.2f", s)
}

func formatCount(i interface{}) string {
	c := int64(0)
	switch v := i.(type) {
	case int64:
		c = v
	case int:
		c = int64(v)
	case float32:
		c = int64(v)
	case float64:
		c = int64(v)
	}
	return humanize.Comma(c)
}

// addEventLog is a helper to add event logs without requiring a context from caller.
func (m *Manager) addEventLog(level, event string) {
	_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
		Time:  time.Now().UnixNano(),
		Type:  "system",
		Level: level,
		Event: event,
	})
}

// GetManager returns the default notify manager (may be nil if not started).
func GetManager() *Manager {
	return defaultManager
}

// GetStore returns the DataStore from the manager (used by web handlers).
func GetStore() datastore.DataStore {
	if defaultManager == nil {
		return nil
	}
	return defaultManager.store
}

// Legacy shim types for backward compatibility with callers that use sync.WaitGroup.
// These are no longer used internally but kept to avoid breaking external consumers.
var _ sync.WaitGroup
