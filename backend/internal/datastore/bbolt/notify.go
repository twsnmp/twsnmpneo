package bbolt

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
	"go.etcd.io/bbolt"
	"golang.org/x/oauth2"
)

// --- Notify OAuth2 Token ---

// GetNotifyOAuth2Token returns the cached OAuth2 token (or nil).
func (s *Store) GetNotifyOAuth2Token() *oauth2.Token {
	s.tokenMu.RLock()
	defer s.tokenMu.RUnlock()
	return s.notifyOAuth2Token
}

// SaveNotifyOAuth2Token stores the OAuth2 token in memory and bbolt.
func (s *Store) SaveNotifyOAuth2Token(token *oauth2.Token) {
	s.tokenMu.Lock()
	s.notifyOAuth2Token = token
	s.tokenMu.Unlock()

	data, err := json.Marshal(token)
	if err != nil {
		return
	}
	_ = s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketConfig)
		if b == nil {
			return fmt.Errorf("bucket config is nil")
		}
		return b.Put(keyNotifyOAuth2Token, data)
	})
}

// DeleteNotifyOAuth2Token removes the OAuth2 token from memory and bbolt.
func (s *Store) DeleteNotifyOAuth2Token() {
	s.tokenMu.Lock()
	s.notifyOAuth2Token = nil
	s.tokenMu.Unlock()

	_ = s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketConfig)
		if b == nil {
			return nil
		}
		return b.Delete(keyNotifyOAuth2Token)
	})
}

// HasValidNotifyOAuth2Token returns true if a valid OAuth2 token exists for the given conf.
func (s *Store) HasValidNotifyOAuth2Token(n *datastore.NotifyConfEnt) bool {
	if n.Provider == "" || n.Provider == "smtp" {
		return true
	}
	s.confMu.RLock()
	current := s.notifyConf
	s.confMu.RUnlock()
	if n.Provider != current.Provider ||
		n.ClientID != current.ClientID ||
		n.ClientSecret != current.ClientSecret ||
		n.MSTenant != current.MSTenant {
		return false
	}
	s.tokenMu.RLock()
	defer s.tokenMu.RUnlock()
	return s.notifyOAuth2Token != nil
}

// --- Mail Templates ---

//go:embed conf/*.html
var embeddedMailTemplates embed.FS

// defaultMailTemplates contains minimal fallback HTML templates.
var defaultMailTemplates = map[string]string{
	"test": `<!DOCTYPE html><html><body><h1>{{.Title}}</h1><p>This is a test mail from TWSNMP NEO.</p></body></html>`,
	"notify": `<!DOCTYPE html><html><body>
<h1>{{.Title}}</h1>
{{range .Logs}}
<p>{{formatLogTime .Time}} [{{levelName .Level}}] {{.NodeName}} {{.Event}}</p>
{{end}}
</body></html>`,
	"report": `<!DOCTYPE html><html><body>
<h1>{{.Title}}</h1>
{{range .Info}}<p><b>{{.Name}}</b>: {{.Value}}</p>{{end}}
{{if .LLMSummary}}<h2>AI Summary</h2><pre>{{.LLMSummary}}</pre>{{end}}
</body></html>`,
}

// LoadMailTemplate loads a mail template from disk or returns the embedded template according to language.
func (s *Store) LoadMailTemplate(t string) string {
	// 1. Check if user override file exists in data directory
	dataDir := filepath.Dir(s.path)
	fname := fmt.Sprintf("mail_%s.html", t)
	fpath := filepath.Join(dataDir, fname)
	if r, err := os.Open(fpath); err == nil {
		defer r.Close()
		if b, err := io.ReadAll(r); err == nil && len(b) > 0 {
			return string(b)
		}
	}

	// 2. Check in-memory cache
	s.templateMu.RLock()
	cached, ok := s.mailTemplates[t]
	s.templateMu.RUnlock()
	if ok {
		return cached
	}

	// 3. Try embedded template for current language (or fallback to en)
	lang := i18n.GetLang()
	if lang == "" {
		lang = "en"
	}
	embeddedPath := fmt.Sprintf("conf/mail_%s_%s.html", t, lang)
	if data, err := embeddedMailTemplates.ReadFile(embeddedPath); err == nil && len(data) > 0 {
		return string(data)
	}
	if lang != "en" {
		if data, err := embeddedMailTemplates.ReadFile(fmt.Sprintf("conf/mail_%s_en.html", t)); err == nil && len(data) > 0 {
			return string(data)
		}
	}

	// 4. Return builtin fallback
	if tmpl, ok := defaultMailTemplates[t]; ok {
		return tmpl
	}
	return ""
}

// SetMailTemplate stores a mail template in the in-memory cache.
func (s *Store) SetMailTemplate(t, content string) {
	s.templateMu.Lock()
	s.mailTemplates[t] = content
	s.templateMu.Unlock()
}

// --- ForEach Iterators ---

// ForEachNodes iterates over all nodes in the in-memory cache.
// Callback returns false to stop iteration.
func (s *Store) ForEachNodes(fn func(*datastore.NodeEnt) bool) {
	s.nodes.Range(func(_, v any) bool {
		return fn(v.(*datastore.NodeEnt))
	})
}

// ForEachNetworks iterates over all networks in the in-memory cache.
func (s *Store) ForEachNetworks(fn func(*datastore.NetworkEnt) bool) {
	s.networks.Range(func(_, v any) bool {
		return fn(v.(*datastore.NetworkEnt))
	})
}

// ForEachLines iterates over all lines in the in-memory cache.
func (s *Store) ForEachLines(fn func(*datastore.LineEnt) bool) {
	s.lines.Range(func(_, v any) bool {
		return fn(v.(*datastore.LineEnt))
	})
}

// ForEachPollings iterates over all pollings in the in-memory cache.
func (s *Store) ForEachPollings(fn func(*datastore.PollingEnt) bool) {
	s.pollings.Range(func(_, v any) bool {
		return fn(clonePolling(v.(*datastore.PollingEnt)))
	})
}

// ForEachLastEventLog iterates event logs in reverse chronological order (newest first).
// Callback returns false to stop iteration.
func (s *Store) ForEachLastEventLog(fn func(*datastore.EventLogEnt) bool) {
	_ = s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEventLog)
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var ev datastore.EventLogEnt
			if err := json.Unmarshal(v, &ev); err != nil {
				continue
			}
			if !fn(&ev) {
				break
			}
		}
		return nil
	})
}

// --- AI Results ---

// GetAIResult returns AI anomaly detection results for a polling.
// This is a stub that reads from a dedicated "ai" bucket if it exists,
// matching twsnmpfk's storage pattern.
func (s *Store) GetAIResult(pollingID string) (*datastore.AIResultEnt, error) {
	var result datastore.AIResultEnt
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("ai"))
		if b == nil {
			return datastore.ErrNotFound
		}
		v := b.Get([]byte(pollingID))
		if v == nil {
			return datastore.ErrNotFound
		}
		return json.Unmarshal(v, &result)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SaveAIResult saves AI anomaly detection results for a polling.
func (s *Store) SaveAIResult(_ context.Context, result *datastore.AIResultEnt) error {
	if result == nil {
		return datastore.ErrInvalidParams
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("ai"))
		if err != nil {
			return err
		}
		return b.Put([]byte(result.PollingID), data)
	})
}

// --- DB Size ---

// GetDBSize returns the total bbolt database size in bytes.
func (s *Store) GetDBSize() int64 {
	var dbSize int64
	_ = s.db.View(func(tx *bbolt.Tx) error {
		dbSize = tx.Size()
		return nil
	})
	return dbSize
}
