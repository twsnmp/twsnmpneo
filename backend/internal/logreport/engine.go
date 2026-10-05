package logreport

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// Reporter is the interface implemented by the report engine for real-time
// ingestion from various protocol receivers (Syslog, NetFlow, sFlow, MQTT, OTel, etc.).
type Reporter interface {
	// ProcessSyslog ingests a raw syslog message map.
	ProcessSyslog(sl map[string]interface{})
	// ProcessNetFlow is a hook for future NetFlow/IPFIX report ingestion.
	ProcessNetFlow(record any)
	// ProcessSFlow is a hook for future sFlow report ingestion.
	ProcessSFlow(record any)
	// ProcessMQTT is a hook for future MQTT sensor/topic report ingestion.
	ProcessMQTT(topic string, payload []byte)
	// ProcessOTel is a hook for future OpenTelemetry report ingestion.
	ProcessOTel(record any)

	// Start starts background worker routines (queue processing, periodic cleanup).
	Start(ctx context.Context) error
	// Stop flushes remaining records and stops background workers.
	Stop()
}

// Engine processes incoming events asynchronously and updates the log report datastore.
type Engine struct {
	store        datastore.DataStore
	syslogQueue  chan Record
	stopCh       chan struct{}
	flushCh      chan chan struct{}
	wg           sync.WaitGroup
	cleanInterval time.Duration
	flushInterval time.Duration
	maxBatchSize  int
}

// Option configures the Engine.
type Option func(*Engine)

// WithCleanInterval overrides the default cleanup interval (1 hour).
func WithCleanInterval(d time.Duration) Option {
	return func(e *Engine) {
		e.cleanInterval = d
	}
}

// WithFlushInterval overrides the default batch flush interval (1 second).
func WithFlushInterval(d time.Duration) Option {
	return func(e *Engine) {
		e.flushInterval = d
	}
}

// NewEngine creates a new Engine writing report data to store.
func NewEngine(store datastore.DataStore, opts ...Option) *Engine {
	e := &Engine{
		store:         store,
		syslogQueue:   make(chan Record, 10000),
		stopCh:        make(chan struct{}),
		flushCh:       make(chan chan struct{}),
		cleanInterval: time.Hour,
		flushInterval: 200 * time.Millisecond,
		maxBatchSize:  500,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// ProcessSyslog parses a syslog message and enqueues it if it matches a known report tag.
func (e *Engine) ProcessSyslog(sl map[string]interface{}) {
	if e == nil || sl == nil {
		return
	}
	tag, _ := sl["tag"].(string)
	if tag == "" {
		tag, _ = sl["app_name"].(string)
	}

	content, _ := sl["content"].(string)
	if content == "" {
		content, _ = sl["message"].(string)
	}

	// If tag is empty or not directly matched, check if content starts with a known tag prefix
	src := TagToSource(tag)
	if src == "" && content != "" {
		cTrim := strings.TrimSpace(content)
		for _, prefix := range []string{"twWifiScan", "twBlueScan", "twpcap", "twwinlog", "twsdrpower"} {
			if strings.HasPrefix(strings.ToLower(cTrim), strings.ToLower(prefix)) {
				tag = prefix
				src = TagToSource(tag)
				break
			}
		}
	}

	if src == "" || content == "" {
		return
	}

	host, _ := sl["hostname"].(string)
	if host == "" {
		host, _ = sl["host"].(string)
	}
	if host == "" {
		if client, ok := sl["client"].(string); ok && client != "" {
			if h, _, err := net.SplitHostPort(client); err == nil {
				host = h
			} else {
				host = client
			}
		}
	}
	if host == "" {
		host = "localhost"
	}

	tNano := time.Now().UnixNano()
	if ts, ok := sl["timestamp"].(time.Time); ok && !ts.IsZero() {
		tNano = ts.UnixNano()
	} else if tsUnix, ok := sl["time"].(int64); ok && tsUnix > 0 {
		tNano = tsUnix
	}

	sev := 6
	if v, ok := sl["severity"].(float64); ok {
		sev = int(v)
	} else if v, ok := sl["severity"].(int); ok {
		sev = v
	}

	rec := Record{
		Time:     tNano,
		Host:     host,
		Tag:      tag,
		Content:  content,
		Severity: sev,
	}

	select {
	case e.syslogQueue <- rec:
	default:
		slog.Warn("logreport engine: syslog queue full, dropping record", "tag", tag, "host", host)
	}
}

// CleanTag extracts and cleans the syslog tag from a tag string.
func CleanTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimSuffix(tag, ":")
	tag = strings.TrimSpace(tag)
	if idx := strings.Index(tag, "["); idx >= 0 {
		tag = strings.TrimSpace(tag[:idx])
	}
	return tag
}

// TagToSource maps syslog tag name to internal source identifier.
func TagToSource(tag string) string {
	cleaned := CleanTag(tag)
	switch strings.ToLower(cleaned) {
	case "twwifiscan":
		return SourceWifiScan
	case "twbluescan":
		return SourceBlueScan
	case "twpcap":
		return SourcePcap
	case "twwinlog":
		return SourceWinLog
	default:
		return ""
	}
}

// ProcessNetFlow is a hook for future NetFlow/IPFIX ingestion.
func (e *Engine) ProcessNetFlow(record any) {
	// To be expanded in upcoming iterations
}

// ProcessSFlow is a hook for future sFlow ingestion.
func (e *Engine) ProcessSFlow(record any) {
	// To be expanded in upcoming iterations
}

// ProcessMQTT is a hook for future MQTT topic / payload ingestion.
func (e *Engine) ProcessMQTT(topic string, payload []byte) {
	// To be expanded in upcoming iterations
}

// ProcessOTel is a hook for future OpenTelemetry metrics / trace ingestion.
func (e *Engine) ProcessOTel(record any) {
	// To be expanded in upcoming iterations
}

// Start launches the ingestion worker loop and periodic cleanup scheduler.
func (e *Engine) Start(ctx context.Context) error {
	e.wg.Add(1)
	go e.workerLoop(ctx)
	return nil
}

// Stop stops workers and waits for queue drain and flush.
func (e *Engine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
}

// Flush triggers an immediate synchronous commit of all queued items.
func (e *Engine) Flush() {
	done := make(chan struct{})
	select {
	case e.flushCh <- done:
		<-done
	case <-time.After(5 * time.Second):
	}
}

func (e *Engine) workerLoop(ctx context.Context) {
	defer e.wg.Done()

	flushTicker := time.NewTicker(e.flushInterval)
	defer flushTicker.Stop()

	cleanTicker := time.NewTicker(e.cleanInterval)
	defer cleanTicker.Stop()

	var session *Session
	ensureSession := func() *Session {
		if session == nil {
			session = NewSession(ctx, e.store)
		}
		return session
	}

	commitSession := func() {
		if session != nil && session.Processed > 0 {
			if err := session.Commit(); err != nil {
				slog.Error("logreport engine: failed to commit session", "error", err)
			}
			session = nil
		}
	}

	for {
		select {
		case <-ctx.Done():
			commitSession()
			return
		case <-e.stopCh:
			// Drain remaining items from queue
			for {
				select {
				case rec := <-e.syslogQueue:
					src := TagToSource(rec.Tag)
					if src != "" {
						ensureSession().Process(src, rec)
					}
				default:
					commitSession()
					return
				}
			}
		case done := <-e.flushCh:
			draining := true
			for draining {
				select {
				case rec := <-e.syslogQueue:
					src := TagToSource(rec.Tag)
					if src != "" {
						ensureSession().Process(src, rec)
					}
				default:
					commitSession()
					close(done)
					draining = false
				}
			}
		case rec := <-e.syslogQueue:
			src := TagToSource(rec.Tag)
			if src != "" {
				s := ensureSession()
				s.Process(src, rec)
				if s.Processed >= e.maxBatchSize {
					commitSession()
				}
			}
		case <-flushTicker.C:
			commitSession()
		case <-cleanTicker.C:
			commitSession()
			if e.store != nil {
				_ = CleanupAll(ctx, e.store, 0, 0)
			}
		}
	}
}
