package bbolt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"go.etcd.io/bbolt"
)

// makeSensorID returns a deterministic hex hash ID matching twsnmpfc.
func makeSensorID(host, sensorType, param string) string {
	raw := fmt.Sprintf("%s:%s:%s", host, sensorType, param)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// ListSensors returns all sensors sorted by LastTime descending.
func (s *Store) ListSensors(_ context.Context) ([]*datastore.SensorEnt, error) {
	var list []*datastore.SensorEnt
	s.sensors.Range(func(_, v any) bool {
		if sn, ok := v.(*datastore.SensorEnt); ok {
			cp := *sn
			cp.StatsLen = len(sn.Stats)
			cp.MonitorsLen = len(sn.Monitors)
			list = append(list, &cp)
		}
		return true
	})
	return list, nil
}

// GetSensor returns a single sensor by ID.
func (s *Store) GetSensor(_ context.Context, id string) (*datastore.SensorEnt, error) {
	if v, ok := s.sensors.Load(id); ok {
		if sn, ok := v.(*datastore.SensorEnt); ok {
			cp := *sn
			cp.StatsLen = len(sn.Stats)
			cp.MonitorsLen = len(sn.Monitors)
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("sensor not found")
}

// SaveSensor persists a sensor to memory cache and bbolt.
func (s *Store) SaveSensor(_ context.Context, sensor *datastore.SensorEnt) error {
	if sensor == nil || sensor.ID == "" {
		return fmt.Errorf("invalid sensor")
	}
	sensor.StatsLen = len(sensor.Stats)
	sensor.MonitorsLen = len(sensor.Monitors)
	s.sensors.Store(sensor.ID, sensor)

	data, err := json.Marshal(sensor)
	if err != nil {
		return fmt.Errorf("marshal sensor: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSensor)
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return b.Put([]byte(sensor.ID), data)
	})
}

// DeleteSensors removes specific sensors by IDs.
func (s *Store) DeleteSensors(_ context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		s.sensors.Delete(id)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSensor)
		if b == nil {
			return nil
		}
		for _, id := range ids {
			_ = b.Delete([]byte(id))
		}
		return nil
	})
}

// DeleteAllSensors purges all sensors from memory and disk.
func (s *Store) DeleteAllSensors(_ context.Context) error {
	s.sensors.Range(func(k, _ any) bool {
		s.sensors.Delete(k)
		return true
	})

	return s.db.Update(func(tx *bbolt.Tx) error {
		_ = tx.DeleteBucket(bucketSensor)
		_, err := tx.CreateBucketIfNotExists(bucketSensor)
		return err
	})
}

// ToggleSensorIgnore toggles the Ignore state of a sensor.
func (s *Store) ToggleSensorIgnore(_ context.Context, id string) error {
	v, ok := s.sensors.Load(id)
	if !ok {
		return fmt.Errorf("sensor not found")
	}
	sn, ok := v.(*datastore.SensorEnt)
	if !ok {
		return fmt.Errorf("invalid sensor entity")
	}

	sn.Ignore = !sn.Ignore
	if sn.Ignore {
		sn.State = "off"
	} else {
		sn.State = "normal"
	}
	s.sensors.Store(sn.ID, sn)

	data, err := json.Marshal(sn)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSensor)
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return b.Put([]byte(sn.ID), data)
	})
}

// UpdateSensor updates or registers a sensor receiving data.
func (s *Store) UpdateSensor(host, sensorType, param string, count int64) {
	if host == "" {
		return
	}
	if count <= 0 {
		count = 1
	}
	id := makeSensorID(host, sensorType, param)
	now := time.Now().UnixNano()

	if v, ok := s.sensors.Load(id); ok {
		sn := v.(*datastore.SensorEnt)
		sn.Total += count
		sn.Send++
		sn.LastTime = now
		if !sn.Ignore && sn.State != "normal" && sn.State != "off" {
			sn.State = "normal"
		}
		return
	}

	// Also check fallback without param if param is empty
	if param == "" {
		idOld := fmt.Sprintf("%s:%s:", host, sensorType)
		if v, ok := s.sensors.Load(idOld); ok {
			sn := v.(*datastore.SensorEnt)
			sn.Total += count
			sn.Send++
			sn.LastTime = now
			return
		}
	}

	newSn := &datastore.SensorEnt{
		ID:        id,
		Host:      host,
		Type:      sensorType,
		Param:     param,
		Total:     count,
		Send:      1,
		State:     "normal",
		FirstTime: now,
		LastTime:  now,
	}
	s.sensors.Store(id, newSn)

	// Async persist to bbolt
	go func(ent *datastore.SensorEnt) {
		data, err := json.Marshal(ent)
		if err == nil {
			_ = s.db.Update(func(tx *bbolt.Tx) error {
				b := tx.Bucket(bucketSensor)
				if b != nil {
					return b.Put([]byte(ent.ID), data)
				}
				return nil
			})
		}
	}(newSn)
}

// CheckSensorStats appends a stats telemetry sample to a sensor.
func (s *Store) CheckSensorStats(host, sensorType, param string, count, send, total int64, ps float64) {
	if host == "" {
		return
	}
	if send < 1 {
		send = 1
	}
	id := makeSensorID(host, sensorType, param)
	now := time.Now().UnixNano()

	statItem := datastore.SensorStatsEnt{
		Time:     now,
		Total:    total,
		Count:    count,
		Send:     send,
		LastSend: send,
		PS:       ps,
	}

	if v, ok := s.sensors.Load(id); ok {
		sn := v.(*datastore.SensorEnt)
		sn.Total += count
		sn.Send += send
		sn.Stats = append(sn.Stats, statItem)
		if len(sn.Stats) > 2880 {
			sn.Stats = sn.Stats[len(sn.Stats)-2880:]
		}
		sn.StatsLen = len(sn.Stats)
		sn.LastTime = now
		return
	}

	newSn := &datastore.SensorEnt{
		ID:          id,
		Host:        host,
		Type:        sensorType,
		Param:       param,
		Total:       count,
		Send:        send,
		State:       "normal",
		Stats:       []datastore.SensorStatsEnt{statItem},
		StatsLen:    1,
		FirstTime:   now,
		LastTime:    now,
	}
	s.sensors.Store(id, newSn)
}

// CheckSensorMonitor appends a system resource monitor sample to a sensor.
func (s *Store) CheckSensorMonitor(host, sensorType, param string, cpu, mem, load, txSpeed, rxSpeed float64, sent, recv, proc int64) {
	if host == "" {
		return
	}
	id := makeSensorID(host, sensorType, param)
	now := time.Now().UnixNano()

	monItem := datastore.SensorMonitorEnt{
		Time:    now,
		CPU:     cpu,
		Mem:     mem,
		Load:    load,
		TxSpeed: txSpeed,
		RxSpeed: rxSpeed,
		Sent:    sent,
		Recv:    recv,
		Process: proc,
	}

	if v, ok := s.sensors.Load(id); ok {
		sn := v.(*datastore.SensorEnt)
		sn.Monitors = append(sn.Monitors, monItem)
		if len(sn.Monitors) > 2880 {
			sn.Monitors = sn.Monitors[len(sn.Monitors)-2880:]
		}
		sn.MonitorsLen = len(sn.Monitors)
		sn.LastTime = now
		return
	}

	newSn := &datastore.SensorEnt{
		ID:          id,
		Host:        host,
		Type:        sensorType,
		Param:       param,
		Total:       1,
		Send:        1,
		State:       "normal",
		Monitors:    []datastore.SensorMonitorEnt{monItem},
		MonitorsLen: 1,
		FirstTime:   now,
		LastTime:    now,
	}
	s.sensors.Store(id, newSn)
}

// EvaluateSensorStates checks sensor timeouts and updates sensor states.
func (s *Store) EvaluateSensorStates(ctx context.Context) {
	now := time.Now().UnixNano()
	to := 1 // default 1 hour timeout
	warnCutoff := time.Now().Add(-time.Duration(to) * time.Hour).UnixNano()
	lowCutoff := time.Now().Add(-time.Duration(to*6) * time.Hour).UnixNano()
	highCutoff := time.Now().Add(-time.Duration(to*24) * time.Hour).UnixNano()

	var dirtySensors []*datastore.SensorEnt

	s.sensors.Range(func(_, v any) bool {
		sn, ok := v.(*datastore.SensorEnt)
		if !ok {
			return true
		}
		oldState := sn.State

		if sn.Ignore {
			sn.State = "off"
		} else if sn.LastTime < highCutoff {
			sn.State = "high"
		} else if sn.LastTime < lowCutoff {
			sn.State = "low"
		} else if sn.LastTime < warnCutoff {
			sn.State = "warn"
		} else {
			sn.State = "normal"
		}

		if oldState != sn.State {
			eventMsg := fmt.Sprintf("センサー状態変化:%s,%s,%s", sn.Host, sn.Type, sn.Param)
			_ = s.AddEventLog(ctx, &datastore.EventLogEnt{
				Type:     "sensor",
				Level:    sn.State,
				NodeName: sn.Host,
				Event:    eventMsg,
			})
			slog.Info("Sensor state changed", "host", sn.Host, "type", sn.Type, "state", sn.State)
		}

		// Calculate periodic stats for continuous feed protocols (syslog, netflow, sflow)
		if sn.Type == "netflow" || sn.Type == "ipfix" || sn.Type == "syslog" || strings.HasPrefix(sn.Type, "otel") {
			count := sn.Total
			send := sn.Send
			if len(sn.Stats) > 0 {
				last := sn.Stats[len(sn.Stats)-1]
				count -= last.Total
				send -= last.LastSend
			}
			if count < 0 {
				count = 0
			}
			if send < 0 {
				send = 0
			}
			sn.Stats = append(sn.Stats, datastore.SensorStatsEnt{
				Time:     now,
				Total:    sn.Total,
				Count:    count,
				Send:     send,
				LastSend: sn.Send,
				PS:       float64(count) / 300.0, // 5 min interval rate
			})
			if len(sn.Stats) > 2880 {
				sn.Stats = sn.Stats[len(sn.Stats)-2880:]
			}
			sn.StatsLen = len(sn.Stats)
		}

		dirtySensors = append(dirtySensors, sn)
		return true
	})

	// Batch persist updated sensors
	if len(dirtySensors) > 0 {
		_ = s.db.Batch(func(tx *bbolt.Tx) error {
			b := tx.Bucket(bucketSensor)
			if b == nil {
				return nil
			}
			for _, sn := range dirtySensors {
				data, err := json.Marshal(sn)
				if err == nil {
					_ = b.Put([]byte(sn.ID), data)
				}
			}
			return nil
		})
	}
}
