package polling

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// SnmpTrapPoller monitors received SNMP TRAP packets.
type SnmpTrapPoller struct {
	store    datastore.DataStore
	logStore *parquet.Store
}

func NewSnmpTrapPoller(store datastore.DataStore, logStore *parquet.Store) *SnmpTrapPoller {
	return &SnmpTrapPoller{
		store:    store,
		logStore: logStore,
	}
}

func (p *SnmpTrapPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	mode := strings.ToLower(pe.Mode)
	switch mode {
	case "", "count", "stats":
	default:
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("unsupported trap mode: %s", pe.Mode),
			Fields:  map[string]interface{}{"error": fmt.Sprintf("unsupported trap mode: %s", pe.Mode)},
		}, nil
	}
	start := time.Now()
	st := time.Now().Add(-time.Duration(pe.PollInt) * time.Second).UnixNano()
	if pe.Result != nil {
		if lt, ok := pe.Result["lastTime"].(float64); ok && int64(lt) > 0 {
			st = int64(lt)
		}
	}
	et := time.Now().UnixNano()

	host := pe.Params
	if host == "" && node != nil {
		host = node.IP
	}

	var reg *regexp.Regexp
	if pe.Filter != "" {
		r, err := regexp.Compile(pe.Filter)
		if err != nil {
			return &Result{State: StateHigh, Message: fmt.Sprintf("invalid trap filter regex: %v", err)}, nil
		}
		reg = r
	}

	var logs []*parquet.ParquetLogRecord
	if p.logStore != nil {
		l, _ := p.logStore.Query(ctx, parquet.LogFilter{
			Type:      "trap",
			StartTime: st,
			EndTime:   et,
			Src:       host,
			Limit:     5000,
		})
		logs = l
	}

	count := 0
	typeMap := make(map[string]int)
	for _, rec := range logs {
		msg := rec.Log
		if reg != nil && !reg.MatchString(msg) {
			continue
		}
		count++
		typeMap[rec.Src]++
	}

	fields := map[string]interface{}{
		"count":    float64(count),
		"lastTime": float64(et),
		"rtt":      float64(time.Since(start).Nanoseconds()),
		"sources":  float64(len(typeMap)),
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("count", float64(count))
	_ = vm.Set("interval", float64(pe.PollInt))
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	if pe.Script == "" {
		state := StateNormal
		if count > 0 && pe.Level != "off" {
			state = failureState(pe.Level)
		}
		return &Result{
			State:   state,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("snmptrap count: %d matching traps", count),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateHigh, Message: fmt.Sprintf("trap script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: time.Since(start), Message: fmt.Sprintf("trap count: %d, script passed", count), Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: time.Since(start), Message: fmt.Sprintf("trap count: %d, threshold breached", count), Fields: fields}, nil
}
