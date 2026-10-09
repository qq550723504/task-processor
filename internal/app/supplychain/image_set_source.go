package supplychainapp

import (
	"context"
	imageapp "task-processor/internal/app/imageagent"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/product/collection"
)

type ImageSetSources struct {
	Sources                record.TargetSourceSelector
	Authorization          preparation.Authorizer
	ExecutionSources       record.TargetExecutionSourceSelector
	ExecutionAuthorization collection.ExecutionAuthorizer
	Products               record.EffectiveTargetProductReader
}

func (r ImageSetSources) ReadImageSetSource(ctx context.Context, identity imageagent.ExecutionIdentity, input imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	if ctx == nil || r.Products == nil {
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	verified, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.ScopeProtocol != imageagent.OrganizationScopeProtocol || verified.TenantID != identity.TenantID || verified.EffectiveOrganizationID != identity.TenantID || verified.UserID != identity.UserID || verified.EffectiveMemberID != identity.MemberID || identity.MemberID == "" || input.ContextID != identity.BusinessTaskID || input.ContextID == "" {
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	scope := collection.Scope{OrganizationID: identity.TenantID, ActorID: identity.UserID, MemberID: identity.MemberID}
	var selected preparation.AuthorizedSource
	var err error
	// Browser and durable execution authorities are explicitly assembled.
	// There is no fallback from a denied browser read to service authority.
	if r.Sources != nil && r.ExecutionSources == nil && r.Authorization != nil && r.ExecutionAuthorization == nil {
		current, authErr := r.Authorization.Authorize(ctx, preparation.PermissionManage)
		if authErr != nil || current != scope {
			return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
		}
		selected, err = r.Sources.Select(ctx, input.ContextID)
	} else if r.Sources == nil && r.ExecutionSources != nil && r.Authorization == nil && r.ExecutionAuthorization != nil {
		if err = r.ExecutionAuthorization.AuthorizeExecution(ctx, scope, preparation.PermissionManage); err != nil {
			return imageagent.ImageSetPreparation{}, err
		}
		selected, err = r.ExecutionSources.SelectForExecution(ctx, scope, input.ContextID)
	} else {
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	if err != nil {
		return imageagent.ImageSetPreparation{}, err
	}
	owner, source, original, err := selected.Read(ctx)
	if err != nil || owner != scope || source.ID != input.ContextID {
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	version := input.EffectiveCatalogVersion
	if version == 0 {
		version = original.Version
	}
	effective, err := r.Products.ReadEffectiveTargetProduct(ctx, selected, version, input.ApplyReceiptID)
	if err != nil {
		return imageagent.ImageSetPreparation{}, err
	}
	assets := make([]imageagent.AuthorizedAsset, 0)
	for _, image := range SourceImages(original) {
		url, safe := imageagent.ValidateSafeImageURL(image.URL)
		if safe != nil {
			continue
		}
		assets = append(assets, imageagent.AuthorizedAsset{ID: image.ID, Type: imageagent.AuthorizedAssetSource, URL: url, SourceURL: url, DisplayURL: url, Label: "商品原始素材", Width: image.Width, Height: image.Height})
	}
	prepared, err := imageapp.PrepareImageProduct(input.ContextID, original, effective, input.ApplyReceiptID, assets)
	prepared.Source.ContextKind = imageagent.ImageSourceSupply
	return prepared, err
}
