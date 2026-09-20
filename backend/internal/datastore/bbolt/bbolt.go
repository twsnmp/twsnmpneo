package bbolt

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"go.etcd.io/bbolt"
)

var (
	bucketNodes     = []byte("nodes")
	bucketLines     = []byte("lines")
	bucketNetworks  = []byte("networks")
	bucketItems     = []byte("items")
	bucketPollings  = []byte("pollings")
	bucketConfig    = []byte("config")
	bucketEventLog  = []byte("eventlog")

	keyMapConf    = []byte("mapConf")
	keyNotifyConf = []byte("notifyConf")
	keyLocConf    = []byte("locConf")
)

// Store implements datastore.DataStore using bbolt.
type Store struct {
	db     *bbolt.DB
	path   string
	closed bool
	mu     sync.RWMutex

	// In-memory cache for ultra-fast queries
	nodes    sync.Map // string -> *datastore.NodeEnt
	lines    sync.Map // string -> *datastore.LineEnt
	networks sync.Map // string -> *datastore.NetworkEnt
	items    sync.Map // string -> *datastore.DrawItemEnt
	pollings sync.Map // string -> *datastore.PollingEnt

	mapConf    datastore.MapConfEnt
	notifyConf datastore.NotifyConfEnt
	locConf    datastore.LocConfEnt
	confMu     sync.RWMutex
}

// New opens or creates a bbolt database file and initializes buckets and caches.
func New(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 2 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open bbolt db: %w", err)
	}

	s := &Store{
		db:   db,
		path: dbPath,
	}

	// Initialize default buckets
	err = db.Update(func(tx *bbolt.Tx) error {
		buckets := [][]byte{
			bucketNodes,
			bucketLines,
			bucketNetworks,
			bucketItems,
			bucketPollings,
			bucketConfig,
			bucketEventLog,
		}
		for _, b := range buckets {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return fmt.Errorf("create bucket %s: %w", string(b), err)
			}
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init buckets: %w", err)
	}

	// Load existing data into memory cache
	if err := s.loadCache(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("load cache: %w", err)
	}

	slog.Info("Initialized bbolt datastore", "path", dbPath)
	return s, nil
}

func (s *Store) loadCache() error {
	return s.db.View(func(tx *bbolt.Tx) error {
		// Load nodes
		if b := tx.Bucket(bucketNodes); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var n datastore.NodeEnt
				if err := json.Unmarshal(v, &n); err == nil {
					s.nodes.Store(n.ID, &n)
				}
				return nil
			})
		}
		// Load lines
		if b := tx.Bucket(bucketLines); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var l datastore.LineEnt
				if err := json.Unmarshal(v, &l); err == nil {
					s.lines.Store(l.ID, &l)
				}
				return nil
			})
		}
		// Load networks
		if b := tx.Bucket(bucketNetworks); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var nw datastore.NetworkEnt
				if err := json.Unmarshal(v, &nw); err == nil {
					s.networks.Store(nw.ID, &nw)
				}
				return nil
			})
		}
		// Load items
		if b := tx.Bucket(bucketItems); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var item datastore.DrawItemEnt
				if err := json.Unmarshal(v, &item); err == nil {
					s.items.Store(item.ID, &item)
				}
				return nil
			})
		}
		// Load pollings
		if b := tx.Bucket(bucketPollings); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var p datastore.PollingEnt
				if err := json.Unmarshal(v, &p); err == nil {
					s.pollings.Store(p.ID, &p)
				}
				return nil
			})
		}
		// Load configs
		if b := tx.Bucket(bucketConfig); b != nil {
			if v := b.Get(keyMapConf); v != nil {
				_ = json.Unmarshal(v, &s.mapConf)
			} else {
				s.initDefaultMapConf()
			}
			if v := b.Get(keyNotifyConf); v != nil {
				_ = json.Unmarshal(v, &s.notifyConf)
			}
			if v := b.Get(keyLocConf); v != nil {
				_ = json.Unmarshal(v, &s.locConf)
			}
		}
		return nil
	})
}

func (s *Store) initDefaultMapConf() {
	s.mapConf = datastore.MapConfEnt{
		MapName:        "TWSNMP NEO",
		PollInt:        60,
		Timeout:        1,
		Retry:          1,
		LogDays:        14,
		SnmpMode:       "v2c",
		Community:      "public",
		EnableArpWatch: true,
		IconSize:       2,
		LogFormat:      "parquet",
	}
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

// Generate unique ID
func makeID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// --- Node Operations ---

func (s *Store) GetNode(_ context.Context, id string) (*datastore.NodeEnt, error) {
	if val, ok := s.nodes.Load(id); ok {
		return val.(*datastore.NodeEnt), nil
	}
	return nil, datastore.ErrNotFound
}

func (s *Store) ListNodes(_ context.Context) ([]*datastore.NodeEnt, error) {
	res := make([]*datastore.NodeEnt, 0)
	s.nodes.Range(func(_, val any) bool {
		res = append(res, val.(*datastore.NodeEnt))
		return true
	})
	sort.Slice(res, func(i, j int) bool {
		return res[i].Name < res[j].Name
	})
	return res, nil
}

func (s *Store) SaveNode(_ context.Context, node *datastore.NodeEnt) error {
	if node == nil {
		return datastore.ErrInvalidParams
	}
	if node.ID == "" {
		node.ID = makeID()
	}
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("marshal node: %w", err)
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketNodes).Put([]byte(node.ID), data)
	})
	if err != nil {
		return err
	}
	s.nodes.Store(node.ID, node)
	return nil
}

func (s *Store) DeleteNode(_ context.Context, id string) error {
	if id == "" {
		return datastore.ErrInvalidID
	}
	err := s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketNodes).Delete([]byte(id))
	})
	if err != nil {
		return err
	}
	s.nodes.Delete(id)
	return nil
}

// --- Line Operations ---

func (s *Store) GetLine(_ context.Context, id string) (*datastore.LineEnt, error) {
	if val, ok := s.lines.Load(id); ok {
		return val.(*datastore.LineEnt), nil
	}
	return nil, datastore.ErrNotFound
}

func (s *Store) ListLines(_ context.Context) ([]*datastore.LineEnt, error) {
	res := make([]*datastore.LineEnt, 0)
	s.lines.Range(func(_, val any) bool {
		res = append(res, val.(*datastore.LineEnt))
		return true
	})
	return res, nil
}

func (s *Store) SaveLine(_ context.Context, line *datastore.LineEnt) error {
	if line == nil {
		return datastore.ErrInvalidParams
	}
	if line.ID == "" {
		line.ID = makeID()
	}
	data, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("marshal line: %w", err)
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketLines).Put([]byte(line.ID), data)
	})
	if err != nil {
		return err
	}
	s.lines.Store(line.ID, line)
	return nil
}

func (s *Store) DeleteLine(_ context.Context, id string) error {
	if id == "" {
		return datastore.ErrInvalidID
	}
	err := s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketLines).Delete([]byte(id))
	})
	if err != nil {
		return err
	}
	s.lines.Delete(id)
	return nil
}

// --- Network Operations ---

func (s *Store) GetNetwork(_ context.Context, id string) (*datastore.NetworkEnt, error) {
	if val, ok := s.networks.Load(id); ok {
		return val.(*datastore.NetworkEnt), nil
	}
	return nil, datastore.ErrNotFound
}

func (s *Store) ListNetworks(_ context.Context) ([]*datastore.NetworkEnt, error) {
	res := make([]*datastore.NetworkEnt, 0)
	s.networks.Range(func(_, val any) bool {
		res = append(res, val.(*datastore.NetworkEnt))
		return true
	})
	return res, nil
}

func (s *Store) SaveNetwork(_ context.Context, nw *datastore.NetworkEnt) error {
	if nw == nil {
		return datastore.ErrInvalidParams
	}
	if nw.ID == "" {
		nw.ID = makeID()
	}
	data, err := json.Marshal(nw)
	if err != nil {
		return fmt.Errorf("marshal network: %w", err)
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketNetworks).Put([]byte(nw.ID), data)
	})
	if err != nil {
		return err
	}
	s.networks.Store(nw.ID, nw)
	return nil
}

func (s *Store) DeleteNetwork(_ context.Context, id string) error {
	if id == "" {
		return datastore.ErrInvalidID
	}
	err := s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketNetworks).Delete([]byte(id))
	})
	if err != nil {
		return err
	}
	s.networks.Delete(id)
	return nil
}

// --- DrawItem Operations ---

func (s *Store) GetDrawItem(_ context.Context, id string) (*datastore.DrawItemEnt, error) {
	if val, ok := s.items.Load(id); ok {
		return val.(*datastore.DrawItemEnt), nil
	}
	return nil, datastore.ErrNotFound
}

func (s *Store) ListDrawItems(_ context.Context) ([]*datastore.DrawItemEnt, error) {
	res := make([]*datastore.DrawItemEnt, 0)
	s.items.Range(func(_, val any) bool {
		res = append(res, val.(*datastore.DrawItemEnt))
		return true
	})
	return res, nil
}

func (s *Store) SaveDrawItem(_ context.Context, item *datastore.DrawItemEnt) error {
	if item == nil {
		return datastore.ErrInvalidParams
	}
	if item.ID == "" {
		item.ID = makeID()
	}
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal item: %w", err)
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketItems).Put([]byte(item.ID), data)
	})
	if err != nil {
		return err
	}
	s.items.Store(item.ID, item)
	return nil
}

func (s *Store) DeleteDrawItem(_ context.Context, id string) error {
	if id == "" {
		return datastore.ErrInvalidID
	}
	err := s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketItems).Delete([]byte(id))
	})
	if err != nil {
		return err
	}
	s.items.Delete(id)
	return nil
}

// --- Polling Operations ---

func (s *Store) GetPolling(_ context.Context, id string) (*datastore.PollingEnt, error) {
	if val, ok := s.pollings.Load(id); ok {
		return val.(*datastore.PollingEnt), nil
	}
	return nil, datastore.ErrNotFound
}

func (s *Store) ListPollings(_ context.Context) ([]*datastore.PollingEnt, error) {
	res := make([]*datastore.PollingEnt, 0)
	s.pollings.Range(func(_, val any) bool {
		res = append(res, val.(*datastore.PollingEnt))
		return true
	})
	return res, nil
}

func (s *Store) SavePolling(_ context.Context, p *datastore.PollingEnt) error {
	if p == nil {
		return datastore.ErrInvalidParams
	}
	if p.ID == "" {
		p.ID = makeID()
	}
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal polling: %w", err)
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketPollings).Put([]byte(p.ID), data)
	})
	if err != nil {
		return err
	}
	s.pollings.Store(p.ID, p)
	return nil
}

func (s *Store) DeletePolling(_ context.Context, id string) error {
	if id == "" {
		return datastore.ErrInvalidID
	}
	err := s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketPollings).Delete([]byte(id))
	})
	if err != nil {
		return err
	}
	s.pollings.Delete(id)
	return nil
}

// --- Config Operations ---

func (s *Store) GetMapConf(_ context.Context) (*datastore.MapConfEnt, error) {
	s.confMu.RLock()
	defer s.confMu.RUnlock()
	c := s.mapConf
	return &c, nil
}

func (s *Store) SaveMapConf(_ context.Context, conf *datastore.MapConfEnt) error {
	if conf == nil {
		return datastore.ErrInvalidParams
	}
	data, err := json.Marshal(conf)
	if err != nil {
		return err
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketConfig).Put(keyMapConf, data)
	})
	if err != nil {
		return err
	}
	s.confMu.Lock()
	s.mapConf = *conf
	s.confMu.Unlock()
	return nil
}

func (s *Store) GetNotifyConf(_ context.Context) (*datastore.NotifyConfEnt, error) {
	s.confMu.RLock()
	defer s.confMu.RUnlock()
	c := s.notifyConf
	return &c, nil
}

func (s *Store) SaveNotifyConf(_ context.Context, conf *datastore.NotifyConfEnt) error {
	if conf == nil {
		return datastore.ErrInvalidParams
	}
	data, err := json.Marshal(conf)
	if err != nil {
		return err
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketConfig).Put(keyNotifyConf, data)
	})
	if err != nil {
		return err
	}
	s.confMu.Lock()
	s.notifyConf = *conf
	s.confMu.Unlock()
	return nil
}

func (s *Store) GetLocConf(_ context.Context) (*datastore.LocConfEnt, error) {
	s.confMu.RLock()
	defer s.confMu.RUnlock()
	c := s.locConf
	return &c, nil
}

func (s *Store) SaveLocConf(_ context.Context, conf *datastore.LocConfEnt) error {
	if conf == nil {
		return datastore.ErrInvalidParams
	}
	data, err := json.Marshal(conf)
	if err != nil {
		return err
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketConfig).Put(keyLocConf, data)
	})
	if err != nil {
		return err
	}
	s.confMu.Lock()
	s.locConf = *conf
	s.confMu.Unlock()
	return nil
}

// --- Event Log Operations ---

func (s *Store) AddEventLog(_ context.Context, event *datastore.EventLogEnt) error {
	if event == nil {
		return datastore.ErrInvalidParams
	}
	if event.Time == 0 {
		event.Time = time.Now().UnixNano()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event log: %w", err)
	}

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, uint64(event.Time))

	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketEventLog).Put(key, data)
	})
}

func (s *Store) ListEventLogs(_ context.Context, limit int) ([]*datastore.EventLogEnt, error) {
	if limit <= 0 {
		limit = 100
	}
	logs := make([]*datastore.EventLogEnt, 0, limit)

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEventLog)
		if b == nil {
			return nil
		}
		c := b.Cursor()
		count := 0
		for k, v := c.Last(); k != nil && count < limit; k, v = c.Prev() {
			var ev datastore.EventLogEnt
			if err := json.Unmarshal(v, &ev); err == nil {
				logs = append(logs, &ev)
				count++
			}
		}
		return nil
	})
	return logs, err
}
