package bbolt

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

func TestLogReportData(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	if b, err := s.GetLogReportData(ctx, "wifiAP", "x"); err != nil || b != nil {
		t.Fatalf("missing entity must return nil,nil: %v %v", b, err)
	}
	if m, err := s.ListLogReportData(ctx, "wifiAP"); err != nil || len(m) != 0 {
		t.Fatalf("empty list: %v %v", m, err)
	}
	if _, err := s.GetLogReportData(ctx, "", "x"); err != datastore.ErrInvalidParams {
		t.Fatalf("empty kind: %v", err)
	}
	if err := s.SaveLogReportData(ctx, "", map[string][]byte{"a": []byte("1")}); err != datastore.ErrInvalidParams {
		t.Fatalf("empty kind: %v", err)
	}
	if err := s.SaveLogReportData(ctx, "wifiAP", map[string][]byte{"a": []byte(`{"ID":"a"}`), "b": []byte(`{"ID":"b"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveLogReportData(ctx, "winTask", map[string][]byte{"c": []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if b, _ := s.GetLogReportData(ctx, "wifiAP", "a"); string(b) != `{"ID":"a"}` {
		t.Fatalf("get: %s", b)
	}
	if m, _ := s.ListLogReportData(ctx, "wifiAP"); len(m) != 2 {
		t.Fatalf("list: %v", m)
	}
	if err := s.DeleteLogReportData(ctx, "wifiAP", []string{"a", "missing"}); err != nil {
		t.Fatal(err)
	}
	if m, _ := s.ListLogReportData(ctx, "wifiAP"); len(m) != 1 {
		t.Fatalf("after delete: %v", m)
	}
	if err := s.DeleteLogReportData(ctx, "unknown", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetLogReportData(ctx, "wifiAP"); err != nil {
		t.Fatal(err)
	}
	if m, _ := s.ListLogReportData(ctx, "wifiAP"); len(m) != 0 {
		t.Fatalf("after reset: %v", m)
	}
	if m, _ := s.ListLogReportData(ctx, "winTask"); len(m) != 1 {
		t.Fatalf("other kind must remain: %v", m)
	}
	if err := s.ResetLogReportData(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if m, _ := s.ListLogReportData(ctx, "winTask"); len(m) != 0 {
		t.Fatalf("after reset all: %v", m)
	}
	// Still usable after dropping the parent bucket.
	if err := s.SaveLogReportData(ctx, "winTask", map[string][]byte{"c": []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
}
