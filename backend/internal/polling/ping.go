package polling

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/ping"
)

type pingFunc func(string, int, int, int, int) *ping.PingEnt

// PingPoller performs connectivity checks via ICMP.
type PingPoller struct {
	doPing pingFunc
}

func NewPingPoller() *PingPoller {
	return &PingPoller{doPing: ping.DoPing}
}

func (p *PingPoller) ping(target string, timeout, retry, size, ttl int) *ping.PingEnt {
	if p.doPing != nil {
		return p.doPing(target, timeout, retry, size, ttl)
	}
	return ping.DoPing(target, timeout, retry, size, ttl)
}

func (p *PingPoller) Poll(_ context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if pe == nil {
		return &Result{State: StateUnknown, Message: "polling is nil"}, nil
	}
	if node == nil || node.IP == "" {
		return &Result{State: StateUnknown, Message: "node not found or missing IP address"}, nil
	}

	switch pe.Mode {
	case "line":
		return p.pollLine(pe, node.IP)
	case "smoke":
		return p.pollSmoke(pe, node.IP)
	default:
		return p.pollSingle(pe, node.IP)
	}
}

func parsePingParams(params string, smoke bool) (size, ttl, count int) {
	size, count = 64, 10
	if params == "" {
		return
	}
	if !strings.Contains(params, "=") {
		if n, err := strconv.Atoi(params); err == nil {
			size = n
		}
		return
	}
	for _, item := range strings.Split(params, ",") {
		keyValue := strings.SplitN(item, "=", 2)
		if len(keyValue) != 2 {
			continue
		}
		n, err := strconv.Atoi(keyValue[1])
		if err != nil {
			continue
		}
		switch keyValue[0] {
		case "ttl":
			if n > 0 && n < 256 {
				ttl = n
			}
		case "size":
			if n >= 0 && n < 3000 {
				size = n
			}
		case "count":
			if smoke && n > 0 && n <= 100 {
				count = n
			}
		}
	}
	return
}

func (p *PingPoller) pollSingle(pe *datastore.PollingEnt, target string) (*Result, error) {
	size, ttl, _ := parsePingParams(pe.Params, false)
	r := p.ping(target, pe.Timeout, pe.Retry, size, ttl)
	if r.Stat != ping.PingOK {
		errMsg := pingError(r)
		return &Result{
			State:   pingFailureState(pe),
			Message: fmt.Sprintf("ping failed: %s", errMsg),
			Fields: map[string]interface{}{
				"rtt":   float64(0),
				"ttl":   float64(0),
				"error": errMsg,
			},
		}, nil
	}

	rtt := time.Duration(r.Time)
	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("ping ok, rtt=%v, ttl=%d", rtt, r.RecvTTL),
		Fields: map[string]interface{}{
			"rtt": float64(r.Time),
			"ttl": float64(r.RecvTTL),
		},
	}, nil
}

func (p *PingPoller) pollLine(pe *datastore.PollingEnt, target string) (*Result, error) {
	rtts := make([]float64, 0, 5)
	speeds := make([]float64, 0, 5)
	failures := 0
	ttl := 0
	lastError := ""

	for i := 0; i < 20; i++ {
		r64 := p.ping(target, pe.Timeout, pe.Retry, 64, 0)
		if r64.Stat != ping.PingOK {
			lastError = pingError(r64)
			failures++
			continue
		}
		r1364 := p.ping(target, pe.Timeout, pe.Retry, 1364, 0)
		if r1364.Stat != ping.PingOK {
			lastError = pingError(r1364)
			failures++
			continue
		}
		if r64.Time == r1364.Time {
			failures++
			continue
		}

		ttl = r64.RecvTTL
		a := float64(64-1364) / float64(r64.Time-r1364.Time)
		b := float64(r64.Time) - a*64
		speed := a * (8 * 1000)
		if speed > 0 && speed < 1000 && b > 0 {
			rtts = append(rtts, b)
			speeds = append(speeds, speed)
			if len(speeds) >= 5 {
				break
			}
		} else {
			failures++
		}
	}

	fields := map[string]interface{}{
		"rtt":      float64(0),
		"rtt_cv":   float64(0),
		"ttl":      float64(ttl),
		"speed":    float64(0),
		"speed_cv": float64(0),
		"fail":     float64(failures),
	}
	if len(speeds) < 3 {
		fields["error"] = lastError
		return &Result{
			State:   pingFailureState(pe),
			Message: "line ping measurement failed",
			Fields:  fields,
		}, nil
	}

	meanRTT, rttCV := meanCV(rtts)
	meanSpeed, speedCV := meanCV(speeds)
	fields["rtt"] = meanRTT
	fields["rtt_cv"] = rttCV
	fields["speed"] = meanSpeed
	fields["speed_cv"] = speedCV
	return &Result{
		State:   StateNormal,
		RTT:     time.Duration(meanRTT),
		Message: fmt.Sprintf("line ping ok, rtt=%v, speed=%v Mbps", time.Duration(meanRTT), meanSpeed),
		Fields:  fields,
	}, nil
}

func meanCV(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	mean := 0.0
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))
	if mean == 0 {
		return 0, 0
	}
	variance := 0.0
	for _, value := range values {
		variance += (value - mean) * (value - mean)
	}
	variance /= float64(len(values))
	return mean, math.Sqrt(variance) / mean
}

func (p *PingPoller) pollSmoke(pe *datastore.PollingEnt, target string) (*Result, error) {
	size, ttl, count := parsePingParams(pe.Params, true)
	valid := make([]float64, 0, count)
	lossCount := 0
	lastTTL := 0
	lastError := ""

	for i := 0; i < count; i++ {
		if i > 0 {
			time.Sleep(200 * time.Millisecond)
		}
		r := p.ping(target, pe.Timeout, pe.Retry, size, ttl)
		if r.Stat == ping.PingOK {
			valid = append(valid, float64(r.Time))
			lastTTL = r.RecvTTL
			continue
		}
		lossCount++
		lastError = pingError(r)
	}

	lossRate := float64(lossCount) / float64(count) * 100
	var minRTT, maxRTT, avgRTT, medianRTT, jitterRTT float64
	if len(valid) > 0 {
		sort.Float64s(valid)
		minRTT, maxRTT = valid[0], valid[len(valid)-1]
		for _, value := range valid {
			avgRTT += value
		}
		avgRTT /= float64(len(valid))
		middle := len(valid) / 2
		if len(valid)%2 == 0 {
			medianRTT = (valid[middle-1] + valid[middle]) / 2
		} else {
			medianRTT = valid[middle]
		}
		jitterRTT = maxRTT - minRTT
	}

	fields := map[string]interface{}{
		"rtt":       avgRTT,
		"min":       minRTT,
		"max":       maxRTT,
		"avg":       avgRTT,
		"mean":      avgRTT,
		"median":    medianRTT,
		"jitter":    jitterRTT,
		"loss":      lossRate,
		"fail":      float64(lossCount),
		"lossCount": float64(lossCount),
		"count":     float64(count),
		"ttl":       float64(lastTTL),
	}
	if len(valid) == 0 {
		fields["error"] = lastError
	}

	state := pingFailureState(pe)
	if lossRate < 100 {
		state = StateNormal
	}
	if pe.Script != "" {
		ok, err := runPingScript(pe.Script, fields, pe.PollInt)
		if err != nil {
			message := fmt.Sprintf("invalid script err=%v", err)
			fields["error"] = message
			return &Result{
				State:   StateUnknown,
				Message: message,
				Fields:  fields,
			}, nil
		}
		if ok {
			state = StateNormal
		} else {
			state = pingFailureState(pe)
		}
	}

	return &Result{
		State:   state,
		RTT:     time.Duration(avgRTT),
		Message: fmt.Sprintf("ping smoke: avg=%v, loss=%.1f%%", time.Duration(avgRTT), lossRate),
		Fields:  fields,
	}, nil
}

func runPingScript(script string, fields map[string]interface{}, interval int) (bool, error) {
	vm := otto.New()
	for key, value := range fields {
		if err := vm.Set(key, value); err != nil {
			return false, fmt.Errorf("set script variable %q: %w", key, err)
		}
	}
	if err := vm.Set("interval", interval); err != nil {
		return false, fmt.Errorf("set script variable interval: %w", err)
	}
	value, err := vm.Run(script)
	if err != nil {
		return false, err
	}
	ok, err := value.ToBoolean()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func pingFailureState(pe *datastore.PollingEnt) string {
	if pe.Level != "" {
		return pe.Level
	}
	return StateHigh
}

func pingError(r *ping.PingEnt) string {
	if r.Error != nil {
		return r.Error.Error()
	}
	return r.Stat.String()
}
