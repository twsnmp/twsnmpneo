package parquet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/parquet-go/parquet-go"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
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
	Level     string // Severity/level filter (e.g. warn, low, high, 0..7)
	Tag       string // Tag / Type filter
}

// Config holds configuration for the Parquet store.
type Config struct {
	Dir            string
	BufferSize     int
	BufferInterval time.Duration
}

type cachedPollingEntry struct {
	logs      []*datastore.PollingLogEnt
	limit     int
	expiresAt time.Time
}

type cachedDiskCountEntry struct {
	count     int
	expiresAt time.Time
}

// Store manages buffered log writes, Parquet file generation, and queries.
type Store struct {
	dir            string
	bufferSize     int
	bufferInterval time.Duration

	mu          sync.Mutex
	typeBuffers map[string][]*ParquetLogRecord
	bufferCount int

	cacheMu         sync.RWMutex
	pollingLogCache map[string]cachedPollingEntry
	diskCountCache  map[string]cachedDiskCountEntry

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
		dir:             cfg.Dir,
		bufferSize:      bufSize,
		bufferInterval:  bufInterval,
		typeBuffers:     make(map[string][]*ParquetLogRecord),
		pollingLogCache: make(map[string]cachedPollingEntry),
		diskCountCache:  make(map[string]cachedDiskCountEntry),
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
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
		record.Src = extractSrc(record.Log)
	}

	s.mu.Lock()
	s.typeBuffers[record.Type] = append(s.typeBuffers[record.Type], record)
	s.bufferCount++
	needsFlush := s.bufferCount >= s.bufferSize
	s.mu.Unlock()

	if record.Type == "polling" && record.Src != "" {
		ent := parsePollingLogRecord(record)
		s.cacheMu.Lock()
		if cached, ok := s.pollingLogCache[record.Src]; ok {
			cached.logs = append(cached.logs, ent)
			maxKeep := cached.limit
			if maxKeep < 200 {
				maxKeep = 200
			}
			if len(cached.logs) > maxKeep {
				cached.logs = cached.logs[len(cached.logs)-maxKeep:]
			}
			s.pollingLogCache[record.Src] = cached
		}
		s.cacheMu.Unlock()
	}

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
		} else {
			s.cacheMu.Lock()
			for k, entry := range s.diskCountCache {
				if strings.HasPrefix(k, logType+":") {
					entry.count += count
					s.diskCountCache[k] = entry
				}
			}
			s.cacheMu.Unlock()
		}
	}
	slog.Info("Flush completed", "syslog", sc, "netflow", nfc, "trap", tc, "arplog", ac, "sflow", sf, "other", oc, "duration", time.Since(st))
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

// CountRecentLogs counts records generated within duration for specified log types.
// Optimized to only scan specified log directories in reverse chronological order and break on cutoff.
func (s *Store) CountRecentLogs(ctx context.Context, d time.Duration, targetTypes ...string) map[string]int {
	if d <= 0 {
		d = 24 * time.Hour
	}
	cutoff := time.Now().Add(-d)
	cutoffNano := cutoff.UnixNano()
	counts := make(map[string]int)

	targetSet := make(map[string]bool)
	for _, t := range targetTypes {
		targetSet[t] = true
	}

	// 1. Count from in-memory buffers (chronological, scan backwards)
	s.mu.Lock()
	for logType, buf := range s.typeBuffers {
		if len(targetSet) > 0 && !targetSet[logType] {
			continue
		}
		c := 0
		for i := len(buf) - 1; i >= 0; i-- {
			if buf[i].Time >= cutoffNano {
				c++
			} else {
				break
			}
		}
		if c > 0 {
			counts[logType] += c
		}
	}
	s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return counts
	}

	// Determine types to inspect on disk
	inspectTypes := targetTypes
	if len(inspectTypes) == 0 {
		entries, err := os.ReadDir(s.dir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					inspectTypes = append(inspectTypes, e.Name())
				}
			}
		}
	}

	// 2. Count from disk parquet files in reverse chronological order (with 10s TTL cache per type)
	now := time.Now()
	for _, logType := range inspectTypes {
		if err := ctx.Err(); err != nil {
			break
		}
		cacheKey := fmt.Sprintf("%s:%d", logType, int64(d.Seconds()))
		s.cacheMu.RLock()
		cached, ok := s.diskCountCache[cacheKey]
		s.cacheMu.RUnlock()
		if ok && now.Before(cached.expiresAt) {
			if cached.count > 0 {
				counts[logType] += cached.count
			}
			continue
		}

		typeDir := filepath.Join(s.dir, logType)
		fileEntries, err := os.ReadDir(typeDir)
		if err != nil {
			s.cacheMu.Lock()
			s.diskCountCache[cacheKey] = cachedDiskCountEntry{count: 0, expiresAt: now.Add(5 * time.Minute)}
			s.cacheMu.Unlock()
			continue
		}

		var files []string
		for _, fe := range fileEntries {
			if !fe.IsDir() && strings.HasSuffix(fe.Name(), ".parquet") {
				files = append(files, fe.Name())
			}
		}
		if len(files) == 0 {
			s.cacheMu.Lock()
			s.diskCountCache[cacheKey] = cachedDiskCountEntry{count: 0, expiresAt: now.Add(5 * time.Minute)}
			s.cacheMu.Unlock()
			continue
		}
		sort.Strings(files)

		diskCount := 0
		filesScanned := 0
		scanDeadline := time.Now().Add(80 * time.Millisecond)
		// Scan backwards from newest file
		for i := len(files) - 1; i >= 0; i-- {
			if err := ctx.Err(); err != nil || time.Now().After(scanDeadline) {
				break
			}
			if filesScanned >= 15 {
				break
			}

			fileName := files[i]
			filePath := filepath.Join(typeDir, fileName)
			f, err := os.Open(filePath)
			if err != nil {
				continue
			}
			stat, err := f.Stat()
			if err != nil || stat.Size() == 0 {
				_ = f.Close()
				continue
			}

			// If file modification time is older than cutoff, all earlier files are older!
			if stat.ModTime().Before(cutoff.Add(-5 * time.Minute)) {
				_ = f.Close()
				break // Stop scanning older files in this directory!
			}

			filesScanned++
			if pf, err := parquet.OpenFile(f, stat.Size()); err == nil {
				diskCount += int(pf.NumRows())
			}
			_ = f.Close()
		}
		if diskCount > 0 {
			counts[logType] += diskCount
		}
		s.cacheMu.Lock()
		s.diskCountCache[cacheKey] = cachedDiskCountEntry{count: diskCount, expiresAt: now.Add(5 * time.Minute)}
		s.cacheMu.Unlock()
	}

	return counts
}

// CountLogs24h counts records generated within the given duration for each log type.
func (s *Store) CountLogs24h(ctx context.Context, d time.Duration) map[string]int {
	return s.CountRecentLogs(ctx, d)
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
		cleanType := filepath.Base(filepath.Clean(filter.Type))
		if cleanType != "." && cleanType != "/" && !strings.Contains(cleanType, "..") && filepath.IsLocal(cleanType) {
			targetDir := filepath.Clean(filepath.Join(s.dir, cleanType))
			rel, err := filepath.Rel(s.dir, targetDir)
			if err == nil && !strings.HasPrefix(rel, "..") {
				dirs = append(dirs, targetDir)
			}
		}
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

	results := make([]*ParquetLogRecord, 0, 256)

	var regexKeyword *regexp.Regexp
	if filter.Filter != "" {
		regexKeyword, _ = regexp.Compile("(?i)" + filter.Filter)
	}
	var regexSrc *regexp.Regexp
	if filter.Src != "" {
		regexSrc, _ = regexp.Compile("(?i)" + filter.Src)
	}
	var regexTag *regexp.Regexp
	if filter.Tag != "" {
		regexTag, _ = regexp.Compile("(?i)" + filter.Tag)
	}

	matchSyslogLevel := func(logStr string, targetLevel string) bool {
		targetLevel = strings.ToLower(targetLevel)
		if targetLevel == "" || targetLevel == "all" || targetLevel == "7" {
			return true
		}
		maxSev := -1
		switch targetLevel {
		case "high", "high+", "2":
			maxSev = 2
		case "low", "low+", "3":
			maxSev = 3
		case "warn", "warn+", "4":
			maxSev = 4
		case "info", "info+", "6":
			maxSev = 6
		}
		// Try finding "severity": N
		if idx := strings.Index(logStr, `"severity":`); idx != -1 {
			rest := strings.TrimSpace(logStr[idx+11:])
			if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '7' {
				sev := int(rest[0] - '0')
				if maxSev >= 0 {
					return sev <= maxSev
				}
			}
		} else if idx := strings.Index(logStr, `"Severity":`); idx != -1 {
			rest := strings.TrimSpace(logStr[idx+11:])
			if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '7' {
				sev := int(rest[0] - '0')
				if maxSev >= 0 {
					return sev <= maxSev
				}
			}
		}
		// String level check
		lower := strings.ToLower(logStr)
		if maxSev == 2 {
			return strings.Contains(lower, `"level":"high"`)
		} else if maxSev == 3 {
			return strings.Contains(lower, `"level":"high"`) || strings.Contains(lower, `"level":"low"`)
		} else if maxSev == 4 {
			return strings.Contains(lower, `"level":"high"`) || strings.Contains(lower, `"level":"low"`) || strings.Contains(lower, `"level":"warn"`)
		}
		return true
	}

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

		// Skip files whose modification time (with 5-minute safety margin) is older than filter.StartTime
		if filter.StartTime > 0 && stat.ModTime().Add(5*time.Minute).UnixNano() < filter.StartTime {
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
					if filter.Src != "" {
						if regexSrc != nil {
							if !regexSrc.MatchString(rec.Src) {
								continue
							}
						} else if !strings.Contains(strings.ToLower(rec.Src), strings.ToLower(filter.Src)) {
							continue
						}
					}
					if filter.Tag != "" {
						if regexTag != nil {
							if !regexTag.MatchString(rec.Log) {
								continue
							}
						} else if !strings.Contains(strings.ToLower(rec.Log), strings.ToLower(filter.Tag)) {
							continue
						}
					}
					if filter.Level != "" && filter.Level != "all" {
						if !matchSyslogLevel(rec.Log, filter.Level) {
							continue
						}
					}
					if filter.Filter != "" {
						if regexKeyword != nil {
							if !regexKeyword.MatchString(rec.Log) && !regexKeyword.MatchString(rec.Src) {
								continue
							}
						} else {
							kw := strings.ToLower(filter.Filter)
							if !strings.Contains(strings.ToLower(rec.Log), kw) && !strings.Contains(strings.ToLower(rec.Src), kw) {
								continue
							}
						}
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

	s.cacheMu.Lock()
	s.diskCountCache = make(map[string]cachedDiskCountEntry)
	if logType == "" || logType == "polling" {
		s.pollingLogCache = make(map[string]cachedPollingEntry)
	}
	s.cacheMu.Unlock()

	if logType != "" {
		cleanType := filepath.Base(filepath.Clean(logType))
		if cleanType == "." || cleanType == "/" || strings.Contains(cleanType, "..") || !filepath.IsLocal(cleanType) {
			return fmt.Errorf("invalid log type: %s", logType)
		}
		typeDir := filepath.Clean(filepath.Join(s.dir, cleanType))
		rel, err := filepath.Rel(s.dir, typeDir)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("invalid log type path: %s", logType)
		}
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

func parsePollingLogRecord(rec *ParquetLogRecord) *datastore.PollingLogEnt {
	ent := &datastore.PollingLogEnt{
		Time:      rec.Time,
		PollingID: rec.Src,
	}
	var payload struct {
		State  string                 `json:"State"`
		Result map[string]interface{} `json:"Result"`
	}
	if err := json.Unmarshal([]byte(rec.Log), &payload); err == nil && payload.Result != nil {
		ent.State = payload.State
		ent.Result = payload.Result
	} else {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(rec.Log), &raw); err == nil {
			ent.Result = raw
			if st, ok := raw["state"].(string); ok {
				ent.State = st
			}
		}
	}
	return ent
}

// GetRecentPollingLog retrieves up to `limit` most recent polling log entries in chronological order.
func (s *Store) GetRecentPollingLog(ctx context.Context, pollingID string, limit int) ([]*datastore.PollingLogEnt, error) {
	if limit <= 0 {
		limit = 100
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.cacheMu.RLock()
	cached, hasCached := s.pollingLogCache[pollingID]
	s.cacheMu.RUnlock()
	if hasCached && cached.limit >= limit {
		res := make([]*datastore.PollingLogEnt, len(cached.logs))
		copy(res, cached.logs)
		if len(res) > limit {
			res = res[len(res)-limit:]
		}
		return res, nil
	}

	var memResults []*datastore.PollingLogEnt

	// 1. Read un-flushed in-memory buffer (newest logs!)
	s.mu.Lock()
	if buf, ok := s.typeBuffers["polling"]; ok && len(buf) > 0 {
		for i := len(buf) - 1; i >= 0; i-- {
			rec := buf[i]
			if rec.Src == pollingID {
				memResults = append(memResults, parsePollingLogRecord(rec))
				if len(memResults) >= limit {
					break
				}
			}
		}
	}
	s.mu.Unlock()

	// Reverse memory results to chronological order (oldest first)
	for i, j := 0, len(memResults)-1; i < j; i, j = i+1, j-1 {
		memResults[i], memResults[j] = memResults[j], memResults[i]
	}

	if len(memResults) >= limit {
		s.cacheMu.Lock()
		s.pollingLogCache[pollingID] = cachedPollingEntry{logs: memResults, limit: limit}
		s.cacheMu.Unlock()
		return memResults, nil
	}

	pollingDir := filepath.Join(s.dir, "polling")
	entries, err := os.ReadDir(pollingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return memResults, nil
		}
		return nil, err
	}

	files := make([]string, 0)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".parquet") {
			files = append(files, filepath.Join(pollingDir, e.Name()))
		}
	}
	if len(files) == 0 {
		s.cacheMu.Lock()
		s.pollingLogCache[pollingID] = cachedPollingEntry{logs: memResults, limit: limit}
		s.cacheMu.Unlock()
		return memResults, nil
	}

	// Sort files oldest first
	sort.Strings(files)

	var diskResults []*datastore.PollingLogEnt
	filesChecked := 0
	scanDeadline := time.Now().Add(200 * time.Millisecond)
	// Scan at most 3 newest files with strict wall-clock deadline to guarantee fast response
	for i := len(files) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil || time.Now().After(scanDeadline) {
			break
		}
		if filesChecked >= 3 {
			break
		}
		filesChecked++

		filePath := files[i]
		f, err := os.Open(filePath)
		if err != nil {
			continue
		}
		stat, err := f.Stat()
		if err != nil || stat.Size() == 0 {
			_ = f.Close()
			continue
		}

		pf, err := parquet.OpenFile(f, stat.Size())
		if err != nil {
			_ = f.Close()
			continue
		}

		reader := parquet.NewGenericReader[ParquetLogRecord](pf)
		buf := make([]ParquetLogRecord, 256)
		var fileLogs []*datastore.PollingLogEnt
		rowsRead := 0
		for {
			if err := ctx.Err(); err != nil || time.Now().After(scanDeadline) {
				break
			}
			n, rErr := reader.Read(buf)
			rowsRead += n
			for j := 0; j < n; j++ {
				rec := &buf[j]
				if rec.Src != pollingID {
					continue
				}
				fileLogs = append(fileLogs, parsePollingLogRecord(rec))
			}
			if rErr != nil || rowsRead >= 2000 {
				break
			}
		}
		_ = reader.Close()
		_ = f.Close()

		diskResults = append(fileLogs, diskResults...)
		if len(diskResults)+len(memResults) >= limit {
			break
		}
	}

	allResults := make([]*datastore.PollingLogEnt, 0, len(diskResults)+len(memResults))
	allResults = append(allResults, diskResults...)
	allResults = append(allResults, memResults...)
	if len(allResults) > limit {
		allResults = allResults[len(allResults)-limit:]
	}

	s.cacheMu.Lock()
	s.pollingLogCache[pollingID] = cachedPollingEntry{
		logs:  allResults,
		limit: limit,
	}
	s.cacheMu.Unlock()

	return allResults, nil
}

// GetAllPollingLog retrieves all polling log entries for a given polling ID in chronological order.
func (s *Store) GetAllPollingLog(ctx context.Context, pollingID string) ([]*datastore.PollingLogEnt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_ = s.Flush()

	pollingDir := filepath.Join(s.dir, "polling")
	entries, err := os.ReadDir(pollingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	files := make([]string, 0)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".parquet") {
			files = append(files, filepath.Join(pollingDir, e.Name()))
		}
	}
	// Sort files oldest first (chronological order)
	sort.Strings(files)

	var results []*datastore.PollingLogEnt
	for _, filePath := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f, err := os.Open(filePath)
		if err != nil {
			continue
		}
		stat, err := f.Stat()
		if err != nil || stat.Size() == 0 {
			_ = f.Close()
			continue
		}

		pf, err := parquet.OpenFile(f, stat.Size())
		if err != nil {
			_ = f.Close()
			continue
		}

		reader := parquet.NewGenericReader[ParquetLogRecord](pf)
		buf := make([]ParquetLogRecord, 256)
		for {
			n, rErr := reader.Read(buf)
			for i := 0; i < n; i++ {
				rec := &buf[i]
				if rec.Src != pollingID {
					continue
				}
				var payload struct {
					State  string                 `json:"State"`
					Result map[string]interface{} `json:"Result"`
				}
				ent := &datastore.PollingLogEnt{
					Time:      rec.Time,
					PollingID: rec.Src,
				}
				if err := json.Unmarshal([]byte(rec.Log), &payload); err == nil && payload.Result != nil {
					ent.State = payload.State
					ent.Result = payload.Result
				} else {
					var raw map[string]interface{}
					if err := json.Unmarshal([]byte(rec.Log), &raw); err == nil {
						ent.Result = raw
						if st, ok := raw["state"].(string); ok {
							ent.State = st
						}
					}
				}
				results = append(results, ent)
			}
			if rErr != nil {
				break
			}
		}
		_ = reader.Close()
		_ = f.Close()
	}

	return results, nil
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

// GetTotalSize returns the total size in bytes of all files stored in the parquet log directory.
func (s *Store) GetTotalSize() int64 {
	if s == nil || s.dir == "" {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(s.dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && d != nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func extractSrc(logStr string) string {
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
