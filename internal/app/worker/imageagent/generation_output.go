package imageagentworker

import (
	"context"
	"task-processor/internal/imageagent"
	"task-processor/internal/integration/httpimage"
	productimage "task-processor/internal/product/image"
)

// No credential resolver, current price, provider client or source fetch is
// reachable from this GET-only recovery closure.
func generationOutputRecovery(fetch func(context.Context, string) ([]byte, error)) imageagent.GenerationOutputRecovery {
	if fetch == nil {
		fetch = func(ctx context.Context, raw string) ([]byte, error) {
			return httpimage.Download(ctx, httpimage.NewPublicImageHTTPClient(), raw, productimage.MaxInlineArtifactBytes)
		}
	}
	return func(ctx context.Context, input imageagent.SlotExecutionInput, fact imageagent.GenerationFact) (imageagent.SlotGeneratedOutput, error) {
		bad := func() (imageagent.SlotGeneratedOutput, error) {
			return imageagent.SlotGeneratedOutput{}, imageagent.ErrValidation
		}
		id := imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: input.TenantID, OwnerUserID: input.UserID, RunID: input.RunID}, PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt}
		if fact.Validate() != nil || fact.State != imageagent.GenerationSucceeded || fact.Intent.Identity != id || fact.Intent.MemberID != input.OrganizationIdentity.MemberID || fact.Intent.CatalogHash != input.AssetCatalog.Manifest.Hash || fact.Success.ResultURL == "" || input.Slot.Role != imageagent.SlotRoleMain || len(input.Slot.SourceAssetIDs) != 1 {
			return bad()
		}
		var sourceURL string
		for _, asset := range input.AssetCatalog.Assets {
			if asset.ID == input.Slot.SourceAssetIDs[0] && asset.Type == imageagent.AuthorizedAssetSource {
				sourceURL = asset.SourceURL
				if sourceURL == "" {
					sourceURL = asset.URL
				}
			}
		}
		if _, err := imageagent.ValidateSafeImageURL(sourceURL); err != nil {
			return bad()
		}
		if _, err := imageagent.ValidateSafeImageURL(fact.Success.ResultURL); err != nil {
			return bad()
		}
		data, err := fetch(ctx, fact.Success.ResultURL)
		if err != nil || len(data) == 0 || len(data) > productimage.MaxInlineArtifactBytes {
			return bad()
		}
		contentType, width, height, err := httpimage.InspectGeneratedArtifact(data)
		if err != nil {
			return bad()
		}
		return imageagent.SlotGeneratedOutput{SlotID: input.Slot.ID, Attempt: input.Attempt, SourceAssetID: input.Slot.SourceAssetIDs[0], Assets: []imageagent.GeneratedAsset{{Bytes: append([]byte(nil), data...), ContentType: contentType, Width: width, Height: height, SourceURL: sourceURL, Operations: []string{productimage.SourceWhiteBackgroundOperation}, ProviderReceiptID: fact.Success.RequestID}}}, nil
	}
}
