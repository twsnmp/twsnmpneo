package bbolt

import (
	"context"
	"sync"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"go.etcd.io/bbolt"
)

func (s *Store) getOrCreateLogReportMap(kind string) *sync.Map {
	if v, ok := s.logReports.Load(kind); ok {
		return v.(*sync.Map)
	}
	actual, _ := s.logReports.LoadOrStore(kind, &sync.Map{})
	return actual.(*sync.Map)
}

func (s *Store) getOrCreateDirtyLogReportMap(kind string) *sync.Map {
	if v, ok := s.dirtyLogReports.Load(kind); ok {
		return v.(*sync.Map)
	}
	actual, _ := s.dirtyLogReports.LoadOrStore(kind, &sync.Map{})
	return actual.(*sync.Map)
}

// GetLogReportData returns the raw JSON of one report entity from the in-memory sync.Map cache, or (nil, nil) if absent.
func (s *Store) GetLogReportData(_ context.Context, kind, id string) ([]byte, error) {
	if kind == "" || id == "" {
		return nil, datastore.ErrInvalidParams
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}
	v, ok := s.logReports.Load(kind)
	if !ok {
		return nil, nil
	}
	val, ok := v.(*sync.Map).Load(id)
	if !ok {
		return nil, nil
	}
	return append([]byte(nil), val.([]byte)...), nil
}

// ListLogReportData returns every entity of a report kind keyed by ID from the in-memory sync.Map cache.
func (s *Store) ListLogReportData(_ context.Context, kind string) (map[string][]byte, error) {
	if kind == "" {
		return nil, datastore.ErrInvalidParams
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}
	ret := make(map[string][]byte)
	v, ok := s.logReports.Load(kind)
	if !ok {
		return ret, nil
	}
	v.(*sync.Map).Range(func(k, val any) bool {
		ret[k.(string)] = append([]byte(nil), val.([]byte)...)
		return true
	})
	return ret, nil
}

// SaveLogReportData updates entities in the in-memory sync.Map cache and marks them dirty for periodic batch persistence.
func (s *Store) SaveLogReportData(_ context.Context, kind string, items map[string][]byte) error {
	if kind == "" {
		return datastore.ErrInvalidParams
	}
	if len(items) == 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	m := s.getOrCreateLogReportMap(kind)
	dm := s.getOrCreateDirtyLogReportMap(kind)
	for id, v := range items {
		if id == "" {
			continue
		}
		cp := append([]byte(nil), v...)
		m.Store(id, cp)
		dm.Store(id, struct{}{})
	}
	return nil
}

// DeleteLogReportData removes the given entity IDs of a report kind from memory and bbolt.
func (s *Store) DeleteLogReportData(_ context.Context, kind string, ids []string) error {
	if kind == "" {
		return datastore.ErrInvalidParams
	}
	if len(ids) == 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	if v, ok := s.logReports.Load(kind); ok {
		m := v.(*sync.Map)
		for _, id := range ids {
			m.Delete(id)
		}
	}
	if dv, ok := s.dirtyLogReports.Load(kind); ok {
		dm := dv.(*sync.Map)
		for _, id := range ids {
			dm.Delete(id)
		}
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		root := tx.Bucket(bucketLogReport)
		if root == nil {
			return nil
		}
		b := root.Bucket([]byte(kind))
		if b == nil {
			return nil
		}
		for _, id := range ids {
			if err := b.Delete([]byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

// ResetLogReportData removes all entities of a kind (or all kinds when kind is empty) from memory and bbolt.
func (s *Store) ResetLogReportData(_ context.Context, kind string) error {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	if kind == "" {
		s.logReports = sync.Map{}
		s.dirtyLogReports = sync.Map{}
	} else {
		s.logReports.Delete(kind)
		s.dirtyLogReports.Delete(kind)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		if kind == "" {
			if err := tx.DeleteBucket(bucketLogReport); err != nil && err != bbolt.ErrBucketNotFound {
				return err
			}
			return nil
		}
		root := tx.Bucket(bucketLogReport)
		if root == nil {
			return nil
		}
		if err := root.DeleteBucket([]byte(kind)); err != nil && err != bbolt.ErrBucketNotFound {
			return err
		}
		return nil
	})
}
