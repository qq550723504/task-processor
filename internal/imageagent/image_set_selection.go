package imageagent

import (
	"context"
	"encoding/json"
	"reflect"
	"task-processor/internal/agentconfig"
	productasset "task-processor/internal/product/asset"
)

const ImageSetApprovalPublicationStarted = "approval.publish_started"

func (s *Service) ApproveImageSet(ctx context.Context, runID string, revision int64, resultDigest, actionID string, selection productasset.ImageSetCommand) error {
	identity, err := s.commandIdentity(ctx, runID, revision, actionID)
	if err != nil {
		return err
	}
	current, err := s.repository.GetProjection(ctx, RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: runID})
	if err != nil {
		return err
	}
	if !s.organizationScope || current.Plan.Set == nil || current.Plan.Revision != revision {
		return ErrCommandBlocked
	}
	if err = ValidateImageSetSelectionIntent(current.Plan, actionID, &selection); err != nil {
		return err
	}
	if resultDigest != current.ResultDigest {
		return ErrRevisionConflict
	}
	return s.workflows.ApproveResults(ctx, ApproveResultsCommand{RunID: runID, PlanRevision: revision, ResultDigest: resultDigest, ActorID: identity.UserID, ActionID: actionID, Identity: identity, Selection: CloneImageSetCommand(&selection)})
}

func CloneImageSetCommand(command *productasset.ImageSetCommand) *productasset.ImageSetCommand {
	if command == nil {
		return nil
	}
	copy := *command
	copy.Choices = append([]productasset.ImageSetChoice(nil), command.Choices...)
	if command.Target != nil {
		target := *command.Target
		copy.Target = &target
	}
	return &copy
}

func ValidateImageSetSelectionIntent(plan Plan, actionID string, selection *productasset.ImageSetCommand) error {
	if plan.Set == nil || selection == nil || selection.ActionID != actionID || !agentconfig.UUID(actionID) || !agentconfig.ImageDigest(selection.SelectionDigest) || !selection.ExpectedHead.Valid() || len(selection.Choices) < 1 || len(selection.Choices) > 40 {
		return ErrCommandBlocked
	}
	source := plan.Set.Source
	selected := selection.Source
	if selected.ItemID != source.OperationID || selected.OriginalPublicationID != source.OriginalPublicationID || selected.OriginalSnapshotVersion != source.OriginalVersion || selected.EffectiveCatalogVersion != source.EffectiveVersion || selected.TargetPlatform != plan.Set.Target.Platform {
		return ErrRevisionConflict
	}
	target := plan.Set.Target
	if target.Platform == "product" {
		if selection.Target != nil {
			return ErrRevisionConflict
		}
	} else {
		expected := &productasset.ImageSetTarget{StoreID: target.StoreID, Site: target.Site, ApplicationID: target.ApplicationID, ApplicationMode: target.ApplicationMode, CategoryID: target.CategoryID, ProductTypeID: target.ProductTypeID, AttributesDigest: target.AttributesDigest, VariantsDigest: target.VariantsDigest}
		if !reflect.DeepEqual(selection.Target, expected) {
			return ErrRevisionConflict
		}
	}
	encoded, err := json.Marshal(selection)
	if err != nil || len(encoded) > 128<<10 {
		return ErrValidation
	}
	return nil
}
