package bbolt

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

var (
	bucketNodes       = []byte("nodes")
	bucketLines       = []byte("lines")
	bucketNetworks    = []byte("networks")
	bucketItems       = []byte("items")
	bucketPollings    = []byte("pollings")
	bucketConfig      = []byte("config")
	bucketEventLog    = []byte("eventlog")
	bucketArp         = []byte("arp")
	bucketOTelMetric  = []byte("otelMetric")
	bucketOTelTrace   = []byte("otelTrace")
	bucketMqttStat    = []byte("mqttStat")
	bucketPKICerts    = []byte("pkiCertificates")
	bucketCertMonitor = []byte("certMonitor")
	bucketLogReport   = []byte("logReport")
	bucketSensor      = []byte("sensor")
	bucketUsers       = []byte("users")

	keyMapConf           = []byte("mapConf")
	keyNotifyConf        = []byte("notifyConf")
	keyLocConf           = []byte("locConf")
	keyBackImage         = []byte("backImage")
	keyDiscoverConf      = []byte("discoverConf")
	keyNotifyOAuth2Token = []byte("notifyOAuth2Token")
	keyCustomIcons       = []byte("customIcons")
	keyAuthSecret        = []byte("authSecret")
)


// Store implements datastore.DataStore using bbolt.
type Store struct {
	db     *bbolt.DB
	path   string
	closed bool
	mu     sync.RWMutex

	// In-memory cache for ultra-fast queries
	nodes       sync.Map // string -> *datastore.NodeEnt
	lines       sync.Map // string -> *datastore.LineEnt
	networks    sync.Map // string -> *datastore.NetworkEnt
	items       sync.Map // string -> *datastore.DrawItemEnt
	pollings    sync.Map // string -> *datastore.PollingEnt
	otelMetrics sync.Map // string -> *datastore.OTelMetricEnt
	mqttStats   sync.Map // string -> *datastore.MqttStatEnt
	sensors     sync.Map // string -> *datastore.SensorEnt

	mapConf      datastore.MapConfEnt
	notifyConf   datastore.NotifyConfEnt
	locConf      datastore.LocConfEnt
	backImage    datastore.BackImageEnt
	discoverConf datastore.DiscoverConfEnt
	confMu       sync.RWMutex

	customIcons []*datastore.IconEnt
	iconsMu     sync.RWMutex

	// Notify OAuth2 token (in-memory cache)
	notifyOAuth2Token *oauth2.Token
	tokenMu           sync.RWMutex

	// Mail templates (in-memory cache loaded from disk or embedded defaults)
	mailTemplates map[string]string
	templateMu    sync.RWMutex
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
			bucketArp,
			bucketOTelMetric,
			bucketOTelTrace,
			bucketMqttStat,
			bucketPKICerts,
			bucketCertMonitor,
			bucketSensor,
			bucketUsers,
		}
		for _, b := range buckets {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return fmt.Errorf("create bucket %s: %w", string(b), err)
			}
		}

		// Initialize default user if none exists
		userBucket := tx.Bucket(bucketUsers)
		if userBucket != nil {
			var hasAdmin bool
			var userCount int
			_ = userBucket.ForEach(func(k, v []byte) error {
				userCount++
				var u datastore.UserEnt
				if err := json.Unmarshal(v, &u); err == nil {
					if u.Role == "admin" && len(u.PasswordHash) > 0 {
						hasAdmin = true
					}
				}
				return nil
			})

			// If no admin user exists, or no user exists at all, seed default twsnmp user
			twsnmpData := userBucket.Get([]byte("twsnmp"))
			if !hasAdmin || twsnmpData == nil || userCount == 0 {
				hash, err := bcrypt.GenerateFromPassword([]byte("twsnmp"), bcrypt.DefaultCost)
				if err != nil {
					return fmt.Errorf("hash default password: %w", err)
				}
				now := time.Now().Unix()
				defaultUser := datastore.UserEnt{
					User:         "twsnmp",
					Name:         "TWSNMP Administrator",
					PasswordHash: string(hash),
					Role:         "admin",
					CreatedAt:    now,
					UpdatedAt:    now,
				}
				data, err := json.Marshal(defaultUser)
				if err != nil {
					return fmt.Errorf("marshal default user: %w", err)
				}
				if err := userBucket.Put([]byte("twsnmp"), data); err != nil {
					return fmt.Errorf("save default user: %w", err)
				}
			} else if twsnmpData != nil {
				// Ensure twsnmp user has a valid password hash
				var u datastore.UserEnt
				if err := json.Unmarshal(twsnmpData, &u); err == nil {
					if u.PasswordHash == "" || !strings.HasPrefix(u.PasswordHash, "$2") {
						hash, err := bcrypt.GenerateFromPassword([]byte("twsnmp"), bcrypt.DefaultCost)
						if err == nil {
							u.PasswordHash = string(hash)
							if u.Role == "" {
								u.Role = "admin"
							}
							if data, err := json.Marshal(u); err == nil {
								_ = userBucket.Put([]byte("twsnmp"), data)
							}
						}
					}
				}
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
	err := s.db.View(func(tx *bbolt.Tx) error {
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
				var rawMap map[string]any
				if err := json.Unmarshal(v, &rawMap); err == nil {
					if _, hasOTel := rawMap["EnableOTel"]; !hasOTel {
						s.mapConf.EnableOTel = true
					}
					if s.mapConf.ReportDays <= 0 {
						s.mapConf.ReportDays = 30
					}
					if s.mapConf.ReportLimit <= 0 {
						s.mapConf.ReportLimit = 10000
					}
					if s.mapConf.ScoreThreshold <= 0 {
						s.mapConf.ScoreThreshold = 35.0
					}
					if s.mapConf.FumbleThreshold <= 0 {
						s.mapConf.FumbleThreshold = 10
					}
				}
			} else {
				s.initDefaultMapConf()
			}
			if v := b.Get(keyNotifyConf); v != nil {
				_ = json.Unmarshal(v, &s.notifyConf)
			}
			if v := b.Get(keyLocConf); v != nil {
				_ = json.Unmarshal(v, &s.locConf)
			}
			if v := b.Get(keyBackImage); v != nil {
				_ = json.Unmarshal(v, &s.backImage)
			}
			if v := b.Get(keyDiscoverConf); v != nil {
				_ = json.Unmarshal(v, &s.discoverConf)
			} else {
				s.discoverConf = datastore.DiscoverConfEnt{
					Timeout:     1,
					Retry:       1,
					AddPolling:  true,
					PortScan:    true,
					AddNetwork:  true,
					AutoLayout:  datastore.AutoLayoutNone,
					SnmpConfigs: []datastore.SnmpConfEnt{},
				}
			}
			// Load OAuth2 token
			if v := b.Get(keyNotifyOAuth2Token); v != nil {
				var t oauth2.Token
				if err := json.Unmarshal(v, &t); err == nil {
					s.notifyOAuth2Token = &t
				}
			}
			// Load Custom Icons
			if v := b.Get(keyCustomIcons); v != nil {
				var icons []*datastore.IconEnt
				if err := json.Unmarshal(v, &icons); err == nil {
					s.customIcons = icons
				}
			}
		}
		// Load OTel metrics
		if b := tx.Bucket(bucketOTelMetric); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var m datastore.OTelMetricEnt
				if err := json.Unmarshal(v, &m); err == nil {
					s.otelMetrics.Store(string(k), &m)
				}
				return nil
			})
		}
		// Load MQTT stats
		if b := tx.Bucket(bucketMqttStat); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var ms datastore.MqttStatEnt
				if err := json.Unmarshal(v, &ms); err == nil {
					s.mqttStats.Store(ms.ID, &ms)
				}
				return nil
			})
		}
		// Load Sensors
		if b := tx.Bucket(bucketSensor); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var sn datastore.SensorEnt
				if err := json.Unmarshal(v, &sn); err == nil {
					sn.StatsLen = len(sn.Stats)
					sn.MonitorsLen = len(sn.Monitors)
					s.sensors.Store(sn.ID, &sn)
				}
				return nil
			})
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Initialize mail templates map
	s.mailTemplates = make(map[string]string)

	// Clean up any orphaned pollings or lines left from previously deleted nodes
	s.cleanupOrphans()
	return nil
}

// cleanupOrphans purges pollings and lines whose referencing nodes or networks no longer exist (matches twsnmpfk spec).
func (s *Store) cleanupOrphans() {
	nodeMap := make(map[string]bool)
	s.nodes.Range(func(k, _ any) bool {
		nodeMap[k.(string)] = true
		return true
	})

	netMap := make(map[string]bool)
	s.networks.Range(func(k, _ any) bool {
		netMap[k.(string)] = true
		return true
	})

	var delPollings []string
	s.pollings.Range(func(k, v any) bool {
		p := v.(*datastore.PollingEnt)
		if p.NodeID != "" && !nodeMap[p.NodeID] {
			delPollings = append(delPollings, p.ID)
		}
		return true
	})

	var delLines []string
	s.lines.Range(func(k, v any) bool {
		l := v.(*datastore.LineEnt)
		ok1 := false
		if strings.HasPrefix(l.NodeID1, "NET:") {
			ok1 = netMap[strings.TrimPrefix(l.NodeID1, "NET:")]
		} else {
			ok1 = nodeMap[l.NodeID1]
		}

		ok2 := false
		if strings.HasPrefix(l.NodeID2, "NET:") {
			ok2 = netMap[strings.TrimPrefix(l.NodeID2, "NET:")]
		} else {
			ok2 = nodeMap[l.NodeID2]
		}

		if !ok1 || !ok2 {
			delLines = append(delLines, l.ID)
		}
		return true
	})

	if len(delPollings) > 0 || len(delLines) > 0 {
		_ = s.db.Update(func(tx *bbolt.Tx) error {
			if pb := tx.Bucket(bucketPollings); pb != nil {
				for _, pid := range delPollings {
					_ = pb.Delete([]byte(pid))
				}
			}
			if lb := tx.Bucket(bucketLines); lb != nil {
				for _, lid := range delLines {
					_ = lb.Delete([]byte(lid))
				}
			}
			return nil
		})
		for _, pid := range delPollings {
			s.pollings.Delete(pid)
		}
		for _, lid := range delLines {
			s.lines.Delete(lid)
		}
		slog.Info("Cleaned up orphaned entries", "pollings", len(delPollings), "lines", len(delLines))
	}
}

func (s *Store) initDefaultMapConf() {
	s.mapConf = datastore.MapConfEnt{
		MapName:         "TWSNMP NEO",
		PollInt:         60,
		Timeout:         1,
		Retry:           1,
		LogDays:         14,
		SnmpMode:        "v2c",
		Community:       "public",
		EnableSyslogd:   true,
		EnableTrapd:     true,
		EnableArpWatch:  true,
		EnableOTel:      true,
		OTelRetention:   24,
		ReportDays:      30,
		ReportLimit:     10000,
		ScoreThreshold:  35.0,
		FumbleThreshold: 10,
		IconSize:        2,
		LogFormat:       "parquet",
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

func (s *Store) SaveNodes(_ context.Context, nodes []*datastore.NodeEnt) error {
	if len(nodes) == 0 {
		return nil
	}
	return s.db.Batch(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		for _, node := range nodes {
			if node == nil {
				continue
			}
			if node.ID == "" {
				node.ID = makeID()
			}
			data, err := json.Marshal(node)
			if err != nil {
				return fmt.Errorf("marshal node %s: %w", node.ID, err)
			}
			if err := b.Put([]byte(node.ID), data); err != nil {
				return err
			}
			s.nodes.Store(node.ID, node)
		}
		return nil
	})
}

func (s *Store) DeleteNode(_ context.Context, id string) error {
	if id == "" {
		return datastore.ErrInvalidID
	}

	// 1. Identify pollings associated with this node
	var delPollings []string
	s.pollings.Range(func(k, v any) bool {
		p := v.(*datastore.PollingEnt)
		if p.NodeID == id {
			delPollings = append(delPollings, p.ID)
		}
		return true
	})

	// 2. Identify lines connected to this node or using its pollings
	var delLines []string
	s.lines.Range(func(k, v any) bool {
		l := v.(*datastore.LineEnt)
		if l.NodeID1 == id || l.NodeID2 == id {
			delLines = append(delLines, l.ID)
			return true
		}
		for _, pid := range delPollings {
			if l.PollingID1 == pid || l.PollingID2 == pid || l.PollingID == pid {
				delLines = append(delLines, l.ID)
				return true
			}
		}
		return true
	})

	// 3. Delete from DB atomically
	err := s.db.Update(func(tx *bbolt.Tx) error {
		if nb := tx.Bucket(bucketNodes); nb != nil {
			if err := nb.Delete([]byte(id)); err != nil {
				return err
			}
		}
		if pb := tx.Bucket(bucketPollings); pb != nil {
			for _, pid := range delPollings {
				_ = pb.Delete([]byte(pid))
			}
		}
		if lb := tx.Bucket(bucketLines); lb != nil {
			for _, lid := range delLines {
				_ = lb.Delete([]byte(lid))
			}
		}
		// Clear PollingID binding on draw items for deleted pollings
		if ib := tx.Bucket(bucketItems); ib != nil {
			s.items.Range(func(k, v any) bool {
				item := v.(*datastore.DrawItemEnt)
				for _, pid := range delPollings {
					if item.PollingID == pid {
						item.PollingID = ""
						if data, err := json.Marshal(item); err == nil {
							_ = ib.Put([]byte(item.ID), data)
						}
						break
					}
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 4. Delete from memory caches
	s.nodes.Delete(id)
	for _, pid := range delPollings {
		s.pollings.Delete(pid)
	}
	for _, lid := range delLines {
		s.lines.Delete(lid)
	}

	// 5. Cleanup any potential remaining orphans
	s.cleanupOrphans()
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

func checkNetwork(n *datastore.NetworkEnt) {
	xMax := 5 // 最小幅は5ポート分
	yMax := 0 // 最小の高さは1ポート分
	for _, p := range n.Ports {
		if xMax < p.X {
			xMax = p.X
		}
		if yMax < p.Y {
			yMax = p.Y
		}
	}
	n.W = (xMax+1)*45 + 20
	n.H = (yMax+1)*55 + 12 + 20
	if n.HPorts < 1 {
		n.HPorts = 24
	}
	if n.SystemID == "" {
		n.SystemID = n.IP
	}
}

func (s *Store) SaveNetwork(_ context.Context, nw *datastore.NetworkEnt) error {
	if nw == nil {
		return datastore.ErrInvalidParams
	}
	if nw.ID == "" {
		nw.ID = makeID()
	}
	checkNetwork(nw)
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

	netTarget := "NET:" + id
	var delLines []string
	s.lines.Range(func(k, v any) bool {
		l := v.(*datastore.LineEnt)
		if l.NodeID1 == netTarget || l.NodeID2 == netTarget || l.NodeID1 == id || l.NodeID2 == id {
			delLines = append(delLines, l.ID)
		}
		return true
	})

	err := s.db.Update(func(tx *bbolt.Tx) error {
		if nb := tx.Bucket(bucketNetworks); nb != nil {
			if err := nb.Delete([]byte(id)); err != nil {
				return err
			}
		}
		if lb := tx.Bucket(bucketLines); lb != nil {
			for _, lid := range delLines {
				_ = lb.Delete([]byte(lid))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.networks.Delete(id)
	for _, lid := range delLines {
		s.lines.Delete(lid)
	}
	s.cleanupOrphans()
	return nil
}

// --- DrawItem Operations ---

func (s *Store) GetDrawItem(_ context.Context, id string) (*datastore.DrawItemEnt, error) {
	if val, ok := s.items.Load(id); ok {
		return val.(*datastore.DrawItemEnt), nil
	}
	var item datastore.DrawItemEnt
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketItems)
		if b == nil {
			return datastore.ErrNotFound
		}
		v := b.Get([]byte(id))
		if v == nil {
			return datastore.ErrNotFound
		}
		return json.Unmarshal(v, &item)
	})
	if err != nil {
		return nil, err
	}
	s.items.Store(item.ID, &item)
	return &item, nil
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

func clonePolling(p *datastore.PollingEnt) *datastore.PollingEnt {
	if p == nil {
		return nil
	}
	cp := *p
	if p.Result != nil {
		cp.Result = make(map[string]interface{}, len(p.Result))
		for k, v := range p.Result {
			cp.Result[k] = v
		}
	}
	return &cp
}

func (s *Store) GetPolling(_ context.Context, id string) (*datastore.PollingEnt, error) {
	if val, ok := s.pollings.Load(id); ok {
		return clonePolling(val.(*datastore.PollingEnt)), nil
	}
	return nil, datastore.ErrNotFound
}

func (s *Store) ListPollings(_ context.Context) ([]*datastore.PollingEnt, error) {
	res := make([]*datastore.PollingEnt, 0)
	s.pollings.Range(func(_, val any) bool {
		res = append(res, clonePolling(val.(*datastore.PollingEnt)))
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
	cp := clonePolling(p)
	data, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("marshal polling: %w", err)
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketPollings).Put([]byte(cp.ID), data)
	})
	if err != nil {
		return err
	}
	s.pollings.Store(cp.ID, cp)
	return nil
}

func (s *Store) DeletePolling(_ context.Context, id string) error {
	if id == "" {
		return datastore.ErrInvalidID
	}

	var delLines []string
	s.lines.Range(func(k, v any) bool {
		l := v.(*datastore.LineEnt)
		if l.PollingID1 == id || l.PollingID2 == id || l.PollingID == id {
			delLines = append(delLines, l.ID)
		}
		return true
	})

	err := s.db.Update(func(tx *bbolt.Tx) error {
		if pb := tx.Bucket(bucketPollings); pb != nil {
			if err := pb.Delete([]byte(id)); err != nil {
				return err
			}
		}
		if lb := tx.Bucket(bucketLines); lb != nil {
			for _, lid := range delLines {
				_ = lb.Delete([]byte(lid))
			}
		}
		// Clear PollingID binding on draw items
		if ib := tx.Bucket(bucketItems); ib != nil {
			s.items.Range(func(k, v any) bool {
				item := v.(*datastore.DrawItemEnt)
				if item.PollingID == id {
					item.PollingID = ""
					if data, err := json.Marshal(item); err == nil {
						_ = ib.Put([]byte(item.ID), data)
					}
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.pollings.Delete(id)
	for _, lid := range delLines {
		s.lines.Delete(lid)
	}
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

func (s *Store) GetBackImage(_ context.Context) (*datastore.BackImageEnt, error) {
	s.confMu.RLock()
	defer s.confMu.RUnlock()
	b := s.backImage
	return &b, nil
}

func (s *Store) SaveBackImage(_ context.Context, bi *datastore.BackImageEnt) error {
	if bi == nil {
		return datastore.ErrInvalidParams
	}
	data, err := json.Marshal(bi)
	if err != nil {
		return err
	}
	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketConfig).Put(keyBackImage, data)
	})
	if err != nil {
		return err
	}
	s.confMu.Lock()
	s.backImage = *bi
	s.confMu.Unlock()
	return nil
}

func (s *Store) GetDiscoverConf(_ context.Context) (*datastore.DiscoverConfEnt, error) {
	s.confMu.RLock()
	defer s.confMu.RUnlock()
	c := s.discoverConf
	return &c, nil
}

func (s *Store) SaveDiscoverConf(_ context.Context, conf *datastore.DiscoverConfEnt) error {
	if conf == nil {
		return datastore.ErrInvalidParams
	}
	data, err := json.Marshal(conf)
	if err != nil {
		return err
	}
	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketConfig).Put(keyDiscoverConf, data)
	})
	if err != nil {
		return err
	}
	s.confMu.Lock()
	s.discoverConf = *conf
	s.confMu.Unlock()
	return nil
}

// --- Custom Icon Operations ---

func (s *Store) GetCustomIcons(_ context.Context) ([]*datastore.IconEnt, error) {
	s.iconsMu.RLock()
	defer s.iconsMu.RUnlock()
	if s.customIcons == nil {
		return []*datastore.IconEnt{}, nil
	}
	res := make([]*datastore.IconEnt, len(s.customIcons))
	for i, ic := range s.customIcons {
		cp := *ic
		res[i] = &cp
	}
	return res, nil
}

func (s *Store) SaveCustomIcon(_ context.Context, icon *datastore.IconEnt) error {
	if icon == nil || icon.Name == "" {
		return datastore.ErrInvalidParams
	}
	if icon.Type != "image" && icon.Image == "" && icon.Code == 0 {
		return datastore.ErrInvalidParams
	}
	s.iconsMu.Lock()
	defer s.iconsMu.Unlock()
	found := false
	for i, ic := range s.customIcons {
		if strings.EqualFold(ic.Name, icon.Name) || (icon.ID != "" && ic.ID == icon.ID) {
			s.customIcons[i] = icon
			found = true
			break
		}
	}
	if !found {
		if icon.ID == "" {
			icon.ID = datastore.GenerateID()
		}
		s.customIcons = append(s.customIcons, icon)
	}
	data, err := json.Marshal(s.customIcons)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketConfig).Put(keyCustomIcons, data)
	})
}

func (s *Store) SaveCustomIcons(_ context.Context, icons []*datastore.IconEnt) error {
	s.iconsMu.Lock()
	defer s.iconsMu.Unlock()
	if icons == nil {
		icons = []*datastore.IconEnt{}
	}
	for _, ic := range icons {
		if ic.ID == "" {
			ic.ID = datastore.GenerateID()
		}
	}
	s.customIcons = icons
	data, err := json.Marshal(s.customIcons)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketConfig).Put(keyCustomIcons, data)
	})
}

func (s *Store) DeleteCustomIcon(_ context.Context, nameOrID string) error {
	s.iconsMu.Lock()
	defer s.iconsMu.Unlock()
	newIcons := make([]*datastore.IconEnt, 0, len(s.customIcons))
	for _, ic := range s.customIcons {
		if !strings.EqualFold(ic.Name, nameOrID) && ic.ID != nameOrID {
			newIcons = append(newIcons, ic)
		}
	}
	s.customIcons = newIcons
	data, err := json.Marshal(s.customIcons)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketConfig).Put(keyCustomIcons, data)
	})
}


// --- Event Log Operations ---

var eventSeq uint32

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

	seq := atomic.AddUint32(&eventSeq, 1)
	key := make([]byte, 12)
	binary.BigEndian.PutUint64(key[0:8], uint64(event.Time))
	binary.BigEndian.PutUint32(key[8:12], seq)

	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketEventLog).Put(key, data)
	})
}

func (s *Store) ListEventLogs(ctx context.Context, limit int) ([]*datastore.EventLogEnt, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.QueryEventLogs(ctx, datastore.EventLogFilter{Limit: limit})
}

func (s *Store) QueryEventLogs(ctx context.Context, filter datastore.EventLogFilter) ([]*datastore.EventLogEnt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 10000
	}
	if limit > 20000 {
		limit = 20000
	}
	logs := make([]*datastore.EventLogEnt, 0, min(limit, 1000))

	var regexType *regexp.Regexp
	if filter.Type != "" && filter.Type != "all" {
		regexType, _ = regexp.Compile("(?i)" + filter.Type)
	}
	var regexNode *regexp.Regexp
	if filter.NodeName != "" {
		regexNode, _ = regexp.Compile("(?i)" + filter.NodeName)
	}
	var regexKeyword *regexp.Regexp
	if filter.Filter != "" {
		regexKeyword, _ = regexp.Compile("(?i)" + filter.Filter)
	}

	getLevelNum := func(l string) int {
		switch strings.ToLower(l) {
		case "high":
			return 3
		case "low":
			return 2
		case "warn":
			return 1
		default:
			return 0
		}
	}
	targetLevel := strings.ToLower(filter.Level)
	matchLevel := func(evLevel string) bool {
		if targetLevel == "" || targetLevel == "all" || targetLevel == "0" {
			return true
		}
		evLower := strings.ToLower(evLevel)
		switch targetLevel {
		case "high", "high+", "3":
			return getLevelNum(evLower) >= 3
		case "low", "low+", "2":
			return getLevelNum(evLower) >= 2
		case "warn", "warn+", "1":
			return getLevelNum(evLower) >= 1
		default:
			return evLower == targetLevel
		}
	}

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEventLog)
		if b == nil {
			return nil
		}
		c := b.Cursor()
		count := 0

		var k, v []byte
		if filter.EndTime > 0 {
			endKey := make([]byte, 12)
			binary.BigEndian.PutUint64(endKey[0:8], uint64(filter.EndTime))
			binary.BigEndian.PutUint32(endKey[8:12], 0xFFFFFFFF)
			k, v = c.Seek(endKey)
			if k == nil {
				k, v = c.Last()
			}
		} else {
			k, v = c.Last()
		}

		for ; k != nil && count < limit; k, v = c.Prev() {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var ev datastore.EventLogEnt
			if err := json.Unmarshal(v, &ev); err != nil {
				continue
			}

			if filter.EndTime > 0 && ev.Time > filter.EndTime {
				continue
			}
			if filter.StartTime > 0 && ev.Time < filter.StartTime {
				break // Keys are monotonically increasing by timestamp, stop early
			}
			if filter.NodeID != "" && ev.NodeID != filter.NodeID {
				continue
			}
			if !matchLevel(ev.Level) {
				continue
			}
			if filter.NodeName != "" {
				if regexNode != nil {
					if !regexNode.MatchString(ev.NodeName) {
						continue
					}
				} else if !strings.Contains(strings.ToLower(ev.NodeName), strings.ToLower(filter.NodeName)) {
					continue
				}
			}
			if filter.Type != "" && filter.Type != "all" {
				if regexType != nil {
					if !regexType.MatchString(ev.Type) {
						continue
					}
				} else if !strings.EqualFold(ev.Type, filter.Type) {
					continue
				}
			}
			if filter.Filter != "" {
				if regexKeyword != nil {
					if !regexKeyword.MatchString(ev.Event) &&
						!regexKeyword.MatchString(ev.NodeName) &&
						!regexKeyword.MatchString(ev.Type) &&
						!regexKeyword.MatchString(ev.Level) {
						continue
					}
				} else {
					kw := strings.ToLower(filter.Filter)
					if !strings.Contains(strings.ToLower(ev.Event), kw) &&
						!strings.Contains(strings.ToLower(ev.NodeName), kw) &&
						!strings.Contains(strings.ToLower(ev.Type), kw) &&
						!strings.Contains(strings.ToLower(ev.Level), kw) {
						continue
					}
				}
			}

			logs = append(logs, &ev)
			count++
		}
		return nil
	})
	return logs, err
}

func (s *Store) DeleteEventLogs(_ context.Context) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket(bucketEventLog); err != nil && err != bbolt.ErrBucketNotFound {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(bucketEventLog)
		return err
	})
}

func (s *Store) CountEventLogs(_ context.Context) (int64, error) {
	var count int64
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEventLog)
		if b == nil {
			return nil
		}
		count = int64(b.Stats().KeyN)
		return nil
	})
	return count, err
}

// SaveArpTable saves the provided ARP table entries.
func (s *Store) SaveArpTable(_ context.Context, entries []*datastore.ArpEnt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}

	return s.db.Batch(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketArp)
		if b == nil {
			return nil
		}
		for _, e := range entries {
			if e == nil || e.IP == "" {
				continue
			}
			data, err := json.Marshal(e)
			if err != nil {
				continue
			}
			_ = b.Put([]byte(e.IP), data)
		}
		return nil
	})
}

// LoadArpTable loads all stored ARP table entries.
func (s *Store) LoadArpTable(_ context.Context) ([]*datastore.ArpEnt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}

	var results []*datastore.ArpEnt
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketArp)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var ent datastore.ArpEnt
			if err := json.Unmarshal(v, &ent); err == nil {
				results = append(results, &ent)
			}
			return nil
		})
	})
	return results, err
}

// DeleteArpEntries removes specific IP entries from the ARP table.
func (s *Store) DeleteArpEntries(_ context.Context, ips []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketArp)
		if b == nil {
			return nil
		}
		for _, ip := range ips {
			_ = b.Delete([]byte(ip))
		}
		return nil
	})
}

// ResetArpTable removes all entries in the ARP table bucket.
func (s *Store) ResetArpTable(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		_ = tx.DeleteBucket(bucketArp)
		_, err := tx.CreateBucketIfNotExists(bucketArp)
		return err
	})
}

// getOTelMetricKey computes unique sha1 key for an OTel metric series.
func getOTelMetricKey(host, service, scope, name string) string {
	h := sha1.Sum(fmt.Appendf(nil, "%s\t%s\t%s\t%s", host, service, scope, name))
	return hex.EncodeToString(h[:])
}

// ListOTelMetrics returns all registered OTel metric summary entries.
func (s *Store) ListOTelMetrics(_ context.Context) ([]*datastore.OTelMetricEnt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}

	results := make([]*datastore.OTelMetricEnt, 0)
	s.otelMetrics.Range(func(_, value any) bool {
		if m, ok := value.(*datastore.OTelMetricEnt); ok {
			results = append(results, m)
		}
		return true
	})

	sort.Slice(results, func(i, j int) bool {
		return results[i].Last > results[j].Last
	})
	return results, nil
}

// GetOTelMetric returns a single metric series with its data points.
func (s *Store) GetOTelMetric(_ context.Context, host, service, scope, name string) (*datastore.OTelMetricEnt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}

	key := getOTelMetricKey(host, service, scope, name)
	if val, ok := s.otelMetrics.Load(key); ok {
		if m, ok := val.(*datastore.OTelMetricEnt); ok {
			return m, nil
		}
	}
	return nil, datastore.ErrNotFound
}

// SaveOTelMetric stores or updates an OTel metric series in memory and bbolt.
func (s *Store) SaveOTelMetric(_ context.Context, m *datastore.OTelMetricEnt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	if m == nil {
		return datastore.ErrInvalidParams
	}

	key := getOTelMetricKey(m.Host, m.Service, m.Scope, m.Name)
	s.otelMetrics.Store(key, m)

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketOTelMetric)
		if b == nil {
			return nil
		}
		data, err := json.Marshal(m)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
}

// DeleteOTelMetric removes a metric series from cache and bbolt.
func (s *Store) DeleteOTelMetric(_ context.Context, host, service, scope, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}

	key := getOTelMetricKey(host, service, scope, name)
	s.otelMetrics.Delete(key)

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketOTelMetric)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(key))
	})
}

// GetOTelTraceBuckets returns a sorted list of trace time buckets (YYYY-MM-DDTHH:mm).
func (s *Store) GetOTelTraceBuckets(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}

	buckets := make([]string, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketOTelTrace)
		if b == nil {
			return nil
		}
		return b.ForEachBucket(func(k []byte) error {
			buckets = append(buckets, string(k))
			return nil
		})
	})
	if err != nil {
		return make([]string, 0), err
	}
	sort.Strings(buckets)
	return buckets, nil
}

// ListOTelTraces returns summarized trace rows for given buckets or all if empty, capped at limit.
func (s *Store) ListOTelTraces(_ context.Context, buckets []string, limit int) ([]*datastore.OTelTraceSummaryEnt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}
	if limit <= 0 {
		limit = 5000
	}

	results := make([]*datastore.OTelTraceSummaryEnt, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		root := tx.Bucket(bucketOTelTrace)
		if root == nil {
			return nil
		}

		var targetBuckets []string
		if len(buckets) > 0 {
			targetBuckets = buckets
		} else {
			c := root.Cursor()
			for k, _ := c.Last(); k != nil; k, _ = c.Prev() {
				targetBuckets = append(targetBuckets, string(k))
			}
		}

		for _, bName := range targetBuckets {
			if len(results) >= limit {
				break
			}
			b := root.Bucket([]byte(bName))
			if b == nil {
				continue
			}
			_ = b.ForEach(func(k, v []byte) error {
				if len(results) >= limit {
					return nil
				}
				var t datastore.OTelTraceEnt
				if err := json.Unmarshal(v, &t); err != nil {
					return nil
				}
				hosts := make([]string, 0)
				services := make([]string, 0)
				scopes := make([]string, 0)
				hostMap := make(map[string]bool)
				serviceMap := make(map[string]bool)
				scopeMap := make(map[string]bool)

				for _, sp := range t.Spans {
					if sp.Host != "" && !hostMap[sp.Host] {
						hostMap[sp.Host] = true
						hosts = append(hosts, sp.Host)
					}
					if sp.Service != "" && !serviceMap[sp.Service] {
						serviceMap[sp.Service] = true
						services = append(services, sp.Service)
					}
					if sp.Scope != "" && !scopeMap[sp.Scope] {
						scopeMap[sp.Scope] = true
						scopes = append(scopes, sp.Scope)
					}
				}

				results = append(results, &datastore.OTelTraceSummaryEnt{
					Bucket:   bName,
					TraceID:  t.TraceID,
					Hosts:    strings.Join(hosts, " "),
					Services: strings.Join(services, " "),
					Scopes:   strings.Join(scopes, " "),
					Start:    t.Start,
					End:      t.End,
					Dur:      t.Dur,
					NumSpan:  len(t.Spans),
				})
				return nil
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Start > results[j].Start
	})
	return results, nil
}

// GetOTelTrace returns a complete trace entity with all its spans.
func (s *Store) GetOTelTrace(_ context.Context, bucket, traceID string) (*datastore.OTelTraceEnt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}

	var result *datastore.OTelTraceEnt
	err := s.db.View(func(tx *bbolt.Tx) error {
		root := tx.Bucket(bucketOTelTrace)
		if root == nil {
			return nil
		}
		b := root.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(traceID))
		if v == nil {
			return nil
		}
		var t datastore.OTelTraceEnt
		if err := json.Unmarshal(v, &t); err == nil {
			result = &t
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, datastore.ErrNotFound
	}
	return result, nil
}

// SaveOTelTraces persists a slice of traces into their respective time buckets.
func (s *Store) SaveOTelTraces(_ context.Context, traces []*datastore.OTelTraceEnt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	if len(traces) == 0 {
		return nil
	}

	return s.db.Batch(func(tx *bbolt.Tx) error {
		root := tx.Bucket(bucketOTelTrace)
		if root == nil {
			return nil
		}
		for _, t := range traces {
			if t.Bucket == "" || t.TraceID == "" {
				continue
			}
			b, err := root.CreateBucketIfNotExists([]byte(t.Bucket))
			if err != nil {
				continue
			}
			data, err := json.Marshal(t)
			if err != nil {
				continue
			}
			_ = b.Put([]byte(t.TraceID), data)
		}
		return nil
	})
}

// GetOTelTraceDAG builds service call dependency graph for selected buckets.
func (s *Store) GetOTelTraceDAG(_ context.Context, buckets []string) (*datastore.OTelTraceDAGEnt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}

	ret := &datastore.OTelTraceDAGEnt{
		Nodes: make([]datastore.OTelTraceDAGNodeEnt, 0),
		Links: make([]datastore.OTelTraceDAGLinkEnt, 0),
	}

	spanMap := make(map[string]string)  // traceID:spanID -> service
	nodeMap := make(map[string]int)     // service -> count
	spanLinkMap := make(map[string]int) // traceID:parentSpanID \t traceID:spanID -> count

	err := s.db.View(func(tx *bbolt.Tx) error {
		root := tx.Bucket(bucketOTelTrace)
		if root == nil {
			return nil
		}
		var targetBuckets []string
		if len(buckets) > 0 {
			targetBuckets = buckets
		} else {
			c := root.Cursor()
			for k, _ := c.Last(); k != nil; k, _ = c.Prev() {
				targetBuckets = append(targetBuckets, string(k))
				if len(targetBuckets) >= 100 {
					break
				}
			}
		}

		for _, bName := range targetBuckets {
			b := root.Bucket([]byte(bName))
			if b == nil {
				continue
			}
			_ = b.ForEach(func(k, v []byte) error {
				var t datastore.OTelTraceEnt
				if err := json.Unmarshal(v, &t); err != nil {
					return nil
				}
				for _, sp := range t.Spans {
					sk := fmt.Sprintf("%s:%s", t.TraceID, sp.SpanID)
					spanMap[sk] = sp.Service
					nodeMap[sp.Service]++
					if sp.ParentSpanID != "" {
						lk := fmt.Sprintf("%s:%s\t%s:%s", t.TraceID, sp.ParentSpanID, t.TraceID, sp.SpanID)
						spanLinkMap[lk]++
					}
				}
				return nil
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	linkMap := make(map[string]int)
	for k, c := range spanLinkMap {
		parts := strings.SplitN(k, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		srcSvc, srcOk := spanMap[parts[0]]
		dstSvc, dstOk := spanMap[parts[1]]
		if srcOk && dstOk && srcSvc != dstSvc {
			linkMap[fmt.Sprintf("%s\t%s", srcSvc, dstSvc)] += c
		}
	}

	for n, c := range nodeMap {
		ret.Nodes = append(ret.Nodes, datastore.OTelTraceDAGNodeEnt{
			Name:  n,
			Count: c,
		})
	}
	for l, c := range linkMap {
		parts := strings.SplitN(l, "\t", 2)
		if len(parts) == 2 {
			ret.Links = append(ret.Links, datastore.OTelTraceDAGLinkEnt{
				Src:   parts[0],
				Dst:   parts[1],
				Count: c,
			})
		}
	}

	return ret, nil
}

// DeleteAllOTelData purges all OTel metrics and traces from bbolt.
func (s *Store) DeleteAllOTelData(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}

	s.otelMetrics.Range(func(key, _ any) bool {
		s.otelMetrics.Delete(key)
		return true
	})

	return s.db.Update(func(tx *bbolt.Tx) error {
		_ = tx.DeleteBucket(bucketOTelMetric)
		_, err := tx.CreateBucketIfNotExists(bucketOTelMetric)
		if err != nil {
			return err
		}
		_ = tx.DeleteBucket(bucketOTelTrace)
		_, err = tx.CreateBucketIfNotExists(bucketOTelTrace)
		return err
	})
}

// CleanOldOTelData deletes metrics and trace buckets older than retention hours.
func (s *Store) CleanOldOTelData(_ context.Context, retentionHours int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	if retentionHours <= 0 {
		retentionHours = 24
	}

	cutoff := time.Now().Add(-time.Hour * time.Duration(retentionHours))
	cutoffNano := cutoff.UnixNano()
	cutoffBucket := cutoff.Format("2006-01-02T15:04")

	var delMetricKeys []string
	s.otelMetrics.Range(func(k, v any) bool {
		if m, ok := v.(*datastore.OTelMetricEnt); ok {
			if m.Last < cutoffNano {
				delMetricKeys = append(delMetricKeys, k.(string))
			}
		}
		return true
	})
	for _, k := range delMetricKeys {
		s.otelMetrics.Delete(k)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		if b := tx.Bucket(bucketOTelMetric); b != nil {
			for _, k := range delMetricKeys {
				_ = b.Delete([]byte(k))
			}
		}

		if root := tx.Bucket(bucketOTelTrace); root != nil {
			var delBuckets [][]byte
			_ = root.ForEachBucket(func(k []byte) error {
				if string(k) < cutoffBucket {
					delBuckets = append(delBuckets, k)
				}
				return nil
			})
			for _, bk := range delBuckets {
				_ = root.DeleteBucket(bk)
			}
		}
		return nil
	})
}

// ListMqttStats returns all MQTT topic statistics with dynamic State computation.
func (s *Store) ListMqttStats(_ context.Context) ([]*datastore.MqttStatEnt, error) {
	now := time.Now()
	warnTime := now.AddDate(0, 0, -1).UnixNano()
	lowTime := now.AddDate(0, 0, -5).UnixNano()

	var ret []*datastore.MqttStatEnt
	s.mqttStats.Range(func(_, v any) bool {
		if stat, ok := v.(*datastore.MqttStatEnt); ok {
			cp := *stat
			if cp.Last < lowTime {
				cp.State = "low"
			} else if cp.Last < warnTime {
				cp.State = "warn"
			} else {
				cp.State = "normal"
			}
			ret = append(ret, &cp)
		}
		return true
	})

	sort.Slice(ret, func(i, j int) bool {
		if ret[i].Topic == ret[j].Topic {
			return ret[i].ClientID < ret[j].ClientID
		}
		return ret[i].Topic < ret[j].Topic
	})

	return ret, nil
}

// SaveMqttStat stores or updates a single MQTT stat in cache and bbolt.
func (s *Store) SaveMqttStat(_ context.Context, stat *datastore.MqttStatEnt) error {
	if stat == nil || stat.ID == "" {
		return fmt.Errorf("invalid mqtt stat")
	}
	s.mqttStats.Store(stat.ID, stat)

	data, err := json.Marshal(stat)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMqttStat)
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return b.Put([]byte(stat.ID), data)
	})
}

// SaveMqttStats batch stores MQTT stats in cache and bbolt.
func (s *Store) SaveMqttStats(_ context.Context, stats []*datastore.MqttStatEnt) error {
	if len(stats) == 0 {
		return nil
	}
	for _, stat := range stats {
		if stat != nil && stat.ID != "" {
			s.mqttStats.Store(stat.ID, stat)
		}
	}

	return s.db.Batch(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMqttStat)
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		for _, stat := range stats {
			if stat != nil && stat.ID != "" {
				data, err := json.Marshal(stat)
				if err == nil {
					_ = b.Put([]byte(stat.ID), data)
				}
			}
		}
		return nil
	})
}

// DeleteMqttStats removes specific MQTT stats by their IDs.
func (s *Store) DeleteMqttStats(_ context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		s.mqttStats.Delete(id)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMqttStat)
		if b == nil {
			return nil
		}
		for _, id := range ids {
			_ = b.Delete([]byte(id))
		}
		return nil
	})
}

// DeleteAllMqttStats purges all MQTT stats from memory and disk.
func (s *Store) DeleteAllMqttStats(_ context.Context) error {
	s.mqttStats.Range(func(k, _ any) bool {
		s.mqttStats.Delete(k)
		return true
	})

	return s.db.Update(func(tx *bbolt.Tx) error {
		_ = tx.DeleteBucket(bucketMqttStat)
		_, err := tx.CreateBucketIfNotExists(bucketMqttStat)
		return err
	})
}

// CleanOldMqttStats deletes MQTT stats whose last message is older than days.
func (s *Store) CleanOldMqttStats(_ context.Context, days int) error {
	if days <= 0 {
		days = 14
	}
	cutoff := time.Now().AddDate(0, 0, -days).UnixNano()

	var delIDs []string
	s.mqttStats.Range(func(k, v any) bool {
		if stat, ok := v.(*datastore.MqttStatEnt); ok {
			if stat.Last < cutoff {
				delIDs = append(delIDs, k.(string))
			}
		}
		return true
	})

	if len(delIDs) == 0 {
		return nil
	}

	for _, id := range delIDs {
		s.mqttStats.Delete(id)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketMqttStat)
		if b == nil {
			return nil
		}
		for _, id := range delIDs {
			_ = b.Delete([]byte(id))
		}
		return nil
	})
}

func (s *Store) ListPKICertificates() ([]*pki.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}
	certificates := make([]*pki.Certificate, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketPKICerts)
		if bucket == nil {
			return fmt.Errorf("PKI certificate bucket not found")
		}
		return bucket.ForEach(func(_, value []byte) error {
			var cert pki.Certificate
			if err := json.Unmarshal(value, &cert); err != nil {
				return fmt.Errorf("decode PKI certificate: %w", err)
			}
			certificates = append(certificates, &cert)
			return nil
		})
	})
	return certificates, err
}

func (s *Store) SavePKICertificate(cert *pki.Certificate) error {
	if cert == nil || cert.Serial == "" {
		return datastore.ErrInvalidParams
	}
	value, err := json.Marshal(cert)
	if err != nil {
		return fmt.Errorf("encode PKI certificate: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketPKICerts)
		if bucket == nil {
			return fmt.Errorf("PKI certificate bucket not found")
		}
		return bucket.Put([]byte(strings.ToLower(cert.Serial)), value)
	})
}

func (s *Store) DeleteAllPKICertificates() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket(bucketPKICerts); err != nil && err != bbolt.ErrBucketNotFound {
			return err
		}
		_, err := tx.CreateBucket(bucketPKICerts)
		return err
	})
}

// CertMonitor methods

func (s *Store) ListCertMonitors(ctx context.Context) ([]*datastore.CertMonitorEnt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}
	monitors := make([]*datastore.CertMonitorEnt, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketCertMonitor)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var ent datastore.CertMonitorEnt
			if err := json.Unmarshal(v, &ent); err == nil {
				monitors = append(monitors, &ent)
			}
			return nil
		})
	})
	return monitors, err
}

func (s *Store) GetCertMonitor(ctx context.Context, id string) (*datastore.CertMonitorEnt, error) {
	if id == "" {
		return nil, datastore.ErrInvalidID
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}
	var ent *datastore.CertMonitorEnt
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketCertMonitor)
		if b == nil {
			return datastore.ErrNotFound
		}
		v := b.Get([]byte(id))
		if v == nil {
			return datastore.ErrNotFound
		}
		ent = &datastore.CertMonitorEnt{}
		return json.Unmarshal(v, ent)
	})
	return ent, err
}

func (s *Store) SaveCertMonitor(ctx context.Context, c *datastore.CertMonitorEnt) error {
	if c == nil {
		return datastore.ErrInvalidParams
	}
	if c.ID == "" {
		c.ID = datastore.GenerateID()
	}
	v, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode cert monitor: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketCertMonitor)
		if b == nil {
			var bErr error
			b, bErr = tx.CreateBucketIfNotExists(bucketCertMonitor)
			if bErr != nil {
				return bErr
			}
		}
		return b.Put([]byte(c.ID), v)
	})
}

func (s *Store) DeleteCertMonitor(ctx context.Context, id string) error {
	if id == "" {
		return datastore.ErrInvalidID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketCertMonitor)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(id))
	})
}

// User & Authentication methods

func (s *Store) GetUser(ctx context.Context, username string) (*datastore.UserEnt, error) {
	if username == "" {
		return nil, datastore.ErrInvalidParams
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}
	var u *datastore.UserEnt
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return datastore.ErrNotFound
		}
		v := b.Get([]byte(username))
		if v == nil {
			return datastore.ErrNotFound
		}
		u = &datastore.UserEnt{}
		return json.Unmarshal(v, u)
	})
	return u, err
}

func (s *Store) ListUsers(ctx context.Context) ([]*datastore.UserEnt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}
	users := make([]*datastore.UserEnt, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var u datastore.UserEnt
			if err := json.Unmarshal(v, &u); err == nil {
				users = append(users, &u)
			}
			return nil
		})
	})
	return users, err
}

func (s *Store) SaveUser(ctx context.Context, u *datastore.UserEnt) error {
	if u == nil || u.User == "" {
		return datastore.ErrInvalidParams
	}
	now := time.Now().Unix()
	if u.CreatedAt == 0 {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	data, err := json.Marshal(u)
	if err != nil {
		return fmt.Errorf("marshal user: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			var bErr error
			b, bErr = tx.CreateBucketIfNotExists(bucketUsers)
			if bErr != nil {
				return bErr
			}
		}
		return b.Put([]byte(u.User), data)
	})
}

func (s *Store) DeleteUser(ctx context.Context, username string) error {
	if username == "" {
		return datastore.ErrInvalidParams
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(username))
	})
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return 0, datastore.ErrDBNotOpen
	}
	count := 0
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b != nil {
			return b.ForEach(func(k, v []byte) error {
				count++
				return nil
			})
		}
		return nil
	})
	return count, err
}

func (s *Store) GetAuthSecret(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, datastore.ErrDBNotOpen
	}
	var secret []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketConfig)
		if b == nil {
			return nil
		}
		v := b.Get(keyAuthSecret)
		if v != nil {
			secret = make([]byte, len(v))
			copy(secret, v)
		}
		return nil
	})
	return secret, err
}

func (s *Store) SaveAuthSecret(ctx context.Context, secret []byte) error {
	if len(secret) == 0 {
		return datastore.ErrInvalidParams
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return datastore.ErrDBNotOpen
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketConfig)
		if b == nil {
			var bErr error
			b, bErr = tx.CreateBucketIfNotExists(bucketConfig)
			if bErr != nil {
				return bErr
			}
		}
		return b.Put(keyAuthSecret, secret)
	})
}


