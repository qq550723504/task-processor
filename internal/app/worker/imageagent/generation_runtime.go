package imageagentworker

import (
	"context"
	"errors"
	"reflect"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	"task-processor/internal/integration/httpimage"
	openai "task-processor/internal/integration/openai"
	resourceadapter "task-processor/internal/integration/orgresource"
	"task-processor/internal/pkg/imagex"
	productimage "task-processor/internal/product/image"
)

type generationResourceAuthorizer struct {
	repository imageagent.Repository
	authorizer imageagent.ExecutionAuthorizer
}

func (a generationResourceAuthorizer) AuthorizeImageGeneration(ctx context.Context, intent imageagent.GenerationIntent) error {
	projection, err := a.repository.GetProjection(ctx, intent.Identity.RunScope)
	if err != nil {
		return imageagent.ErrIdentityRequired
	}
	run := projection.Run
	if run.ScopeProtocol != imageagent.OrganizationScopeProtocol || run.MemberID != intent.MemberID || run.TenantID != intent.Identity.TenantID || run.UserID != intent.Identity.OwnerUserID || run.ID != intent.Identity.RunID {
		return imageagent.ErrIdentityRequired
	}
	return a.authorizer.AuthorizeExecution(ctx, imageagent.ExecutionIdentity{ScopeProtocol: run.ScopeProtocol, RunID: run.ID, TenantID: run.TenantID, UserID: run.UserID, MemberID: run.MemberID, BusinessTaskID: run.BusinessTaskID})
}

func buildOrganizationGeneration(cfg config.ImageAgentGenerationConfig, db, commercial *gorm.DB, repository imageagent.Repository, authorizer imageagent.ExecutionAuthorizer, logger *logrus.Logger, sourceContexts ...imageagent.ImageSetContextReader) (*imageagent.GenerationExecution, imageagent.GenerationRecovery, error) {
	if len(sourceContexts) > 1 {
		return nil, nil, imageagent.ErrValidation
	}
	facts, ok := repository.(imageagent.GenerationFactRepository)
	if !ok || db == nil || commercial == nil || authorizer == nil {
		return nil, nil, imageagent.ErrValidation
	}
	executionAuthorizer := imageagent.ExecutionAuthorizer(imageSetGenerationAuthorizer{repository: repository, live: authorizer})
	if len(sourceContexts) == 1 {
		guard, ok := sourceContexts[0].(imageagent.ImageSetSourceGuard)
		if !ok {
			return nil, nil, imageagent.ErrCommandBlocked
		}
		executionAuthorizer = imageSetGenerationAuthorizer{repository: repository, live: authorizer, sources: guard}
	}
	resources, err := resourceadapter.NewGormImageGenerationRepository(commercial, resourceadapter.TransactionConfig{}, facts, generationResourceAuthorizer{repository, executionAuthorizer})
	if err != nil {
		return nil, nil, err
	}
	recovery, err := imageagent.NewGenerationRecovery(facts, resources)
	if err != nil {
		return nil, nil, err
	}
	// Removing a current price disables NEW work, never settlement of an
	// existing immutable price/proof. Ordinary startup does not install schema.
	if cfg == (config.ImageAgentGenerationConfig{}) {
		return nil, recovery, nil
	}
	if !cfg.Configured() {
		return nil, nil, imageagent.ErrBudgetQuoteUnavailable
	}
	limits, err := resourceadapter.NewGormMemberLimitRepository(commercial, resourceadapter.TransactionConfig{})
	if err != nil {
		return nil, nil, err
	}
	profile, err := loadEmbeddedImagePolicyResolver()
	if err != nil {
		return nil, nil, err
	}
	var componentLogger *logrus.Entry
	if logger != nil {
		componentLogger = logrus.NewEntry(logger).WithField("component", "image-agent-grsai")
	}
	factory := generationProviderFactory{resolver: organizationCredentialAdmission{resolver: openai.NewOrganizationCredentialResolver(db)}, price: cfg, profile: profile, logger: componentLogger}
	executor, err := imageagent.NewGenerationExecution(imageagent.GenerationExecutionDependencies{Facts: facts, Resources: resources, Authorizer: executionAuthorizer, MaxSourceBytes: productimage.MaxInlineArtifactBytes,
		PrepareProvider: factory.prepare, RevalidateProvider: factory.revalidate,
		ReadMemberLimit: func(ctx context.Context, org, member string) (imageagent.GenerationMemberLimit, error) {
			read, err := limits.ReadMonthlyLimit(ctx, org, member)
			return imageagent.GenerationMemberLimit{Version: read.Version, MonthStart: read.MonthStart}, err
		},
		ReadSourceBytes: func(ctx context.Context, input imageagent.SlotExecutionInput) ([]byte, error) {
			catalog, err := repository.GetAssetCatalog(ctx, imageagent.RunScope{TenantID: input.TenantID, OwnerUserID: input.UserID, RunID: input.RunID})
			if err != nil {
				return nil, err
			}
			if err := validateGenerationSourceCatalog(catalog, input.AssetCatalog); err != nil {
				return nil, err
			}
			return readGenerationSource(ctx, input)
		},
	})
	return executor, recovery, err
}

func validateGenerationSourceCatalog(persisted, supplied imageagent.AssetCatalog) error {
	canonical, err := imageagent.NormalizeAssetCatalog(supplied)
	// The content hash binds every asset and product input. CreatedAt is
	// bookkeeping: PostgreSQL stores fewer fractional digits than Temporal JSON.
	if err != nil || persisted.Manifest.Version != canonical.Manifest.Version || persisted.Manifest.Hash != canonical.Manifest.Hash {
		return imageagent.ErrRevisionConflict
	}
	return nil
}

func revalidateGenerationImageSet(ctx context.Context, repository imageagent.Repository, contexts imageagent.ImageSetSourceGuard, input imageagent.SlotExecutionInput) error {
	if input.ImageSet == nil {
		return nil
	}
	if contexts == nil {
		return imageagent.ErrCommandBlocked
	}
	projection, err := repository.GetProjection(ctx, imageagent.RunScope{TenantID: input.TenantID, OwnerUserID: input.UserID, RunID: input.RunID})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(projection.Plan.Set, input.ImageSet) {
		return imageagent.ErrRevisionConflict
	}
	return contexts.AuthorizeImageSetSource(ctx, input.OrganizationIdentity, projection)
}

type imageSetGenerationAuthorizer struct {
	repository imageagent.Repository
	live       imageagent.ExecutionAuthorizer
	sources    imageagent.ImageSetSourceGuard
}

func (a imageSetGenerationAuthorizer) AuthorizeExecution(ctx context.Context, id imageagent.ExecutionIdentity) error {
	if err := a.live.AuthorizeExecution(ctx, id); err != nil {
		return err
	}
	projection, err := a.repository.GetProjection(ctx, imageagent.RunScope{TenantID: id.TenantID, OwnerUserID: id.UserID, RunID: id.RunID})
	if err != nil {
		return err
	}
	if projection.Plan.Set == nil {
		return nil
	}
	return revalidateGenerationImageSet(ctx, a.repository, a.sources, imageagent.SlotExecutionInput{RunID: id.RunID, TenantID: id.TenantID, UserID: id.UserID, OrganizationIdentity: id, ImageSet: projection.Plan.Set})
}

func readGenerationSource(ctx context.Context, input imageagent.SlotExecutionInput) ([]byte, error) {
	if len(input.Slot.SourceAssetIDs) != 1 || input.AssetCatalog.Manifest.Hash == "" {
		return nil, imageagent.ErrValidation
	}
	catalog, err := imageagent.NormalizeAssetCatalog(input.AssetCatalog)
	if err != nil {
		return nil, imageagent.ErrValidation
	}
	for _, asset := range catalog.Assets {
		if asset.ID != input.Slot.SourceAssetIDs[0] || asset.Type != imageagent.AuthorizedAssetSource {
			continue
		}
		data, err := httpimage.Download(ctx, httpimage.NewPublicImageHTTPClient(), asset.URL, productimage.MaxInlineArtifactBytes)
		if err != nil {
			return nil, errors.New("source image is unavailable")
		}
		if _, err = imagex.Inspect(data); err != nil {
			return nil, imageagent.ErrValidation
		}
		return data, nil
	}
	return nil, imageagent.ErrValidation
}
