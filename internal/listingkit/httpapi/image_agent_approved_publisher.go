package httpapi

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
)

type ImageAgentProjectionSource interface {
	GetProjection(context.Context, imageagent.RunScope) (imageagent.RunProjection, error)
}

type ImageAgentTaskResultStore interface {
	MutateTaskResult(context.Context, string, listingkit.TaskResultMutation) (*listingkit.Task, error)
}

type imageAgentApprovedPublisher struct {
	projections ImageAgentProjectionSource
	tasks       ImageAgentTaskResultStore
}

// NewImageAgentApprovedPublisher projects final user-approved candidates into
// the canonical ListingKit task result transaction. Candidate IDs make exact
// Temporal activity retries idempotent.
func NewImageAgentApprovedPublisher(projections ImageAgentProjectionSource, tasks ImageAgentTaskResultStore) (imageagent.ApprovedAssetPublisher, error) {
	if projections == nil || tasks == nil {
		return nil, fmt.Errorf("image agent projection source and ListingKit task result store are required")
	}
	return &imageAgentApprovedPublisher{projections: projections, tasks: tasks}, nil
}

func (p *imageAgentApprovedPublisher) PublishApproved(ctx context.Context, input imageagent.PublishApprovedInput) error {
	scope := imageagent.RunScope{TenantID: strings.TrimSpace(input.TenantID), OwnerUserID: strings.TrimSpace(input.UserID), RunID: strings.TrimSpace(input.RunID)}
	projection, err := p.projections.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("load approved image agent projection: %w", err)
	}
	if projection.Run.ActivePlanRevision != input.PlanRevision || projection.Plan.Revision != input.PlanRevision || strings.TrimSpace(projection.Run.BusinessTaskID) == "" {
		return imageagent.ErrRevisionConflict
	}
	approved := make(map[string]struct{}, len(input.CandidateAssetIDs))
	for _, rawID := range input.CandidateAssetIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return imageagent.ErrValidation
		}
		approved[id] = struct{}{}
	}
	records := make([]asset.AssetRecord, 0, len(approved))
	for _, slot := range projection.Slots {
		for _, candidate := range slot.Candidates {
			if _, ok := approved[candidate.AssetID]; !ok {
				continue
			}
			records = append(records, approvedAssetRecord(projection.Run.BusinessTaskID, slot.Slot, candidate))
			delete(approved, candidate.AssetID)
		}
	}
	if len(approved) != 0 || len(records) == 0 {
		return imageagent.ErrRevisionConflict
	}
	_, err = p.tasks.MutateTaskResult(ctx, projection.Run.BusinessTaskID, func(task *listingkit.Task) error {
		if task == nil || task.TenantID != scope.TenantID || listingkit.ResolveTaskUserID(task) != scope.OwnerUserID || task.Result == nil || task.Result.StandardProductSnapshot == nil {
			return imageagent.ErrRunNotFound
		}
		bundle := task.Result.StandardProductSnapshot.AssetBundle
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
		task.Result.StandardProductSnapshot.AssetBundle = bundle
		task.Result.StandardProductSnapshot.AssetInventorySummary = asset.InventorySummaryFromBundle(bundle)
		task.Result.AssetBundle = bundle
		task.Result.AssetInventorySummary = task.Result.StandardProductSnapshot.AssetInventorySummary
		return nil
	})
	if err != nil {
		return fmt.Errorf("persist approved image agent candidates: %w", err)
	}
	return nil
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
	return asset.AssetRecord{
		ID: candidate.AssetID, TaskID: taskID, Kind: kind, Origin: asset.OriginGenerated,
		Role: string(slot.Role), URL: candidate.URL, Generator: "image-agent",
		Lineage:  &asset.AssetLineage{SourceAssetIDs: []string{candidate.SourceAssetID}, ParentAssetIDs: []string{candidate.SourceAssetID}, Step: "image-agent"},
		Metadata: cloneImageAgentMetadata(candidate.Metadata),
	}
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
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
