package httpapi

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"time"
)

const ModuleName = "ecoservices"
const Base = "/api/v1/ecoservices"
const AdminBase = "/api/v1/admin/ecoservices"

type routeModule struct{ handler *Handler }

func NewModule(h *Handler) kernelmodule.Module { return routeModule{h} }
func (routeModule) Name() string               { return ModuleName }
func (m routeModule) Enabled(c *config.Config) bool {
	return m.handler != nil && c != nil && c.Workbench.Enabled
}
func (m routeModule) Register(registry *kernelmodule.Registry) error {
	registry.AddRoutes(Routes(m.handler)...)
	return nil
}
func Routes(h *Handler) []httproute.Descriptor {
	if h == nil {
		h = &Handler{}
	}
	read, join, manage, purchase := authz.PermissionWorkbenchEcoservicesRead, authz.PermissionWorkbenchEcoservicesJoin, authz.PermissionWorkbenchEcoservicesManage, authz.PermissionWorkbenchEcoservicesPurchase
	type spec struct {
		method, path, permission string
		handler                  gin.HandlerFunc
		catalog, platform, slow  bool
	}
	var result []httproute.Descriptor
	for _, r := range []spec{
		{http.MethodGet, Base + "/catalog", read, h.read("catalog", false), true, false, false},
		{http.MethodGet, Base + "/catalog/:id", read, h.read("catalog", false), true, false, false},
		{http.MethodGet, Base + "/applications", join, h.read("applications", false), false, false, false},
		{http.MethodPost, Base + "/applications", join, h.mutate("application_submit", false), false, false, false},
		{http.MethodPost, Base + "/applications/:id/agreement", join, h.mutate("agreement_accept", false), false, false, false},
		{http.MethodPost, Base + "/applications/:id/merchant", join, h.merchantSubmit, false, false, true},
		{http.MethodGet, Base + "/applications/:id/merchant", join, h.merchantRead, false, false, true},
		{http.MethodPost, Base + "/applications/:id/merchant/resume", join, h.merchantResume, false, false, true},
		{http.MethodPost, Base + "/applications/:id/files", join, h.upload("APPLICATION"), false, false, true},
		{http.MethodGet, Base + "/provider/listings", manage, h.read("provider_listings", false), false, false, false},
		{http.MethodPost, Base + "/provider/listings", manage, h.mutate("listing_create", false), false, false, false},
		{http.MethodPut, Base + "/provider/listings/:id", manage, h.mutate("listing_update", false), false, false, false},
		{http.MethodPost, Base + "/provider/listings/:id/publish", manage, h.mutate("listing_publish", false), false, false, false},
		{http.MethodGet, Base + "/requests", read, h.read("requests", false), false, false, false},
		{http.MethodGet, Base + "/requests/:id", read, h.read("requests", false), false, false, false},
		{http.MethodPost, Base + "/catalog/:id/requests", purchase, h.mutate("request_create", false), false, false, false},
		{http.MethodPost, Base + "/requests/:id/confirm-quote", purchase, h.mutate("confirm_quote", false), false, false, false},
		{http.MethodPost, Base + "/requests/:id/accept", purchase, h.mutate("accept", false), false, false, false},
		{http.MethodPost, Base + "/requests/:id/reject", purchase, h.mutate("reject", false), false, false, false},
		{http.MethodPost, Base + "/requests/:id/cancel", purchase, h.mutate("cancel", false), false, false, false},
		{http.MethodPost, Base + "/requests/:id/refund-proposals", purchase, h.mutate("refund_propose", false), false, false, false},
		{http.MethodPost, Base + "/requests/:id/refund-confirmation", purchase, h.mutate("refund_confirm", false), false, false, false},
		{http.MethodPost, Base + "/provider/requests/:id/quote", manage, h.mutate("quote", false), false, false, false},
		{http.MethodPost, Base + "/provider/requests/:id/start", manage, h.mutate("start", false), false, false, false},
		{http.MethodPost, Base + "/provider/requests/:id/delivery", manage, h.mutate("deliver", false), false, false, false},
		{http.MethodPost, Base + "/provider/requests/:id/refund-proposals", manage, h.mutate("refund_propose", false), false, false, false},
		{http.MethodPost, Base + "/provider/requests/:id/refund-confirmation", manage, h.mutate("refund_confirm", false), false, false, false},
		{http.MethodPost, Base + "/orders/:id/checkout", purchase, h.checkout, false, false, true},
		{http.MethodPost, Base + "/applications/files", join, h.upload("APPLICATION"), false, false, true},
		{http.MethodPost, Base + "/requests/files", purchase, h.upload("REQUEST"), false, false, true},
		{http.MethodPost, Base + "/requests/:id/files", purchase, h.upload("REQUEST"), false, false, true},
		{http.MethodPost, Base + "/provider/requests/:id/files", manage, h.upload("REQUEST"), false, false, true},
		{http.MethodGet, Base + "/files/:id", read, h.download(false, "REQUEST"), false, false, true},
		{http.MethodGet, Base + "/applications/files/:id", join, h.download(false, "APPLICATION"), false, false, true},
		{http.MethodGet, AdminBase + "/applications", authz.PermissionListingKitPlatformAdm, h.read("applications", true), false, true, false},
		{http.MethodPost, AdminBase + "/applications/:id/approve", authz.PermissionListingKitPlatformAdm, h.mutate("application_review", true), false, true, false},
		{http.MethodPost, AdminBase + "/applications/:id/reject", authz.PermissionListingKitPlatformAdm, h.mutate("application_reject", true), false, true, false},
		{http.MethodGet, AdminBase + "/requests", authz.PermissionListingKitPlatformAdm, h.read("requests", true), false, true, false},
		{http.MethodGet, AdminBase + "/requests/:id", authz.PermissionListingKitPlatformAdm, h.read("requests", true), false, true, false},
		{http.MethodGet, AdminBase + "/requests/:id/financial", authz.PermissionListingKitPlatformAdm, func(c *gin.Context) { h.financialFacts(c, false) }, false, true, false},
		{http.MethodPost, AdminBase + "/requests/:id/fees", authz.PermissionListingKitPlatformAdm, func(c *gin.Context) { h.financialFacts(c, true) }, false, true, true},
		{http.MethodGet, AdminBase + "/due-orders", authz.PermissionListingKitPlatformAdm, h.read("due_orders", true), false, true, false},
		{http.MethodPost, AdminBase + "/requests/:id/refund-approve", authz.PermissionListingKitPlatformAdm, h.mutate("refund_review", true), false, true, false},
		{http.MethodPost, AdminBase + "/requests/:id/refund-reject", authz.PermissionListingKitPlatformAdm, h.mutate("refund_review_reject", true), false, true, false},
		{http.MethodGet, AdminBase + "/files/:id", authz.PermissionListingKitPlatformAdm, h.download(true, ""), false, true, true},
	} {
		policy, org := httproute.AuthPolicyCurrentIdentity, httproute.OrganizationAccessPolicyLiveWrite
		if r.catalog {
			org = httproute.OrganizationAccessPolicyCachedRead
		}
		if r.platform {
			policy, org = httproute.AuthPolicyCurrentIdentityWithVerifiedRoles, httproute.OrganizationAccessPolicyNone
		}
		deadline := 10 * time.Second
		if r.slow {
			deadline = 30 * time.Second
		}
		result = append(result, httproute.Descriptor{Method: r.method, Path: r.path, Module: ModuleName, Permission: r.permission, AuthPolicy: policy, OrganizationAccessPolicy: org, RequestTimeout: deadline, RejectUnreadRequestBody: r.method == http.MethodGet, Handler: httproute.WithRequestBodyReadTimeout(deadline, r.handler)})
	}
	result = append(result, httproute.Descriptor{Method: http.MethodPost, Path: NotifyPath, Module: ModuleName, AuthPolicy: httproute.AuthPolicyPublic, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone, RequestTimeout: 10 * time.Second, Handler: httproute.WithRequestBodyReadTimeout(10*time.Second, h.notify)})
	return result
}
