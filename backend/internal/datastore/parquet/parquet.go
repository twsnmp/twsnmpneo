package parquet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/parquet-go/parquet-go"
)

// ParquetLogRecord defines the schema for columnar log persistence.
type ParquetLogRecord struct {
	Time      int64  `parquet:"time,snappy" json:"time"`
	Timestamp int64  `parquet:"timestamp,timestamp(nanosecond),snappy" json:"timestamp"`
	Type      string `parquet:"type,dict,snappy" json:"type"`
	Src       string `parquet:"src,dict,snappy" json:"src"`
	Log       string `parquet:"log,zstd" json:"log"`
}

// LogFilter defines query search options across Parquet files.
type LogFilter struct {
	StartTime int64  // UnixNano
	EndTime   int64  // UnixNano
	Type      string // Log type (syslog, trap, netflow, etc.)
	Src       string // Source filter
	Filter    string // Text search keyword
	Limit     int
}

// Config holds configuration for the Parquet store.
type Config struct {
	Dir            string
	BufferSize     int
	BufferInterval time.Duration
}

// Store manages buffered log writes, Parquet file generation, and queries.
type Store struct {
	dir            string
	bufferSize     int
	bufferInterval time.Duration

	mu          sync.Mutex
	typeBuffers map[string][]*ParquetLogRecord
	bufferCount int

	stopCh chan struct{}
	doneCh chan struct{}
	closed bool
}

// New creates and initializes a Parquet log store.
func New(cfg Config) (*Store, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("empty parquet directory path")
	}
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, fmt.Errorf("create parquet directory: %w", err)
	}

	bufSize := cfg.BufferSize
	if bufSize <= 0 {
		bufSize = 5000
	}
	bufInterval := cfg.BufferInterval
	if bufInterval <= 0 {
		bufInterval = 10 * time.Second
	}

	s := &Store{
		dir:            cfg.Dir,
		bufferSize:     bufSize,
		bufferInterval: bufInterval,
		typeBuffers:    make(map[string][]*ParquetLogRecord),
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}

	go s.flushLoop()
	slog.Info("Initialized Parquet log store", "dir", s.dir)
	return s, nil
}

func (s *Store) flushLoop() {
	defer close(s.doneCh)
	ticker := time.NewTicker(s.bufferInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = s.Flush()
		case <-s.stopCh:
			return
		}
	}
}

// WriteLog buffers a log record. If buffer exceeds limit, a flush is triggered.
func (s *Store) WriteLog(record *ParquetLogRecord) error {
	if record == nil {
		return fmt.Errorf("nil record")
	}
	if record.Time == 0 {
		record.Time = time.Now().UnixNano()
	}
	if record.Timestamp == 0 {
		record.Timestamp = record.Time
	}
	if record.Src == "" {
		record.Src = extractSrc(record.Type, record.Log)
	}

	s.mu.Lock()
	s.typeBuffers[record.Type] = append(s.typeBuffers[record.Type], record)
	s.bufferCount++
	needsFlush := s.bufferCount >= s.bufferSize
	s.mu.Unlock()

	if needsFlush {
		return s.Flush()
	}
	return nil
}

// Flush writes all in-memory buffered records to Parquet files on disk.
func (s *Store) Flush() error {
	st := time.Now()
	s.mu.Lock()
	if s.bufferCount == 0 {
		s.mu.Unlock()
		return nil
	}
	toFlush := s.typeBuffers
	s.typeBuffers = make(map[string][]*ParquetLogRecord)
	s.bufferCount = 0
	s.mu.Unlock()

	sc := 0
	nfc := 0
	tc := 0
	ac := 0
	sf := 0
	oc := 0

	var firstErr error
	for logType, records := range toFlush {
		count := len(records)
		if count == 0 {
			continue
		}
		switch logType {
		case "syslog":
			sc += count
		case "netflow":
			nfc += count
		case "trap", "snmptrap":
			tc += count
		case "arplog", "arp":
			ac += count
		case "sflow", "sflowCounter":
			sf += count
		default:
			oc += count
		}
		if err := s.writeParquetFile(logType, records); err != nil {
			slog.Error("Failed to flush parquet file", "type", logType, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	log.Printf("syslog=%d,netflow=%d,trap=%d,arplog=%d,sflow=%d,other=%d,dur=%v", sc, nfc, tc, ac, sf, oc, time.Since(st))
	return firstErr
}

func (s *Store) writeParquetFile(logType string, records []*ParquetLogRecord) error {
	typeDir := filepath.Join(s.dir, logType)
	if err := os.MkdirAll(typeDir, 0755); err != nil {
		return fmt.Errorf("create type dir: %w", err)
	}

	now := time.Now()
	randBytes := make([]byte, 4)
	_, _ = rand.Read(randBytes)
	fileName := fmt.Sprintf("%s_%s_%s.parquet",
		logType,
		now.Format("20060102150405"),
		hex.EncodeToString(randBytes),
	)
	filePath := filepath.Join(typeDir, fileName)

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create parquet file: %w", err)
	}
	defer f.Close()

	writer := parquet.NewGenericWriter[ParquetLogRecord](f)

	valRecords := make([]ParquetLogRecord, len(records))
	for i, r := range records {
		valRecords[i] = *r
	}

	_, err = writer.Write(valRecords)
	if err != nil {
		_ = writer.Close()
		return fmt.Errorf("write parquet data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close parquet writer: %w", err)
	}
	return nil
}

// CountByType returns the number of records stored for each log type.
func (s *Store) CountByType(ctx context.Context) (map[string]int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_ = s.Flush()

	counts := make(map[string]int64)

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return counts, nil
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		logType := e.Name()
		typeDir := filepath.Join(s.dir, logType)
		fileEntries, err := os.ReadDir(typeDir)
		if err != nil {
			continue
		}

		var total int64
		for _, fe := range fileEntries {
			if fe.IsDir() || !strings.HasSuffix(fe.Name(), ".parquet") {
				continue
			}
			filePath := filepath.Join(typeDir, fe.Name())
			f, err := os.Open(filePath)
			if err != nil {
				continue
			}
			stat, err := f.Stat()
			if err != nil {
				_ = f.Close()
				continue
			}
			pf, err := parquet.OpenFile(f, stat.Size())
			if err != nil {
				_ = f.Close()
				continue
			}
			total += pf.NumRows()
			_ = f.Close()
		}
		counts[logType] = total
	}
	return counts, nil
}

// Query searches across parquet files matching the filter.
func (s *Store) Query(ctx context.Context, filter LogFilter) ([]*ParquetLogRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// First flush any pending buffers so query results are up-to-date
	_ = s.Flush()

	limit := filter.Limit
	if limit <= 0 {
		limit = 10000
	}
	if limit > 20000 {
		limit = 20000
	}

	// Determine directories to scan
	dirs := make([]string, 0)
	if filter.Type != "" {
		dirs = append(dirs, filepath.Join(s.dir, filter.Type))
	} else {
		entries, err := os.ReadDir(s.dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, filepath.Join(s.dir, e.Name()))
			}
		}
	}

	// Collect parquet files
	files := make([]string, 0)
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".parquet") {
				files = append(files, filepath.Join(d, e.Name()))
			}
		}
	}

	// Sort files newest first (reverse lexicographical based on timestamp naming)
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})

	results := make([]*ParquetLogRecord, 0, limit)
	keyword := strings.ToLower(filter.Filter)

	for _, file := range files {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if len(results) >= limit {
			break
		}

		f, err := os.Open(file)
		if err != nil {
			continue
		}

		stat, err := f.Stat()
		if err != nil {
			_ = f.Close()
			continue
		}

		pf, err := parquet.OpenFile(f, stat.Size())
		if err != nil {
			_ = f.Close()
			continue
		}

		reader := parquet.NewGenericReader[ParquetLogRecord](pf)
		buffer := make([]ParquetLogRecord, 256)

		for {
			if len(results) >= limit || ctx.Err() != nil {
				break
			}
			n, err := reader.Read(buffer)
			if n > 0 {
				for i := 0; i < n; i++ {
					rec := buffer[i]
					if filter.StartTime > 0 && rec.Time < filter.StartTime {
						continue
					}
					if filter.EndTime > 0 && rec.Time > filter.EndTime {
						continue
					}
					if filter.Src != "" && !strings.Contains(rec.Src, filter.Src) {
						continue
					}
					if keyword != "" && !strings.Contains(strings.ToLower(rec.Log), keyword) {
						continue
					}
					cp := rec
					results = append(results, &cp)
					if len(results) >= limit {
						break
					}
				}
			}
			if err != nil {
				break
			}
		}
		_ = reader.Close()
		_ = f.Close()
	}

	// Sort descending by time
	sort.Slice(results, func(i, j int) bool {
		return results[i].Time > results[j].Time
	})
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Rotate removes files older than retentionDays.
func (s *Store) Rotate(retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deleted := 0

	err := filepath.Walk(s.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".parquet") && info.ModTime().Before(cutoff) {
			if rmErr := os.Remove(path); rmErr == nil {
				deleted++
			}
		}
		return nil
	})
	return deleted, err
}

// DeleteLogs removes all parquet files for the given logType, or all types if empty.
func (s *Store) DeleteLogs(_ context.Context, logType string) error {
	s.mu.Lock()
	if logType != "" {
		delete(s.typeBuffers, logType)
	} else {
		s.typeBuffers = make(map[string][]*ParquetLogRecord)
		s.bufferCount = 0
	}
	s.mu.Unlock()

	if logType != "" {
		typeDir := filepath.Join(s.dir, logType)
		return os.RemoveAll(typeDir)
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			_ = os.RemoveAll(filepath.Join(s.dir, e.Name()))
		}
	}
	return nil
}

// Close flushes all data and terminates background threads.
func (s *Store) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	close(s.stopCh)
	<-s.doneCh
	return s.Flush()
}

func extractSrc(logType, logStr string) string {
	if logStr == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(logStr), &m); err == nil {
		for _, key := range []string{"src", "Src", "host", "Host", "ip", "IP", "From", "from"} {
			if v, ok := m[key]; ok {
				return fmt.Sprintf("%v", v)
			}
		}
	}
	return ""
}
