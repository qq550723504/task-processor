// Package dataservice owns limited data-service credentials and customization.
package dataservice

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/netip"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"task-processor/internal/product/collection"
)

const (
	PermissionManage  = "workbench.data-api.manage"
	PermissionMarket  = "workbench.data-market.use"
	PermissionAcquire = "amazon.acquire"
	PermissionResult  = "amazon.result.read"
)

var (
	ErrInvalid     = errors.New("invalid data service request")
	ErrForbidden   = errors.New("data service access denied")
	ErrConflict    = errors.New("data service revision or command conflict")
	ErrNotFound    = errors.New("data service resource not found")
	ErrUnavailable = errors.New("data service dependency unavailable")
	ErrUnknown     = errors.New("data service commit outcome unknown")
)

type KeyInput struct {
	Name           string    `json:"name"`
	ExpiresAt      time.Time `json:"expiresAt"`
	DailyRows      int64     `json:"dailyRows"`
	MonthlyCostFen int64     `json:"monthlyCostFen"`
	Permissions    []string  `json:"permissions"`
	CIDRs          []string  `json:"cidrs,omitempty"`
}
type Credential struct {
	ID        string           `json:"id"`
	Scope     collection.Scope `json:"-"`
	Input     KeyInput         `json:"limits"`
	Digest    string           `json:"-"`
	Suffix    string           `json:"suffix"`
	State     string           `json:"state"`
	Revision  int64            `json:"revision"`
	CreatedAt time.Time        `json:"createdAt"`
}
type KeyCreated struct {
	Key      Credential `json:"key"`
	Secret   string     `json:"secret,omitempty"`
	Replayed bool       `json:"replayed"`
}
type KeyPatch struct {
	State  string    `json:"state"`
	Limits *KeyInput `json:"limits,omitempty"`
}
type Principal struct {
	Scope    collection.Scope
	KeyID    string
	Revision int64
}
type CredentialHistoryPage struct {
	Items      []Credential `json:"items"`
	NextCursor string       `json:"nextCursor,omitempty"`
}
type CredentialRepository interface {
	Create(context.Context, Credential, string, string) (Credential, bool, error)
	Read(context.Context, string) (Credential, error)
	List(context.Context, collection.Scope) ([]Credential, error)
	History(context.Context, collection.Scope, string, int) (CredentialHistoryPage, error)
	Change(context.Context, collection.Scope, string, string, string, int64, KeyPatch) (Credential, error)
	Creation(context.Context, collection.Scope, string) (Credential, error)
}

// Access always uses the original canonical grant. Implementations must read
// authoritative current permissions; no human bearer is retained for workers.
type Access interface {
	Resolve(context.Context, string) (collection.Scope, error)
	Check(context.Context, collection.Scope, string) error
}
type CredentialService struct {
	store  CredentialRepository
	access Access
	now    func() time.Time
}

func NewCredentialService(store CredentialRepository, access Access) (*CredentialService, error) {
	if store == nil || access == nil {
		return nil, ErrUnavailable
	}
	return &CredentialService{store, access, time.Now}, nil
}

func NormalizeKeyInput(input KeyInput, now time.Time) (KeyInput, error) {
	if input.Name == "" || input.Name != strings.TrimSpace(input.Name) || len(input.Name) > 80 || !utf8.ValidString(input.Name) || strings.ContainsAny(input.Name, "\x00\r\n") || !input.ExpiresAt.After(now) || input.ExpiresAt.After(now.Add(365*24*time.Hour)) || input.DailyRows < 1 || input.DailyRows > 1000000000 || input.MonthlyCostFen < 1 || input.MonthlyCostFen > 100000000000 || len(input.Permissions) < 1 || len(input.Permissions) > 2 || len(input.CIDRs) > 20 {
		return KeyInput{}, ErrInvalid
	}
	input.ExpiresAt = input.ExpiresAt.UTC().Truncate(time.Microsecond)
	input.Permissions = append([]string(nil), input.Permissions...)
	sort.Strings(input.Permissions)
	for i, p := range input.Permissions {
		if (p != PermissionAcquire && p != PermissionResult) || (i > 0 && input.Permissions[i-1] == p) {
			return KeyInput{}, ErrInvalid
		}
	}
	normal := []string{}
	seen := map[string]bool{}
	for _, s := range input.CIDRs {
		p, err := netip.ParsePrefix(s)
		if err != nil || p.Addr().Is4In6() {
			return KeyInput{}, ErrInvalid
		}
		s = p.Masked().String()
		if !seen[s] {
			normal = append(normal, s)
			seen[s] = true
		}
	}
	sort.Strings(normal)
	input.CIDRs = normal
	return input, nil
}
func secretDigest(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(hash[:])
}
func (s *CredentialService) Create(ctx context.Context, command string, input KeyInput) (KeyCreated, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	scope, err := s.access.Resolve(ctx, PermissionManage)
	if err != nil || scope.Validate() != nil {
		return KeyCreated{}, ErrForbidden
	}
	if !collection.ValidID(command) {
		return KeyCreated{}, ErrInvalid
	}
	input, err = NormalizeKeyInput(input, s.now())
	if err != nil {
		return KeyCreated{}, err
	}
	for _, p := range input.Permissions {
		if err = s.access.Check(ctx, scope, p); err != nil {
			return KeyCreated{}, err
		}
	}
	bytes := make([]byte, 32)
	if _, err = rand.Read(bytes); err != nil {
		return KeyCreated{}, ErrUnavailable
	}
	secret := base64.RawURLEncoding.EncodeToString(bytes)
	key := Credential{ID: uuid.NewString(), Scope: scope, Input: input, Digest: secretDigest(secret), Suffix: secret[len(secret)-4:], State: "ACTIVE", Revision: 1, CreatedAt: s.now().UTC()}
	stored, replayed, err := s.store.Create(ctx, key, command, collection.Digest(input))
	if err != nil {
		return KeyCreated{}, err
	}
	result := KeyCreated{Key: stored, Replayed: replayed}
	if !replayed {
		result.Secret = secret
	}
	return result, nil
}
func (s *CredentialService) List(ctx context.Context) ([]Credential, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	scope, err := s.access.Resolve(ctx, PermissionManage)
	if err != nil {
		return nil, err
	}
	return s.store.List(ctx, scope)
}

// List contains every unexpired, non-revoked credential, including disabled
// credentials. Revoked and expired metadata has a separate bounded history.
func (s *CredentialService) History(ctx context.Context, cursor string, limit int) (CredentialHistoryPage, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	scope, err := s.access.Resolve(ctx, PermissionManage)
	if err != nil {
		return CredentialHistoryPage{}, err
	}
	if (cursor != "" && !collection.ValidID(cursor)) || limit < 1 || limit > 100 {
		return CredentialHistoryPage{}, ErrInvalid
	}
	return s.store.History(ctx, scope, cursor, limit)
}

// Creation verifies a committed create command after acknowledgement loss.
// Only original metadata is returned; the digest and secret are never replayed.
func (s *CredentialService) Creation(ctx context.Context, command string) (Credential, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	scope, err := s.access.Resolve(ctx, PermissionManage)
	if err != nil {
		return Credential{}, err
	}
	if !collection.ValidID(command) {
		return Credential{}, ErrInvalid
	}
	return s.store.Creation(ctx, scope, command)
}
func (s *CredentialService) Change(ctx context.Context, id, command string, revision int64, patch KeyPatch) (Credential, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	scope, err := s.access.Resolve(ctx, PermissionManage)
	if err != nil {
		return Credential{}, err
	}
	if !collection.ValidID(id) || !collection.ValidID(command) || revision < 1 || (patch.State != "ACTIVE" && patch.State != "DISABLED" && patch.State != "REVOKED") {
		return Credential{}, ErrInvalid
	}
	if patch.Limits != nil {
		input, err := NormalizeKeyInput(*patch.Limits, s.now())
		if err != nil {
			return Credential{}, err
		}
		for _, p := range input.Permissions {
			if err = s.access.Check(ctx, scope, p); err != nil {
				return Credential{}, err
			}
		}
		patch.Limits = &input
	}
	return s.store.Change(ctx, scope, id, command, collection.Digest(struct {
		ID       string
		Revision int64
		Patch    KeyPatch
	}{id, revision, patch}), revision, patch)
}
func (s *CredentialService) Authenticate(ctx context.Context, header, peer, permission string) (Principal, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if permission != PermissionAcquire && permission != PermissionResult {
		return Principal{}, ErrForbidden
	}
	parts := strings.Split(strings.TrimPrefix(header, "DataKey "), ".")
	if !strings.HasPrefix(header, "DataKey ") || len(parts) != 2 || !collection.ValidID(parts[0]) || len(parts[1]) != 43 {
		return Principal{}, ErrForbidden
	}
	key, err := s.store.Read(ctx, parts[0])
	if err != nil {
		return Principal{}, ErrForbidden
	}
	computed := secretDigest(parts[1])
	if subtle.ConstantTimeCompare([]byte(computed), []byte(key.Digest)) != 1 || key.State != "ACTIVE" || !s.now().Before(key.Input.ExpiresAt) || key.Scope.Validate() != nil {
		return Principal{}, ErrForbidden
	}
	allowed := false
	for _, p := range key.Input.Permissions {
		allowed = allowed || p == permission
	}
	if !allowed {
		return Principal{}, ErrForbidden
	}
	if len(key.Input.CIDRs) > 0 {
		ip, err := netip.ParseAddr(peer)
		if err != nil {
			return Principal{}, ErrForbidden
		}
		match := false
		for _, value := range key.Input.CIDRs {
			p, err := netip.ParsePrefix(value)
			if err != nil {
				return Principal{}, ErrUnavailable
			}
			match = match || p.Contains(ip.Unmap())
		}
		if !match {
			return Principal{}, ErrForbidden
		}
	}
	if err = s.access.Check(ctx, key.Scope, permission); err != nil {
		return Principal{}, err
	}
	return Principal{Scope: key.Scope, KeyID: key.ID, Revision: key.Revision}, nil
}
