package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// ArpLogPoller monitors ARP events (new IP/MAC, changes, conflicts).
type ArpLogPoller struct {
	store    datastore.DataStore
	logStore *parquet.Store
}

func NewArpLogPoller(store datastore.DataStore, logStore *parquet.Store) *ArpLogPoller {
	return &ArpLogPoller{
		store:    store,
		logStore: logStore,
	}
}

func (p *ArpLogPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	start := time.Now()
	st := time.Now().Add(-time.Duration(pe.PollInt) * time.Second).UnixNano()
	if pe.Result != nil {
		if lt, ok := pe.Result["lastTime"].(float64); ok && int64(lt) > 0 {
			st = int64(lt)
		}
	}
	et := time.Now().UnixNano()

	filter := pe.Filter

	var logs []*parquet.ParquetLogRecord
	if p.logStore != nil {
		l, _ := p.logStore.Query(ctx, parquet.LogFilter{
			Type:      "arpwatch",
			StartTime: st,
			EndTime:   et,
			Limit:     5000,
		})
		logs = l
	}

	count := 0
	type miniArp struct {
		IP    string `json:"ip"`
		MAC   string `json:"mac"`
		State string `json:"state"`
	}

	ipMap := make(map[string]int)
	macMap := make(map[string]int)
	stateMap := make(map[string]int)

	for _, rec := range logs {
		var ae miniArp
		_ = json.Unmarshal([]byte(rec.Log), &ae)
		if filter != "" && ae.State != filter {
			continue
		}
		count++
		if ae.IP != "" {
			ipMap[ae.IP]++
		}
		if ae.MAC != "" {
			macMap[ae.MAC]++
		}
		if ae.State != "" {
			stateMap[ae.State]++
		}
	}

	fields := map[string]interface{}{
		"count":    float64(count),
		"IPs":      float64(len(ipMap)),
		"MACs":     float64(len(macMap)),
		"states":   float64(len(stateMap)),
		"pattern":  float64(len(ipMap) + len(macMap)),
		"lastTime": float64(et),
		"rtt":      float64(time.Since(start).Nanoseconds()),
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
			Message: fmt.Sprintf("arplog count: %d matching events", count),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateHigh, Message: fmt.Sprintf("arplog script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: time.Since(start), Message: fmt.Sprintf("arplog count: %d, script passed", count), Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: time.Since(start), Message: fmt.Sprintf("arplog count: %d, threshold breached", count), Fields: fields}, nil
}
