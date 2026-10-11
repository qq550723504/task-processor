package httpapi

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/aiworkbench"
	pc "task-processor/internal/aiworkbench/projectcenter"
	ph "task-processor/internal/aiworkbench/projectcenter/httpapi"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	ps "task-processor/internal/integration/persistence/aiworkbench/projectcenter"
	kernel "task-processor/internal/kernel/module"
	"task-processor/internal/knowledge"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
	"time"
)

func WithProjectCenter(db *gorm.DB) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.projectCenters++; o.projectCenterDB = db }
}

type projectModule struct{ handler *ph.Handler }

func (projectModule) Name() string                { return ph.ModuleName }
func (projectModule) Enabled(*config.Config) bool { return true }
func (m projectModule) Register(r *kernel.Registry) error {
	r.AddRoutes(ph.Routes(m.handler)...)
	return nil
}

// Composition only: the Project domain never acquires source owner databases.
type projectSources struct {
	resolver  organizationIdentityResolver
	policy    *authz.ListingKitAuthorizer
	chat      *aiWorkbenchApplication
	knowledge *knowledge.Service
	stores    *storecenter.MemberScopedStoreRepository
	products  sourcing.PublishedAcquisitionReader
}

func (a *projectSources) fresh(ctx context.Context, s pc.Scope) (context.Context, authidentity.AuthenticatedIdentity, error) {
	cap, ok := ctx.Value(productReviewCapabilityContextKey{}).(productReviewRequestCapability)
	if !ok || cap.actorID != s.ActorID || cap.effectiveOrganizationID != s.OrganizationID || !cap.tokenExpiresAt.After(time.Now()) {
		return nil, authidentity.AuthenticatedIdentity{}, pc.ErrForbidden
	}
	id, e := a.resolver.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: authidentity.AuthenticatedIdentity{UserID: cap.actorID, HomeOrganizationID: cap.homeOrganizationID, TokenExpiresAt: cap.tokenExpiresAt}, BearerToken: cap.bearerToken, RequestedOrganizationID: s.OrganizationID})
	original, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx)
	if e != nil || !authenticated || id.UserID != s.ActorID || id.TenantID != s.OrganizationID || id.EffectiveOrganizationID != s.OrganizationID || !id.TokenExpiresAt.Equal(cap.tokenExpiresAt) || id.EffectiveMemberID != original.EffectiveMemberID {
		return nil, authidentity.AuthenticatedIdentity{}, pc.ErrForbidden
	}
	return authidentity.WithAuthenticatedIdentity(ctx, id), id, nil
}
func (a *projectSources) authorize(ctx context.Context, s pc.Scope, manage bool) error {
	ctx, id, e := a.fresh(ctx, s)
	if e != nil {
		return e
	}
	permission := pc.PermissionRead
	if manage {
		permission = pc.PermissionManage
	}
	if !authz.AllowedOrganization(ctx, a.policy, id.UserID, id.TenantID, id.Roles, permission) {
		return pc.ErrForbidden
	}
	return nil
}
func (a *projectSources) Resolve(ctx context.Context, s pc.Scope, r pc.Reference) (pc.ReferenceView, error) {
	ctx, id, e := a.fresh(ctx, s)
	if e != nil {
		return pc.ReferenceView{}, e
	}
	allowed := func(p string) bool {
		return authz.AllowedOrganization(ctx, a.policy, id.UserID, id.TenantID, id.Roles, p)
	}
	v := pc.ReferenceView{SlotID: r.SlotID, Kind: r.Kind, TargetID: r.TargetID, Available: true}
	switch r.Kind {
	case "STORE":
		if a.stores == nil || !allowed(authz.PermissionWorkbenchStoreRead) {
			return pc.ReferenceView{}, pc.ErrNotFound
		}
		item, e := a.stores.Get(ctx, s.OrganizationID, r.TargetID)
		if e != nil {
			return pc.ReferenceView{}, e
		}
		v.Title = item.Name()
		v.Href = "/workbench/stores/" + r.TargetID
	case "CONVERSATION":
		if a.chat == nil || !a.chat.admittedOrganization(s.OrganizationID) || !allowed(authz.PermissionWorkbenchChatRead) {
			return pc.ReferenceView{}, pc.ErrNotFound
		}
		item, e := a.chat.store.Get(ctx, aiworkbench.Scope{OrganizationID: s.OrganizationID, ActorID: s.ActorID}, r.TargetID)
		if e != nil {
			return pc.ReferenceView{}, e
		}
		v.Title = item.Title
		if v.Title == "" {
			v.Title = "未命名会话"
		}
		v.Href = "/workbench/ai/chat/" + r.TargetID
	case "BUSINESS_TASK":
		if a.chat == nil || !a.chat.admittedOrganization(s.OrganizationID) || !allowed(authz.PermissionWorkbenchTaskRead) {
			return pc.ReferenceView{}, pc.ErrNotFound
		}
		scope := aiworkbench.Scope{OrganizationID: s.OrganizationID, ActorID: s.ActorID}
		item, e := a.chat.store.GetTask(ctx, scope, r.TargetID)
		if e != nil {
			return pc.ReferenceView{}, e
		}
		projection, e := a.chat.taskView(ctx, scope, item, false)
		if e != nil {
			return pc.ReferenceView{}, e
		}
		v.Title = item.Title
		v.Href = "/workbench/ai/tasks/" + r.TargetID
		if projection.ProjectionAvailable {
			v.TaskState = string(projection.State)
		}
		if projection.ReviewID != "" && projection.ProductDetailsAvailable {
			v.ResultHref = v.Href
		}
	case "KNOWLEDGE_BASE", "KNOWLEDGE_SOURCE":
		if a.knowledge == nil || !allowed(authz.PermissionWorkbenchKnowledgeRead) {
			return pc.ReferenceView{}, pc.ErrNotFound
		}
		scope := knowledge.Scope{OrganizationID: s.OrganizationID, ActorID: s.ActorID}
		if r.Kind == "KNOWLEDGE_BASE" {
			item, e := a.knowledge.GetBase(ctx, scope, r.TargetID)
			if e != nil || item.State != knowledge.Active {
				return pc.ReferenceView{}, pc.ErrNotFound
			}
			v.Title = item.Name
			v.Href = "/workbench/ai/knowledge/" + item.ID
		} else {
			item, e := a.knowledge.GetSource(ctx, scope, r.TargetID)
			if e != nil || item.State != knowledge.Active {
				return pc.ReferenceView{}, pc.ErrNotFound
			}
			base, e := a.knowledge.GetBase(ctx, scope, item.BaseID)
			if e != nil || base.State != knowledge.Active {
				return pc.ReferenceView{}, pc.ErrNotFound
			}
			v.Title = item.Name
			v.Href = "/workbench/ai/knowledge/" + item.BaseID
		}
	case "PRODUCT":
		if a.products == nil || !allowed(authz.PermissionProductSourcingWrite) {
			return pc.ReferenceView{}, pc.ErrNotFound
		}
		item, e := a.products.ReadPublished(ctx, r.TargetID)
		if e != nil {
			return pc.ReferenceView{}, e
		}
		v.Title = item.Snapshot.Snapshot.Title
		v.Href = "/workbench/supply/acquisition/operation/" + r.TargetID
	default:
		return pc.ReferenceView{}, pc.ErrInvalid
	}
	return v, nil
}
func buildProjectCenter(ctx context.Context, db *gorm.DB, deps routeAuthDependencies, policy *authz.ListingKitAuthorizer, chat *aiWorkbenchApplication, k *knowledge.Service, storeDB *gorm.DB, products sourcing.PublishedAcquisitionReader) (kernel.Module, error) {
	if deps.organizationResolver == nil || policy == nil {
		return nil, pc.ErrUnavailable
	}
	if e := ps.VerifySchema(ctx, db); e != nil {
		return nil, e
	}
	store, e := ps.New(db)
	if e != nil {
		return nil, e
	}
	sources := &projectSources{resolver: deps.organizationResolver, policy: policy, chat: chat, knowledge: k, products: products}
	if storeDB != nil {
		sources.stores, e = storecenter.NewMemberScopedStoreRepository(storeDB, currentStoreMemberAuthorizer{authorizer: policy})
		if e != nil {
			return nil, e
		}
	}
	service := &pc.Service{Store: store, Reader: sources, Authorize: sources.authorize}
	h := &ph.Handler{Service: service, Bind: func(ctx context.Context, header string) (context.Context, pc.Scope, error) {
		ctx, e := (productReviewCapabilityBinder{}).Bind(ctx, header)
		if e != nil {
			return nil, pc.Scope{}, pc.ErrForbidden
		}
		id, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
		return ctx, pc.Scope{OrganizationID: id.TenantID, ActorID: id.UserID}, nil
	}}
	return projectModule{h}, nil
}
