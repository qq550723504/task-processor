// Package subjectverification owns enterprise verification applications, not IAM.
package subjectverification

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"task-processor/internal/authidentity"
)

var (
	ErrUnavailable = errors.New("verification unavailable")
	ErrInvalid     = errors.New("invalid verification request")
	ErrConflict    = errors.New("verification application conflict")
	ErrNotFound    = errors.New("verification application not found")
)

const (
	Unknown  = "OUTCOME_UNKNOWN"
	Pending  = "PENDING"
	Verified = "VERIFIED"
)

type Input struct {
	CompanyName    string `json:"companyName"`
	CreditCode     string `json:"creditCode"`
	LegalName      string `json:"legalName,omitempty"`
	Consent        bool   `json:"consent"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type Actor struct{ OrganizationID, UserID, VerifiedPhone string }
type Application struct {
	ID, Scope, OrganizationID, ActorID, IdempotencyKey, InputDigest       string
	CompanyName, CreditCode, PhoneDigest, MaskedPhone, Correlation, State string
	EncryptedURL                                                          []byte
	ProviderOrganizationID, ProviderAdminID                               string
	CreatedAt, ExpiresAt, ProviderVerifiedAt, ObservedAt                  time.Time
}
type CreateRequest struct{ CompanyName, CreditCode, LegalName, Phone, Correlation string }
type Link struct {
	URL       string
	ExpiresAt time.Time
}
type Event struct {
	MessageID, Digest, Correlation, CompanyName, CreditCode, Phone string
	ProviderOrganizationID, ProviderAdminID                        string
	VerifiedAt                                                     time.Time
}
type Receipt struct{ Scope, MessageID, Digest, Correlation string }
type Provider interface {
	Create(context.Context, CreateRequest) (Link, error)
}
type Store interface {
	Reserve(context.Context, Application) (Application, bool, error)
	Read(context.Context, string) (Application, error)
	SaveLink(context.Context, string, []byte, time.Time) error
	Apply(context.Context, Receipt, func(*Application) string) error
}
type Protection interface {
	Digest(string, string) string
	Seal(string, string) ([]byte, error)
	Open(string, []byte) (string, error)
}
type Service struct {
	Store      Store
	Provider   Provider
	Protection Protection
	Scope      string
	Now        func() time.Time
}

func (s *Service) enabled() bool {
	return s != nil && s.Store != nil && s.Provider != nil && s.Protection != nil && authidentity.IsBoundedIdentifier(s.Scope)
}
func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s *Service) Start(ctx context.Context, actor Actor, in Input) (Application, error) {
	if !s.enabled() {
		return Application{}, ErrUnavailable
	}
	phone := NormalizePhone(actor.VerifiedPhone)
	in.CompanyName = NormalizeName(in.CompanyName)
	in.CreditCode = strings.TrimSpace(in.CreditCode)
	in.LegalName = strings.TrimSpace(in.LegalName)
	if !authidentity.IsBoundedIdentifier(actor.OrganizationID) || !authidentity.IsBoundedIdentifier(actor.UserID) || phone == "" || !in.Consent || !validText(in.CompanyName, 256) || !regexp.MustCompile(`^[0-9A-Z]{18}$`).MatchString(in.CreditCode) || len(in.LegalName) > 128 || (in.LegalName != "" && !validText(in.LegalName, 128)) {
		return Application{}, ErrInvalid
	}
	key, err := uuid.Parse(in.IdempotencyKey)
	if err != nil || key == uuid.Nil {
		return Application{}, ErrInvalid
	}
	in.IdempotencyKey = key.String()
	payload, _ := json.Marshal(struct {
		Input Input
		Phone string
	}{in, phone})
	a := Application{ID: uuid.NewString(), Scope: s.Scope, OrganizationID: actor.OrganizationID, ActorID: actor.UserID, IdempotencyKey: in.IdempotencyKey, InputDigest: s.Protection.Digest("input", string(payload)), CompanyName: in.CompanyName, CreditCode: in.CreditCode, PhoneDigest: s.Protection.Digest("phone", phone), MaskedPhone: phone[:3] + "****" + phone[7:], Correlation: base64.StdEncoding.EncodeToString([]byte(uuid.NewString())), State: Unknown, CreatedAt: s.now()}
	a, created, err := s.Store.Reserve(ctx, a)
	if err != nil {
		return Application{}, err
	}
	if a.Scope != s.Scope || a.ActorID != actor.UserID || a.IdempotencyKey != in.IdempotencyKey || a.InputDigest != s.Protection.Digest("input", string(payload)) {
		return Application{}, ErrConflict
	}
	if !created {
		return a, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	link, err := s.Provider.Create(callCtx, CreateRequest{in.CompanyName, in.CreditCode, in.LegalName, phone, a.Correlation})
	if err != nil || !ValidProviderURL(link.URL) || !link.ExpiresAt.After(s.now()) || link.ExpiresAt.After(s.now().Add(31*24*time.Hour)) {
		return a, nil
	}
	encrypted, err := s.Protection.Seal(a.Binding(), link.URL)
	if err != nil {
		return a, nil
	}
	if err = s.Store.SaveLink(ctx, a.ID, encrypted, link.ExpiresAt); err != nil {
		return a, nil
	}
	return s.Store.Read(ctx, actor.OrganizationID)
}
func (s *Service) Read(ctx context.Context, actor Actor) (Application, string, error) {
	if !s.enabled() {
		return Application{}, "", ErrUnavailable
	}
	if !authidentity.IsBoundedIdentifier(actor.OrganizationID) || !authidentity.IsBoundedIdentifier(actor.UserID) {
		return Application{}, "", ErrInvalid
	}
	a, err := s.Store.Read(ctx, actor.OrganizationID)
	if err != nil {
		return a, "", err
	}
	if a.Scope != s.Scope {
		return Application{}, "", ErrConflict
	}
	if a.State != Pending || a.ActorID != actor.UserID || !a.ExpiresAt.After(s.now()) {
		return a, "", nil
	}
	link, err := s.Protection.Open(a.Binding(), a.EncryptedURL)
	if err != nil || !ValidProviderURL(link) {
		return a, "", ErrUnavailable
	}
	return a, link, nil
}
func (s *Service) Observe(ctx context.Context, e Event) error {
	if !s.enabled() {
		return ErrUnavailable
	}
	if !validText(e.MessageID, 128) || len(e.Digest) == 0 || len(e.Digest) > 128 || len(e.Correlation) == 0 || len(e.Correlation) > 1000 {
		return ErrInvalid
	}
	phone := NormalizePhone(e.Phone)
	return s.Store.Apply(ctx, Receipt{s.Scope, e.MessageID, e.Digest, e.Correlation}, func(a *Application) string {
		if a.Scope != s.Scope || phone == "" || s.Protection.Digest("phone", phone) != a.PhoneDigest || NormalizeName(e.CompanyName) != a.CompanyName || e.CreditCode != a.CreditCode || !authidentity.IsBoundedIdentifier(e.ProviderOrganizationID) || !authidentity.IsBoundedIdentifier(e.ProviderAdminID) || e.VerifiedAt.IsZero() || e.VerifiedAt.After(s.now().Add(5*time.Minute)) {
			return "SUBJECT_MISMATCH"
		}
		if a.State == Verified {
			return "DUPLICATE_RESULT"
		}
		a.State = Verified
		a.ProviderOrganizationID = e.ProviderOrganizationID
		a.ProviderAdminID = e.ProviderAdminID
		a.ProviderVerifiedAt = e.VerifiedAt.UTC()
		a.ObservedAt = s.now()
		a.EncryptedURL = nil
		return "VERIFIED"
	})
}
func (a Application) Binding() string {
	return a.Scope + "\x00" + a.OrganizationID + "\x00" + a.ActorID + "\x00" + a.ID
}

var phonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

func NormalizePhone(phone string) string {
	phone = strings.TrimPrefix(phone, "+86")
	if !phonePattern.MatchString(phone) {
		return ""
	}
	return phone
}
func NormalizeName(name string) string {
	return strings.NewReplacer("(", "（", ")", "）").Replace(strings.TrimSpace(name))
}
func validText(v string, max int) bool {
	if v == "" || len(v) > max || v != strings.TrimSpace(v) {
		return false
	}
	return strings.IndexFunc(v, unicode.IsControl) < 0
}
func ValidProviderURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" || len(raw) > 8192 {
		return false
	}
	for _, host := range []string{"qian.tencent.cn", "qian.tencent.com", "ess.tencent.cn", "essurl.cn"} {
		if u.Hostname() == host || strings.HasSuffix(u.Hostname(), "."+host) {
			return true
		}
	}
	return false
}
