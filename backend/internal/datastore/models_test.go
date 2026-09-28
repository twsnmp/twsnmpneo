package datastore

import (
	"encoding/json"
	"testing"
)

func TestNotifyConfUnmarshalLegacyFields(t *testing.T) {
	var conf NotifyConfEnt
	err := json.Unmarshal([]byte(`{
		"MailUser":"legacy-user",
		"MailPassword":"legacy-password",
		"WebhookURL":"https://legacy.example/hook"
	}`), &conf)
	if err != nil {
		t.Fatalf("unmarshal legacy notify config: %v", err)
	}
	if conf.User != "legacy-user" || conf.Password != "legacy-password" ||
		conf.WebHookNotify != "https://legacy.example/hook" {
		t.Fatalf("legacy fields were not migrated: %+v", conf)
	}
}

func TestNotifyConfUnmarshalPrefersCurrentFields(t *testing.T) {
	var conf NotifyConfEnt
	err := json.Unmarshal([]byte(`{
		"User":"current-user",
		"MailUser":"legacy-user",
		"WebHookNotify":"https://current.example/hook",
		"WebhookURL":"https://legacy.example/hook"
	}`), &conf)
	if err != nil {
		t.Fatalf("unmarshal notify config: %v", err)
	}
	if conf.User != "current-user" || conf.WebHookNotify != "https://current.example/hook" {
		t.Fatalf("current fields should take precedence: %+v", conf)
	}
}
