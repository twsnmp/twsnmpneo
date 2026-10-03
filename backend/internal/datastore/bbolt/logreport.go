package bbolt

import (
	"context"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"go.etcd.io/bbolt"
)

// GetLogReportData returns the raw JSON of one report entity, or (nil, nil) if absent.
func (s *Store) GetLogReportData(_ context.Context, kind, id string) ([]byte, error) {
	if kind == "" || id == "" {
		return nil, datastore.ErrInvalidParams
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}
	var ret []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		root := tx.Bucket(bucketLogReport)
		if root == nil {
			return nil
		}
		b := root.Bucket([]byte(kind))
		if b == nil {
			return nil
		}
		if v := b.Get([]byte(id)); v != nil {
			ret = append([]byte(nil), v...)
		}
		return nil
	})
	return ret, err
}

// ListLogReportData returns every entity of a report kind keyed by ID.
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
	err := s.db.View(func(tx *bbolt.Tx) error {
		root := tx.Bucket(bucketLogReport)
		if root == nil {
			return nil
		}
		b := root.Bucket([]byte(kind))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			ret[string(k)] = append([]byte(nil), v...)
			return nil
		})
	})
	return ret, err
}

// SaveLogReportData upserts entities of a report kind in a single transaction.
func (s *Store) SaveLogReportData(_ context.Context, kind string, items map[string][]byte) error {
	if kind == "" {
		return datastore.ErrInvalidParams
	}
	if len(items) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists(bucketLogReport)
		if err != nil {
			return err
		}
		b, err := root.CreateBucketIfNotExists([]byte(kind))
		if err != nil {
			return err
		}
		for id, v := range items {
			if id == "" {
				continue
			}
			if err := b.Put([]byte(id), v); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteLogReportData removes the given entity IDs of a report kind.
func (s *Store) DeleteLogReportData(_ context.Context, kind string, ids []string) error {
	if kind == "" {
		return datastore.ErrInvalidParams
	}
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
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

// ResetLogReportData removes all entities of a kind (or all kinds when kind is empty).
func (s *Store) ResetLogReportData(_ context.Context, kind string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
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
