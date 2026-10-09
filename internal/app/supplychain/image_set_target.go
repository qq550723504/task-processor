package supplychainapp

import (
	"context"
	"task-processor/internal/agentconfig"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
)

type ImageSetTargetRules struct {
	Records record.TargetRepository
	Sources asset.SourceSelectionReader
	Rules   record.TargetRuleReader
}

func (r ImageSetTargetRules) ResolveImageSetTarget(ctx context.Context, identity imageagent.ExecutionIdentity, input imageagent.PrepareImageSetInput, source imageagent.ImageSetPreparation) (imageagent.ImageTarget, map[string]imageagent.OfficialImagePlacement, error) {
	if input.ContextID != identity.BusinessTaskID || source.Source.OperationID != input.ContextID || len(input.OfficialPlacements) == 0 || len(input.OfficialPlacements) > agentconfig.MaxSetTasks {
		return imageagent.ImageTarget{}, nil, imageagent.ErrValidation
	}
	slots := make([]goods.OfficialImageSlot, 0, len(input.OfficialPlacements))
	positions := map[string]imageagent.OfficialImagePlacement{}
	for id, position := range input.OfficialPlacements {
		if id == "" || !agentconfig.ValidSetText(id, 64) || position.Site != input.Target.Site {
			return imageagent.ImageTarget{}, nil, imageagent.ErrValidation
		}
		slots = append(slots, goods.OfficialImageSlot{Group: position.Group, SKC: position.SKC, SKU: position.SKU, Type: position.Type, Sort: position.Sort, AssetID: id})
		positions[id] = position
	}
	resolved, err := r.readTarget(ctx, identity, input.Target, source.Source, slots)
	if err != nil {
		return imageagent.ImageTarget{}, nil, err
	}
	seen := map[string]bool{}
	for _, slot := range slots {
		position := collection.Digest([]any{slot.Group, slot.SKC, slot.SKU, slot.Sort})
		if !resolved.requirements.AllowsSlot(slot) || seen[position] {
			return imageagent.ImageTarget{}, nil, imageagent.ErrRevisionConflict
		}
		seen[position] = true
		if !goods.OfficialImageSizeAllowed(slot.Group, slot.Type, 1024, 1024) {
			return imageagent.ImageTarget{}, nil, imageagent.ErrCommandBlocked
		}
	}
	return resolved.target, positions, nil
}

type imageSetResolvedTarget struct {
	target       imageagent.ImageTarget
	requirements goods.OfficialImageRequirements
}

func (r ImageSetTargetRules) readTarget(ctx context.Context, identity imageagent.ExecutionIdentity, input imageagent.ImageTargetSelection, source imageagent.ImageSourceBinding, slots []goods.OfficialImageSlot) (imageSetResolvedTarget, error) {
	var empty imageSetResolvedTarget
	if ctx == nil || r.Records == nil || r.Sources == nil || r.Rules == nil || identity.ScopeProtocol != imageagent.OrganizationScopeProtocol || input.Platform != "shein" || input.Site != "shein-us" || !collection.ValidID(input.RecordID) || !collection.ValidID(input.StoreID) {
		return empty, imageagent.ErrCommandBlocked
	}
	verified, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.MemberID == "" || verified.TenantID != identity.TenantID || verified.EffectiveOrganizationID != identity.TenantID || verified.UserID != identity.UserID || verified.EffectiveMemberID != identity.MemberID {
		return empty, imageagent.ErrIdentityRequired
	}
	scope := collection.Scope{OrganizationID: identity.TenantID, ActorID: identity.UserID, MemberID: identity.MemberID}
	target, err := r.Records.ReadTargetRecord(ctx, scope, input.RecordID)
	if err != nil {
		return empty, err
	}
	if target.ID != input.RecordID || target.TargetID != record.TargetIdentity(scope, target.Source.ID, input.StoreID) || target.Source.ID != target.Input.SourceID || target.Input.StoreID != input.StoreID || target.Merchant.OrganizationID != scope.OrganizationID || target.Merchant.StoreID != input.StoreID || target.Merchant.Site != input.Site || target.Source.Source.ProductKey != source.ProductID || target.Source.Source.PublicationID != source.OriginalPublicationID || target.Source.Source.Version != source.OriginalVersion || target.EffectiveVersion != source.EffectiveVersion || target.ApplyReceiptID != source.ApplyReceiptID || target.Input.EffectiveVersion != source.EffectiveVersion || target.Input.ApplyReceiptID != source.ApplyReceiptID || target.Input.Draft.Product.CategoryID != input.CategoryID {
		return empty, imageagent.ErrRevisionConflict
	}
	current, err := r.Sources.ReadSourceSelection(ctx, asset.SourceSelectionRequest{ItemID: target.Source.ID, OriginalPublicationID: source.OriginalPublicationID, OriginalSnapshotVersion: source.OriginalVersion, EffectiveCatalogVersion: source.EffectiveVersion, ApplyReceiptID: source.ApplyReceiptID, TargetPlatform: input.Platform})
	if err != nil {
		return empty, err
	}
	if current.TenantID != scope.OrganizationID || current.ActorID != scope.ActorID || current.MemberID != scope.MemberID || current.ItemID != target.Source.ID || current.ProductKey != source.ProductID || current.OriginalPublicationID != source.OriginalPublicationID || current.OriginalSnapshotVersion != source.OriginalVersion || current.EffectiveCatalogVersion != source.EffectiveVersion || current.ApplyReceiptID != source.ApplyReceiptID || current.TargetPlatform != input.Platform {
		return empty, imageagent.ErrIdentityRequired
	}
	head, err := r.Records.ReadTargetHead(ctx, scope, target.TargetID)
	if err != nil || head.ID != target.ID || head.Revision != target.Revision {
		return empty, imageagent.ErrRevisionConflict
	}
	binding, rules, err := r.Rules.ReadTargetRules(ctx, scope, input.StoreID, target.Input.Draft)
	if err != nil {
		return empty, err
	}
	if binding != target.Merchant || !binding.ApplicationType.Valid() || binding.ApplicationID == "" || string(rules.ApplicationMode) != string(binding.ApplicationType) {
		return empty, imageagent.ErrRevisionConflict
	}
	productType, leaf := goods.ProductTypeForCategory(rules.Categories, input.CategoryID)
	if !leaf || rules.Attributes.ProductTypeID != productType {
		return empty, imageagent.ErrCommandBlocked
	}
	requirements, err := goods.ResolveOfficialImageRequirements(target.Input.Draft.Product, rules.Fill, slots)
	if err != nil {
		return empty, imageagent.ErrCommandBlocked
	}
	value := imageagent.ImageTarget{Platform: input.Platform, RecordID: target.ID, StoreID: binding.StoreID, Site: binding.Site, ApplicationID: binding.ApplicationID, ApplicationMode: string(binding.ApplicationType), CategoryID: input.CategoryID, ProductTypeID: productType, AttributesDigest: collection.Digest(target.Input.Draft.Product.Attributes), VariantsDigest: collection.Digest(target.Input.Draft.Product.SKCs), RequirementVersion: requirements.Version}
	value.RequirementDigest = collection.Digest(struct {
		Target       imageagent.ImageTarget
		Rules        goods.OfficialRuleSnapshot
		Requirements goods.OfficialImageRequirements
	}{value, rules, requirements})
	return imageSetResolvedTarget{target: value, requirements: requirements}, nil
}
