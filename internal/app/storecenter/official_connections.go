package storecenterapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"time"

	"task-processor/internal/storecenter"
)

type OfficialConnections struct {
	store        storecenter.OfficialConnectionStore
	applications *OfficialApplicationRegistry
}

func NewOfficialConnections(store storecenter.OfficialConnectionStore, applications *OfficialApplicationRegistry) (*OfficialConnections, error) {
	if store == nil || applications == nil {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	return &OfficialConnections{store: store, applications: applications}, nil
}

// Missing developer setup affects this capability, while Store CRUD and local
// disconnect remain usable. No fixture provider is mounted in this case.
func NewUnconfiguredOfficialConnections(store storecenter.OfficialConnectionStore) (*OfficialConnections, error) {
	if store == nil {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	return &OfficialConnections{store: store}, nil
}

func (a *OfficialConnections) Begin(ctx context.Context, c storecenter.OfficialConnectionCommand) (storecenter.OfficialConnectionBegin, error) {
	entry, err := a.applications.selectApplication(c.ApplicationID)
	if err != nil {
		return storecenter.OfficialConnectionBegin{}, err
	}
	stateBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, stateBytes); err != nil {
		return storecenter.OfficialConnectionBegin{}, storecenter.ErrOfficialConnectionUnavailable
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	hash := sha256.Sum256([]byte(state))
	url, err := entry.provider.AuthorizationURL(state)
	if err != nil {
		return storecenter.OfficialConnectionBegin{}, storecenter.ErrOfficialConnectionUnavailable
	}
	attempt, err := a.store.BeginOfficialConnection(ctx, c, entry.application, hex.EncodeToString(hash[:]), time.Now().UTC())
	if err != nil {
		return storecenter.OfficialConnectionBegin{}, err
	}
	return storecenter.OfficialConnectionBegin{AttemptID: attempt.AttemptID, AuthorizationURL: url, ExpiresAt: attempt.ExpiresAt}, nil
}
func (a *OfficialConnections) Complete(ctx context.Context, c storecenter.CompleteOfficialConnection) (storecenter.OfficialConnectionView, error) {
	if len(c.State) < 1 || len(c.State) > 128 || len(c.TempToken) > 4096 || strings.TrimSpace(c.State) != c.State {
		return storecenter.OfficialConnectionView{}, storecenter.ErrNotFound
	}
	binding, err := a.store.ReadOfficialAttemptBinding(ctx, c.OrganizationID, c.StoreID, c.AttemptID)
	if err != nil {
		return storecenter.OfficialConnectionView{}, err
	}
	entry, err := a.applications.resolve(binding.AppID, binding.AppVersion)
	if err != nil {
		return storecenter.OfficialConnectionView{}, err
	}
	app := entry.application
	if c.AppID != app.AppID {
		return storecenter.OfficialConnectionView{}, storecenter.ErrNotFound
	}
	hash := sha256.Sum256([]byte(c.State))
	attempt, dispatch, err := a.store.ClaimOfficialExchange(ctx, c.OrganizationID, c.StoreID, c.AttemptID, app.AppID, app.Version, hex.EncodeToString(hash[:]), time.Now().UTC())
	if err != nil {
		return storecenter.OfficialConnectionView{}, err
	}
	if dispatch {
		// The durable claim is the sole permission to exchange. A lost response or
		// crash after it is UNKNOWN; subsequent callbacks never send another POST.
		if c.TempToken == "" {
			_, _ = a.store.CompleteOfficialConnection(ctx, attempt, storecenter.ConnectionStatusDisconnected, time.Now().UTC())
			return storecenter.OfficialConnectionView{}, storecenter.ErrOfficialAuthorizationRejected
		}
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		credential, callErr := entry.provider.Exchange(callCtx, c.TempToken, c.State)
		cancel()
		if callErr != nil {
			if errors.Is(callErr, storecenter.ErrOfficialAuthorizationRejected) {
				_, err := a.store.CompleteOfficialConnection(ctx, attempt, storecenter.ConnectionStatusDisconnected, time.Now().UTC())
				if err != nil {
					return storecenter.OfficialConnectionView{}, storecenter.ErrOfficialExchangeUnknown
				}
				return storecenter.OfficialConnectionView{}, callErr
			}
			return storecenter.OfficialConnectionView{}, storecenter.ErrOfficialExchangeUnknown
		}
		if credential.AppID != app.AppID {
			return storecenter.OfficialConnectionView{}, storecenter.ErrOfficialExchangeUnknown
		}
		key, encrypted, err := entry.protection.Seal(attempt, credential)
		if err != nil {
			return storecenter.OfficialConnectionView{}, storecenter.ErrOfficialExchangeUnknown
		}
		if err := a.store.SaveOfficialCredential(ctx, attempt, key, encrypted, credential.OpenKeyID); err != nil {
			return storecenter.OfficialConnectionView{}, err
		}
		attempt.State = "credential_received"
		attempt.KeyID = key
		attempt.Ciphertext = encrypted
	}
	if attempt.State == "exchange_dispatched" {
		return storecenter.OfficialConnectionView{}, storecenter.ErrOfficialExchangeUnknown
	}
	if attempt.State == "verified" {
		return a.store.ReadOfficialConnection(ctx, c.OrganizationID, c.StoreID)
	}
	if attempt.State != "credential_received" {
		return storecenter.OfficialConnectionView{}, storecenter.ErrOfficialAuthorizationRejected
	}
	return a.queryAttempt(ctx, attempt)
}

func (a *OfficialConnections) ResumeQuery(ctx context.Context, org, storeID, attemptID string) (storecenter.OfficialConnectionView, error) {
	attempt, err := a.store.ReadOfficialQueryAttempt(ctx, org, storeID, attemptID)
	if err != nil {
		return storecenter.OfficialConnectionView{}, err
	}
	if _, err := a.applications.resolve(attempt.AppID, attempt.AppVersion); err != nil {
		return storecenter.OfficialConnectionView{}, storecenter.ErrOfficialConnectionUnavailable
	}
	return a.queryAttempt(ctx, attempt)
}

func (a *OfficialConnections) queryAttempt(ctx context.Context, attempt storecenter.OfficialConnectionAttempt) (storecenter.OfficialConnectionView, error) {
	entry, err := a.applications.resolve(attempt.AppID, attempt.AppVersion)
	if err != nil {
		return storecenter.OfficialConnectionView{}, err
	}
	credential, err := entry.protection.Open(attempt, attempt.KeyID, attempt.Ciphertext)
	if err != nil || credential.AppID != entry.application.AppID {
		return storecenter.OfficialConnectionView{}, storecenter.ErrOfficialConnectionUnavailable
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = entry.provider.QueryStore(callCtx, credential)
	cancel()
	status := storecenter.ConnectionStatusConnected
	if err != nil {
		status = storecenter.ConnectionStatusUnavailable
		if errors.Is(err, storecenter.ErrOfficialCredentialExpired) {
			status = storecenter.ConnectionStatusExpired
		}
	}
	view, saveErr := a.store.CompleteOfficialConnection(ctx, attempt, status, time.Now().UTC())
	if saveErr != nil {
		return storecenter.OfficialConnectionView{}, saveErr
	}
	if err != nil {
		return view, storecenter.ErrOfficialConnectionUnavailable
	}
	return view, nil
}
func (a *OfficialConnections) Disconnect(ctx context.Context, c storecenter.OfficialConnectionCommand) (storecenter.OfficialConnectionView, error) {
	return a.store.DisconnectOfficialConnection(ctx, c, time.Now().UTC())
}
func (a *OfficialConnections) Read(ctx context.Context, org, store string) (storecenter.OfficialConnectionView, error) {
	view, err := a.store.ReadOfficialConnection(ctx, org, store)
	if err != nil {
		return view, err
	}
	if a.applications == nil {
		view.Status = storecenter.ConnectionStatusUnavailable
	} else if view.AppID != "" {
		if _, err := a.applications.resolve(view.AppID, view.AppRevision); err != nil {
			view.Status = storecenter.ConnectionStatusUnavailable
		}
	}
	return view, nil
}
func (a *OfficialConnections) Applications(ctx context.Context, org, store string) ([]storecenter.OfficialApplicationChoice, error) {
	if _, err := a.store.ReadOfficialConnection(ctx, org, store); err != nil {
		return nil, err
	}
	return a.applications.Applications(), nil
}
func (a *OfficialConnections) Status(ctx context.Context, input storecenter.ConnectionStatusInput) (storecenter.ConnectionStatus, error) {
	if a.applications == nil {
		return storecenter.ConnectionStatusUnavailable, storecenter.ErrOfficialConnectionUnavailable
	}
	attempt, view, err := a.store.ReadOfficialCredential(ctx, input)
	if err != nil {
		return storecenter.ConnectionStatusUnavailable, err
	}
	if view.Status == storecenter.ConnectionStatusExpired || attempt.Ciphertext == "" {
		return view.Status, nil
	}
	if attempt.State != "verified" {
		return storecenter.ConnectionStatusUnavailable, nil
	}
	entry, err := a.applications.resolve(attempt.AppID, attempt.AppVersion)
	if err != nil {
		return storecenter.ConnectionStatusUnavailable, storecenter.ErrOfficialConnectionUnavailable
	}
	credential, err := entry.protection.Open(attempt, attempt.KeyID, attempt.Ciphertext)
	if err != nil || credential.AppID != entry.application.AppID {
		return storecenter.ConnectionStatusUnavailable, storecenter.ErrOfficialConnectionUnavailable
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = entry.provider.QueryStore(callCtx, credential)
	cancel()
	status := storecenter.ConnectionStatusConnected
	if err != nil {
		status = storecenter.ConnectionStatusUnavailable
		if errors.Is(err, storecenter.ErrOfficialCredentialExpired) {
			status = storecenter.ConnectionStatusExpired
		}
	}
	if err := a.store.ObserveOfficialConnection(ctx, attempt, status, time.Now().UTC()); err != nil {
		return storecenter.ConnectionStatusUnavailable, err
	}
	if status == storecenter.ConnectionStatusUnavailable {
		return status, storecenter.ErrOfficialConnectionUnavailable
	}
	return status, nil
}
