package storeobservationsapp

import (
	"context"
	"errors"
	storeapp "task-processor/internal/app/storecenter"
	"task-processor/internal/authidentity"
	o "task-processor/internal/marketplace/shein/observations"
	"task-processor/internal/storecenter"
	"time"
)

type Access struct {
	Authorization Authorization
	Official      *storeapp.OfficialObservationAccess
}

func (a Access) Authorize(ctx context.Context, s o.Scope, k o.Kind, sync bool) error {
	return a.Authorization.Authorize(ctx, s, k, sync)
}
func (a Access) Open(ctx context.Context, s o.Scope, store string, k o.Kind, sync bool, b *o.Binding) (o.Merchant, error) {
	if !k.Valid() || a.Official == nil {
		return nil, o.ErrUnavailable
	}
	purpose := storecenter.ObservationPurposeProducts
	if k == o.Orders {
		purpose = storecenter.ObservationPurposeOrders
	}
	return a.Official.Authorize(ctx, storecenter.ObservationSubject{OrganizationID: s.OrganizationID, ActorID: s.ActorID, MemberID: s.MemberID, Purpose: purpose, Sync: sync}, store, b)
}

// Inject the current member-scoped Store repository; no second Store fact source.
type Directory struct{ Stores storecenter.Repository }

func (d Directory) ListStores(ctx context.Context, s o.Scope) ([]string, error) {
	if d.Stores == nil || !s.Valid() {
		return nil, o.ErrUnavailable
	}
	out := []string{}
	seen := map[string]bool{}
	for page := 1; page <= 5; page++ {
		result, e := d.Stores.List(ctx, s.OrganizationID, storecenter.StoreListQuery{Page: page, PageSize: 100, Platform: storecenter.PlatformShein, Status: storecenter.RecordStatusActive})
		if errors.Is(e, storecenter.ErrNotFound) {
			return nil, o.ErrForbidden
		}
		if e != nil || result.Total > 500 || result.Total < 0 || len(result.Stores) > 100 {
			return nil, o.ErrUnavailable
		}
		for _, store := range result.Stores {
			if !o.ValidID(store.ID()) || seen[store.ID()] {
				return nil, o.ErrUnavailable
			}
			seen[store.ID()] = true
			out = append(out, store.ID())
		}
		if int64(page*100) >= result.Total {
			return out, nil
		}
	}
	return nil, o.ErrUnavailable
}

type Application struct {
	Service *o.Service
	Ready   func() bool
}
type Capabilities struct {
	Available   bool   `json:"available"`
	CanSync     bool   `json:"canSync"`
	Kind        o.Kind `json:"kind"`
	Site        string `json:"site"`
	PlatformURL string `json:"platformUrl"`
}

func Scope(ctx context.Context) (o.Scope, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	s := o.Scope{OrganizationID: id.EffectiveOrganizationID, ActorID: id.UserID, MemberID: id.EffectiveMemberID}
	if !ok || !s.Valid() || id.TenantID != s.OrganizationID || !time.Now().Before(id.TokenExpiresAt) {
		return s, o.ErrForbidden
	}
	return s, nil
}
func (a *Application) Available() bool {
	return a != nil && a.Service != nil && a.Service.Repository != nil && a.Service.Access != nil && a.Service.Directory != nil && a.Service.Starter != nil && a.Ready != nil && a.Ready()
}
func (a *Application) Capabilities(ctx context.Context, s o.Scope, k o.Kind) (Capabilities, error) {
	c := Capabilities{Kind: k, Site: "shein-us", PlatformURL: "https://sellerhub.shein.com/"}
	if a == nil || a.Service == nil || a.Service.Access == nil {
		return c, o.ErrUnavailable
	}
	if e := a.Service.Access.Authorize(ctx, s, k, false); e != nil {
		return c, e
	}
	c.Available = a.Available()
	e := a.Service.Access.Authorize(ctx, s, k, true)
	if errors.Is(e, o.ErrForbidden) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	c.CanSync = c.Available
	return c, nil
}
