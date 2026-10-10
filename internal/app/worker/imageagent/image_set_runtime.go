package imageagentworker

import (
	"context"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	imagestore "task-processor/internal/imageagent/store"
	imagetools "task-processor/internal/imageagent/tools"
	productasset "task-processor/internal/product/asset"
)

// The current application supplies its existing source/rule and Asset owners.
// This worker consumes only the set protocol and never assembles retired
// Extract/Render/Review capabilities or another approval inventory.
func NewImageSetTemporalDependencies(cfg *config.Config, imageDB, resourceDB *gorm.DB, authorizer imageagent.ExecutionAuthorizer, contexts imageagent.ImageSetContextReader, publisher imageagent.ApprovedImageSetPublisher, approvals productasset.ApprovalCommitReader, logger *logrus.Logger) (appruntime.ImageAgentTemporalDependencies, error) {
	var empty appruntime.ImageAgentTemporalDependencies
	if cfg == nil || imageDB == nil || resourceDB == nil || authorizer == nil || contexts == nil || publisher == nil || approvals == nil || !cfg.ImageAgent.Generation.Configured() {
		return empty, imageagent.ErrValidation
	}
	repository := imagestore.NewOrganizationRepository(imageDB)
	executor, recovery, err := buildOrganizationGeneration(cfg.ImageAgent.Generation, imageDB, resourceDB, repository, authorizer, logger, contexts)
	if err != nil {
		return empty, err
	}
	artifacts, err := buildImageAgentDurableArtifactStore(cfg, defaultImageAgentArtifactTiming, logger)
	if err != nil {
		return empty, err
	}
	slots := imageSetSlotExecutor{generation: executor, builder: imagetools.NewImageSetResultBuilder()}
	return appruntime.ImageAgentTemporalDependencies{Repository: repository, ExecutionAuthorizer: authorizer, SlotExecutor: slots, StagedSlotExecutor: slots, ArtifactStore: artifacts, ImageSetPublisher: publisher, ImageSetApprovals: approvals, GenerationRecovery: recovery, GenerationOutputRecovery: generationOutputRecovery(nil), PublicationLeaseDuration: defaultImageAgentArtifactTiming.PublicationLeaseDuration}, nil
}

type imageSetSlotExecutor struct {
	generation *imageagent.GenerationExecution
	builder    *imagetools.ProductImageSlotExecutor
}

func (e imageSetSlotExecutor) ExecuteSlot(context.Context, imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	return imageagent.SlotExecutionResult{}, imageagent.ErrCommandBlocked
}
func (e imageSetSlotExecutor) GenerateSlot(context.Context, imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	return imageagent.SlotGeneratedOutput{}, imageagent.ErrCommandBlocked
}
func (e imageSetSlotExecutor) QuoteSlot(ctx context.Context, input imageagent.SlotExecutionInput, policy imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	if input.ImageSet == nil || e.generation == nil {
		return imageagent.SlotUsageQuote{}, imageagent.ErrCommandBlocked
	}
	return e.generation.QuoteSlot(ctx, input, policy)
}
func (e imageSetSlotExecutor) GenerateQuotedSlot(ctx context.Context, input imageagent.SlotExecutionInput, quote imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	if input.ImageSet == nil || e.generation == nil {
		return imageagent.SlotGeneratedOutput{}, imageagent.ErrCommandBlocked
	}
	return e.generation.GenerateQuotedSlot(ctx, input, quote)
}
func (e imageSetSlotExecutor) BuildSlotResult(ctx context.Context, input imageagent.SlotExecutionInput, output imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	if input.ImageSet == nil || e.builder == nil {
		return imageagent.SlotExecutionResult{}, imageagent.ErrCommandBlocked
	}
	return e.builder.BuildSlotResult(ctx, input, output)
}
