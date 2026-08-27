package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
)

type ImageAgentProjectionSource interface {
	GetProjection(context.Context, imageagent.RunScope) (imageagent.RunProjection, error)
}

type imageAgentApprovedPublisher struct {
	projections ImageAgentProjectionSource
	tasks       listingkit.ImageAgentPublicationTransactionRepository
}

func NewImageAgentApprovedPublisher(projections ImageAgentProjectionSource, tasks listingkit.ImageAgentPublicationTransactionRepository) (imageagent.ApprovedAssetPublisher, error) {
	if projections == nil || tasks == nil {
		return nil, fmt.Errorf("image agent projection source and ListingKit publication transaction store are required")
	}
	return &imageAgentApprovedPublisher{projections: projections, tasks: tasks}, nil
}

func (p *imageAgentApprovedPublisher) PublishApproved(ctx context.Context, input imageagent.PublishApprovedInput) (imageagent.PublicationAcknowledgement, error) {
	scope := imageagent.RunScope{TenantID: strings.TrimSpace(input.TenantID), OwnerUserID: strings.TrimSpace(input.UserID), RunID: strings.TrimSpace(input.RunID)}
	projection, err := p.projections.GetProjection(ctx, scope)
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, fmt.Errorf("load approved image agent projection: %w", err)
	}
	if projection.Run.TenantID != scope.TenantID || projection.Run.UserID != scope.OwnerUserID || projection.Run.ID != scope.RunID || projection.Run.Status != imageagent.RunStatusAwaitingFinalApproval || projection.Run.ActivePlanRevision != input.PlanRevision || projection.Plan.Revision != input.PlanRevision || strings.TrimSpace(projection.Run.BusinessTaskID) == "" || strings.TrimSpace(input.IdempotencyKey) == "" {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrRevisionConflict
	}
	approvedIDs, records, err := validateApprovedProjectionAndCandidates(projection, input.CandidateAssetIDs)
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, err
	}
	digest, err := approvedResultDigest(projection.Slots)
	if err != nil || projection.ResultDigest == "" || projection.ResultDigest != digest {
		return imageagent.PublicationAcknowledgement{}, imageagent.ErrRevisionConflict
	}
	fingerprint, err := imageAgentPublicationFingerprint(scope, projection.Run.BusinessTaskID, input.PlanRevision, digest, approvedIDs)
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, err
	}
	ack := listingkit.ImageAgentPublicationAcknowledgement{TaskID: projection.Run.BusinessTaskID, RunID: scope.RunID, PlanRevision: input.PlanRevision, ResultDigest: digest, IdempotencyKey: input.IdempotencyKey, CandidateAssetIDs: append([]string(nil), approvedIDs...)}
	stored, err := p.tasks.CommitImageAgentPublication(ctx, listingkit.ImageAgentPublicationCommit{TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, TaskID: projection.Run.BusinessTaskID, IdempotencyKey: input.IdempotencyKey, Fingerprint: fingerprint, Acknowledgement: ack}, func(task *listingkit.Task) error {
		if task == nil || task.TenantID != scope.TenantID || listingkit.ResolveTaskUserID(task) != scope.OwnerUserID || task.Result == nil || task.Result.StandardProductSnapshot == nil {
			return imageagent.ErrRunNotFound
		}
		applyApprovedAssetRecords(task.Result, records)
		return nil
	})
	if err != nil {
		return imageagent.PublicationAcknowledgement{}, fmt.Errorf("persist approved image agent candidates: %w", err)
	}
	return imageagent.PublicationAcknowledgement{TaskID: stored.TaskID, RunID: stored.RunID, PlanRevision: stored.PlanRevision, ResultDigest: stored.ResultDigest, IdempotencyKey: stored.IdempotencyKey, CandidateAssetIDs: append([]string(nil), stored.CandidateAssetIDs...)}, nil
}

func validateApprovedProjectionAndCandidates(projection imageagent.RunProjection, requested []string) ([]string, []asset.AssetRecord, error) {
	if len(projection.Slots) != len(projection.Plan.Slots) || len(requested) == 0 {
		return nil, nil, imageagent.ErrRevisionConflict
	}
	requestedSet := make(map[string]struct{}, len(requested))
	for _, rawID := range requested {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return nil, nil, imageagent.ErrValidation
		}
		if _, duplicate := requestedSet[id]; duplicate {
			return nil, nil, imageagent.ErrRevisionConflict
		}
		requestedSet[id] = struct{}{}
	}
	orderedIDs := make([]string, 0, len(requested))
	records := make([]asset.AssetRecord, 0, len(requested))
	for index, declared := range projection.Plan.Slots {
		slot := projection.Slots[index]
		if slot.Slot.ID != declared.ID || slot.Slot.Role != declared.Role || slot.Slot.Status != imageagent.SlotStatusAccepted || slot.Attempt <= 0 || len(slot.Candidates) == 0 {
			return nil, nil, imageagent.ErrRevisionConflict
		}
		for _, candidate := range slot.Candidates {
			id := strings.TrimSpace(candidate.AssetID)
			if _, ok := requestedSet[id]; !ok {
				return nil, nil, imageagent.ErrRevisionConflict
			}
			validatedURL, err := imageagent.ValidateSafeImageURL(candidate.URL)
			if err != nil {
				return nil, nil, imageagent.ErrValidation
			}
			candidate.URL = validatedURL
			orderedIDs = append(orderedIDs, id)
			records = append(records, approvedAssetRecord(projection.Run.BusinessTaskID, slot.Slot, candidate))
			delete(requestedSet, id)
		}
	}
	if len(requestedSet) != 0 || !slices.Equal(orderedIDs, requested) {
		return nil, nil, imageagent.ErrRevisionConflict
	}
	return orderedIDs, records, nil
}

func approvedResultDigest(slots []imageagent.SlotProjection) (string, error) {
	type digestSlot struct {
		SlotID            string   `json:"slot_id"`
		CandidateAssetIDs []string `json:"candidate_asset_ids"`
	}
	payload := make([]digestSlot, 0, len(slots))
	for _, slot := range slots {
		if slot.Slot.Status != imageagent.SlotStatusAccepted || len(slot.Candidates) == 0 {
			return "", imageagent.ErrRevisionConflict
		}
		ids := make([]string, 0, len(slot.Candidates))
		for _, candidate := range slot.Candidates {
			if strings.TrimSpace(candidate.AssetID) == "" {
				return "", imageagent.ErrRevisionConflict
			}
			ids = append(ids, strings.TrimSpace(candidate.AssetID))
		}
		payload = append(payload, digestSlot{SlotID: strings.TrimSpace(slot.Slot.ID), CandidateAssetIDs: ids})
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func imageAgentPublicationFingerprint(scope imageagent.RunScope, taskID string, revision int64, digest string, candidateIDs []string) (string, error) {
	encoded, err := json.Marshal(struct {
		Scope        imageagent.RunScope
		TaskID       string
		Revision     int64
		Digest       string
		CandidateIDs []string
	}{scope, taskID, revision, digest, candidateIDs})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func applyApprovedAssetRecords(result *listingkit.ListingKitResult, records []asset.AssetRecord) {
	bundle := result.StandardProductSnapshot.AssetBundle
	if bundle == nil {
		bundle = &asset.Bundle{}
	}
	existing := make(map[string]struct{}, len(bundle.Assets))
	for _, item := range bundle.Assets {
		existing[item.ID] = struct{}{}
	}
	missing := make([]asset.AssetRecord, 0, len(records))
	for _, record := range records {
		if _, ok := existing[record.ID]; !ok {
			missing = append(missing, record)
		}
	}
	if len(missing) > 0 {
		bundle = asset.RebuildBundleWithRecords(bundle, missing)
	}
	selection := bundle.Selection
	if selection == nil {
		selection = &asset.Selection{}
		bundle.Selection = selection
	}
	for _, record := range records {
		if record.Kind == asset.KindMainImage {
			selection.MainAssetID = record.ID
			continue
		}
		if !containsImageAgentAssetID(selection.GalleryAssetIDs, record.ID) {
			selection.GalleryAssetIDs = append(selection.GalleryAssetIDs, record.ID)
		}
	}
	result.StandardProductSnapshot.AssetBundle = bundle
	result.StandardProductSnapshot.AssetInventorySummary = asset.InventorySummaryFromBundle(bundle)
	result.AssetBundle = bundle
	result.AssetInventorySummary = result.StandardProductSnapshot.AssetInventorySummary
}

func approvedAssetRecord(taskID string, slot imageagent.Slot, candidate imageagent.AssetCandidate) asset.AssetRecord {
	kind := asset.KindSceneImage
	switch slot.Role {
	case imageagent.SlotRoleMain:
		kind = asset.KindMainImage
	case imageagent.SlotRoleDetail:
		kind = asset.KindDetailCrop
	case imageagent.SlotRoleSellingPoint:
		kind = asset.KindSellingPointImage
	case imageagent.SlotRoleSize:
		kind = asset.KindSizeSceneImage
	}
	return asset.AssetRecord{ID: candidate.AssetID, TaskID: taskID, Kind: kind, Origin: asset.OriginGenerated, Role: string(slot.Role), URL: candidate.URL, Generator: "image-agent", Lineage: &asset.AssetLineage{SourceAssetIDs: []string{candidate.SourceAssetID}, ParentAssetIDs: []string{candidate.SourceAssetID}, Step: "image-agent"}, Metadata: cloneImageAgentMetadata(candidate.Metadata)}
}

func cloneImageAgentMetadata(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func containsImageAgentAssetID(values []string, target string) bool {
	return slices.Contains(values, target)
}
