package logreport

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// MaxSeriesSize is the maximum number of time-series samples kept per entity.
const MaxSeriesSize = 12 * 24 * 7

// Record is the part of a stored syslog message used by the reports.
type Record struct {
	Time     int64
	Host     string
	Tag      string
	Content  string
	Severity int
}

// ParseRecord converts a stored syslog JSON document into a Record.
// src and t are used when the document does not carry a host / time.
func ParseRecord(logJSON, src string, t int64) (Record, bool) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(logJSON), &m); err != nil {
		return Record{}, false
	}
	str := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	r := Record{
		Time:     t,
		Host:     str("hostname", "host"),
		Tag:      str("tag", "app_name"),
		Content:  str("content", "message"),
		Severity: 6,
	}
	if r.Host == "" {
		r.Host = src
	}
	if v, ok := m["severity"].(float64); ok {
		r.Severity = int(v)
	}
	return r, r.Tag != "" && r.Content != ""
}

type nodeInfo struct{ Name, ID string }

// Session accumulates report updates for one polling run.
// Entities are loaded on demand, modified in memory and written by Commit.
type Session struct {
	ctx    context.Context
	store  datastore.DataStore
	cache  map[string]map[string]any
	dirty  map[string]map[string]struct{}
	loaded map[string]bool
	nodes  map[string]nodeInfo
	// Processed is the number of records that updated a report.
	Processed int
}

// NewSession creates a Session writing to store.
func NewSession(ctx context.Context, store datastore.DataStore) *Session {
	return &Session{
		ctx:    ctx,
		store:  store,
		cache:  map[string]map[string]any{},
		dirty:  map[string]map[string]struct{}{},
		loaded: map[string]bool{},
	}
}

func (s *Session) kindCache(kind string) map[string]any {
	c := s.cache[kind]
	if c == nil {
		c = map[string]any{}
		s.cache[kind] = c
	}
	return c
}

// getEnt returns the entity (cached or loaded from the store) or nil.
func getEnt[T any](s *Session, kind, id string) *T {
	c := s.kindCache(kind)
	if v, ok := c[id]; ok {
		e, _ := v.(*T)
		return e
	}
	b, err := s.store.GetLogReportData(s.ctx, kind, id)
	if err != nil || b == nil {
		return nil
	}
	e := new(T)
	if json.Unmarshal(b, e) != nil {
		return nil
	}
	c[id] = e
	return e
}

// putEnt stores e in the session and marks it for writing.
func putEnt[T any](s *Session, kind, id string, e *T) {
	s.kindCache(kind)[id] = e
	d := s.dirty[kind]
	if d == nil {
		d = map[string]struct{}{}
		s.dirty[kind] = d
	}
	d[id] = struct{}{}
}

// allEnts loads the whole kind into the session and returns every entity.
func allEnts[T any](s *Session, kind string) map[string]*T {
	c := s.kindCache(kind)
	if !s.loaded[kind] {
		if m, err := s.store.ListLogReportData(s.ctx, kind); err == nil {
			for id, b := range m {
				if _, ok := c[id]; ok {
					continue
				}
				e := new(T)
				if json.Unmarshal(b, e) == nil {
					c[id] = e
				}
			}
		}
		s.loaded[kind] = true
	}
	ret := make(map[string]*T, len(c))
	for id, v := range c {
		if e, ok := v.(*T); ok {
			ret[id] = e
		}
	}
	return ret
}

func (s *Session) lookupNode(ip string) nodeInfo {
	if s.nodes == nil {
		s.nodes = map[string]nodeInfo{}
		if nodes, err := s.store.ListNodes(s.ctx); err == nil {
			for _, n := range nodes {
				if n.IP != "" {
					s.nodes[n.IP] = nodeInfo{Name: n.Name, ID: n.ID}
				}
			}
		}
	}
	if n, ok := s.nodes[ip]; ok {
		return n
	}
	return nodeInfo{Name: ip}
}

// Commit writes modified entities and refreshes the deviation scores of the
// kinds that have one.
func (s *Session) Commit() error {
	var firstErr error
	for kind, ids := range s.dirty {
		items := make(map[string][]byte, len(ids))
		for id := range ids {
			b, err := json.Marshal(s.cache[kind][id])
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			items[id] = b
		}
		if err := s.store.SaveLogReportData(s.ctx, kind, items); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for kind := range s.dirty {
		var err error
		switch kind {
		case KindWinLogon:
			err = recalcScores[WinLogonEnt](s, kind)
		case KindWinKerberos:
			err = recalcScores[WinKerberosEnt](s, kind)
		case KindRADIUSFlow:
			err = recalcScores[RADIUSFlowEnt](s, kind)
		case KindTLSFlow:
			err = recalcScores[TLSFlowEnt](s, kind)
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	s.dirty = map[string]map[string]struct{}{}
	return firstErr
}

// recalcScores computes the deviation score (mean 50, sd 10) from penalties.
func recalcScores[T any, P interface {
	*T
	scoreInfo() *ScoreInfo
}](s *Session, kind string) error {
	m, err := s.store.ListLogReportData(s.ctx, kind)
	if err != nil {
		return err
	}
	ents := make(map[string]P, len(m))
	xs := make([]float64, 0, len(m))
	for id, b := range m {
		e := new(T)
		if json.Unmarshal(b, e) != nil {
			continue
		}
		p := P(e)
		si := p.scoreInfo()
		if si.Penalty > 100 {
			si.Penalty = 100
		}
		xs = append(xs, float64(100-si.Penalty))
		ents[id] = p
	}
	if len(xs) == 0 {
		return nil
	}
	mean, sd := meanSD(xs)
	out := make(map[string][]byte, len(ents))
	for id, p := range ents {
		si := p.scoreInfo()
		if sd != 0 {
			si.Score = 10*(float64(100-si.Penalty)-mean)/sd + 50
		} else {
			si.Score = 50
		}
		si.ValidScore = true
		b, err := json.Marshal(p)
		if err != nil {
			continue
		}
		out[id] = b
	}
	return s.store.SaveLogReportData(s.ctx, kind, out)
}

func meanSD(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	var v float64
	for _, x := range xs {
		v += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(v / float64(len(xs)))
}

// Process applies one syslog record of the given source (polling type).
// It returns true when the record updated a report.
func (s *Session) Process(source string, r Record) bool {
	if !strings.EqualFold(r.Tag, SyslogTag(source)) {
		return false
	}
	m := parseKV(r.Content)
	t := m["type"]
	if t == "" {
		return false
	}
	var ok bool
	switch source {
	case SourceWifiScan:
		ok = s.processWifi(r, t, m)
	case SourceBlueScan:
		ok = s.processBlue(r, t, m)
	case SourcePcap:
		ok = s.processPcap(r, t, m)
	case SourceWinLog:
		ok = s.processWinLog(r, t, m)
	}
	if ok {
		s.Processed++
	}
	return ok
}

// parseKV parses "k1=v1,k2=v2" as emitted by the tw* tools.
func parseKV(c string) map[string]string {
	m := map[string]string{}
	for _, kv := range strings.Split(c, ",") {
		a := strings.SplitN(kv, "=", 2)
		if len(a) == 2 {
			m[a[0]] = a[1]
		}
	}
	return m
}

func atoi(s string) int {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return 0
		}
		return int(f)
	}
	return int(n)
}

func atof(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// parseTime parses an RFC3339 time as emitted by the tw* tools; def is returned on failure.
func parseTime(s string, def int64) int64 {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UnixNano()
	}
	return def
}

func makeID(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}
