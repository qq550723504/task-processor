package temporal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/shared/aiidentity"
	"time"
)

const slotResultPersistedEventType = "slot.result.persisted"

var errPublicationOwnerRequiresActivity = errors.New("publication owner requires a Temporal activity context")

type RecoveryWorkflowStarter func(context.Context, EffectRecoveryWorkflowInput) error

type ActivityDependencies struct {
	Repository               imageagent.Repository
	SlotEffects              imageagent.SlotExternalEffectRepository
	SlotExecutor             imageagent.SlotExecutor
	Publisher                imageagent.ApprovedAssetPublisher
	PublisherV3              imageagent.ApprovedAssetPublisherV3
	SlotEffectsV3            imageagent.SlotExternalEffectV3Repository
	StagedSlotExecutor       imageagent.StagedSlotExecutor
	ArtifactStore            DurableArtifactStore
	PublicationOwner         func(context.Context) (string, error)
	PublicationLeaseDuration time.Duration
	RecoveryWorkflowStarter  RecoveryWorkflowStarter
}

type Activities struct {
	repository               imageagent.Repository
	slotEffects              imageagent.SlotExternalEffectRepository
	slotExecutor             imageagent.SlotExecutor
	publisher                imageagent.ApprovedAssetPublisher
	publisherV3              imageagent.ApprovedAssetPublisherV3
	slotEffectsV3            imageagent.SlotExternalEffectV3Repository
	stagedSlotExecutor       imageagent.StagedSlotExecutor
	artifactStore            DurableArtifactStore
	publicationOwner         func(context.Context) (string, error)
	publicationLeaseDuration time.Duration
	recoveryWorkflowStarter  RecoveryWorkflowStarter
}

func NewActivities(dependencies ActivityDependencies) (*Activities, error) {
	if dependencies.Repository == nil {
		return nil, fmt.Errorf("image agent repository is required")
	}
	if dependencies.SlotEffects == nil {
		if slotEffects, ok := dependencies.Repository.(imageagent.SlotExternalEffectRepository); ok {
			dependencies.SlotEffects = slotEffects
		}
	}
	if dependencies.SlotExecutor == nil {
		return nil, fmt.Errorf("image agent slot executor is required")
	}
	if dependencies.SlotEffects == nil {
		return nil, fmt.Errorf("image agent slot external effect repository is required")
	}
	if dependencies.Publisher == nil {
		return nil, fmt.Errorf("image agent approved asset publisher is required")
	}
	v3Requested := dependencies.SlotEffectsV3 != nil || dependencies.StagedSlotExecutor != nil || dependencies.ArtifactStore != nil
	if v3Requested {
		if dependencies.SlotEffectsV3 == nil {
			if slotEffects, ok := dependencies.Repository.(imageagent.SlotExternalEffectV3Repository); ok {
				dependencies.SlotEffectsV3 = slotEffects
			}
		}
		if dependencies.SlotEffectsV3 == nil {
			return nil, fmt.Errorf("image agent v3 slot external effect repository is required")
		}
		if dependencies.StagedSlotExecutor == nil {
			return nil, fmt.Errorf("image agent staged slot executor is required")
		}
		if dependencies.ArtifactStore == nil {
			return nil, fmt.Errorf("image agent durable artifact store is required")
		}
		if dependencies.PublisherV3 == nil {
			return nil, fmt.Errorf("image agent v3 approved asset publisher is required")
		}
		if dependencies.PublicationOwner == nil {
			dependencies.PublicationOwner = temporalPublicationOwner
		}
		if dependencies.PublicationLeaseDuration <= 0 {
			dependencies.PublicationLeaseDuration = 2 * time.Minute
		}
	}
	return &Activities{
		repository: dependencies.Repository, slotEffects: dependencies.SlotEffects, slotExecutor: dependencies.SlotExecutor, publisher: dependencies.Publisher, publisherV3: dependencies.PublisherV3,
		slotEffectsV3: dependencies.SlotEffectsV3, stagedSlotExecutor: dependencies.StagedSlotExecutor, artifactStore: dependencies.ArtifactStore,
		publicationOwner: dependencies.PublicationOwner, publicationLeaseDuration: dependencies.PublicationLeaseDuration,
		recoveryWorkflowStarter: dependencies.RecoveryWorkflowStarter,
	}, nil
}

func restoreActivityIdentity(ctx context.Context, identity imageagent.ExecutionIdentity) (context.Context, error) {
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	identity.BusinessTaskID = strings.TrimSpace(identity.BusinessTaskID)
	identity.TraceID = strings.TrimSpace(identity.TraceID)
	if identity.TenantID == "" || identity.UserID == "" {
		return nil, fmt.Errorf("captured image agent tenant and user identity are required")
	}
	ctx = authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: identity.TenantID, UserID: identity.UserID})
	return aiidentity.WithIdentity(ctx, aiidentity.Identity{
		TenantID: identity.TenantID, UserID: identity.UserID, BusinessTaskID: identity.BusinessTaskID, TraceID: identity.TraceID,
	}), nil
}
