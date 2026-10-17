package certmgr

import (
	"net/http"
	"strings"
	"sync"
)

// EmbeddedHTTP01Provider implements lego's challenge.Provider
// while allowing embedding into an existing HTTP server without port conflicts.
type EmbeddedHTTP01Provider struct {
	tokens sync.Map
}

// NewEmbeddedHTTP01Provider creates a new embedded HTTP-01 challenge provider.
func NewEmbeddedHTTP01Provider() *EmbeddedHTTP01Provider {
	return &EmbeddedHTTP01Provider{}
}

// Present stores the token and keyAuth for incoming ACME validation.
func (p *EmbeddedHTTP01Provider) Present(domain, token, keyAuth string) error {
	p.tokens.Store(token, keyAuth)
	return nil
}

// CleanUp removes the token after ACME validation completes.
func (p *EmbeddedHTTP01Provider) CleanUp(domain, token, keyAuth string) error {
	p.tokens.Delete(token)
	return nil
}

// ServeHTTP checks if the incoming request is an ACME challenge.
// Returns true if handled, false otherwise.
func (p *EmbeddedHTTP01Provider) ServeHTTP(w http.ResponseWriter, r *http.Request) bool {
	prefix := "/.well-known/acme-challenge/"
	if strings.HasPrefix(r.URL.Path, prefix) {
		token := strings.TrimPrefix(r.URL.Path, prefix)
		if keyAuth, ok := p.tokens.Load(token); ok {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(keyAuth.(string)))
			return true
		}
	}
	return false
}
