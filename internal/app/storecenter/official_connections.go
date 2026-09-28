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
	store      storecenter.OfficialConnectionStore
	provider   storecenter.OfficialConnectionProvider
	protection storecenter.OfficialCredentialProtection
}

func NewOfficialConnections(store storecenter.OfficialConnectionStore, provider storecenter.OfficialConnectionProvider, protection storecenter.OfficialCredentialProtection) (*OfficialConnections, error) {
	if store == nil || provider == nil || protection == nil {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	return &OfficialConnections{store: store, provider: provider, protection: protection}, nil
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
	if a.provider == nil || a.protection == nil {
		return storecenter.OfficialConnectionBegin{}, storecenter.ErrOfficialConnectionUnavailable
	}
	stateBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, stateBytes); err != nil {
		return storecenter.OfficialConnectionBegin{}, storecenter.ErrOfficialConnectionUnavailable
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	hash := sha256.Sum256([]byte(state))
	url, err := a.provider.AuthorizationURL(state)
	if err != nil {
		return storecenter.OfficialConnectionBegin{}, storecenter.ErrOfficialConnectionUnavailable
	}
	attempt, err := a.store.BeginOfficialConnection(ctx, c, a.provider.Application(), hex.EncodeToString(hash[:]), time.Now().UTC())
	if err != nil {
		return storecenter.OfficialConnectionBegin{}, err
	}
	return storecenter.OfficialConnectionBegin{AttemptID: attempt.AttemptID, AuthorizationURL: url, ExpiresAt: attempt.ExpiresAt}, nil
}
func (a *OfficialConnections) Complete(ctx context.Context, c storecenter.CompleteOfficialConnection) (storecenter.OfficialConnectionView, error) {
	if a.provider == nil || a.protection == nil {
		return storecenter.OfficialConnectionView{}, storecenter.ErrOfficialConnectionUnavailable
	}
	if len(c.State) < 1 || len(c.State) > 128 || len(c.TempToken) > 4096 || strings.TrimSpace(c.State) != c.State {
		return storecenter.OfficialConnectionView{}, storecenter.ErrNotFound
	}
	app := a.provider.Application()
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
		credential, callErr := a.provider.Exchange(callCtx, c.TempToken, c.State)
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
		key, encrypted, err := a.protection.Seal(attempt, credential)
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
	credential, err := a.protection.Open(attempt, attempt.KeyID, attempt.Ciphertext)
	if err != nil {
		return storecenter.OfficialConnectionView{}, storecenter.ErrOfficialConnectionUnavailable
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = a.provider.QueryStore(callCtx, credential)
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
	if a.provider == nil || a.protection == nil {
		view.Status = storecenter.ConnectionStatusUnavailable
	}
	return view, nil
}
func (a *OfficialConnections) Status(ctx context.Context, input storecenter.ConnectionStatusInput) (storecenter.ConnectionStatus, error) {
	if a.provider == nil || a.protection == nil {
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
	app := a.provider.Application()
	if attempt.AppID != app.AppID || attempt.AppVersion != app.Version {
		return storecenter.ConnectionStatusUnavailable, storecenter.ErrOfficialConnectionUnavailable
	}
	credential, err := a.protection.Open(attempt, attempt.KeyID, attempt.Ciphertext)
	if err != nil {
		return storecenter.ConnectionStatusUnavailable, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = a.provider.QueryStore(callCtx, credential)
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
