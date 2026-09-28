// Package notify : 通知処理 - OAuth2認証
package notify

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"
)

type notifyOAuth2Flow struct {
	state       string
	redirectURL string
	conf        datastore.NotifyConfEnt
	expiresAt   time.Time
}

var oauth2FlowState struct {
	sync.Mutex
	flow *notifyOAuth2Flow
}

// GetNotifyOAuth2TokenStep1 starts the OAuth2 authorization flow.
// Returns the authorization URL that the user should visit.
func GetNotifyOAuth2TokenStep1(store datastore.DataStore, redirectURL string) (string, error) {
	conf := getNotifyConf(store)
	config := getNotifyOAuth2Config(conf, redirectURL)
	if config == nil {
		return "", fmt.Errorf("no oauth2 config")
	}
	state, err := randCryptoString(32)
	if err != nil {
		return "", fmt.Errorf("generate oauth2 state: %w", err)
	}
	oauth2FlowState.Lock()
	defer oauth2FlowState.Unlock()
	if oauth2FlowState.flow != nil && time.Now().Before(oauth2FlowState.flow.expiresAt) {
		return "", fmt.Errorf("oauth2 authorization already in progress")
	}
	oauth2FlowState.flow = &notifyOAuth2Flow{
		state:       state,
		redirectURL: redirectURL,
		conf:        conf,
		expiresAt:   time.Now().Add(5 * time.Minute),
	}
	return config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce), nil
}

// CompleteNotifyOAuth2Callback validates the state and exchanges the authorization code.
func CompleteNotifyOAuth2Callback(store datastore.DataStore, code, state string) error {
	oauth2FlowState.Lock()
	flow := oauth2FlowState.flow
	if flow == nil || time.Now().After(flow.expiresAt) ||
		subtle.ConstantTimeCompare([]byte(flow.state), []byte(state)) != 1 {
		oauth2FlowState.Unlock()
		return fmt.Errorf("invalid or expired oauth2 state")
	}
	oauth2FlowState.flow = nil
	oauth2FlowState.Unlock()
	if code == "" {
		return fmt.Errorf("authorization code is empty")
	}
	config := getNotifyOAuth2Config(flow.conf, flow.redirectURL)
	if config == nil {
		return fmt.Errorf("no oauth2 config")
	}
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("fail to get token: %w", err)
	}
	store.SaveNotifyOAuth2Token(token)
	return nil
}

func getNotifyConf(store datastore.DataStore) datastore.NotifyConfEnt {
	conf, err := store.GetNotifyConf(context.Background())
	if err != nil || conf == nil {
		return datastore.NotifyConfEnt{}
	}
	return *conf
}

func getNotifyOAuth2Config(conf datastore.NotifyConfEnt, redirectURL string) *oauth2.Config {
	switch conf.Provider {
	case "google":
		return &oauth2.Config{
			ClientID:     conf.ClientID,
			ClientSecret: conf.ClientSecret,
			Endpoint:     google.Endpoint,
			RedirectURL:  redirectURL,
			Scopes:       []string{"https://mail.google.com/"},
		}
	case "microsoft", "mscustom":
		return &oauth2.Config{
			ClientID:     conf.ClientID,
			ClientSecret: conf.ClientSecret,
			Endpoint:     microsoft.AzureADEndpoint(conf.MSTenant),
			RedirectURL:  redirectURL,
			Scopes:       []string{"https://outlook.office.com/SMTP.Send", "offline_access"},
		}
	default:
		return nil
	}
}

// refreshOAuth2Token tries to refresh the OAuth2 token if it's expired.
func refreshOAuth2Token(store datastore.DataStore) *oauth2.Token {
	oldToken := store.GetNotifyOAuth2Token()
	if oldToken == nil {
		return nil
	}
	if oldToken.Valid() {
		return oldToken
	}
	conf := getNotifyConf(store)
	config := getNotifyOAuth2Config(conf, "")
	if config == nil {
		return nil
	}
	tokenSource := config.TokenSource(context.Background(), oldToken)
	newToken, err := tokenSource.Token()
	if err != nil {
		log.Printf("Fail to refresh token err=%v", err)
		return nil
	}
	log.Printf("oauth2 token updated old=%v new=%v", oldToken.Expiry, newToken.Expiry)
	store.SaveNotifyOAuth2Token(newToken)
	return newToken
}

func randCryptoString(length int) (string, error) {
	b := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// DeleteNotifyOAuth2Token removes the stored OAuth2 token.
func DeleteNotifyOAuth2Token(store datastore.DataStore) {
	store.DeleteNotifyOAuth2Token()
}
