package referralregistration

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/mail"
	"net/netip"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	persistence "task-processor/internal/integration/persistence/referral"
	"task-processor/internal/referral"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sealedAdmission struct {
	Request Request
	Secret  string
}

// InstallSchema is an explicit operational composition entrypoint, never serving.
// All connection coordinates must be present; environment defaults cannot select a target.
func InstallSchema(ctx context.Context, dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil || ctx == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.Port() == "" || u.User == nil || u.User.Username() == "" || len(u.Path) < 2 || u.Fragment != "" {
		return referral.ErrInvalid
	}
	password, ok := u.User.Password()
	if !ok || password == "" {
		return referral.ErrInvalid
	}
	query := u.Query()
	if len(query) != 1 || len(query["sslmode"]) != 1 {
		return referral.ErrInvalid
	}
	mode := query.Get("sslmode")
	if mode != "disable" && mode != "require" && mode != "verify-full" {
		return referral.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return referral.ErrUnavailable
	}
	pool, err := db.DB()
	if err != nil {
		return referral.ErrUnavailable
	}
	defer pool.Close()
	pool.SetMaxOpenConns(1)
	pool.SetMaxIdleConns(1)
	if err = pool.PingContext(ctx); err != nil {
		return referral.ErrUnavailable
	}
	return persistence.Install(ctx, db)
}

func digest(key []byte, values ...string) string {
	b, _ := json.Marshal(values)
	h := hmac.New(sha256.New, key)
	_, _ = h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Service) valid() bool {
	if s == nil || s.Store == nil || s.Provider == nil || s.Now == nil || s.Issuer == "" || s.Instance == "" || s.Organization == "" {
		return false
	}
	e, p := s.Keys.Encryption[s.Keys.Active], s.Keys.Proof[s.Keys.Active]
	return len(e) == 32 && len(p) == 32 && len(s.Keys.Lookup) == 32 && !hmac.Equal(e, p) && !hmac.Equal(e, s.Keys.Lookup) && !hmac.Equal(p, s.Keys.Lookup)
}

func validText(v string, maximum int) bool {
	return v != "" && len(v) <= maximum && utf8.ValidString(v) && strings.TrimSpace(v) == v && !strings.ContainsFunc(v, unicode.IsControl)
}

func (s *Service) seal(i referral.Intent, p sealedAdmission) ([]byte, error) {
	block, err := aes.NewCipher(s.Keys.Encryption[i.KeyID])
	if err != nil {
		return nil, referral.ErrUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, referral.ErrUnavailable
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, referral.ErrUnavailable
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, referral.ErrInvalid
	}
	return aead.Seal(nonce, nonce, b, []byte(i.Issuer+"/"+i.ID)), nil
}

func (s *Service) open(i referral.Intent) (sealedAdmission, error) {
	var p sealedAdmission
	block, err := aes.NewCipher(s.Keys.Encryption[i.KeyID])
	if err != nil {
		return p, referral.ErrUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(i.Ciphertext) < aead.NonceSize() {
		return p, referral.ErrUnavailable
	}
	n := aead.NonceSize()
	b, err := aead.Open(nil, i.Ciphertext[:n], i.Ciphertext[n:], []byte(i.Issuer+"/"+i.ID))
	if err != nil {
		return p, referral.ErrUnavailable
	}
	if json.Unmarshal(b, &p) != nil {
		return p, referral.ErrUnavailable
	}
	return p, nil
}

// Start admits a durable intent only. Resume explicitly continues the external
// operation using the returned capability; no unconfirmed admission can create a user.
func (s *Service) Start(ctx context.Context, r Request) (Admission, error) {
	if !s.valid() || ctx == nil {
		return Admission{}, referral.ErrUnavailable
	}
	if !s.enter() {
		return Admission{}, referral.ErrLimited
	}
	defer s.leave()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if len(r.Key) != 64 { // A stable 256-bit client key is not an email lookup credential.
		return Admission{}, referral.ErrInvalid
	}
	if _, err := hex.DecodeString(r.Key); err != nil {
		return Admission{}, referral.ErrInvalid
	}
	if !validText(r.Code, 200) || !validText(r.Email, 200) || !validText(r.GivenName, 120) || !validText(r.FamilyName, 120) {
		return Admission{}, referral.ErrInvalid
	}
	address, err := mail.ParseAddress(r.Email)
	if err != nil || address.Address != r.Email {
		return Admission{}, referral.ErrInvalid
	}
	r.Email = strings.ToLower(r.Email)
	keyHash := digest(s.Keys.Lookup, "admission-key", r.Key)
	fp := digest(s.Keys.Lookup, "request", r.Code, r.Email, r.GivenName, r.FamilyName)
	now := s.Now().UTC().Truncate(time.Microsecond)
	ip, err := netip.ParseAddr(r.ClientIP)
	if err != nil || ip.Zone() != "" {
		return Admission{}, referral.ErrInvalid
	}
	if err = s.Store.AllowIP(ctx, digest(s.Keys.Lookup, "client-ip", ip.Unmap().String()), now); err != nil {
		return Admission{}, err
	}
	if err = s.Store.Cleanup(ctx, now); err != nil {
		return Admission{}, err
	}
	id := make([]byte, 32)
	if _, err = rand.Read(id); err != nil {
		return Admission{}, referral.ErrUnavailable
	}
	subject, err := uuid.NewV7()
	if err != nil {
		return Admission{}, referral.ErrUnavailable
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return Admission{}, referral.ErrUnavailable
	}
	p := sealedAdmission{Request: r, Secret: base64.RawURLEncoding.EncodeToString(secret)}
	i := referral.Intent{ID: base64.RawURLEncoding.EncodeToString(id), Subject: subject.String(), Issuer: s.Issuer, Instance: s.Instance, Organization: s.Organization, KeyID: s.Keys.Active, KeyHash: keyHash, EmailHash: digest(s.Keys.Lookup, "email", r.Email), Fingerprint: fp, SecretHash: digest(s.Keys.Lookup, "resume", p.Secret), CreatedAt: now, CreateExpiresAt: now.Add(15 * time.Minute), CompletionExpiresAt: now.Add(24 * time.Hour), State: "PREPARED"}
	i.Ciphertext, err = s.seal(i, p)
	if err != nil {
		return Admission{}, err
	}
	i, err = s.Store.Admit(ctx, i, r.Code)
	if err != nil {
		return Admission{}, err
	}
	if i.Fingerprint != fp {
		return Admission{}, referral.ErrConflict
	}
	if !now.Before(i.CompletionExpiresAt) || i.State == "CONSUMED" {
		return Admission{}, referral.ErrExpired
	}
	p, err = s.open(i)
	if err != nil {
		return Admission{}, err
	}
	return Admission{i.ID, i.Subject, p.Secret, i.CreateExpiresAt, i.CompletionExpiresAt}, nil
}
