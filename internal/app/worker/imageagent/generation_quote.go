package imageagentworker

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	openai "task-processor/internal/integration/openai"
	"task-processor/internal/shared/aiidentity"
)

type imageGenerationQuoteReader struct{ factory generationProviderFactory }

func NewImageGenerationQuoteReader(db *gorm.DB, price config.ImageAgentGenerationConfig) (imageagent.ImageSetQuoteReader, error) {
	if db == nil || !price.Configured() {
		return nil, imageagent.ErrBudgetQuoteUnavailable
	}
	return imageGenerationQuoteReader{factory: generationProviderFactory{resolver: organizationCredentialAdmission{resolver: openai.NewOrganizationCredentialResolver(db)}, price: price}}, nil
}

func (r imageGenerationQuoteReader) ReadImageGenerationQuote(ctx context.Context, identity imageagent.ExecutionIdentity) (imageagent.ImageGenerationQuote, error) {
	if ctx == nil {
		return imageagent.ImageGenerationQuote{}, imageagent.ErrIdentityRequired
	}
	verified, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if ctx == nil || !ok || identity.ScopeProtocol != imageagent.OrganizationScopeProtocol || identity.TenantID == "" || identity.UserID == "" || identity.MemberID == "" || verified.TenantID != identity.TenantID || verified.EffectiveOrganizationID != identity.TenantID || verified.UserID != identity.UserID || verified.EffectiveMemberID != identity.MemberID {
		return imageagent.ImageGenerationQuote{}, imageagent.ErrIdentityRequired
	}
	ctx = aiidentity.WithIdentity(ctx, aiidentity.Identity{TenantID: identity.TenantID, UserID: identity.UserID, BusinessTaskID: identity.BusinessTaskID, AgentRunID: identity.RunID, TraceID: identity.TraceID})
	_, metadata, err := r.factory.resolve(ctx)
	if err != nil {
		return imageagent.ImageGenerationQuote{}, err
	}
	return imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: metadata.PriceVersion, Points: metadata.Points, RouteReference: metadata.RouteReference, CredentialReference: metadata.CredentialReference, ConfigurationVersion: metadata.ConfigurationVersion}, nil
}
