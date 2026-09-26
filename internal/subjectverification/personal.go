package subjectverification

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/authidentity"
)

const Rejected = "REJECTED"

type PersonalLimits struct{ Total, Daily, IntervalSeconds int }

func DefaultPersonalLimits() PersonalLimits { return PersonalLimits{5, 3, 60} }
func (l PersonalLimits) Valid() bool {
	return l.Total > 0 && l.Total <= 100 && l.Daily > 0 && l.Daily <= l.Total && l.IntervalSeconds > 0 && l.IntervalSeconds <= 86400
}

type PersonalInput struct {
	Name           string `json:"name"`
	IDNumber       string `json:"idNumber"`
	MetaInfo       string `json:"metaInfo"`
	Consent        bool   `json:"consent"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type PersonalApplication struct {
	ID, UserID, Scope, IdempotencyKey, InputDigest, IdentityDigest, PhoneDigest, MaskedPhone, State string
	ProviderSceneID                                                                                 int64
	ProviderCertifyID                                                                               string
	EncryptedURL                                                                                    []byte
	CreatedAt, ExpiresAt, PhoneVerifiedAt, VerifiedAt, RefreshAfter                                 time.Time
	RefreshToken                                                                                    string
}

func (a PersonalApplication) Binding() string {
	return "personal\x00" + a.Scope + "\x00" + a.UserID + "\x00" + a.ID
}

type PersonalQuota struct {
	TotalLimit     int       `json:"totalLimit"`
	TotalUsed      int       `json:"totalUsed"`
	TotalRemaining int       `json:"totalRemaining"`
	DailyLimit     int       `json:"dailyLimit"`
	DailyUsed      int       `json:"dailyUsed"`
	DailyRemaining int       `json:"dailyRemaining"`
	ServerTime     time.Time `json:"serverTime"`
	ResetAt        time.Time `json:"resetAt"`
	NextAllowedAt  time.Time `json:"nextAllowedAt"`
}
type PersonalSnapshot struct {
	Application PersonalApplication
	Quota       PersonalQuota
}
type PersonalView struct {
	UserID          string        `json:"userId"`
	State           string        `json:"state"`
	ApplicationID   string        `json:"applicationId,omitempty"`
	MaskedPhone     string        `json:"maskedPhone"`
	CanStart        bool          `json:"canStart"`
	CanRefresh      bool          `json:"canRefresh"`
	PhoneReady      bool          `json:"phoneReady"`
	VerificationURL string        `json:"verificationUrl,omitempty"`
	ExpiresAt       *time.Time    `json:"expiresAt,omitempty"`
	VerifiedAt      *time.Time    `json:"verifiedAt,omitempty"`
	Quota           PersonalQuota `json:"quota"`
}
type PersonalLimitError struct {
	Code       string
	RetryAfter int
}

func (e *PersonalLimitError) Error() string { return e.Code }
func CheckPersonalAdmission(s PersonalSnapshot) error {
	a, q := s.Application, s.Quota
	if a.State == Verified || ((a.State == Pending || a.State == Unknown) && a.ExpiresAt.After(q.ServerTime)) {
		return ErrConflict
	}
	if q.TotalUsed >= q.TotalLimit {
		return &PersonalLimitError{Code: "VERIFICATION_TOTAL_LIMIT"}
	}
	if q.DailyUsed >= q.DailyLimit {
		return &PersonalLimitError{Code: "VERIFICATION_DAILY_LIMIT", RetryAfter: max(1, int(math.Ceil(q.ResetAt.Sub(q.ServerTime).Seconds())))}
	}
	if q.NextAllowedAt.After(q.ServerTime) {
		return &PersonalLimitError{Code: "VERIFICATION_COOLDOWN", RetryAfter: max(1, int(math.Ceil(q.NextAllowedAt.Sub(q.ServerTime).Seconds())))}
	}
	return nil
}

type PersonalIdentity struct{ Name, IDNumber, Phone string }
type PersonalLink struct{ CertifyID, URL string }
type PersonalResult struct{ Passed, SubCode string }
type PersonalProvider interface {
	VerifyPhone(context.Context, PersonalIdentity) (bool, error)
	CreateFace(context.Context, string, int64, PersonalIdentity, string) (PersonalLink, error)
	QueryFace(context.Context, int64, string) (PersonalResult, error)
}
type PersonalStore interface {
	ReservePersonal(context.Context, PersonalApplication, PersonalLimits) (PersonalApplication, bool, error)
	ReadPersonal(context.Context, string, PersonalLimits) (PersonalSnapshot, error)
	// UpdatePersonal serializes with reservation and only updates the latest application.
	UpdatePersonal(context.Context, string, string, func(*PersonalApplication, time.Time) error) error
}
type PersonalService struct {
	Store      PersonalStore
	Provider   PersonalProvider
	Protection Protection
	Scope      string
	SceneID    int64
	Limits     PersonalLimits
}

func (s *PersonalService) enabled() bool {
	return s != nil && s.Store != nil && s.Provider != nil && s.Protection != nil && authidentity.IsBoundedIdentifier(s.Scope) && s.SceneID > 0 && s.Limits.Valid()
}

var identityCardPattern = regexp.MustCompile(`^[0-9]{17}[0-9X]$`)

func (s *PersonalService) Start(ctx context.Context, actor Actor, in PersonalInput) error {
	if !s.enabled() {
		return ErrUnavailable
	}
	phone := NormalizePhone(actor.VerifiedPhone)
	in.Name = strings.TrimSpace(in.Name)
	in.IDNumber = strings.ToUpper(strings.TrimSpace(in.IDNumber))
	key, err := uuid.Parse(in.IdempotencyKey)
	var meta map[string]json.RawMessage
	if !authidentity.IsBoundedIdentifier(actor.UserID) || phone == "" || !in.Consent || !validText(in.Name, 128) || !identityCardPattern.MatchString(in.IDNumber) || err != nil || key == uuid.Nil || len(in.MetaInfo) > 8192 || json.Unmarshal([]byte(in.MetaInfo), &meta) != nil || len(meta) == 0 {
		return ErrInvalid
	}
	// Device metadata is deliberately not part of business idempotency.
	payload, _ := json.Marshal([]string{in.Name, in.IDNumber, phone})
	identity, _ := json.Marshal([]string{in.Name, in.IDNumber})
	a := PersonalApplication{ID: uuid.NewString(), UserID: actor.UserID, Scope: s.Scope, ProviderSceneID: s.SceneID, IdempotencyKey: key.String(), InputDigest: s.Protection.Digest("personal-input", string(payload)), IdentityDigest: s.Protection.Digest("personal-identity", string(identity)), PhoneDigest: s.Protection.Digest("personal-phone", phone), MaskedPhone: phone[:3] + "****" + phone[7:], State: Unknown}
	a, created, err := s.Store.ReservePersonal(ctx, a, s.Limits)
	if err != nil {
		return err
	}
	if !created {
		return nil
	}
	inIdentity := PersonalIdentity{in.Name, in.IDNumber, phone}
	call, cancel := context.WithTimeout(ctx, 7*time.Second)
	matched, err := s.Provider.VerifyPhone(call, inIdentity)
	cancel()
	if err != nil {
		return nil
	}
	err = s.Store.UpdatePersonal(ctx, actor.UserID, a.ID, func(current *PersonalApplication, now time.Time) error {
		if current.State != Unknown || !current.ExpiresAt.After(now) {
			return ErrConflict
		}
		if !matched {
			current.State = Rejected
		} else {
			current.PhoneVerifiedAt = now
		}
		return nil
	})
	if err != nil || !matched {
		return nil
	}
	call, cancel = context.WithTimeout(ctx, 7*time.Second)
	link, err := s.Provider.CreateFace(call, a.ID, a.ProviderSceneID, inIdentity, in.MetaInfo)
	cancel()
	if err != nil || !validText(link.CertifyID, 256) || !ValidPersonalURL(link.URL) {
		return nil
	}
	encrypted, err := s.Protection.Seal(a.Binding(), link.URL)
	if err != nil {
		return nil
	}
	// A failed save never causes a second initialization request.
	_ = s.Store.UpdatePersonal(ctx, actor.UserID, a.ID, func(current *PersonalApplication, now time.Time) error {
		if current.State != Unknown || current.PhoneVerifiedAt.IsZero() || !current.ExpiresAt.After(now) {
			return ErrConflict
		}
		current.State = Pending
		current.ProviderCertifyID = link.CertifyID
		current.EncryptedURL = encrypted
		current.ExpiresAt = now.Add(30 * time.Minute)
		return nil
	})
	return nil
}
func (s *PersonalService) Read(ctx context.Context, actor Actor) (PersonalView, error) {
	if !s.enabled() {
		return PersonalView{}, ErrUnavailable
	}
	if !authidentity.IsBoundedIdentifier(actor.UserID) {
		return PersonalView{}, ErrInvalid
	}
	snap, err := s.Store.ReadPersonal(ctx, actor.UserID, s.Limits)
	if err != nil {
		return PersonalView{}, err
	}
	a, q := snap.Application, snap.Quota
	if a.ID != "" && a.Scope != s.Scope {
		return PersonalView{}, ErrConflict
	}
	q.TotalRemaining = max(0, q.TotalLimit-q.TotalUsed)
	q.DailyRemaining = max(0, q.DailyLimit-q.DailyUsed)
	phone := NormalizePhone(actor.VerifiedPhone)
	v := PersonalView{UserID: actor.UserID, State: "NOT_STARTED", PhoneReady: phone != "", Quota: q}
	if phone != "" {
		v.MaskedPhone = phone[:3] + "****" + phone[7:]
	}
	v.CanStart = v.PhoneReady && CheckPersonalAdmission(snap) == nil
	if a.ID == "" {
		return v, nil
	}
	v.ApplicationID = a.ID
	v.State = a.State
	v.MaskedPhone = a.MaskedPhone
	v.ExpiresAt = &a.ExpiresAt
	samePhone := phone != "" && s.Protection.Digest("personal-phone", phone) == a.PhoneDigest
	if (a.State == Pending || a.State == Unknown) && !a.ExpiresAt.After(q.ServerTime) {
		v.State = "EXPIRED"
	}
	v.CanRefresh = a.State == Pending && samePhone && a.ProviderCertifyID != ""
	if a.State == Verified {
		v.VerifiedAt = &a.VerifiedAt
	}
	if v.State == Pending && samePhone {
		v.VerificationURL, err = s.Protection.Open(a.Binding(), a.EncryptedURL)
		if err != nil || !ValidPersonalURL(v.VerificationURL) {
			return PersonalView{}, ErrUnavailable
		}
	}
	return v, nil
}
func (s *PersonalService) Refresh(ctx context.Context, actor Actor, id string) error {
	if !s.enabled() {
		return ErrUnavailable
	}
	if !authidentity.IsBoundedIdentifier(actor.UserID) || NormalizePhone(actor.VerifiedPhone) == "" {
		return ErrInvalid
	}
	token := uuid.NewString()
	var claimed PersonalApplication
	err := s.Store.UpdatePersonal(ctx, actor.UserID, id, func(a *PersonalApplication, now time.Time) error {
		if a.Scope != s.Scope || a.State != Pending || a.ProviderCertifyID == "" || a.PhoneVerifiedAt.IsZero() || s.Protection.Digest("personal-phone", NormalizePhone(actor.VerifiedPhone)) != a.PhoneDigest {
			return ErrConflict
		}
		if a.RefreshAfter.After(now) {
			return &PersonalLimitError{Code: "VERIFICATION_REFRESH_BUSY", RetryAfter: max(1, int(math.Ceil(a.RefreshAfter.Sub(now).Seconds())))}
		}
		a.RefreshToken = token
		a.RefreshAfter = now.Add(10 * time.Second)
		claimed = *a
		return nil
	})
	if err != nil {
		return err
	}
	call, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	result, err := s.Provider.QueryFace(call, claimed.ProviderSceneID, claimed.ProviderCertifyID)
	if err != nil {
		return ErrUnavailable
	}
	return s.Store.UpdatePersonal(ctx, actor.UserID, id, func(a *PersonalApplication, now time.Time) error {
		if a.State != Pending || a.RefreshToken != token || a.PhoneVerifiedAt.IsZero() {
			return ErrConflict
		}
		if result.Passed == "T" {
			a.State = Verified
			a.VerifiedAt = now
		} else if result.Passed == "F" && isFinalFaceRejection(result.SubCode) {
			a.State = Rejected
		}
		if a.State != Pending {
			a.EncryptedURL = nil
		}
		return nil
	})
}
func isFinalFaceRejection(code string) bool {
	switch code {
	case "201", "202", "203", "204", "205", "206":
		return true
	}
	return false
}
func ValidPersonalURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && len(raw) <= 8192 && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == "" && u.Port() == ""
}
