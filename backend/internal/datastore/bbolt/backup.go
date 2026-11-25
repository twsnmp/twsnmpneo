package bbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"go.etcd.io/bbolt"
)

var configOnlyBuckets = map[string]bool{
	"nodes":           true,
	"lines":           true,
	"networks":        true,
	"items":           true,
	"pollings":        true,
	"config":          true,
	"users":           true,
	"certs":           true,
	"pkiCertificates": true,
	"certMonitor":     true,
	"sensor":          true,
}

// GetBackupConf retrieves the backup configuration from the in-memory cache.
func (s *Store) GetBackupConf(_ context.Context) (*datastore.BackupConfEnt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed || s.db == nil {
		return nil, datastore.ErrDBNotOpen
	}

	s.confMu.RLock()
	defer s.confMu.RUnlock()

	c := s.backupConf
	if c.ScheduleTime == "" {
		c.ScheduleTime = "03:00"
	}
	return &c, nil
}

// SaveBackupConf saves the backup configuration to the config bucket and updates the in-memory cache.
func (s *Store) SaveBackupConf(_ context.Context, conf *datastore.BackupConfEnt) error {
	if conf == nil {
		return datastore.ErrInvalidParams
	}
	if conf.ScheduleTime == "" {
		conf.ScheduleTime = "03:00"
	}

	data, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("marshal backup config: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed || s.db == nil {
		return datastore.ErrDBNotOpen
	}

	if err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketConfig)
		if b == nil {
			var err error
			b, err = tx.CreateBucketIfNotExists(bucketConfig)
			if err != nil {
				return err
			}
		}
		return b.Put(keyBackupConf, data)
	}); err != nil {
		return err
	}

	s.confMu.Lock()
	s.backupConf = *conf
	s.confMu.Unlock()
	return nil
}

// BackupDB performs a transaction-safe backup of the bbolt database.
// If configOnly is true, only configuration/topology buckets are copied.
func (s *Store) BackupDB(ctx context.Context, dstFile string, configOnly bool) (int64, error) {
	s.FlushAll()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed || s.db == nil {
		return 0, datastore.ErrDBNotOpen
	}

	dstDir := filepath.Dir(dstFile)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create backup dir: %w", err)
	}

	tmpFile := dstFile + ".tmp"
	_ = os.Remove(tmpFile)

	var copyErr error
	if !configOnly {
		// Full database backup using transaction-safe CopyFile
		copyErr = s.db.View(func(tx *bbolt.Tx) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			return tx.CopyFile(tmpFile, 0600)
		})
	} else {
		// Config-only backup: copy only config and topology buckets into a new bbolt file
		copyErr = s.copyConfigOnly(ctx, tmpFile)
	}

	if copyErr != nil {
		_ = os.Remove(tmpFile)
		return 0, fmt.Errorf("backup copy failed: %w", copyErr)
	}

	// Atomically move tmpFile to dstFile
	_ = os.Remove(dstFile)
	if err := os.Rename(tmpFile, dstFile); err != nil {
		_ = os.Remove(tmpFile)
		return 0, fmt.Errorf("failed to finalize backup file: %w", err)
	}

	fi, err := os.Stat(dstFile)
	if err != nil {
		return 0, fmt.Errorf("failed to stat backup file: %w", err)
	}

	return fi.Size(), nil
}

func (s *Store) copyConfigOnly(ctx context.Context, dstPath string) error {
	dstDB, err := bbolt.Open(dstPath, 0600, nil)
	if err != nil {
		return fmt.Errorf("open dest db: %w", err)
	}
	defer dstDB.Close()

	dstTx, err := dstDB.Begin(true)
	if err != nil {
		return fmt.Errorf("begin dest tx: %w", err)
	}
	defer func() {
		if dstTx != nil {
			_ = dstTx.Rollback()
		}
	}()

	err = s.db.View(func(srcTx *bbolt.Tx) error {
		return srcTx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			bucketName := string(name)
			if !configOnlyBuckets[bucketName] {
				// Skip non-config buckets (eventlog, otelTrace, otelMetric, arp, mqttStat, logReport, etc.)
				return nil
			}

			dstBkt, err := dstTx.CreateBucketIfNotExists(name)
			if err != nil {
				return fmt.Errorf("create bucket %s: %w", bucketName, err)
			}
			_ = dstBkt.SetSequence(b.Sequence())

			return copyBucketRecursive(ctx, b, dstBkt)
		})
	})
	if err != nil {
		return err
	}

	err = dstTx.Commit()
	dstTx = nil // prevent rollback in defer
	return err
}

func copyBucketRecursive(ctx context.Context, src *bbolt.Bucket, dst *bbolt.Bucket) error {
	return src.ForEach(func(k, v []byte) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if v == nil {
			// Sub-bucket
			subSrc := src.Bucket(k)
			if subSrc == nil {
				return nil
			}
			subDst, err := dst.CreateBucketIfNotExists(k)
			if err != nil {
				return fmt.Errorf("create sub-bucket %s: %w", string(k), err)
			}
			_ = subDst.SetSequence(subSrc.Sequence())
			return copyBucketRecursive(ctx, subSrc, subDst)
		}
		return dst.Put(k, v)
	})
}

// RotateBackups removes older backup files in backupDir when count exceeds maxGenerations + 1.
func (s *Store) RotateBackups(backupDir string, maxGenerations int) error {
	if maxGenerations < 0 {
		return nil
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read backup dir: %w", err)
	}

	type fileInfo struct {
		name    string
		modTime time.Time
	}

	var backups []fileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".tmp") {
			continue
		}
		if strings.HasPrefix(name, "twsnmpneo.db.") || strings.HasPrefix(name, "backup_") {
			info, err := e.Info()
			if err == nil {
				backups = append(backups, fileInfo{
					name:    name,
					modTime: info.ModTime(),
				})
			}
		}
	}

	keepCount := maxGenerations + 1
	if len(backups) <= keepCount {
		return nil
	}

	// Sort ascending by modification time (oldest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].modTime.Before(backups[j].modTime)
	})

	deleteCount := len(backups) - keepCount
	for i := 0; i < deleteCount; i++ {
		fileToDelete := filepath.Join(backupDir, backups[i].name)
		if err := os.Remove(fileToDelete); err != nil {
			slog.Warn("Failed to delete rotated backup", "file", fileToDelete, "error", err)
		} else {
			slog.Info("Rotated old backup file", "file", fileToDelete)
		}
	}

	return nil
}

// ListBackups lists all available backup files in backupDir sorted newest first.
func (s *Store) ListBackups(backupDir string) ([]*datastore.BackupFileInfo, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*datastore.BackupFileInfo{}, nil
		}
		return nil, fmt.Errorf("read backup dir: %w", err)
	}

	var list []*datastore.BackupFileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".tmp") {
			continue
		}
		if strings.HasPrefix(name, "twsnmpneo.db.") || strings.HasPrefix(name, "backup_") {
			info, err := e.Info()
			if err == nil {
				configOnly := strings.Contains(name, ".config.") || strings.Contains(name, "_config_")
				list = append(list, &datastore.BackupFileInfo{
					Name:       name,
					Size:       info.Size(),
					ModTime:    info.ModTime().UnixNano(),
					ConfigOnly: configOnly,
				})
			}
		}
	}

	// Sort newest first
	sort.Slice(list, func(i, j int) bool {
		return list[i].ModTime > list[j].ModTime
	})

	return list, nil
}

// resolveBackupFilePath validates a backup filename and resolves it against actual files in backupDir.
func resolveBackupFilePath(backupDir, name string) (string, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid backup filename: %s", name)
	}
	absDir, err := filepath.Abs(backupDir)
	if err != nil {
		return "", fmt.Errorf("invalid backup directory: %w", err)
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return "", fmt.Errorf("backup file not found: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if entry.Name() == name {
			target := filepath.Join(absDir, entry.Name())
			if !strings.HasPrefix(target, absDir+string(filepath.Separator)) {
				return "", fmt.Errorf("invalid backup filename: %s", name)
			}
			return target, nil
		}
	}
	return "", fmt.Errorf("backup file not found: %w", os.ErrNotExist)
}

// DeleteBackup deletes a specific backup file safely preventing path traversal.
func (s *Store) DeleteBackup(backupDir, name string) error {
	target, err := resolveBackupFilePath(backupDir, name)
	if err != nil {
		return err
	}
	return os.Remove(target)
}

// GetBackupFilePath validates and returns the full path of a backup file.
func (s *Store) GetBackupFilePath(backupDir, name string) (string, error) {
	return resolveBackupFilePath(backupDir, name)
}

// RestoreDB restores the database from a backup file.
// Before restoration, the current database file is automatically saved as <db>.pre_restore.<timestamp>.
func (s *Store) RestoreDB(backupFile string) error {
	s.FlushAll()

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file does not exist: %w", err)
	}

	if s.db != nil && !s.closed {
		_ = s.db.Close()
		s.closed = true
	}

	// Back up existing DB before overwrite if it exists
	if _, err := os.Stat(s.path); err == nil {
		preRestore := fmt.Sprintf("%s.pre_restore.%s", s.path, time.Now().Format("20060102150405"))
		if err := copyRawFile(s.path, preRestore); err != nil {
			return fmt.Errorf("failed to backup current db before restore: %w", err)
		}
		slog.Info("Current database saved before restore", "file", preRestore)
	}

	if err := copyRawFile(backupFile, s.path); err != nil {
		return fmt.Errorf("failed to copy backup to database path: %w", err)
	}

	// Reopen restored database
	db, err := bbolt.Open(s.path, 0600, &bbolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return fmt.Errorf("failed to reopen restored database: %w", err)
	}
	s.db = db
	s.closed = false

	// Clear and reload in-memory caches from the restored database
	s.nodes = sync.Map{}
	s.lines = sync.Map{}
	s.networks = sync.Map{}
	s.items = sync.Map{}
	s.pollings = sync.Map{}
	s.otelMetrics = sync.Map{}
	s.mqttStats = sync.Map{}
	s.sensors = sync.Map{}
	s.users = sync.Map{}
	s.aiResults = sync.Map{}
	s.arpTable = sync.Map{}
	s.certMonitors = sync.Map{}
	s.pkiCerts = sync.Map{}
	s.logReports = sync.Map{}
	s.dirtyNodes = sync.Map{}
	s.dirtyPollings = sync.Map{}
	s.dirtyArp = sync.Map{}
	s.dirtyLogReports = sync.Map{}
	s.recentEventsMu.Lock()
	s.recentEvents = nil
	s.recentEventsMu.Unlock()
	s.pendingEventsMu.Lock()
	s.pendingEvents = nil
	s.pendingEventsMu.Unlock()
	s.eventLogCount.Store(0)
	if err := s.loadCache(); err != nil {
		slog.Warn("Failed to reload cache after restore", "error", err)
	}

	slog.Info("Database restore completed successfully", "source", backupFile)
	return nil
}

// CompactDB compacts the bbolt database into dstFile.
func (s *Store) CompactDB(dstFile string) error {
	s.FlushAll()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed || s.db == nil {
		return datastore.ErrDBNotOpen
	}

	dstDir := filepath.Dir(dstFile)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("create compact dir: %w", err)
	}

	dst, err := bbolt.Open(dstFile, 0600, nil)
	if err != nil {
		return fmt.Errorf("open dst db for compaction: %w", err)
	}
	defer dst.Close()

	return bbolt.Compact(dst, s.db, 1024*1024*64)
}

func copyRawFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer d.Close()

	if _, err := io.Copy(d, s); err != nil {
		return err
	}
	return nil
}
