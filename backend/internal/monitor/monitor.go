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
	DataDir  string
	Store    datastore.DataStore
	Interval time.Duration
}

// Monitor manages system resource tracking.
type Monitor struct {
	dataDir   string
	store     datastore.DataStore
	interval  time.Duration
	mu        sync.RWMutex
	data      []*MonitorDataEnt
	startTime time.Time
}

// New creates a new resource monitor.
func New(cfg Config) *Monitor {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &Monitor{
		dataDir:   cfg.DataDir,
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

		i := 0
		for {
			select {
			case <-ctx.Done():
				slog.Info("Stopped system resource monitor")
				return
			case <-ticker.C:
				m.UpdateNow()
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
	m.mu.Lock()
	defer m.mu.Unlock()

	if n, err := gopsnet.IOCounters(true); err == nil {
		for _, nif := range n {
			if isMonitorIF(nif.Name) {
				ent.Bytes += float64(nif.BytesRecv) + float64(nif.BytesSent)
			}
		}
		if len(m.data) > 0 {
			prev := m.data[len(m.data)-1]
			if ent.Bytes >= prev.Bytes && ent.Time > prev.Time {
				timeDiffSec := float64(ent.Time-prev.Time) / 1e9
				if timeDiffSec > 0 {
					ent.Net = (ent.Bytes - prev.Bytes) * 8.0 / timeDiffSec
				}
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

	// Database Size
	ent.DBSize = m.calculateDBSize()

	// Ring buffer maintain
	if len(m.data) >= maxMonitorData {
		m.data = append(m.data[:0], m.data[1:]...)
	}
	m.data = append(m.data, ent)

	return ent
}

func (m *Monitor) calculateDBSize() int64 {
	var totalSize int64
	dbFile := filepath.Join(m.dataDir, "twsnmpneo.db")
	if fi, err := os.Stat(dbFile); err == nil {
		totalSize += fi.Size()
	}
	logDir := filepath.Join(m.dataDir, "logs")
	_ = filepath.Walk(logDir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			totalSize += info.Size()
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

// Backup creates a copy of the database to backup_<timestamp>.db in dataDir.
func (m *Monitor) Backup() (string, int64, error) {
	srcFile := filepath.Join(m.dataDir, "twsnmpneo.db")
	src, err := os.Open(srcFile)
	if err != nil {
		return "", 0, fmt.Errorf("source db not found: %w", err)
	}
	defer src.Close()

	backupName := fmt.Sprintf("backup_%s.db", time.Now().Format("20060102_150405"))
	dstFile := filepath.Join(m.dataDir, backupName)
	dst, err := os.OpenFile(dstFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		return "", 0, fmt.Errorf("failed to copy database: %w", err)
	}

	slog.Info("Database backup completed", "file", backupName, "size", written)
	return backupName, written, nil
}
