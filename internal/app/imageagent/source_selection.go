package imageagentapp

import (
	"context"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/asset"
)

// Source selection shares the same source owner as plan preparation. It reads
// original identities from that owner and accepts no browser media claims.
type ImageSetSourceSelectionReader struct{ Sources ImageSetSourceReader }

func (r ImageSetSourceSelectionReader) ReadSourceSelection(ctx context.Context, input asset.SourceSelectionRequest) (asset.SourceSelection, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	kind := imageagent.ImageSourceContextKind(input.ContextKind)
	if ctx == nil || !ok || r.Sources == nil || !kind.Valid() || id.EffectiveOrganizationID == "" || id.EffectiveOrganizationID != id.TenantID || id.EffectiveMemberID == "" || (input.TargetPlatform != "product" && input.TargetPlatform != "shein") {
		return asset.SourceSelection{}, asset.ErrSourceApprovalForbidden
	}
	identity := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: id.TenantID, UserID: id.UserID, MemberID: id.EffectiveMemberID, BusinessTaskID: input.ItemID}
	prepared, err := r.Sources.ReadImageSetSource(ctx, identity, imageagent.PrepareImageSetInput{ContextKind: kind, ContextID: input.ItemID, EffectiveCatalogVersion: input.EffectiveCatalogVersion, ApplyReceiptID: input.ApplyReceiptID, Target: imageagent.ImageTargetSelection{Platform: input.TargetPlatform}})
	if err != nil {
		return asset.SourceSelection{}, err
	}
	s := prepared.Source
	if s.ContextKind != kind || s.OperationID != input.ItemID || s.OriginalPublicationID != input.OriginalPublicationID || s.OriginalVersion != input.OriginalSnapshotVersion || s.EffectiveVersion != input.EffectiveCatalogVersion || s.ApplyReceiptID != input.ApplyReceiptID {
		return asset.SourceSelection{}, asset.ErrApprovalConflict
	}
	result := asset.SourceSelection{ContextKind: string(kind), TenantID: id.TenantID, ActorID: id.UserID, MemberID: id.EffectiveMemberID, ItemID: s.OperationID, ProductKey: s.ProductID, OriginalPublicationID: s.OriginalPublicationID, OriginalSnapshotVersion: s.OriginalVersion, EffectiveCatalogVersion: s.EffectiveVersion, ApplyReceiptID: s.ApplyReceiptID, TargetPlatform: input.TargetPlatform}
	for _, original := range prepared.Catalog.Assets {
		if original.Type == imageagent.AuthorizedAssetSource {
			result.Images = append(result.Images, asset.SourceImage{ID: original.ID, URL: original.URL, ReferenceHash: asset.ReferenceHash(original.ID, original.URL), Width: original.Width, Height: original.Height})
		}
	}
	return result, nil
}
