package httpapi

import (
	"context"
	"errors"
	"strconv"
	d "task-processor/internal/agentcustomization"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/authidentity"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/product/collection"
)

type supplyDraftInspector struct{ app *supplyapp.Application }

func privateDraftInspector(module *supplyChainModule) d.DraftInspector {
	if module == nil || module.app == nil {
		return nil
	}
	return supplyDraftInspector{module.app}
}
func (r supplyDraftInspector) Inspect(ctx context.Context, scope d.Scope, selection d.DraftSelection, head bool) (d.DraftSnapshot, error) {
	var out d.DraftSnapshot
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || scope.Platform || identity.UserID != scope.ActorID || identity.EffectiveOrganizationID != scope.OrganizationID || identity.TenantID != scope.OrganizationID {
		return out, d.ErrForbidden
	}
	if r.app == nil || r.app.Authorization == nil || r.app.Records == nil || r.app.Sources == nil {
		return out, d.ErrUnavailable
	}
	owner, e := r.app.Authorization.Authorize(ctx, preparation.PermissionRead)
	if e != nil {
		return out, draftError(e)
	}
	if owner.OrganizationID != scope.OrganizationID || owner.ActorID != scope.ActorID || owner.MemberID != identity.EffectiveMemberID {
		return out, d.ErrForbidden
	}
	saved, e := r.app.ReadRecord(ctx, selection.RecordID)
	if e != nil {
		return out, draftError(e)
	}
	if saved.ID != selection.RecordID || saved.Revision != selection.ExpectedRevision {
		return out, d.ErrRevision
	}
	if saved.Merchant.OrganizationID != scope.OrganizationID || saved.Merchant.Site != "shein-us" || saved.Input.SourceID != saved.Source.ID || saved.Input.StoreID != saved.Merchant.StoreID || saved.TargetID != record.TargetIdentity(owner, saved.Source.ID, saved.Input.StoreID) {
		return out, d.ErrUnavailable
	}
	if head {
		current, e := r.app.ReadTarget(ctx, saved.Source.ID, saved.Input.StoreID)
		if e != nil {
			return out, draftError(e)
		}
		if current.ID != saved.ID || collection.Digest(current) != collection.Digest(saved) {
			return out, d.ErrRevision
		}
		if e = r.app.StageProjection.RequireUploadReady(ctx, owner, saved); e != nil {
			return out, draftError(e)
		}
		if r.app.PublicationReceipts == nil || r.app.PublicationStores == nil {
			return out, d.ErrUnavailable
		}
		published, e := r.app.Publication(ctx, saved.Source.ID, saved.Merchant.StoreID)
		if e != nil {
			return out, draftError(e)
		}
		if published.RecordID == saved.ID {
			return out, d.ErrRevision
		}
	}
	title := ""
	for _, name := range saved.Result.Product.Names {
		if name.Name != "" {
			title = name.Name
			break
		}
	}
	runes := []rune(title)
	if len(runes) > 200 {
		title = string(runes[:200])
	}
	out.DraftBinding = d.DraftBinding{RecordID: saved.ID, Revision: saved.Revision, SourceID: saved.Source.ID, PreparationID: saved.Source.PreparationID, StoreID: saved.Merchant.StoreID, Platform: "shein", Site: saved.Merchant.Site, ProductKey: saved.Source.Source.ProductKey, ProductVersion: strconv.FormatUint(saved.EffectiveVersion, 10), Title: title, RecordHash: collection.Digest(saved), ProductHash: saved.ProductHash, RulesHash: saved.RulesHash, InventoryHash: saved.InventoryHash, SavedAt: saved.CreatedAt}
	out.Issues = []d.DraftIssue{}
	out.OfflineTrial = saved.Merchant.ApplicationID == "offline-private-draft-trial"
	for _, issue := range saved.Result.Issues {
		out.Issues = append(out.Issues, d.DraftIssue{Field: issue.Field, Code: issue.Code, Message: issue.Message})
	}
	out.ReadyForUpload = saved.Result.ReadyForUpload
	if !d.ValidDraft(out) {
		return d.DraftSnapshot{}, d.ErrUnavailable
	}
	return out, nil
}
func draftError(e error) error {
	switch {
	case errors.Is(e, preparation.ErrForbidden), errors.Is(e, collection.ErrForbidden), errors.Is(e, record.ErrForbidden):
		return d.ErrForbidden
	case errors.Is(e, preparation.ErrNotFound), errors.Is(e, collection.ErrNotFound), errors.Is(e, record.ErrNotFound):
		return d.ErrNotFound
	case errors.Is(e, preparation.ErrConflict), errors.Is(e, collection.ErrConflict), errors.Is(e, record.ErrConflict), errors.Is(e, record.ErrNotReady):
		return d.ErrRevision
	default:
		return d.ErrUnavailable
	}
}
