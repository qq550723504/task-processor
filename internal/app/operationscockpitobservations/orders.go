package operationscockpitobservations

import (
	"context"
	"errors"
	"task-processor/internal/authidentity"
	o "task-processor/internal/marketplace/shein/observations"
	c "task-processor/internal/operationscockpit"
)

type Orders struct{ Service *o.Service }

func (a Orders) Read(ctx context.Context, scope c.Scope, stores []c.StoreReference) (c.ObservationProjection, error) {
	result := c.ObservationProjection{State: "unavailable", Sources: []c.ObservationEvidence{}, Latest: []c.ObservationEvidence{}, ActionPath: "/workbench/store-orders"}
	if a.Service == nil {
		return result, nil
	}
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || id.UserID != scope.ActorID || id.TenantID != scope.OrganizationID || id.EffectiveOrganizationID != scope.OrganizationID {
		return result, c.ErrForbidden
	}
	selected := []string{}
	for _, store := range stores {
		if store.Platform == "shein" && store.Status == "active" {
			selected = append(selected, store.ID)
		}
	}
	if len(selected) == 0 {
		result.State = "no_active_store"
		return result, nil
	}
	view, err := a.Service.List(ctx, o.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: id.EffectiveMemberID}, o.Query{Kind: o.Orders, Stores: selected, Limit: 1})
	if errors.Is(err, o.ErrForbidden) || errors.Is(err, o.ErrNotFound) {
		result.State = "permission_required"
		return result, nil
	}
	if err != nil {
		return result, nil
	}
	if len(view.Syncs) > 500 || len(view.Latest) > 500 || view.Summary.Exceptional < 0 {
		return result, c.ErrUnavailable
	}
	evidence := func(s o.Sync) c.ObservationEvidence {
		e := c.ObservationEvidence{StoreID: s.StoreID, SyncID: s.ID, Status: s.Status, ObservedAt: s.ObservedAt, ErrorCode: s.ErrorCode}
		if s.Range != nil {
			e.Start = &s.Range.Start
			e.End = &s.Range.End
		}
		return e
	}
	for _, s := range view.Syncs {
		result.Sources = append(result.Sources, evidence(s))
	}
	for _, s := range view.Latest {
		result.Latest = append(result.Latest, evidence(s))
	}
	result.Exceptional = view.Summary.Exceptional
	result.Complete = view.Complete
	result.State = "available"
	if !view.Complete {
		result.State = "incomplete"
	}
	for _, s := range view.Latest {
		if s.Status == "failed" || s.Status == "suspended" || s.Status == "partial" {
			result.State = "latest_incomplete"
			result.Complete = false
		}
	}
	return result, nil
}
