package monitor

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/montanaflynn/stats"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	gopsnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

const (
	maxMonitorData = 12 * 24 * 7 // 1 week at 5 min intervals (2016 points)
)

// MonitorDataEnt represents a resource monitor snapshot matching twsnmpfk.
type MonitorDataEnt struct {
	Time         int64   `json:"Time"`
	CPU          float64 `json:"CPU"`
	Mem          float64 `json:"Mem"`
	MyCPU        float64 `json:"MyCPU"`
	MyMem        float64 `json:"MyMem"`
	Swap         float64 `json:"Swap"`
	Disk         float64 `json:"Disk"`
	Load         float64 `json:"Load"`
	Bytes        float64 `json:"Bytes"`
	Net          float64 `json:"Net"`
	Conn         int     `json:"Conn"`
	Proc         int     `json:"Proc"`
	DBSize       int64   `json:"DBSize"`
	HeapAlloc    int64   `json:"HeapAlloc"`
	Sys          int64   `json:"Sys"`
	NumGoroutine int     `json:"NumGoroutine"`
}

// Config holds monitor configuration.
type Config struct {
	DataDir   string
	BackupDir string
	Store     datastore.DataStore
	Interval  time.Duration
}

// Monitor manages system resource tracking.
type Monitor struct {
	dataDir        string
	backupDir      string
	store          datastore.DataStore
	interval       time.Duration
	mu             sync.RWMutex
	data           []*MonitorDataEnt
	startTime      time.Time
	lastDBSize     int64
	lastDBCalcTime time.Time
	isCalcRunning  bool
	dbCalcMu       sync.Mutex

	// Backup scheduler & state
	backupMu       sync.Mutex
	backupRunning  bool
	lastBackupTime int64
	lastBackupFile string
	lastBackupSize int64
	nextBackupTime int64
	backupCancel   context.CancelFunc
}

// New creates a new resource monitor.
func New(cfg Config) *Monitor {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	bDir := cfg.BackupDir
	if bDir == "" {
		bDir = filepath.Join(cfg.DataDir, "backup")
	}
	return &Monitor{
		dataDir:   cfg.DataDir,
		backupDir: bDir,
		store:     cfg.Store,
		interval:  interval,
		data:      make([]*MonitorDataEnt, 0, maxMonitorData),
		startTime: time.Now(),
	}
}

// Start begins periodic background resource monitoring.
func (m *Monitor) Start(ctx context.Context) {
	slog.Info("Starting system resource monitor", "interval", m.interval)

	ticker := time.NewTicker(m.interval)
	go func() {
		defer ticker.Stop()
		// Initial collection asynchronously so startup is not blocked
		m.UpdateNow()
		m.initBackupState(ctx)

		i := 0
		for {
			select {
			case <-ctx.Done():
				slog.Info("Stopped system resource monitor")
				return
			case <-ticker.C:
				m.UpdateNow()
				m.checkScheduledBackup(ctx)
				i++
				if i%12 == 0 { // Check alerts every hour
					m.checkResourceAlert(ctx)
				}
			}
		}
	}()
}

// UpdateNow collects a fresh resource snapshot.
func (m *Monitor) UpdateNow() *MonitorDataEnt {
	ent := &MonitorDataEnt{
		Time: time.Now().UnixNano(),
	}

	// CPU percentage
	if cpus, err := cpu.Percent(0, false); err == nil && len(cpus) > 0 {
		ent.CPU = cpus[0]
	}

	// Load average
	if l, err := load.Avg(); err == nil && l != nil {
		ent.Load = l.Load1
	}

	// Go runtime memory
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	ent.HeapAlloc = int64(ms.HeapAlloc)
	ent.Sys = int64(ms.Sys)
	ent.NumGoroutine = runtime.NumGoroutine()

	// Virtual and Swap Memory
	if v, err := mem.VirtualMemory(); err == nil && v != nil {
		ent.Mem = v.UsedPercent
	}
	if s, err := mem.SwapMemory(); err == nil && s != nil {
		ent.Swap = s.UsedPercent
	}

	// Disk Usage
	diskPath := m.dataDir
	if diskPath == "" {
		diskPath = "."
	}
	if d, err := disk.Usage(diskPath); err == nil && d != nil {
		ent.Disk = d.UsedPercent
	}

	// Net IO counters & traffic rate
	if n, err := gopsnet.IOCounters(true); err == nil {
		for _, nif := range n {
			if isMonitorIF(nif.Name) {
				ent.Bytes += float64(nif.BytesRecv) + float64(nif.BytesSent)
			}
		}
	}

	// Connections
	if conn, err := gopsnet.Connections("tcp"); err == nil {
		ent.Conn = len(conn)
	}

	// Processes
	if pids, err := process.Pids(); err == nil {
		ent.Proc = len(pids)
	}

	// Self Process
	pid := os.Getpid()
	if pr, err := process.NewProcess(int32(pid)); err == nil {
		if v, err := pr.CPUPercent(); err == nil {
			ent.MyCPU = v
		}
		if v, err := pr.MemoryPercent(); err == nil {
			ent.MyMem = float64(v)
		}
	}

	// Database Size (asynchronously cached to prevent disk walk contention)
	ent.DBSize = m.getDBSize()

	// Ring buffer maintain under mutex lock
	m.mu.Lock()
	if len(m.data) > 0 {
		prev := m.data[len(m.data)-1]
		if ent.Bytes >= prev.Bytes && ent.Time > prev.Time {
			timeDiffSec := float64(ent.Time-prev.Time) / 1e9
			if timeDiffSec > 0 {
				ent.Net = (ent.Bytes - prev.Bytes) * 8.0 / timeDiffSec
			}
		}

		// Avoid adding duplicate samples if called before half the configured interval
		minInterval := m.interval / 2
		if minInterval < 50*time.Millisecond {
			minInterval = 50 * time.Millisecond
		}
		if time.Since(time.Unix(0, prev.Time)) < minInterval {
			m.data[len(m.data)-1] = ent
			m.mu.Unlock()
			return ent
		}
	}
	if len(m.data) >= maxMonitorData {
		m.data = append(m.data[:0], m.data[1:]...)
	}
	m.data = append(m.data, ent)
	m.mu.Unlock()

	return ent
}

func (m *Monitor) getDBSize() int64 {
	m.dbCalcMu.Lock()
	cached := m.lastDBSize
	lastTime := m.lastDBCalcTime
	isCalculating := m.isCalcRunning
	needCalc := (cached == 0 || time.Since(lastTime) >= 10*time.Minute) && !isCalculating
	if needCalc {
		m.isCalcRunning = true
	}
	m.dbCalcMu.Unlock()

	if !needCalc {
		return cached
	}

	if cached == 0 {
		// First time: calculate synchronously to establish baseline
		size := m.calculateDBSize()
		m.dbCalcMu.Lock()
		m.lastDBSize = size
		m.lastDBCalcTime = time.Now()
		m.isCalcRunning = false
		m.dbCalcMu.Unlock()
		return size
	}

	// Subsequent runs: calculate asynchronously in background
	go func() {
		size := m.calculateDBSize()
		m.dbCalcMu.Lock()
		m.lastDBSize = size
		m.lastDBCalcTime = time.Now()
		m.isCalcRunning = false
		m.dbCalcMu.Unlock()
	}()

	return cached
}

func (m *Monitor) calculateDBSize() int64 {
	var totalSize int64
	dbFile := filepath.Join(m.dataDir, "twsnmpneo.db")
	if fi, err := os.Stat(dbFile); err == nil {
		totalSize += fi.Size()
	}
	logDir := filepath.Join(m.dataDir, "logs")
	_ = filepath.WalkDir(logDir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && d != nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				totalSize += info.Size()
			}
		}
		return nil
	})
	return totalSize
}

// GetData returns the list of collected monitor snapshots.
func (m *Monitor) GetData() []*MonitorDataEnt {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*MonitorDataEnt, len(m.data))
	copy(res, m.data)
	return res
}

// GetUptime returns system monitor uptime.
func (m *Monitor) GetUptime() time.Duration {
	return time.Since(m.startTime)
}

func isMonitorIF(n string) bool {
	if runtime.GOOS == "darwin" {
		if strings.HasPrefix(n, "utun") || strings.HasPrefix(n, "lo") {
			return false
		}
	}
	if runtime.GOOS == "linux" {
		if strings.HasPrefix(n, "lo") || strings.HasPrefix(n, "docker") || strings.HasPrefix(n, "veth") {
			return false
		}
	}
	return true
}

func (m *Monitor) checkResourceAlert(ctx context.Context) {
	m.mu.RLock()
	if len(m.data) < 2 {
		m.mu.RUnlock()
		return
	}
	mems := make([]float64, 0, len(m.data))
	myMems := make([]float64, 0, len(m.data))
	loads := make([]float64, 0, len(m.data))
	var latestDisk float64
	for _, d := range m.data {
		mems = append(mems, d.Mem)
		myMems = append(myMems, d.MyMem)
		loads = append(loads, d.Load)
		latestDisk = d.Disk
	}
	m.mu.RUnlock()

	memMean, _ := stats.Mean(mems)
	myMemMean, _ := stats.Mean(myMems)
	loadMean, _ := stats.Mean(loads)

	if m.store == nil {
		return
	}

	// Memory alert
	level := ""
	if myMemMean > 90.0 && memMean > 90.0 {
		level = "high"
	} else if myMemMean > 80.0 && memMean > 80.0 {
		level = "low"
	} else if myMemMean > 60.0 && memMean > 60.0 {
		level = "warn"
	}
	if level != "" {
		_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "system",
			Level: level,
			Event: fmt.Sprintf(i18n.Trans("Memory usage warning (Host: %.1f%%, Process: %.1f%%)"), memMean, myMemMean),
		})
	}

	// Storage alert
	level = ""
	if latestDisk > 95.0 {
		level = "high"
	} else if latestDisk > 90.0 {
		level = "low"
	}
	if level != "" {
		_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "system",
			Level: level,
			Event: fmt.Sprintf(i18n.Trans("Storage usage warning (Disk: %.1f%%)"), latestDisk),
		})
	}

	// Load alert
	if loadMean > float64(runtime.NumCPU()) {
		_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "system",
			Level: "high",
			Event: fmt.Sprintf(i18n.Trans("CPU high load warning (Load Avg: %.2f / CPU count: %d)"), loadMean, runtime.NumCPU()),
		})
	}
}

// Backup creates a copy of the database to twsnmpneo.db.<timestamp> in backupDir.
func (m *Monitor) Backup() (string, int64, error) {
	return m.BackupWithOptions(context.Background(), false)
}

// BackupWithOptions creates a backup with configurable configOnly setting and event logging.
func (m *Monitor) BackupWithOptions(ctx context.Context, configOnly bool) (string, int64, error) {
	m.backupMu.Lock()
	if m.backupRunning {
		m.backupMu.Unlock()
		return "", 0, fmt.Errorf("backup is already running")
	}
	bCtx, cancel := context.WithCancel(ctx)
	m.backupCancel = cancel
	m.backupRunning = true
	m.backupMu.Unlock()

	defer func() {
		m.backupMu.Lock()
		m.backupRunning = false
		m.backupCancel = nil
		m.backupMu.Unlock()
	}()

	st := time.Now()
	timestamp := st.Format("20060102150405")
	var backupName string
	if configOnly {
		backupName = fmt.Sprintf("twsnmpneo.db.config.%s", timestamp)
	} else {
		backupName = fmt.Sprintf("twsnmpneo.db.%s", timestamp)
	}
	dstFile := filepath.Join(m.backupDir, backupName)

	if m.store != nil {
		_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "system",
			Level: "info",
			Event: fmt.Sprintf(i18n.Trans("Database backup started: %s"), backupName),
		})
	}

	var size int64
	var err error
	if m.store != nil {
		size, err = m.store.BackupDB(bCtx, dstFile, configOnly)
	} else {
		srcFile := filepath.Join(m.dataDir, "twsnmpneo.db")
		_ = os.MkdirAll(filepath.Dir(dstFile), 0755)
		if err = copyRawFile(srcFile, dstFile); err == nil {
			if fi, sErr := os.Stat(dstFile); sErr == nil {
				size = fi.Size()
			}
		}
	}

	if err != nil {
		slog.Error("Database backup failed", "file", backupName, "error", err)
		if m.store != nil {
			_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "system",
				Level: "warn",
				Event: fmt.Sprintf(i18n.Trans("Database backup failed: %v"), err),
			})
		}
		return "", 0, err
	}

	m.backupMu.Lock()
	m.lastBackupTime = time.Now().UnixNano()
	m.lastBackupFile = backupName
	m.lastBackupSize = size
	m.backupMu.Unlock()

	slog.Info("Database backup completed", "file", backupName, "size", size, "duration", time.Since(st))
	if m.store != nil {
		_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "system",
			Level: "info",
			Event: fmt.Sprintf(i18n.Trans("Database backup completed: %s (%s, %v)"), backupName, renderByteSize(size), time.Since(st).Truncate(time.Millisecond)),
		})

		// Rotate old backups according to retention settings
		conf, cErr := m.store.GetBackupConf(ctx)
		if cErr == nil && conf != nil && conf.Generation >= 0 {
			_ = m.store.RotateBackups(m.backupDir, conf.Generation)
		}
	}

	return backupName, size, nil
}

// StopBackup cancels an ongoing backup if running.
func (m *Monitor) StopBackup() bool {
	m.backupMu.Lock()
	defer m.backupMu.Unlock()
	if m.backupRunning && m.backupCancel != nil {
		m.backupCancel()
		slog.Info("Requested cancellation of active database backup")
		return true
	}
	return false
}

// GetBackupDir returns the directory where backups are stored.
func (m *Monitor) GetBackupDir() string {
	return m.backupDir
}

// GetBackupStatus returns the current backup status including configuration and file list.
func (m *Monitor) GetBackupStatus(ctx context.Context) (*datastore.BackupStatusEnt, error) {
	if m.store == nil {
		return nil, datastore.ErrDBNotOpen
	}

	conf, err := m.store.GetBackupConf(ctx)
	if err != nil {
		return nil, err
	}

	files, _ := m.store.ListBackups(m.backupDir)

	m.backupMu.Lock()
	running := m.backupRunning
	lastTime := m.lastBackupTime
	lastFile := m.lastBackupFile
	lastSize := m.lastBackupSize
	nextTime := m.nextBackupTime
	m.backupMu.Unlock()

	// If last backup not in memory yet, pick from most recent file
	if lastTime == 0 && len(files) > 0 {
		lastTime = files[0].ModTime
		lastFile = files[0].Name
		lastSize = files[0].Size
	}

	return &datastore.BackupStatusEnt{
		Config:         *conf,
		Running:        running,
		LastBackupTime: lastTime,
		LastBackupFile: lastFile,
		LastBackupSize: lastSize,
		NextBackupTime: nextTime,
		Backups:        files,
	}, nil
}

func (m *Monitor) initBackupState(ctx context.Context) {
	if m.store == nil {
		return
	}
	conf, err := m.store.GetBackupConf(ctx)
	if err != nil || conf == nil {
		return
	}

	// Initialize last backup from disk files
	files, _ := m.store.ListBackups(m.backupDir)
	if len(files) > 0 {
		m.backupMu.Lock()
		m.lastBackupTime = files[0].ModTime
		m.lastBackupFile = files[0].Name
		m.lastBackupSize = files[0].Size
		m.backupMu.Unlock()
	}

	m.calculateNextBackup(conf)
}

func (m *Monitor) calculateNextBackup(conf *datastore.BackupConfEnt) {
	if conf == nil || conf.Mode == "" {
		m.backupMu.Lock()
		m.nextBackupTime = 0
		m.backupMu.Unlock()
		return
	}

	now := time.Now()
	schedTime := conf.ScheduleTime
	if schedTime == "" {
		schedTime = "03:00"
	}
	hour, min := 3, 0
	if parts := strings.Split(schedTime, ":"); len(parts) == 2 {
		if h, err := time.Parse("15", parts[0]); err == nil {
			hour = h.Hour()
		}
		if mVal, err := time.Parse("04", parts[1]); err == nil {
			min = mVal.Minute()
		}
	}

	target := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if !now.Before(target) {
		target = target.Add(24 * time.Hour)
	}

	m.backupMu.Lock()
	m.nextBackupTime = target.UnixNano()
	m.backupMu.Unlock()
}

func (m *Monitor) checkScheduledBackup(ctx context.Context) {
	if m.store == nil {
		return
	}

	conf, err := m.store.GetBackupConf(ctx)
	if err != nil || conf == nil || conf.Mode == "" {
		return
	}

	m.backupMu.Lock()
	running := m.backupRunning
	next := m.nextBackupTime
	m.backupMu.Unlock()

	if running {
		return
	}

	nowUnix := time.Now().UnixNano()

	// Handle immediate modes
	isImmediate := conf.Mode == "once" || conf.Mode == "once_daily"
	isDue := next > 0 && nowUnix >= next

	if !isImmediate && !isDue {
		if next == 0 {
			m.calculateNextBackup(conf)
		}
		return
	}

	// Trigger backup
	configOnly := conf.ConfigOnly
	go func() {
		_, _, _ = m.BackupWithOptions(context.Background(), configOnly)

		// Transition mode after immediate backup
		if conf.Mode == "once" {
			conf.Mode = ""
			_ = m.store.SaveBackupConf(context.Background(), conf)
			m.backupMu.Lock()
			m.nextBackupTime = 0
			m.backupMu.Unlock()
		} else if conf.Mode == "once_daily" {
			conf.Mode = "daily"
			_ = m.store.SaveBackupConf(context.Background(), conf)
			m.calculateNextBackup(conf)
		} else {
			m.calculateNextBackup(conf)
		}
	}()
}

func renderByteSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
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

