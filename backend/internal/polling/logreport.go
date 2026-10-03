package polling

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/logreport"
)

const (
	logReportPageSize   = 5000
	logReportMaxRecords = 100000
	logReportFirstRun   = 24 * time.Hour
	logReportCleanEvery = time.Hour
)

// LogReportPoller builds the Wi-Fi / Bluetooth / packet capture / Windows
// event reports by searching stored syslog messages (polling types
// twwifiscan, twbluescan, twpcap and twwinlog). Nothing is processed while
// syslog messages are received, only when such a polling exists.
//
// Polling fields:
//   - Params: optional sender (hostname / IP regex) to restrict the search.
//   - PollInt: search interval. The next run starts where the previous ended.
type LogReportPoller struct {
	store    datastore.DataStore
	logStore *parquet.Store
	source   string

	mu        sync.Mutex
	lastClean time.Time
}

// NewLogReportPoller creates a poller for one source (logreport.Source*).
func NewLogReportPoller(store datastore.DataStore, logStore *parquet.Store, source string) *LogReportPoller {
	return &LogReportPoller{store: store, logStore: logStore, source: source}
}

func (p *LogReportPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, _ *datastore.NodeEnt) (*Result, error) {
	start := time.Now()
	if p.store == nil || p.logStore == nil {
		return &Result{State: StateUnknown, Message: "log store is not available"}, nil
	}
	src := p.source
	if src == "" {
		src = pe.Mode
	}
	if src == "" {
		src = pe.Type
	}
	et := start.UnixNano()
	st := et - int64(logReportFirstRun)
	if pe.Result != nil {
		if cnt, ok := pe.Result["count"].(float64); ok && cnt > 0 {
			if lt, ok := pe.Result["lastTime"].(float64); ok && int64(lt) > 0 && int64(lt) <= et {
				st = int64(lt)
			}
		}
	}
	if st < et-int64(logReportFirstRun) {
		st = et - int64(logReportFirstRun)
	}

	recs, truncated := p.queryRecords(ctx, st, et, pe.Params, src)
	sess := logreport.NewSession(ctx, p.store)
	for _, rec := range recs {
		sess.Process(src, rec)
	}
	if err := sess.Commit(); err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("report update failed: %v", err),
		}, nil
	}
	p.cleanup(ctx)

	fields := map[string]interface{}{
		"scanned":   float64(len(recs)),
		"count":     float64(sess.Processed),
		"truncated": truncated,
		"lastTime":  float64(et),
		"rtt":       float64(time.Since(start).Nanoseconds()),
	}
	msg := fmt.Sprintf("%s report: %d of %d logs processed", logreport.SyslogTag(src), sess.Processed, len(recs))
	if truncated {
		msg += " (older logs skipped)"
	}
	return &Result{State: StateNormal, RTT: time.Since(start), Message: msg, Fields: fields}, nil
}

// queryRecords returns the records of this source in [st+1, et] sorted oldest first.
//
// The log store cuts a result at Limit without any guarantee about which
// records are kept, so a window that returns a full page is split in two
// until every part is complete. The second return value is true when records
// were skipped because of the safety limits.
func (p *LogReportPoller) queryRecords(ctx context.Context, st, et int64, host, source string) ([]logreport.Record, bool) {
	var recs []logreport.Record
	truncated := false
	var fetch func(from, to int64)
	fetch = func(from, to int64) {
		if from > to || ctx.Err() != nil {
			return
		}
		if len(recs) >= logReportMaxRecords {
			truncated = true
			return
		}
		logs, err := p.logStore.Query(ctx, parquet.LogFilter{
			Type:      "syslog",
			StartTime: from,
			EndTime:   to,
			Src:       host,
			Tag:       logreport.SyslogTag(source),
			Limit:     logReportPageSize,
		})
		if err != nil {
			slog.Warn("log report query failed", "source", source, "error", err)
			return
		}
		if len(logs) >= logReportPageSize {
			// Split at the median time of the sample; it lies inside the data.
			times := make([]int64, len(logs))
			for i, l := range logs {
				times[i] = l.Time
			}
			sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
			mid := times[len(times)/2]
			if mid >= to {
				mid = to - 1
			}
			if mid >= from {
				fetch(from, mid)
				fetch(mid+1, to)
				return
			}
			truncated = true // all records share one timestamp
		}
		for _, l := range logs {
			if rec, ok := logreport.ParseRecord(l.Log, l.Src, l.Time); ok {
				recs = append(recs, rec)
			}
		}
	}
	fetch(st+1, et)
	sortRecords(recs)
	return recs, truncated
}

func sortRecords(recs []logreport.Record) {
	sort.SliceStable(recs, func(i, j int) bool { return recs[i].Time < recs[j].Time })
}

func (p *LogReportPoller) cleanup(ctx context.Context) {
	p.mu.Lock()
	if time.Since(p.lastClean) < logReportCleanEvery {
		p.mu.Unlock()
		return
	}
	p.lastClean = time.Now()
	p.mu.Unlock()
	if err := logreport.Cleanup(ctx, p.store, p.source, logreport.DefaultRetentionDays, logreport.DefaultMaxEntries); err != nil {
		slog.Warn("log report cleanup failed", "source", p.source, "error", err)
	}
}
