package notify

import (
	"bytes"
	"context"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

func TestMailNotifyTemplate_WithAndWithoutURL(t *testing.T) {
	store, cleanup, err := setupTestDB(t)
	if err != nil {
		t.Fatalf("setupTestDB err=%v", err)
	}
	defer cleanup()

	ctx := context.Background()
	i18n.SetLang("ja")

	now := time.Now().UnixNano()
	list := []*datastore.EventLogEnt{
		{Type: "polling", NodeName: "Server1", Level: "high", Event: "ping timeout", Time: now},
	}

	// 1. With URL
	notifyConf := &datastore.NotifyConfEnt{
		Subject:  "TWSNMP",
		Interval: 60,
		URL:      "http://192.168.1.100:8080",
	}
	if err := store.SaveNotifyConf(ctx, notifyConf); err != nil {
		t.Fatalf("SaveNotifyConf err=%v", err)
	}

	mgr := &Manager{store: store}
	bodyWithURL := mgr.eventLogListToString(false, list)

	if !strings.Contains(bodyWithURL, `href="http://192.168.1.100:8080"`) {
		t.Errorf("expected notification email to contain URL link, got:\n%s", bodyWithURL)
	}
	if !strings.Contains(bodyWithURL, "TWSNMP NEOへ") {
		t.Errorf("expected notification email to contain button text 'TWSNMP NEOへ', got:\n%s", bodyWithURL)
	}

	// 2. Without URL
	notifyConf.URL = ""
	if err := store.SaveNotifyConf(ctx, notifyConf); err != nil {
		t.Fatalf("SaveNotifyConf err=%v", err)
	}
	bodyWithoutURL := mgr.eventLogListToString(false, list)
	if strings.Contains(bodyWithoutURL, `class="totwsnmp"`) {
		t.Errorf("expected notification email to omit URL button when URL is empty, got:\n%s", bodyWithoutURL)
	}

	// 3. English template with URL
	i18n.SetLang("en")
	notifyConf.URL = "https://neo.example.com"
	if err := store.SaveNotifyConf(ctx, notifyConf); err != nil {
		t.Fatalf("SaveNotifyConf err=%v", err)
	}
	bodyEn := mgr.eventLogListToString(false, list)
	if !strings.Contains(bodyEn, `href="https://neo.example.com"`) {
		t.Errorf("expected English email to contain URL link, got:\n%s", bodyEn)
	}
	if !strings.Contains(bodyEn, "Go to TWSNMP NEO") {
		t.Errorf("expected English email to contain 'Go to TWSNMP NEO', got:\n%s", bodyEn)
	}
}

func TestMailTestTemplate_WithURL(t *testing.T) {
	store, cleanup, err := setupTestDB(t)
	if err != nil {
		t.Fatalf("setupTestDB err=%v", err)
	}
	defer cleanup()

	i18n.SetLang("ja")
	tmplStr := store.LoadMailTemplate("test")
	tmpl, err := template.New("test").Parse(tmplStr)
	if err != nil {
		t.Fatalf("failed to parse test template: %v", err)
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, map[string]interface{}{
		"Title": "Test Mail",
		"URL":   "http://192.168.1.50:8080",
	})
	if err != nil {
		t.Fatalf("failed to execute test template: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `href="http://192.168.1.50:8080"`) {
		t.Errorf("expected test mail to contain URL link, got:\n%s", out)
	}
	if !strings.Contains(out, "TWSNMP NEOへ") {
		t.Errorf("expected test mail to contain 'TWSNMP NEOへ', got:\n%s", out)
	}
}

func TestMailReportTemplate_WithURL(t *testing.T) {
	store, cleanup, err := setupTestDB(t)
	if err != nil {
		t.Fatalf("setupTestDB err=%v", err)
	}
	defer cleanup()

	i18n.SetLang("ja")
	f := template.FuncMap{
		"levelName":     levelName,
		"formatLogTime": formatLogTime,
		"formatScore":   formatScore,
		"scoreClass":    scoreClass,
		"aiScoreClass":  aiScoreClass,
		"formatAITime":  formatAITime,
		"formatCount":   formatCount,
	}
	tmplStr := store.LoadMailTemplate("report")
	tmpl, err := template.New("report").Funcs(f).Parse(tmplStr)
	if err != nil {
		t.Fatalf("failed to parse report template: %v", err)
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, map[string]interface{}{
		"Title": "Daily Report",
		"URL":   "http://192.168.1.50:8080",
		"Info":  []reportInfoEnt{},
	})
	if err != nil {
		t.Fatalf("failed to execute report template: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `href="http://192.168.1.50:8080"`) {
		t.Errorf("expected report mail to contain URL link, got:\n%s", out)
	}
	if !strings.Contains(out, "TWSNMP NEOへ") {
		t.Errorf("expected report mail to contain 'TWSNMP NEOへ', got:\n%s", out)
	}
}
