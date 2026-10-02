package httpapi

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	confighttp "task-processor/internal/agentconfig/httpapi"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/agent/titletext"
	"task-processor/internal/integration/knowledgeauth"
	configstore "task-processor/internal/integration/persistence/agentconfig"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/knowledge"
	"task-processor/internal/listing/readiness/tools/readinessinspect"
	"task-processor/internal/product/asset/tools/assetinspect"
	"task-processor/internal/product/catalog/tools/canonicalinspect"
	"task-processor/internal/product/sourcing/tools/sourceevidenceinspect"
	"task-processor/internal/workbenchcontext"
	"time"
)

// productTitleRegistration is the single code-owned registration consumed by
// both the market catalog and the executable runtime. No database allowlist.
func productTitleRegistration() (commercetool.AgentDefinition, []commercetool.Definition) {
	tools := []commercetool.Definition{canonicalinspect.Definition(), sourceevidenceinspect.Definition(), assetinspect.Definition(), readinessinspect.Definition()}
	d := commercetool.AgentDefinition{ID: "product.title.agent", Version: "v1.0.0"}
	for _, t := range tools {
		d.AllowedTools = append(d.AllowedTools, t.Ref)
	}
	return d, tools
}

type productAgentCatalog struct{}

func (productAgentCatalog) ReadCatalog(context.Context) ([]agentconfig.CatalogEntry, error) {
	d, _ := productTitleRegistration()
	return []agentconfig.CatalogEntry{{Definition: d, Name: "商品标题优化智能体", Description: "读取商品证据生成标题建议，经过人工审核后显式应用。", ParameterSchema: agentconfig.ParameterSchema}}, nil
}
func WithAgentConfiguration(db *gorm.DB) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.agentConfigurationDB = db }
}
func buildAgentConfigurationModule(ctx context.Context, db *gorm.DB, resolver organizationIdentityResolver, auth *authz.ListingKitAuthorizer, k *knowledge.Service, runtime *productAgentApplication) (kernelmodule.Module, error) {
	if runtime != nil && (runtime.config.RunDB != db || runtime.store == nil || !runtime.store.UsesPool(db)) {
		return nil, agentconfig.ErrUnavailable
	}
	if e := configstore.VerifySchema(ctx, db); e != nil {
		return nil, e
	}
	repo, e := configstore.New(db)
	if e != nil {
		return nil, e
	}
	knowledgeAuth, e := knowledgeauth.NewAuthorizer(resolver, auth)
	if e != nil {
		return nil, e
	}
	h := &confighttp.Handler{Repository: repo, Catalog: productAgentCatalog{}, Bind: (productReviewCapabilityBinder{now: time.Now}).Bind}
	h.Authorize = func(ctx context.Context, permissions ...string) (agent.Scope, error) {
		id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
		if !ok || !agent.ValidID(id.EffectiveOrganizationID) || !agent.ValidID(id.UserID) || !agent.ValidID(id.EffectiveMemberID) || id.TenantID != id.EffectiveOrganizationID {
			return agent.Scope{}, agentconfig.ErrForbidden
		}
		for _, p := range permissions {
			if auth.Authorize(id.UserID, id.Roles, p) {
				return agent.Scope{OrganizationID: id.EffectiveOrganizationID, ActorID: id.UserID}, nil
			}
		}
		return agent.Scope{}, agentconfig.ErrForbidden
	}
	h.Knowledge = func(ctx context.Context, scope agent.Scope, id string) error {
		if k == nil {
			return agentconfig.ErrUnavailable
		}
		ctx, e := knowledgeRequestContext(ctx)
		if e != nil {
			return agentconfig.ErrForbidden
		}
		fresh, e := knowledgeAuth.AuthorizeKnowledge(ctx)
		if e != nil || fresh.OrganizationID != scope.OrganizationID || fresh.ActorID != scope.ActorID {
			return agentconfig.ErrForbidden
		}
		base, e := k.GetBase(ctx, fresh, id)
		if e != nil {
			return agentconfig.ErrUnavailable
		}
		if base.OrganizationID != scope.OrganizationID || base.State != knowledge.Active {
			return agentconfig.ErrUnavailable
		}
		return nil
	}
	h.Capabilities = func(ctx context.Context, _ agentconfig.CatalogEntry) []agentconfig.Capability {
		now := time.Now().UTC()
		text := agentconfig.Capability{ID: "text.generate", Support: "REQUIRED", Readiness: "UNAVAILABLE", Reason: "当前环境尚未开放标题执行", ObservedAt: now}
		if runtime != nil && runtime.model != nil && runtime.config.Manager != nil {
			original, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
			capability, bound := ctx.Value(productReviewCapabilityContextKey{}).(productReviewRequestCapability)
			if ok && bound && capability.actorID == original.UserID && capability.effectiveOrganizationID == original.EffectiveOrganizationID {
				fresh, err := resolver.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: authidentity.AuthenticatedIdentity{UserID: capability.actorID, HomeOrganizationID: capability.homeOrganizationID, TokenExpiresAt: capability.tokenExpiresAt}, BearerToken: capability.bearerToken, RequestedOrganizationID: capability.effectiveOrganizationID})
				allowed := false
				for _, organizationID := range runtime.config.AllowedOrganizationIDs {
					allowed = allowed || organizationID == fresh.EffectiveOrganizationID
				}
				if err == nil && allowed && fresh.UserID == original.UserID && fresh.TenantID == original.EffectiveOrganizationID && fresh.EffectiveOrganizationID == fresh.TenantID {
					status := runtime.model.RouteReadinessForVerifiedOrganization(authidentity.WithAuthenticatedIdentity(ctx, fresh), fresh.EffectiveOrganizationID)
					switch status {
					case titletext.TextRouteAvailable:
						text.Readiness = "AVAILABLE"
						text.Reason = "文本配置已接入，执行时重新确认权限、点数与预算"
					case titletext.TextRouteNeedsConfiguration:
						text.Readiness = "NEEDS_CONFIGURATION"
						text.Reason = "当前企业标题模型凭据需由部署者配置"
					default:
						text.Reason = "当前企业标题执行尚未开放"
					}
				}
			}
		}
		read := "UNAVAILABLE"
		reason := "当前环境未接入企业知识引用"
		if k != nil {
			read = "REQUIRES_AUTHORIZATION"
			reason = "选用知识时重新确认知识权限与可读版本"
			if _, e := h.Authorize(ctx, authz.PermissionWorkbenchKnowledgeRead); e == nil {
				read = "AVAILABLE"
				reason = "可选企业知识引用，执行前显式确认"
			}
		}
		return []agentconfig.Capability{text, {ID: "knowledge.context", Support: "OPTIONAL", Readiness: read, Reason: reason, ObservedAt: now}, {ID: "image.generate", Support: "NOT_SUPPORTED", Readiness: "UNAVAILABLE", Reason: "本智能体未提供图片执行", ObservedAt: now}, {ID: "platform.write", Support: "NOT_SUPPORTED", Readiness: "UNAVAILABLE", Reason: "本智能体不执行远端平台写入", ObservedAt: now}}
	}
	h.Recent = func(ctx context.Context, scope agent.Scope, id, cursor string, size int) (any, string, error) {
		if runtime == nil {
			return nil, "", agentconfig.ErrUnavailable
		}
		ctx, e := knowledgeRequestContext(ctx)
		if e != nil {
			return nil, "", agentconfig.ErrForbidden
		}
		fresh, e := runtime.freshIdentity(ctx)
		if e != nil || fresh.UserID != scope.ActorID || fresh.TenantID != scope.OrganizationID {
			return nil, "", agentconfig.ErrForbidden
		}
		return repo.Recent(ctx, scope, id, cursor, size, runtime.store, func(b agent.Binding) error { _, e := runtime.Authorize(ctx, b); return e })
	}
	return confighttp.NewModule(h), nil
}
