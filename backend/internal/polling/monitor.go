package polling

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	gopsnet "github.com/shirou/gopsutil/v3/net"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/monitor"
)

// MonitorPoller monitors host system resources (CPU, Memory, Disk, etc.).
type MonitorPoller struct {
	sysMon *monitor.Monitor
}

func NewMonitorPoller(sysMon *monitor.Monitor) *MonitorPoller {
	return &MonitorPoller{
		sysMon: sysMon,
	}
}

func (p *MonitorPoller) Poll(_ context.Context, pe *datastore.PollingEnt, _ *datastore.NodeEnt) (*Result, error) {
	var mData *monitor.MonitorDataEnt
	if p.sysMon != nil {
		data := p.sysMon.GetData()
		if len(data) > 0 {
			mData = data[len(data)-1]
		}
	}

	// Fallback to on-demand collection if monitor instance is not available or empty
	if mData == nil {
		mData = collectMonitorSnapshot()
	}

	fields := map[string]interface{}{
		"cpu":       mData.CPU,
		"mem":       mData.Mem,
		"disk":      mData.Disk,
		"load":      mData.Load,
		"net":       mData.Net,
		"conn":      float64(mData.Conn),
		"myCpu":     mData.MyCPU,
		"myMem":     mData.MyMem,
		"goRoutine": float64(mData.NumGoroutine),
		"heap":      float64(mData.HeapAlloc),
		"swap":      mData.Swap,
		"lastTime":  float64(mData.Time),
	}

	script := pe.Script
	if script == "" {
		return &Result{
			State:   StateNormal,
			Message: fmt.Sprintf("system ok: CPU:%.1f%% Mem:%.1f%% Disk:%.1f%%", mData.CPU, mData.Mem, mData.Disk),
			Fields:  fields,
		}, nil
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	for k, v := range fields {
		_ = vm.Set(k, v)
	}
	_ = vm.Set("interval", float64(pe.PollInt))

	value, err := vm.Run(script)
	if err != nil {
		return &Result{
			State:   StateHigh,
			Message: fmt.Sprintf("monitor script error: %v", err),
			Fields:  fields,
		}, nil
	}

	pass, _ := value.ToBoolean()
	if pass {
		return &Result{
			State:   StateNormal,
			Message: fmt.Sprintf("monitor script passed: CPU:%.1f%% Mem:%.1f%%", mData.CPU, mData.Mem),
			Fields:  fields,
		}, nil
	}

	return &Result{
		State:   failureState(pe.Level),
		Message: fmt.Sprintf("monitor threshold exceeded: CPU:%.1f%% Mem:%.1f%%", mData.CPU, mData.Mem),
		Fields:  fields,
	}, nil
}

func collectMonitorSnapshot() *monitor.MonitorDataEnt {
	ent := &monitor.MonitorDataEnt{
		Time:         time.Now().UnixNano(),
		NumGoroutine: runtime.NumGoroutine(),
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	ent.HeapAlloc = int64(m.HeapAlloc)
	ent.Sys = int64(m.Sys)

	if v, err := mem.VirtualMemory(); err == nil {
		ent.Mem = v.UsedPercent
	}
	if s, err := mem.SwapMemory(); err == nil {
		ent.Swap = s.UsedPercent
	}
	if c, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(c) > 0 {
		ent.CPU = c[0]
	}
	if d, err := disk.Usage("/"); err == nil {
		ent.Disk = d.UsedPercent
	}
	if l, err := load.Avg(); err == nil {
		ent.Load = l.Load1
	}
	if n, err := gopsnet.IOCounters(false); err == nil && len(n) > 0 {
		ent.Bytes = float64(n[0].BytesRecv + n[0].BytesSent)
	}

	return ent
}
