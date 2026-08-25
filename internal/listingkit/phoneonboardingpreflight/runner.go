package phoneonboardingpreflight

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

const preflightSessionLifetime = 5 * time.Minute
const cleanupTimeout = 10 * time.Second

// Attempt retains only the opaque references needed to complete one preflight.
// The phone number, technical email, OTP code, and session token stay in memory.
type Attempt struct {
	OrganizationID string
	UserID         string
	SessionID      string
	sessionToken   string
	id             string
}

// Runner performs one interactive native-phone onboarding preflight.
type Runner struct {
	client Client
	random io.Reader
	now    func() time.Time
	output io.Writer
}

// NewRunner constructs an isolated preflight runner.
func NewRunner(client Client, random io.Reader, now func() time.Time, output io.Writer) (*Runner, error) {
	if client == nil || random == nil || now == nil || output == nil {
		return nil, errors.New("invalid phone onboarding preflight runner")
	}
	return &Runner{client: client, random: random, now: now, output: output}, nil
}

// Start creates a disposable organization and technical user before sending an
// OTP SMS challenge. The phone number is never retained after this call.
func (r *Runner) Start(ctx context.Context, phone string) (*Attempt, error) {
	normalizedPhone, err := normalizeE164(phone)
	if err != nil {
		return nil, errors.New("invalid phone number")
	}
	attemptID, err := newULID(r.now(), r.random)
	if err != nil {
		return nil, errors.New("preflight identifier generation failed")
	}
	attempt := &Attempt{id: attemptID}

	organizationID, err := r.client.CreateOrganization(ctx, "lk-phone-preflight-"+attemptID)
	if err != nil {
		return nil, r.fail(attemptID, "organization_create", err)
	}
	attempt.OrganizationID = organizationID

	userID, err := r.client.CreateTechnicalUser(ctx, TechnicalUserInput{
		OrganizationID: organizationID,
		Username:       "lkp-" + attemptID,
		TechnicalEmail: "u-" + attemptID + "@phone.invalid",
		Phone:          normalizedPhone,
	})
	if err != nil {
		return nil, r.fail(attemptID, "user_create", err)
	}
	attempt.UserID = userID

	if err := r.client.AddOTPSMS(ctx, userID); err != nil {
		return nil, r.fail(attemptID, "otp_sms_add", err)
	}
	material, err := r.client.CreateSMSChallenge(ctx, userID, preflightSessionLifetime)
	if err != nil {
		return nil, r.fail(attemptID, "challenge_create", err)
	}
	attempt.SessionID = material.ID
	attempt.sessionToken = material.Token
	if err := r.status("status=challenge_sent attempt=%s organization_id=%s user_id=%s session_id=%s\n", attemptID, organizationID, userID, material.ID); err != nil {
		return nil, errors.New("preflight output failed")
	}
	return attempt, nil
}

// Verify verifies one OTP code, proves its factor state, and deletes the
// session before returning. It never retries a code automatically.
func (r *Runner) Verify(ctx context.Context, attempt *Attempt, code string) (SessionProof, error) {
	if attempt == nil || strings.TrimSpace(attempt.id) == "" || strings.TrimSpace(attempt.SessionID) == "" || strings.TrimSpace(attempt.sessionToken) == "" {
		return SessionProof{}, errors.New("invalid phone onboarding preflight attempt")
	}
	defer func() { attempt.sessionToken = "" }()

	replacementToken, err := r.client.VerifySMS(ctx, attempt.SessionID, strings.TrimSpace(code))
	if err != nil {
		return SessionProof{}, r.failAfterCleanup(attempt, "code_verify", err)
	}
	attempt.sessionToken = replacementToken
	proof, err := r.client.GetSession(ctx, attempt.SessionID, replacementToken)
	if err != nil || !matchingVerifiedFactors(proof, attempt) {
		return SessionProof{}, r.failAfterCleanup(attempt, "session_read")
	}
	if err := r.deleteSession(attempt); err != nil {
		return SessionProof{}, r.fail(attempt.id, "session_delete", err)
	}
	if err := r.status("status=otp_verified attempt=%s user_factor=true otp_sms_factor=true\n", attempt.id); err != nil {
		return SessionProof{}, errors.New("preflight output failed")
	}
	return proof, nil
}

// Abandon deletes a created session after an interrupted hidden-input read.
// It uses a fresh bounded context because the caller's overall context may
// already have expired.
func (r *Runner) Abandon(attempt *Attempt) error {
	if attempt == nil || strings.TrimSpace(attempt.id) == "" || strings.TrimSpace(attempt.SessionID) == "" {
		return errors.New("invalid phone onboarding preflight attempt")
	}
	defer func() { attempt.sessionToken = "" }()
	return r.failAfterCleanup(attempt, "code_verify")
}

func (r *Runner) failAfterCleanup(attempt *Attempt, step string, cause ...error) error {
	if err := r.deleteSession(attempt); err != nil {
		return r.fail(attempt.id, "session_delete", err)
	}
	return r.fail(attempt.id, step, cause...)
}

func (r *Runner) deleteSession(attempt *Attempt) error {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	return r.client.DeleteSession(ctx, attempt.SessionID)
}

func (r *Runner) fail(attemptID, step string, cause ...error) error {
	if err := r.status("status=failed attempt=%s step=%s\n", attemptID, step); err != nil {
		return errors.New("preflight output failed")
	}
	if detail := safeFailureDetail(cause...); detail != "" {
		return fmt.Errorf("phone onboarding preflight failed at %s: %s", step, detail)
	}
	return fmt.Errorf("phone onboarding preflight failed at %s", step)
}

func safeFailureDetail(cause ...error) string {
	if len(cause) == 0 || cause[0] == nil {
		return ""
	}
	const marker = ": ZITADEL returned HTTP status "
	message := cause[0].Error()
	index := strings.LastIndex(message, marker)
	if index < 0 {
		return ""
	}
	status := strings.TrimSpace(message[index+len(marker):])
	if len(status) != 3 || status[0] < '1' || status[0] > '5' || status[1] < '0' || status[1] > '9' || status[2] < '0' || status[2] > '9' {
		return ""
	}
	return "HTTP status " + status
}

func (r *Runner) status(format string, args ...any) error {
	_, err := fmt.Fprintf(r.output, format, args...)
	return err
}

func matchingVerifiedFactors(proof SessionProof, attempt *Attempt) bool {
	return proof.UserID == attempt.UserID && proof.OrganizationID == attempt.OrganizationID &&
		!proof.UserVerifiedAt.IsZero() && !proof.OTPSMSVerifiedAt.IsZero()
}

func normalizeE164(phone string) (string, error) {
	normalized := strings.TrimSpace(phone)
	if len(normalized) < 3 || len(normalized) > 16 || normalized[0] != '+' || normalized[1] < '1' || normalized[1] > '9' {
		return "", errors.New("invalid E.164 phone number")
	}
	for _, digit := range normalized[2:] {
		if digit < '0' || digit > '9' {
			return "", errors.New("invalid E.164 phone number")
		}
	}
	return normalized, nil
}

func newULID(now time.Time, random io.Reader) (string, error) {
	identifier, err := ulid.New(ulid.Timestamp(now.UTC()), random)
	if err != nil {
		return "", err
	}
	return identifier.String(), nil
}
