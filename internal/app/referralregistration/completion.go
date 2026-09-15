package referralregistration

import (
	"context"
	"crypto/hmac"
	"errors"
	"task-processor/internal/authidentity"
	"task-processor/internal/referral"
	"time"
)

func (s *Service) Resume(ctx context.Context, id, secret string) error {
	if !s.valid() || ctx == nil {
		return referral.ErrUnavailable
	}
	if !s.enter() {
		return referral.ErrLimited
	}
	defer s.leave()
	if !validText(id, 200) || !validText(secret, 200) {
		return referral.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	i, err := s.Store.Find(ctx, "id", s.Issuer, id)
	if err != nil {
		return err
	}
	if i.ID != id || !s.owns(i) || !hmac.Equal([]byte(i.SecretHash), []byte(digest(s.Keys.Lookup, "resume", secret))) {
		return referral.ErrInvalid
	}
	now := s.Now()
	if err = s.Store.Cleanup(ctx, now); err != nil {
		return err
	}
	if !now.Before(i.CompletionExpiresAt) || i.State == "CONSUMED" {
		return referral.ErrExpired
	}
	payload, err := s.open(i)
	if err != nil {
		return err
	}
	proof, err := s.proof(i)
	if err != nil {
		return err
	}
	lease, err := s.Store.Claim(ctx, i.ID)
	if err != nil {
		return err
	}
	if lease.IsZero() {
		return referral.ErrPending
	}
	user, err := s.Provider.Read(ctx, i.Subject)
	var createErr error
	if errors.Is(err, referral.ErrMissing) {
		if !s.Now().Before(i.CreateExpiresAt) {
			return referral.ErrExpired
		}
		if ctx.Err() != nil {
			return referral.ErrUnknown
		}
		// Recheck authoritative database time after potentially slow readback.
		// Subtract the full round trip conservatively; no DB lock spans HTTP.
		permitStarted := time.Now()
		remaining, permitErr := s.Store.PermitCreate(ctx, i.ID, lease)
		if permitErr != nil {
			return permitErr
		}
		dispatchDeadline := permitStarted.Add(remaining)
		if !time.Now().Before(dispatchDeadline) {
			return referral.ErrExpired
		}
		// A dispatched mutation may have succeeded even when its response is lost.
		// Read only this immutable subject before deciding the outcome.
		createContext, cancel := context.WithDeadline(ctx, dispatchDeadline)
		if createContext.Err() != nil {
			cancel()
			return referral.ErrExpired
		}
		createErr = s.Provider.Create(createContext, Creation{Subject: i.Subject, Organization: i.Organization, Email: payload.Request.Email, GivenName: payload.Request.GivenName, FamilyName: payload.Request.FamilyName, Proof: proof})
		cancel()
		user, err = s.Provider.Read(ctx, i.Subject)
	}
	if ctx.Err() == nil && errors.Is(createErr, referral.ErrConflict) && (errors.Is(err, referral.ErrMissing) || (err == nil && !matches(i, payload, user, proof))) {
		return referral.ErrConflict
	}
	if err != nil || ctx.Err() != nil {
		return referral.ErrUnknown
	}
	if !matches(i, payload, user, proof) {
		return referral.ErrUnknown
	}
	return s.Store.Created(ctx, i.ID)
}

func (s *Service) Complete(ctx context.Context) (out referral.Receipt, resultErr error) {
	if !s.valid() {
		return referral.Receipt{}, referral.ErrUnavailable
	}
	if !s.enter() {
		return referral.Receipt{}, referral.ErrLimited
	}
	defer s.leave()
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !s.Now().Before(identity.TokenExpiresAt) {
		return referral.Receipt{}, referral.ErrUnauthenticated
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	receipt, err := s.Store.Receipt(ctx, s.Issuer, identity.UserID)
	if err == nil {
		if receipt.Issuer != s.Issuer || receipt.Subject != identity.UserID {
			return referral.Receipt{}, referral.ErrConflict
		}
		return receipt, nil
	}
	if !errors.Is(err, referral.ErrMissing) {
		return referral.Receipt{}, err
	}
	// Another Complete can commit after our initial receipt/Intent reads. On a
	// pre-consumption failure, one bounded receipt read may resolve that success.
	expected := referral.Intent{Issuer: s.Issuer, Subject: identity.UserID}
	resolveConcurrent := true
	defer func() {
		if resultErr == nil || !resolveConcurrent || ctx.Err() != nil {
			return
		}
		replayed, replayErr := s.receiptForIntent(ctx, expected)
		if replayErr == nil {
			out, resultErr = replayed, nil
		} else if errors.Is(replayErr, referral.ErrConflict) || errors.Is(replayErr, referral.ErrUnauthenticated) {
			out, resultErr = referral.Receipt{}, replayErr
		}
	}()
	if err = s.Store.Cleanup(ctx, s.Now()); err != nil {
		return referral.Receipt{}, err
	}
	i, err := s.Store.Find(ctx, "subject", s.Issuer, identity.UserID)
	if err != nil {
		return referral.Receipt{}, err
	}
	if !s.owns(i) || i.Subject != identity.UserID {
		return referral.Receipt{}, referral.ErrConflict
	}
	expected = i
	if i.State == "CONSUMED" {
		resolveConcurrent = false
		return s.receiptForIntent(ctx, i)
	}
	if !s.Now().Before(i.CompletionExpiresAt) {
		return referral.Receipt{}, referral.ErrExpired
	}
	payload, err := s.open(i)
	if err != nil {
		return referral.Receipt{}, err
	}
	proof, err := s.proof(i)
	if err != nil {
		return referral.Receipt{}, err
	}
	user, err := s.Provider.Read(ctx, i.Subject)
	if err != nil {
		return referral.Receipt{}, referral.ErrUnknown
	}
	if !matches(i, payload, user, proof) {
		return referral.Receipt{}, referral.ErrUnknown
	}
	if !user.Verified {
		return referral.Receipt{}, referral.ErrPending
	}
	if ctx.Err() != nil {
		return referral.Receipt{}, referral.ErrUnknown
	}
	if !s.Now().Before(identity.TokenExpiresAt) {
		return referral.Receipt{}, referral.ErrUnauthenticated
	}
	if err = s.Store.Created(ctx, i.ID); err != nil {
		return referral.Receipt{}, err
	}
	resolveConcurrent = false // Preserve an unknown consumption COMMIT for the next request.
	return s.Store.Consume(ctx, i, s.Now())
}

func (s *Service) receiptForIntent(ctx context.Context, i referral.Intent) (referral.Receipt, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.UserID != i.Subject || !s.Now().Before(identity.TokenExpiresAt) {
		return referral.Receipt{}, referral.ErrUnauthenticated
	}
	receipt, err := s.Store.Receipt(ctx, i.Issuer, i.Subject)
	if err != nil {
		return referral.Receipt{}, err
	}
	if receipt.Issuer != i.Issuer || receipt.Subject != i.Subject || (i.ID != "" && (receipt.IntentID != i.ID || receipt.Referrer != i.Referrer || receipt.Fingerprint != i.Fingerprint)) {
		return referral.Receipt{}, referral.ErrConflict
	}
	return receipt, nil
}

func (s *Service) owns(i referral.Intent) bool {
	return i.Issuer == s.Issuer && i.Instance == s.Instance && i.Organization == s.Organization
}

func (s *Service) proof(i referral.Intent) (string, error) {
	key := s.Keys.Proof[i.KeyID]
	if len(key) != 32 {
		return "", referral.ErrUnavailable
	}
	return i.KeyID + ":" + digest(key, "referral-registration-proof", i.KeyID, i.Issuer, i.Instance, i.Organization, i.ID, i.Subject, i.Referrer, i.Fingerprint), nil
}

func matches(i referral.Intent, p sealedAdmission, u User, proof string) bool {
	return u.Subject == i.Subject && u.Organization == i.Organization && u.Email == p.Request.Email && hmac.Equal([]byte(u.Proof), []byte(proof))
}

func (s *Service) currentSubject(ctx context.Context) (string, error) {
	if !s.valid() {
		return "", referral.ErrUnavailable
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !s.Now().Before(identity.TokenExpiresAt) {
		return "", referral.ErrUnauthenticated
	}
	return identity.UserID, nil
}
func (s *Service) ReadSelf(ctx context.Context) (referral.Projection, error) {
	subject, err := s.currentSubject(ctx)
	if err != nil {
		return referral.Projection{}, err
	}
	return (referral.Service{Store: s.Store}).Read(ctx, s.Issuer, subject)
}
func (s *Service) CreateSelfCode(ctx context.Context) (string, error) {
	subject, err := s.currentSubject(ctx)
	if err != nil {
		return "", err
	}
	return (referral.Service{Store: s.Store}).CreateCode(ctx, s.Issuer, subject)
}
