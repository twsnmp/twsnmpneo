package pki

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

const (
	acmeMaxRequestBytes = 1 << 20
	acmeMaxNonces       = 10000
	acmeMaxAccounts     = 10000
	acmeMaxOrders       = 5000
	acmeMaxIdentifiers  = 20
	acmeNonceLifetime   = 10 * time.Minute
)

// ACMEConfig controls the bounded RFC 8555 service registered on an Echo instance.
// BaseURL must be the externally visible absolute URL prefix for this ACME directory.
// ChallengeValidator is optional; without it all offered challenges remain unsupported
// and are rejected rather than being marked valid.
type ACMEConfig struct {
	BaseURL            string
	TermsOfService     string
	ChallengeValidator func(context.Context, ACMEChallenge) error
}

// ACMEChallenge contains the material an integration needs to independently validate
// an ACME challenge. Returning nil from the configured validator asserts that the
// identifier control proof has actually been verified.
type ACMEChallenge struct {
	Type             string
	Identifier       string
	Token            string
	KeyAuthorization string
}

type acmeAccount struct {
	ID       string
	Key      crypto.PublicKey
	Thumb    string
	Contact  []string
	Status   string
	OrderIDs []string
}

type acmeIdentifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type acmeChallengeRecord struct {
	ID         string
	AccountID  string
	AuthzID    string
	Identifier acmeIdentifier
	Type       string
	Token      string
	Status     string
	Error      map[string]string `json:"error,omitempty"`
}

type acmeAuthorization struct {
	ID         string
	AccountID  string
	Identifier acmeIdentifier
	Status     string
	Expires    time.Time
	Challenges []string
}

type acmeOrder struct {
	ID             string
	AccountID      string
	Identifiers    []acmeIdentifier
	Status         string
	Expires        time.Time
	Authorization  []string
	FinalizeURL    string
	CertificateURL string
	Serial         string
}

type acmeService struct {
	manager *Manager
	baseURL string
	terms   string
	check   func(context.Context, ACMEChallenge) error

	mu          sync.Mutex
	closed      bool
	accounts    map[string]*acmeAccount
	thumbprints map[string]string
	orders      map[string]*acmeOrder
	authzs      map[string]*acmeAuthorization
	challenges  map[string]*acmeChallengeRecord
	nonces      map[string]time.Time
}

// ACMERegistration is the lifecycle handle returned by RegisterACME.
// Its in-memory account/order state ends when the process stops; certificates and
// revocations themselves are persisted by the supplied PKI Manager.
type ACMERegistration struct {
	service *acmeService
}

// RegisterACME adds the ACME directory and protocol endpoints to Echo and returns
// a handle that can be closed during application shutdown. BaseURL must match the
// externally advertised scheme, host, and optional path prefix exactly.
func RegisterACME(e *echo.Echo, manager *Manager, cfg ACMEConfig) (*ACMERegistration, error) {
	if e == nil || manager == nil {
		return nil, fmt.Errorf("echo instance and PKI manager are required")
	}
	base, err := validateACMEBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	s := &acmeService{
		manager: manager, baseURL: base, terms: cfg.TermsOfService, check: cfg.ChallengeValidator,
		accounts: make(map[string]*acmeAccount), thumbprints: make(map[string]string),
		orders: make(map[string]*acmeOrder), authzs: make(map[string]*acmeAuthorization),
		challenges: make(map[string]*acmeChallengeRecord), nonces: make(map[string]time.Time),
	}
	prefix := strings.TrimRight(mustParseURL(base).Path, "/")
	if prefix == "/" {
		prefix = ""
	}
	register := func(method, path string, handler echo.HandlerFunc) {
		e.Add(method, prefix+path, s.withNonce(handler))
	}
	register(http.MethodGet, "/directory", s.directory)
	register(http.MethodGet, "/new-nonce", s.newNonce)
	register(http.MethodHead, "/new-nonce", s.newNonce)
	register(http.MethodPost, "/new-account", s.newAccount)
	register(http.MethodPost, "/new-order", s.newOrder)
	register(http.MethodPost, "/account/:id", s.account)
	register(http.MethodPost, "/orders/:id", s.accountOrders)
	register(http.MethodPost, "/order/:id", s.order)
	register(http.MethodPost, "/authz/:id", s.authorization)
	register(http.MethodPost, "/challenge/:id", s.challenge)
	register(http.MethodPost, "/finalize/:id", s.finalize)
	register(http.MethodPost, "/cert/:id", s.certificate)
	register(http.MethodPost, "/revoke-cert", s.revokeCertificate)
	return &ACMERegistration{service: s}, nil
}

// Close prevents future stateful requests. Echo owns the listener and must be shut
// down by the application using Echo.Shutdown.
func (r *ACMERegistration) Close() error {
	if r == nil || r.service == nil {
		return nil
	}
	r.service.mu.Lock()
	defer r.service.mu.Unlock()
	r.service.closed = true
	return nil
}

func validateACMEBaseURL(value string) (string, error) {
	u, err := url.Parse(strings.TrimRight(value, "/"))
	if err != nil || u.Scheme != "https" || u.Host == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("ACME BaseURL must be an absolute https URL without credentials, query, or fragment")
	}
	if u.RawPath != "" || strings.Contains(u.Path, "//") || strings.ContainsAny(u.Path, "\r\n") {
		return "", fmt.Errorf("invalid ACME BaseURL path")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func mustParseURL(value string) *url.URL {
	u, _ := url.Parse(value)
	return u
}

func (s *acmeService) url(path string) string { return s.baseURL + path }

func (s *acmeService) withNonce(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		nonce, err := s.issueNonce()
		if err == nil {
			c.Response().Header().Set("Replay-Nonce", nonce)
			c.Response().Header().Set("Cache-Control", "no-store")
		}
		err = next(c)
		if problem, ok := err.(*acmeProblemError); ok {
			c.Response().Header().Set(echo.HeaderContentType, "application/problem+json")
			return c.JSON(problem.status, problem.body)
		}
		return err
	}
}

type acmeProblemError struct {
	status int
	body   map[string]any
}

func (e *acmeProblemError) Error() string {
	return fmt.Sprint(e.body["detail"])
}

func (s *acmeService) issueNonce() (string, error) {
	b := make([]byte, 24)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	nonce := base64.RawURLEncoding.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanNoncesLocked()
	if s.closed {
		return "", errors.New("ACME service is closed")
	}
	if len(s.nonces) >= acmeMaxNonces {
		return "", fmt.Errorf("ACME nonce capacity reached")
	}
	s.nonces[nonce] = time.Now().Add(acmeNonceLifetime)
	return nonce, nil
}

func (s *acmeService) cleanNoncesLocked() {
	now := time.Now()
	for nonce, expires := range s.nonces {
		if !expires.After(now) {
			delete(s.nonces, nonce)
		}
	}
}

func (s *acmeService) consumeNonce(nonce string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanNoncesLocked()
	if s.closed {
		return false
	}
	expires, ok := s.nonces[nonce]
	if !ok || !expires.After(time.Now()) {
		return false
	}
	delete(s.nonces, nonce)
	return true
}

func (s *acmeService) readJWS(c echo.Context, path string) (parsedACMEJWS, error) {
	if c.Request().Method != http.MethodPost {
		return parsedACMEJWS{}, acmeProblem(http.StatusMethodNotAllowed, "malformed", "ACME requests must use POST")
	}
	mediaType, _, mediaErr := mime.ParseMediaType(c.Request().Header.Get(echo.HeaderContentType))
	if mediaErr != nil || mediaType != "application/jose+json" {
		return parsedACMEJWS{}, acmeProblem(http.StatusUnsupportedMediaType, "malformed", "Content-Type must be application/jose+json")
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Response(), c.Request().Body, acmeMaxRequestBytes))
	if err != nil {
		return parsedACMEJWS{}, acmeProblem(http.StatusRequestEntityTooLarge, "malformed", "request body exceeds the ACME size limit")
	}
	jws, err := decodeACMEJWS(body, s.url(path))
	if err != nil {
		return parsedACMEJWS{}, acmeProblem(http.StatusBadRequest, "malformed", err.Error())
	}
	return jws, nil
}

func (s *acmeService) authenticate(c echo.Context, path string, newAccount bool) (*acmeAccount, parsedACMEJWS, error) {
	jws, err := s.readJWS(c, path)
	if err != nil {
		return nil, parsedACMEJWS{}, err
	}
	var account *acmeAccount
	if newAccount {
		if jws.protected.KID != "" {
			return nil, parsedACMEJWS{}, acmeProblem(http.StatusBadRequest, "malformed", "new-account requires a JWK protected header")
		}
	} else {
		if jws.protected.KID == "" {
			return nil, parsedACMEJWS{}, acmeProblem(http.StatusBadRequest, "malformed", "this endpoint requires an account kid")
		}
		id, err := s.accountIDFromURL(jws.protected.KID)
		if err != nil {
			return nil, parsedACMEJWS{}, acmeProblem(http.StatusUnauthorized, "unauthorized", "unknown account key identifier")
		}
		s.mu.Lock()
		account = s.accounts[id]
		s.mu.Unlock()
		if account == nil {
			return nil, parsedACMEJWS{}, acmeProblem(http.StatusUnauthorized, "unauthorized", "unknown account")
		}
		if !validACMEAlgorithm(jws.protected.Alg, account.Key) {
			return nil, parsedACMEJWS{}, acmeProblem(http.StatusBadRequest, "badSignatureAlgorithm", "unsupported JWS algorithm")
		}
		if err := verifyACMESignature(jws.protected.Alg, account.Key, jws.input, jws.signature); err != nil {
			return nil, parsedACMEJWS{}, acmeProblem(http.StatusUnauthorized, "unauthorized", "JWS signature verification failed")
		}
	}
	if !s.consumeNonce(jws.protected.Nonce) {
		return nil, parsedACMEJWS{}, acmeProblem(http.StatusBadRequest, "badNonce", "JWS nonce is invalid, expired, or already used")
	}
	return account, jws, nil
}

func (s *acmeService) accountIDFromURL(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != mustParseURL(s.baseURL).Scheme || u.Host != mustParseURL(s.baseURL).Host ||
		u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("invalid account URL")
	}
	prefix := strings.TrimRight(mustParseURL(s.baseURL).Path, "/")
	id := strings.TrimPrefix(u.Path, prefix+"/account/")
	if id == "" || id == u.Path || strings.Contains(id, "/") {
		return "", fmt.Errorf("invalid account URL")
	}
	if value != s.url("/account/"+id) {
		return "", fmt.Errorf("account URL is not canonical")
	}
	return id, nil
}

func (s *acmeService) directory(c echo.Context) error {
	directory := map[string]any{
		"newNonce": s.url("/new-nonce"), "newAccount": s.url("/new-account"),
		"newOrder": s.url("/new-order"), "revokeCert": s.url("/revoke-cert"),
	}
	meta := map[string]any{}
	if s.terms != "" {
		meta["termsOfService"] = s.terms
	}
	if len(meta) != 0 {
		directory["meta"] = meta
	}
	return c.JSON(http.StatusOK, directory)
}

func (s *acmeService) newNonce(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

func (s *acmeService) newAccount(c echo.Context) error {
	_, jws, err := s.authenticate(c, "/new-account", true)
	if err != nil {
		return err
	}
	var request struct {
		Contact              []string `json:"contact"`
		TermsOfServiceAgreed bool     `json:"termsOfServiceAgreed"`
		OnlyReturnExisting   bool     `json:"onlyReturnExisting"`
	}
	if len(jws.payload) != 0 {
		if err := json.Unmarshal(jws.payload, &request); err != nil {
			return acmeProblem(http.StatusBadRequest, "malformed", "invalid new-account payload")
		}
	}
	if len(request.Contact) > 10 {
		return acmeProblem(http.StatusBadRequest, "malformed", "at most 10 contact URIs are allowed")
	}
	for _, contact := range request.Contact {
		contactURL, parseErr := url.Parse(contact)
		if parseErr != nil || strings.ContainsAny(contact, "\r\n") ||
			!(contactURL.Scheme == "mailto" && contactURL.Opaque != "" ||
				contactURL.Scheme == "https" && contactURL.Host != "") {
			return acmeProblem(http.StatusBadRequest, "invalidContact", "unsupported contact URI")
		}
	}
	if request.OnlyReturnExisting {
		s.mu.Lock()
		id := s.thumbprints[jws.thumbprint]
		account := s.accounts[id]
		if account != nil {
			account = cloneACMEAccount(account)
		}
		s.mu.Unlock()
		if account == nil {
			return acmeProblem(http.StatusNotFound, "accountDoesNotExist", "account does not exist")
		}
		c.Response().Header().Set("Location", s.url("/account/"+id))
		return c.JSON(http.StatusOK, accountJSON(account, s.url("/orders/"+id)))
	}
	if s.terms != "" && !request.TermsOfServiceAgreed {
		return acmeProblem(http.StatusBadRequest, "userActionRequired", "terms of service agreement is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return acmeProblem(http.StatusServiceUnavailable, "serverInternal", "ACME service is closed")
	}
	if id := s.thumbprints[jws.thumbprint]; id != "" {
		account := s.accounts[id]
		account.Contact = append([]string(nil), request.Contact...)
		c.Response().Header().Set("Location", s.url("/account/"+id))
		return c.JSON(http.StatusOK, accountJSON(account, s.url("/orders/"+id)))
	}
	if len(s.accounts) >= acmeMaxAccounts {
		return acmeProblem(http.StatusServiceUnavailable, "rateLimited", "ACME account capacity reached")
	}
	id, err := randomACMEID()
	if err != nil {
		return err
	}
	account := &acmeAccount{ID: id, Key: jws.publicKey, Thumb: jws.thumbprint, Contact: request.Contact, Status: "valid"}
	s.accounts[id] = account
	s.thumbprints[jws.thumbprint] = id
	c.Response().Header().Set("Location", s.url("/account/"+id))
	return c.JSON(http.StatusCreated, accountJSON(account, s.url("/orders/"+id)))
}

func accountJSON(account *acmeAccount, orders string) map[string]any {
	return map[string]any{"status": account.Status, "contact": account.Contact, "orders": orders}
}

func cloneACMEAccount(account *acmeAccount) *acmeAccount {
	clone := *account
	clone.Contact = append([]string(nil), account.Contact...)
	clone.OrderIDs = append([]string(nil), account.OrderIDs...)
	return &clone
}

func (s *acmeService) account(c echo.Context) error {
	account, jws, err := s.authenticate(c, "/account/"+c.Param("id"), false)
	if err != nil {
		return err
	}
	if len(jws.payload) != 0 {
		return acmeProblem(http.StatusBadRequest, "malformed", "account POST-as-GET payload must be empty")
	}
	if account.ID != c.Param("id") {
		return acmeProblem(http.StatusNotFound, "accountDoesNotExist", "account not found")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return c.JSON(http.StatusOK, accountJSON(account, s.url("/orders/"+account.ID)))
}

func (s *acmeService) accountOrders(c echo.Context) error {
	account, jws, err := s.authenticate(c, "/orders/"+c.Param("id"), false)
	if err != nil {
		return err
	}
	if len(jws.payload) != 0 {
		return acmeProblem(http.StatusBadRequest, "malformed", "orders POST-as-GET payload must be empty")
	}
	if account.ID != c.Param("id") {
		return acmeProblem(http.StatusNotFound, "accountDoesNotExist", "account not found")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	urls := make([]string, 0, len(account.OrderIDs))
	for _, id := range account.OrderIDs {
		urls = append(urls, s.url("/order/"+id))
	}
	return c.JSON(http.StatusOK, map[string]any{"orders": urls})
}

func (s *acmeService) newOrder(c echo.Context) error {
	account, jws, err := s.authenticate(c, "/new-order", false)
	if err != nil {
		return err
	}
	var request struct {
		Identifiers []acmeIdentifier `json:"identifiers"`
		NotBefore   time.Time        `json:"notBefore,omitempty"`
		NotAfter    time.Time        `json:"notAfter,omitempty"`
	}
	if err := json.Unmarshal(jws.payload, &request); err != nil {
		return acmeProblem(http.StatusBadRequest, "malformed", "invalid new-order payload")
	}
	if len(request.Identifiers) == 0 || len(request.Identifiers) > acmeMaxIdentifiers {
		return acmeProblem(http.StatusBadRequest, "malformed", fmt.Sprintf("new order must contain between 1 and %d identifiers", acmeMaxIdentifiers))
	}
	identifiers := make([]acmeIdentifier, len(request.Identifiers))
	seen := make(map[string]bool, len(request.Identifiers))
	for i, identifier := range request.Identifiers {
		normalized, err := normalizeACMEIdentifier(identifier)
		if err != nil {
			return acmeProblem(http.StatusBadRequest, "unsupportedIdentifier", err.Error())
		}
		key := normalized.Type + ":" + normalized.Value
		if seen[key] {
			return acmeProblem(http.StatusBadRequest, "malformed", "duplicate order identifier")
		}
		seen[key] = true
		identifiers[i] = normalized
	}
	if !request.NotBefore.IsZero() || !request.NotAfter.IsZero() {
		return acmeProblem(http.StatusBadRequest, "badPublicKey", "custom certificate validity dates are unsupported")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.accounts[account.ID] == nil {
		return acmeProblem(http.StatusServiceUnavailable, "serverInternal", "ACME service is closed")
	}
	if len(s.orders) >= acmeMaxOrders {
		return acmeProblem(http.StatusServiceUnavailable, "rateLimited", "ACME order capacity reached")
	}
	orderID, err := randomACMEID()
	if err != nil {
		return err
	}
	order := &acmeOrder{
		ID: orderID, AccountID: account.ID, Identifiers: identifiers, Status: "pending",
		Expires: time.Now().Add(24 * time.Hour), FinalizeURL: s.url("/finalize/" + orderID),
	}
	for _, identifier := range identifiers {
		authzID, err := randomACMEID()
		if err != nil {
			return err
		}
		authz := &acmeAuthorization{
			ID: authzID, AccountID: account.ID, Identifier: identifier, Status: "pending",
			Expires: order.Expires,
		}
		challengeTypes := []string{"http-01", "tls-alpn-01"}
		if identifier.Type == "dns" {
			challengeTypes = append(challengeTypes, "dns-01")
		}
		for _, challengeType := range challengeTypes {
			challengeID, err := randomACMEID()
			if err != nil {
				return err
			}
			token, err := randomACMEID()
			if err != nil {
				return err
			}
			challenge := &acmeChallengeRecord{
				ID: challengeID, AccountID: account.ID, AuthzID: authzID,
				Identifier: identifier, Type: challengeType, Token: token, Status: "pending",
			}
			s.challenges[challengeID] = challenge
			authz.Challenges = append(authz.Challenges, challengeID)
		}
		s.authzs[authzID] = authz
		order.Authorization = append(order.Authorization, s.url("/authz/"+authzID))
	}
	s.orders[orderID] = order
	account.OrderIDs = append(account.OrderIDs, orderID)
	c.Response().Header().Set("Location", s.url("/order/"+orderID))
	return c.JSON(http.StatusCreated, s.orderJSON(order))
}

func normalizeACMEIdentifier(identifier acmeIdentifier) (acmeIdentifier, error) {
	switch identifier.Type {
	case "dns":
		value := strings.ToLower(strings.TrimSuffix(identifier.Value, "."))
		if value == "" || len(value) > 253 || strings.Contains(value, "*") || strings.ContainsAny(value, "\r\n /\\") {
			return acmeIdentifier{}, fmt.Errorf("invalid or unsupported DNS identifier")
		}
		for _, label := range strings.Split(value, ".") {
			if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return acmeIdentifier{}, fmt.Errorf("invalid DNS identifier")
			}
			for _, r := range label {
				if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
					return acmeIdentifier{}, fmt.Errorf("invalid DNS identifier")
				}
			}
		}
		return acmeIdentifier{Type: "dns", Value: value}, nil
	case "ip":
		ip := net.ParseIP(identifier.Value)
		if ip == nil {
			return acmeIdentifier{}, fmt.Errorf("invalid IP identifier")
		}
		return acmeIdentifier{Type: "ip", Value: ip.String()}, nil
	default:
		return acmeIdentifier{}, fmt.Errorf("only DNS and IP identifiers are supported")
	}
}

func (s *acmeService) orderJSON(order *acmeOrder) map[string]any {
	result := map[string]any{
		"status": order.Status, "expires": order.Expires.UTC().Format(time.RFC3339),
		"identifiers": order.Identifiers, "authorizations": order.Authorization,
		"finalize": order.FinalizeURL,
	}
	if order.CertificateURL != "" {
		result["certificate"] = order.CertificateURL
	}
	if order.Status == "invalid" {
		result["error"] = map[string]string{"type": "urn:ietf:params:acme:error:unauthorized", "detail": "authorization challenge validation failed"}
	}
	return result
}

func (s *acmeService) order(c echo.Context) error {
	account, _, err := s.authenticate(c, "/order/"+c.Param("id"), false)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	order := s.orders[c.Param("id")]
	if order == nil || order.AccountID != account.ID {
		return acmeProblem(http.StatusNotFound, "orderNotFound", "order not found")
	}
	if !order.Expires.After(time.Now()) && (order.Status == "pending" || order.Status == "ready") {
		order.Status = "invalid"
	}
	return c.JSON(http.StatusOK, s.orderJSON(order))
}

func (s *acmeService) authorization(c echo.Context) error {
	account, _, err := s.authenticate(c, "/authz/"+c.Param("id"), false)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	authz := s.authzs[c.Param("id")]
	if authz == nil || authz.AccountID != account.ID {
		return acmeProblem(http.StatusNotFound, "authorizationNotFound", "authorization not found")
	}
	if !authz.Expires.After(time.Now()) && authz.Status == "pending" {
		authz.Status = "invalid"
		s.setOrderStatusLocked(authz.ID, "invalid")
	}
	challenges := make([]map[string]any, 0, len(authz.Challenges))
	for _, id := range authz.Challenges {
		challenge := s.challenges[id]
		item := map[string]any{
			"type": challenge.Type, "url": s.url("/challenge/" + id),
			"status": challenge.Status, "token": challenge.Token,
		}
		if challenge.Error != nil {
			item["error"] = challenge.Error
		}
		challenges = append(challenges, item)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"identifier": authz.Identifier, "status": authz.Status,
		"expires": authz.Expires.UTC().Format(time.RFC3339), "challenges": challenges,
	})
}

func (s *acmeService) challenge(c echo.Context) error {
	account, jws, err := s.authenticate(c, "/challenge/"+c.Param("id"), false)
	if err != nil {
		return err
	}
	challengeID := c.Param("id")
	s.mu.Lock()
	challenge := s.challenges[challengeID]
	if challenge == nil || challenge.AccountID != account.ID {
		s.mu.Unlock()
		return acmeProblem(http.StatusNotFound, "malformed", "challenge not found")
	}
	if len(jws.payload) != 0 && string(jws.payload) != "{}" {
		s.mu.Unlock()
		return acmeProblem(http.StatusBadRequest, "malformed", "challenge response payload must be empty")
	}
	if challenge.Status != "pending" {
		response := s.challengeJSON(challenge)
		s.mu.Unlock()
		return c.JSON(http.StatusOK, response)
	}
	authz := s.authzs[challenge.AuthzID]
	if authz == nil || !authz.Expires.After(time.Now()) {
		challenge.Status = "invalid"
		challenge.Error = map[string]string{
			"type": "urn:ietf:params:acme:error:unauthorized", "detail": "authorization has expired",
		}
		if authz != nil {
			authz.Status = "invalid"
			s.setOrderStatusLocked(authz.ID, "invalid")
		}
		response := s.challengeJSON(challenge)
		s.mu.Unlock()
		return c.JSON(http.StatusOK, response)
	}
	challenge.Status = "processing"
	challengeCopy := *challenge
	s.mu.Unlock()

	var validateErr error
	keyAuth := challengeCopy.Token + "." + account.Thumb
	if s.check == nil {
		validateErr = errors.New("challenge validation is not configured; challenge type unsupported")
	} else {
		validateErr = s.check(c.Request().Context(), ACMEChallenge{
			Type: challengeCopy.Type, Identifier: challengeCopy.Identifier.Value,
			Token: challengeCopy.Token, KeyAuthorization: keyAuth,
		})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge = s.challenges[challengeID]
	if challenge == nil {
		return acmeProblem(http.StatusNotFound, "malformed", "challenge no longer exists")
	}
	if validateErr != nil {
		challenge.Status = "invalid"
		detail := validateErr.Error()
		if len(detail) > 512 {
			detail = detail[:512]
		}
		challenge.Error = map[string]string{"type": "urn:ietf:params:acme:error:unauthorized", "detail": detail}
		authz := s.authzs[challenge.AuthzID]
		if authz != nil {
			authz.Status = "invalid"
			s.setOrderStatusLocked(authz.ID, "invalid")
		}
	} else {
		challenge.Status = "valid"
		if authz := s.authzs[challenge.AuthzID]; authz != nil {
			authz.Status = "valid"
			s.updateOrderReadinessLocked(authz.ID)
		}
	}
	return c.JSON(http.StatusOK, s.challengeJSON(challenge))
}

func (s *acmeService) challengeJSON(challenge *acmeChallengeRecord) map[string]any {
	result := map[string]any{
		"type": challenge.Type, "url": s.url("/challenge/" + challenge.ID),
		"status": challenge.Status, "token": challenge.Token,
	}
	if challenge.Error != nil {
		result["error"] = challenge.Error
	}
	return result
}

func (s *acmeService) setOrderStatusLocked(authzID, status string) {
	for _, order := range s.orders {
		for _, urlValue := range order.Authorization {
			if strings.HasSuffix(urlValue, "/authz/"+authzID) && order.Status != "valid" {
				order.Status = status
			}
		}
	}
}

func (s *acmeService) updateOrderReadinessLocked(authzID string) {
	for _, order := range s.orders {
		for _, urlValue := range order.Authorization {
			if strings.HasSuffix(urlValue, "/authz/"+authzID) {
				allValid := true
				for _, authzURL := range order.Authorization {
					id := strings.TrimPrefix(authzURL, s.url("/authz/"))
					if authz := s.authzs[id]; authz == nil || authz.Status != "valid" {
						allValid = false
						break
					}
				}
				if allValid && order.Status == "pending" {
					order.Status = "ready"
				}
			}
		}
	}
}

func (s *acmeService) finalize(c echo.Context) error {
	account, jws, err := s.authenticate(c, "/finalize/"+c.Param("id"), false)
	if err != nil {
		return err
	}
	var request struct {
		CSR string `json:"csr"`
	}
	if err := json.Unmarshal(jws.payload, &request); err != nil || request.CSR == "" {
		return acmeProblem(http.StatusBadRequest, "malformed", "finalize payload must contain a CSR")
	}
	csrDER, err := base64.RawURLEncoding.DecodeString(request.CSR)
	if err != nil || len(csrDER) > 128*1024 {
		return acmeProblem(http.StatusBadRequest, "malformed", "invalid or oversized CSR")
	}
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil || csr.CheckSignature() != nil {
		return acmeProblem(http.StatusBadRequest, "badCSR", "CSR signature is invalid")
	}
	s.mu.Lock()
	order := s.orders[c.Param("id")]
	if order == nil || order.AccountID != account.ID {
		s.mu.Unlock()
		return acmeProblem(http.StatusNotFound, "orderNotFound", "order not found")
	}
	if order.Status == "valid" && order.Serial != "" {
		response := s.orderJSON(order)
		s.mu.Unlock()
		return c.JSON(http.StatusOK, response)
	}
	if !order.Expires.After(time.Now()) && (order.Status == "pending" || order.Status == "ready") {
		order.Status = "invalid"
	}
	if order.Status != "ready" {
		s.mu.Unlock()
		return acmeProblem(http.StatusBadRequest, "orderNotReady", "all authorizations must be valid before finalization")
	}
	if !csrMatchesACMEOrder(csr, order.Identifiers) {
		s.mu.Unlock()
		return acmeProblem(http.StatusBadRequest, "badCSR", "CSR identifiers must exactly match the order identifiers")
	}
	order.Status = "processing"
	s.mu.Unlock()

	certificate, err := s.manager.IssueCertificateFromCSR(csrDER, "acme")
	s.mu.Lock()
	defer s.mu.Unlock()
	order = s.orders[c.Param("id")]
	if err != nil {
		order.Status = "ready"
		return acmeProblem(http.StatusBadRequest, "badCSR", "certificate signing failed")
	}
	order.Serial = certificate.Serial
	order.CertificateURL = s.url("/cert/" + order.ID)
	order.Status = "valid"
	return c.JSON(http.StatusOK, s.orderJSON(order))
}

func csrMatchesACMEOrder(csr *x509.CertificateRequest, identifiers []acmeIdentifier) bool {
	if csr == nil || len(csr.EmailAddresses) != 0 || len(csr.DNSNames)+len(csr.IPAddresses) != len(identifiers) {
		return false
	}
	actual := make([]string, 0, len(identifiers))
	for _, name := range csr.DNSNames {
		normal, err := normalizeACMEIdentifier(acmeIdentifier{Type: "dns", Value: name})
		if err != nil {
			return false
		}
		actual = append(actual, normal.Type+":"+normal.Value)
	}
	for _, ip := range csr.IPAddresses {
		actual = append(actual, "ip:"+ip.String())
	}
	expected := make([]string, 0, len(identifiers))
	for _, identifier := range identifiers {
		expected = append(expected, identifier.Type+":"+identifier.Value)
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if strings.Join(actual, "\x00") != strings.Join(expected, "\x00") {
		return false
	}
	commonName := strings.ToLower(strings.TrimSuffix(csr.Subject.CommonName, "."))
	for _, identifier := range identifiers {
		if identifier.Type == "dns" && commonName == identifier.Value {
			return true
		}
		if identifier.Type == "ip" {
			cnIP := net.ParseIP(csr.Subject.CommonName)
			if cnIP != nil && cnIP.Equal(net.ParseIP(identifier.Value)) {
				return true
			}
		}
	}
	return false
}

func (s *acmeService) certificate(c echo.Context) error {
	account, _, err := s.authenticate(c, "/cert/"+c.Param("id"), false)
	if err != nil {
		return err
	}
	s.mu.Lock()
	order := s.orders[c.Param("id")]
	if order == nil || order.AccountID != account.ID || order.Status != "valid" || order.Serial == "" {
		s.mu.Unlock()
		return acmeProblem(http.StatusNotFound, "certificateNotFound", "certificate not found")
	}
	serial := order.Serial
	s.mu.Unlock()
	cert, err := s.manager.GetCertificate(serial)
	if err != nil {
		return acmeProblem(http.StatusNotFound, "certificateNotFound", "certificate not found")
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/pem-certificate-chain")
	c.Response().WriteHeader(http.StatusOK)
	_, err = c.Response().Write(append([]byte(cert.CertPEM), []byte(s.manager.GetCACertPEM())...))
	return err
}

func (s *acmeService) revokeCertificate(c echo.Context) error {
	account, jws, err := s.authenticate(c, "/revoke-cert", false)
	if err != nil {
		return err
	}
	var request struct {
		Certificate string `json:"certificate"`
		Reason      *int   `json:"reason"`
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(jws.payload, &payload); err != nil {
		return acmeProblem(http.StatusBadRequest, "malformed", "invalid revoke-cert payload")
	}
	if err := json.Unmarshal(jws.payload, &request); err != nil || request.Certificate == "" {
		return acmeProblem(http.StatusBadRequest, "malformed", "revoke-cert payload must contain a certificate")
	}
	if request.Reason != nil && (*request.Reason < 0 || *request.Reason > 10) {
		return acmeProblem(http.StatusBadRequest, "malformed", "invalid revocation reason")
	}
	der, err := base64.RawURLEncoding.DecodeString(request.Certificate)
	if err != nil {
		return acmeProblem(http.StatusBadRequest, "malformed", "invalid certificate encoding")
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return acmeProblem(http.StatusBadRequest, "malformed", "invalid certificate")
	}
	serial := cert.SerialNumber.Text(16)
	s.mu.Lock()
	var ownedSerial string
	for _, order := range s.orders {
		if order.AccountID == account.ID && strings.EqualFold(order.Serial, serial) {
			ownedSerial = order.Serial
			break
		}
	}
	s.mu.Unlock()
	if ownedSerial == "" {
		return acmeProblem(http.StatusUnauthorized, "unauthorized", "account does not own this ACME certificate")
	}
	issued, err := s.manager.GetCertificate(ownedSerial)
	if err != nil {
		return acmeProblem(http.StatusUnauthorized, "unauthorized", "account does not own this ACME certificate")
	}
	block, _ := pem.Decode([]byte(issued.CertPEM))
	if block == nil || !bytes.Equal(block.Bytes, cert.Raw) {
		return acmeProblem(http.StatusBadRequest, "malformed", "certificate does not match the issued ACME certificate")
	}
	if err := s.manager.RevokeCertificate(ownedSerial); err != nil {
		return acmeProblem(http.StatusBadRequest, "alreadyRevoked", "certificate could not be revoked")
	}
	return c.NoContent(http.StatusOK)
}

func randomACMEID() (string, error) {
	b := make([]byte, 18)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func acmeProblem(status int, code, detail string) error {
	return &acmeProblemError{status: status, body: map[string]any{
		"type": "urn:ietf:params:acme:error:" + code, "detail": detail, "status": status,
	}}
}
