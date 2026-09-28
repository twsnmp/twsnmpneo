package notify

import (
	"context"
	"net/url"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

func TestNotifyOAuth2UsesPublicInstanceCallback(t *testing.T) {
	store, cleanup, err := setupTestDB(t)
	if err != nil {
		t.Fatalf("create test datastore: %v", err)
	}
	defer cleanup()
	if err := store.SaveNotifyConf(context.Background(), &datastore.NotifyConfEnt{
		Provider:     "google",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}); err != nil {
		t.Fatalf("save notify config: %v", err)
	}

	oauth2FlowState.Lock()
	oauth2FlowState.flow = nil
	oauth2FlowState.Unlock()
	t.Cleanup(func() {
		oauth2FlowState.Lock()
		oauth2FlowState.flow = nil
		oauth2FlowState.Unlock()
	})

	authURL, err := GetNotifyOAuth2TokenStep1(store, "https://monitor.example/api/notify/oauth2/callback")
	if err != nil {
		t.Fatalf("start oauth2 flow: %v", err)
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	if got := u.Query().Get("redirect_uri"); got != "https://monitor.example/api/notify/oauth2/callback" {
		t.Fatalf("redirect_uri = %q", got)
	}
	if u.Query().Get("state") == "" {
		t.Fatal("authorization URL has no state parameter")
	}
}
