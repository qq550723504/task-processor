package assetpublication

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"task-processor/internal/imageagent"
	productasset "task-processor/internal/product/asset"
)

type ProjectionSource interface {
	GetProjection(context.Context, imageagent.RunScope) (imageagent.RunProjection, error)
}

type Publisher struct {
	projections ProjectionSource
	assets      productasset.Repository
	publicURLs  imageagent.DurableAssetPublicURLResolver
}

func NewPublisher(projections ProjectionSource, assets productasset.Repository, publicURLs imageagent.DurableAssetPublicURLResolver) (*Publisher, error) {
	if nilValue(projections) || nilValue(assets) || nilValue(publicURLs) {
		return nil, fmt.Errorf("image agent projection source, product asset repository, and public URL resolver are required")
	}
	return &Publisher{projections: projections, assets: assets, publicURLs: publicURLs}, nil
}

// NewV2Publisher keeps only the frozen URL-based Temporal wire while writing
// to the same product asset repository. It has no ListingKit dependency.
func NewV2Publisher(projections ProjectionSource, assets productasset.Repository) (*Publisher, error) {
	if nilValue(projections) || nilValue(assets) {
		return nil, fmt.Errorf("image agent projection source and product asset repository are required")
	}
	return &Publisher{projections: projections, assets: assets}, nil
}

func (p *Publisher) PublishApproved(ctx context.Context, input imageagent.PublishApprovedInput) (imageagent.PublicationAcknowledgement, error) {
	if p == nil || p.projections == nil || p.assets == nil || ctx == nil {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrValidation
	}
	if err := ctx.Err(); err != nil {
		return imageagent.PublicationAcknowledgement{}, err
	}
	if !canonical(input.RunID) || !canonical(input.TenantID) || !canonical(input.UserID) ||
		!canonical(input.IdempotencyKey) || input.PlanRevision <= 0 || len(input.CandidateAssetIDs) == 0 {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrValidation
	}
	scope := imageagent.RunScope{TenantID: input.TenantID, OwnerUserID: input.UserID, RunID: input.RunID}
	projection, err := p.projections.GetProjection(ctx, scope)
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, fmt.Errorf("load approved image agent projection: %w", err)
	}
	commit, digest, err := p.approvalCommitV2(projection, input)
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, err
	}
	receipt, err := p.assets.CommitApproval(ctx, commit)
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, fmt.Errorf("commit approved product assets: %w", err)
	}
	return publicationAcknowledgement(commit, input.RunID, input.PlanRevision, digest, receipt), nil
}

func (p *Publisher) PublishApprovedV3(ctx context.Context, input imageagent.PublishApprovedV3Input) (imageagent.PublicationAcknowledgement, error) {
	if p == nil || p.projections == nil || p.assets == nil || p.publicURLs == nil {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrValidation
	}
	if ctx == nil {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrValidation
	}
	if err := ctx.Err(); err != nil {
		return imageagent.PublicationAcknowledgement{}, err
	}
	if !canonical(input.RunID) || !canonical(input.TenantID) || !canonical(input.UserID) ||
		!canonical(input.IdempotencyKey) || input.PlanRevision <= 0 || len(input.CandidateAssetIDs) == 0 {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrValidation
	}
	scope := imageagent.RunScope{TenantID: input.TenantID, OwnerUserID: input.UserID, RunID: input.RunID}
	projection, err := p.projections.GetProjection(ctx, scope)
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, fmt.Errorf("load approved image agent projection: %w", err)
	}
	commit, digest, err := p.approvalCommit(projection, input)
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, err
	}
	receipt, err := p.assets.CommitApproval(ctx, commit)
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, fmt.Errorf("commit approved product assets: %w", err)
	}
	return publicationAcknowledgement(commit, input.RunID, input.PlanRevision, digest, receipt), nil
}

func (p *Publisher) approvalCommitV2(projection imageagent.RunProjection, input imageagent.PublishApprovedInput) (productasset.ApprovalCommit, string, error) {
	if projection.Run.TenantID != input.TenantID || projection.Run.UserID != input.UserID || projection.Run.ID != input.RunID ||
		projection.Run.Status != imageagent.RunStatusAwaitingFinalApproval || projection.Run.ActivePlanRevision != input.PlanRevision ||
		projection.Plan.Revision != input.PlanRevision || len(projection.Plan.Slots) != len(projection.Slots) {
		return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
	}
	product, err := imageagent.NormalizeProductContextRef(projection.AssetCatalog.ProductContext)
	if err != nil || product.ProductID == "" {
		return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
	}
	digest, err := imageagent.ResultDigestV2(projection.Plan, projection.Slots)
	if err != nil || projection.ResultDigest == "" || projection.ResultDigest != digest {
		return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
	}
	approved := make([]productasset.ApprovedAsset, 0, len(input.CandidateAssetIDs))
	observed := make([]string, 0, len(input.CandidateAssetIDs))
	mainAssets := 0
	for slotIndex, declared := range projection.Plan.Slots {
		slot := projection.Slots[slotIndex]
		role, roleErr := approvedRole(declared.Role)
		if roleErr != nil || slot.Slot.ID != declared.ID || slot.Slot.Role != declared.Role ||
			slot.Slot.Status != imageagent.SlotStatusAccepted || slot.Attempt <= 0 || len(slot.Candidates) == 0 {
			return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
		}
		for _, candidate := range slot.Candidates {
			if !canonical(candidate.AssetID) || !canonical(candidate.SourceAssetID) || candidate.DurableAsset != (imageagent.DurableAssetIdentity{}) {
				return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
			}
			url, err := imageagent.ValidateSafeImageURL(candidate.URL)
			if err != nil {
				return productasset.ApprovalCommit{}, "", imageagent.ErrValidation
			}
			if role == productasset.RoleMain {
				mainAssets++
			}
			observed = append(observed, candidate.AssetID)
			approved = append(approved, productasset.ApprovedAsset{
				ID: candidate.AssetID, RunID: input.RunID, PlanRevision: input.PlanRevision,
				SlotID: declared.ID, Attempt: slot.Attempt, Role: role, URL: url,
				SourceAssetID: candidate.SourceAssetID, Width: candidate.Width, Height: candidate.Height,
				Operations: append([]string(nil), candidate.Operations...),
			})
		}
	}
	if mainAssets != 1 || !slices.Equal(observed, input.CandidateAssetIDs) {
		return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
	}
	commit := productasset.ApprovalCommit{
		TenantID: input.TenantID, ProductKey: product.ProductID, TargetPlatform: strings.TrimSpace(projection.Run.TargetPlatform), ActionID: input.IdempotencyKey,
		SourceSnapshotVersion: sourceSnapshotVersion(projection.AssetCatalog), Assets: approved,
	}
	if err := productasset.ValidateApprovalCommit(commit); err != nil {
		return productasset.ApprovalCommit{}, "", fmt.Errorf("validate approved product assets: %w", err)
	}
	return commit, digest, nil
}

func publicationAcknowledgement(commit productasset.ApprovalCommit, runID string, revision int64, digest string, receipt productasset.ApprovalReceipt) imageagent.PublicationAcknowledgement {
	return imageagent.PublicationAcknowledgement{
		ProductKey: commit.ProductKey, RunID: runID, PlanRevision: revision,
		ResultDigest: digest, ActionID: receipt.ActionID, AssetIDs: append([]string(nil), receipt.AssetIDs...),
	}
}

func (p *Publisher) approvalCommit(projection imageagent.RunProjection, input imageagent.PublishApprovedV3Input) (productasset.ApprovalCommit, string, error) {
	if projection.Run.TenantID != input.TenantID || projection.Run.UserID != input.UserID || projection.Run.ID != input.RunID ||
		projection.Run.Status != imageagent.RunStatusAwaitingFinalApproval || projection.Run.ActivePlanRevision != input.PlanRevision ||
		projection.Plan.Revision != input.PlanRevision || len(projection.Plan.Slots) != len(projection.Slots) {
		return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
	}
	product, err := imageagent.NormalizeProductContextRef(projection.AssetCatalog.ProductContext)
	if err != nil || product.ProductID == "" {
		return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
	}
	digest, err := imageagent.ResultDigestV3(projection.Plan, projection.Slots)
	if err != nil || projection.ResultDigest == "" || projection.ResultDigest != digest {
		return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
	}
	requested := make([]string, len(input.CandidateAssetIDs))
	for index, candidateID := range input.CandidateAssetIDs {
		if !canonical(candidateID) {
			return productasset.ApprovalCommit{}, "", imageagent.ErrValidation
		}
		requested[index] = candidateID
	}
	approved := make([]productasset.ApprovedAsset, 0, len(requested))
	observed := make([]string, 0, len(requested))
	mainAssets := 0
	for slotIndex, declared := range projection.Plan.Slots {
		slot := projection.Slots[slotIndex]
		role, roleErr := approvedRole(declared.Role)
		if roleErr != nil || slot.Slot.ID != declared.ID || slot.Slot.Role != declared.Role ||
			slot.Slot.Status != imageagent.SlotStatusAccepted || slot.Attempt <= 0 || len(slot.Candidates) == 0 {
			return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
		}
		for candidateIndex, candidate := range slot.Candidates {
			if !canonical(candidate.AssetID) || !canonical(candidate.SourceAssetID) || candidate.URL != "" || len(candidate.Metadata) != 0 {
				return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
			}
			execution := imageagent.SlotExecutionInput{
				RunID: input.RunID, TenantID: input.TenantID, UserID: input.UserID,
				PlanRevision: input.PlanRevision, Slot: declared, Attempt: slot.Attempt,
			}
			if err := imageagent.ValidatePublishedAssetIdentityForSlot(execution, candidate.DurableAsset, candidateIndex); err != nil {
				return productasset.ApprovalCommit{}, "", err
			}
			identity, err := imageagent.NormalizeDurableAssetIdentity(candidate.DurableAsset)
			if err != nil {
				return productasset.ApprovalCommit{}, "", err
			}
			url, err := imageagent.ValidateSafeImageURL(p.publicURLs.PublicURL(identity.ObjectKey))
			if err != nil {
				return productasset.ApprovalCommit{}, "", imageagent.ErrValidation
			}
			if role == productasset.RoleMain {
				mainAssets++
			}
			observed = append(observed, candidate.AssetID)
			approved = append(approved, productasset.ApprovedAsset{
				ID: candidate.AssetID, RunID: input.RunID, PlanRevision: input.PlanRevision,
				SlotID: declared.ID, Attempt: slot.Attempt, Role: role, URL: url,
				SourceAssetID: candidate.SourceAssetID, Width: candidate.Width, Height: candidate.Height,
				Operations: append([]string(nil), candidate.Operations...),
			})
		}
	}
	if mainAssets != 1 || !slices.Equal(observed, requested) {
		return productasset.ApprovalCommit{}, "", imageagent.ErrRevisionConflict
	}
	commit := productasset.ApprovalCommit{
		TenantID: input.TenantID, ProductKey: product.ProductID, TargetPlatform: strings.TrimSpace(projection.Run.TargetPlatform), ActionID: input.IdempotencyKey,
		SourceSnapshotVersion: sourceSnapshotVersion(projection.AssetCatalog), Assets: approved,
	}
	if err := productasset.ValidateApprovalCommit(commit); err != nil {
		return productasset.ApprovalCommit{}, "", fmt.Errorf("validate approved product assets: %w", err)
	}
	return commit, digest, nil
}

func approvedRole(role imageagent.SlotRole) (productasset.Role, error) {
	if err := imageagent.ValidateSlotRole(role); err != nil {
		return "", err
	}
	if role == imageagent.SlotRoleMain {
		return productasset.RoleMain, nil
	}
	return productasset.RoleGallery, nil
}

func canonical(value string) bool {
	return value != "" && strings.TrimSpace(value) == value
}

func nilValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func sourceSnapshotVersion(catalog imageagent.AssetCatalog) uint64 {
	return catalog.ProductContext.SourceSnapshotVersion
}

var _ imageagent.ApprovedAssetPublisherV3 = (*Publisher)(nil)
var _ imageagent.ApprovedAssetPublisher = (*Publisher)(nil)
