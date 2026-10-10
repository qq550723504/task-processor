package authz

import (
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

const (
	PermissionWorkbenchEcoservicesRead          = "workbench.ecoservices.read"
	PermissionWorkbenchEcoservicesJoin          = "workbench.ecoservices.join"
	PermissionWorkbenchEcoservicesManage        = "workbench.ecoservices.manage"
	PermissionWorkbenchEcoservicesPurchase      = "workbench.ecoservices.purchase"
	PermissionWorkbenchKnowledgeRead            = "workbench.knowledge.read"
	PermissionWorkbenchKnowledgeManage          = "workbench.knowledge.manage"
	PermissionWorkbenchChatRead                 = "workbench.chat.read"
	PermissionWorkbenchChatUse                  = "workbench.chat.use"
	PermissionWorkbenchTaskRead                 = "workbench.task.read"
	PermissionWorkbenchCollectionRead           = "workbench.collection.read"
	PermissionWorkbenchCollectionManage         = "workbench.collection.manage"
	PermissionWorkbenchSupplyRead               = "workbench.supply.read"
	PermissionWorkbenchSupplyManage             = "workbench.supply.manage"
	PermissionWorkbenchListingSubmit            = "workbench.listing.submit"
	PermissionListingKitAdminRead               = "listingkit.admin.read"
	PermissionListingKitAdminWrite              = "listingkit.admin.write"
	PermissionListingKitPromptWrite             = "listingkit.prompt.write"
	PermissionListingKitPlatformAdm             = "listingkit.platform_admin"
	PermissionProductSourcingWrite              = "product_sourcing.write"
	PermissionLocalAgentWrite                   = "local_agent.write"
	PermissionWorkbenchAgentRead                = "workbench.agent.read"
	PermissionWorkbenchAgentUse                 = "workbench.agent.use"
	PermissionWorkbenchAgentConfigure           = "workbench.agent.configure"
	PermissionImageAgentRead                    = "listingkit.image_agent.read"
	PermissionImageAgentWrite                   = "listingkit.image_agent.write"
	PermissionWorkbenchStoreRead                = "workbench.store.read"
	PermissionWorkbenchStoreProductsRead        = "workbench.store.products.read"
	PermissionWorkbenchStoreProductsSync        = "workbench.store.products.sync"
	PermissionWorkbenchStoreOrdersRead          = "workbench.store.orders.read"
	PermissionWorkbenchStoreOrdersSync          = "workbench.store.orders.sync"
	PermissionWorkbenchStoreCreate              = "workbench.store.create"
	PermissionWorkbenchStoreUpdate              = "workbench.store.update"
	PermissionWorkbenchStoreLifecycle           = "workbench.store.lifecycle"
	PermissionWorkbenchStoreDelete              = "workbench.store.delete"
	PermissionWorkbenchSourceAccountRead        = "workbench.source_account.read"
	PermissionWorkbenchSourceAccountManage      = "workbench.source_account.manage"
	PermissionWorkbenchOrganizationMemberRead   = "workbench.organization_member.read"
	PermissionWorkbenchOrganizationMemberManage = "workbench.organization_member.manage"
	PermissionWorkbenchCommercialRead           = "workbench.commercial.read"
	PermissionWorkbenchCommercialPurchase       = "workbench.commercial.purchase"
	PermissionWorkbenchCommercialWalletTopUp    = "workbench.commercial.wallet_topup"
)

var workbenchStorePermissions = []string{
	PermissionWorkbenchStoreRead,
	PermissionWorkbenchStoreCreate,
	PermissionWorkbenchStoreUpdate,
	PermissionWorkbenchStoreLifecycle,
	PermissionWorkbenchStoreDelete,
}

var workbenchStoreObservationPermissions = []string{PermissionWorkbenchStoreProductsRead, PermissionWorkbenchStoreProductsSync, PermissionWorkbenchStoreOrdersRead, PermissionWorkbenchStoreOrdersSync}

var workbenchKnowledgePermissions = []string{PermissionWorkbenchKnowledgeRead, PermissionWorkbenchKnowledgeManage}

var workbenchEcoservicesPermissions = []string{PermissionWorkbenchEcoservicesRead, PermissionWorkbenchEcoservicesJoin, PermissionWorkbenchEcoservicesManage, PermissionWorkbenchEcoservicesPurchase}

var workbenchChatPermissions = []string{PermissionWorkbenchChatRead, PermissionWorkbenchChatUse, PermissionWorkbenchTaskRead}

var workbenchToolPermissions = []string{PermissionWorkbenchToolsRead, PermissionWorkbenchToolsManage, PermissionWorkbenchToolsCustomize}

var workbenchSupplyPermissions = []string{PermissionWorkbenchCollectionRead, PermissionWorkbenchCollectionManage, PermissionWorkbenchSupplyRead, PermissionWorkbenchSupplyManage, PermissionWorkbenchListingSubmit}

var workbenchSourceAccountPermissions = []string{
	PermissionWorkbenchSourceAccountRead,
	PermissionWorkbenchSourceAccountManage,
}

var workbenchOrganizationMemberPermissions = []string{
	PermissionWorkbenchOrganizationMemberRead,
	PermissionWorkbenchOrganizationMemberManage,
}

var workbenchCommercialPermissions = []string{
	PermissionWorkbenchCommercialRead,
	PermissionWorkbenchCommercialPurchase,
	PermissionWorkbenchCommercialWalletTopUp,
}

// WorkbenchPermissions is a bounded display contract, never a policy source.
func WorkbenchPermissions() []string {
	result := []string{PermissionListingKitAdminRead, PermissionListingKitAdminWrite, PermissionProductSourcingWrite, PermissionLocalAgentWrite, PermissionImageAgentRead, PermissionImageAgentWrite, PermissionWorkbenchAgentRead, PermissionWorkbenchAgentUse, PermissionWorkbenchAgentConfigure}
	for _, group := range [][]string{[]string{"workbench.project.read", "workbench.project.manage"}, workbenchToolPermissions, workbenchChatPermissions, workbenchKnowledgePermissions, workbenchEcoservicesPermissions, workbenchSupplyPermissions, workbenchStorePermissions, workbenchStoreObservationPermissions, workbenchSourceAccountPermissions, workbenchOrganizationMemberPermissions, workbenchCommercialPermissions} {
		result = append(result, group...)
	}
	return result
}

const listingKitModel = `
[request_definition]
r = sub, obj

[policy_definition]
p = sub, obj

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (r.sub == p.sub || g(r.sub, p.sub)) && r.obj == p.obj
`

type ListingKitAuthorizer struct {
	enforcer         *casbin.Enforcer
	rolePolicyReader RolePolicyReader
	moduleEnforcer   *casbin.Enforcer
	protectedRoles   map[string]bool
}

var (
	defaultListingKitAuthorizerOnce sync.Once
	defaultListingKitAuthorizer     *ListingKitAuthorizer
)

func NewListingKitAuthorizer(platformAdminUsers []string, platformAdminRoles []string) (*ListingKitAuthorizer, error) {
	m, err := model.NewModelFromString(listingKitModel)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}

	for _, policy := range [][]string{
		{"listingkit_viewer", PermissionWorkbenchChatRead},
		{"listingkit_viewer", PermissionWorkbenchEcoservicesRead},
		{"listingkit_operator", PermissionWorkbenchEcoservicesRead},
		{"listingkit_operator", PermissionWorkbenchEcoservicesManage},
		{"listingkit_admin", PermissionWorkbenchEcoservicesRead},
		{"listingkit_admin", PermissionWorkbenchEcoservicesJoin},
		{"listingkit_admin", PermissionWorkbenchEcoservicesManage},
		{"listingkit_admin", PermissionWorkbenchEcoservicesPurchase},
		{"platform_admin", PermissionWorkbenchEcoservicesRead},
		{"platform_admin", PermissionWorkbenchEcoservicesJoin},
		{"platform_admin", PermissionWorkbenchEcoservicesManage},
		{"platform_admin", PermissionWorkbenchEcoservicesPurchase},
		{"listingkit_viewer", PermissionWorkbenchTaskRead},
		{"listingkit_operator", PermissionWorkbenchChatRead},
		{"listingkit_operator", PermissionWorkbenchChatUse},
		{"listingkit_operator", PermissionWorkbenchTaskRead},
		{"listingkit_admin", PermissionWorkbenchChatRead},
		{"listingkit_admin", PermissionWorkbenchChatUse},
		{"listingkit_admin", PermissionWorkbenchTaskRead},
		{"platform_admin", PermissionWorkbenchChatRead},
		{"platform_admin", PermissionWorkbenchChatUse},
		{"platform_admin", PermissionWorkbenchTaskRead},
		{"listingkit_viewer", PermissionWorkbenchAgentRead},
		{"listingkit_operator", PermissionWorkbenchAgentRead},
		{"listingkit_operator", PermissionWorkbenchAgentUse},
		{"listingkit_admin", PermissionWorkbenchAgentRead},
		{"listingkit_admin", PermissionWorkbenchAgentUse},
		{"listingkit_admin", PermissionWorkbenchAgentConfigure},
		{"platform_admin", PermissionWorkbenchAgentRead},
		{"platform_admin", PermissionWorkbenchAgentUse},
		{"platform_admin", PermissionWorkbenchAgentConfigure},
		{"listingkit_operator", PermissionWorkbenchKnowledgeRead},
		{"listingkit_admin", PermissionWorkbenchKnowledgeRead},
		{"listingkit_admin", PermissionWorkbenchKnowledgeManage},
		{"platform_admin", PermissionWorkbenchKnowledgeRead},
		{"platform_admin", PermissionWorkbenchKnowledgeManage},
		{"listingkit_viewer", PermissionWorkbenchOrganizationMemberRead},
		{"listingkit_operator", PermissionWorkbenchOrganizationMemberRead},
		{"listingkit_admin", PermissionWorkbenchOrganizationMemberRead},
		{"listingkit_admin", PermissionWorkbenchOrganizationMemberManage},
		{"platform_admin", PermissionWorkbenchOrganizationMemberRead},
		{"platform_admin", PermissionWorkbenchOrganizationMemberManage},
		{"listingkit_viewer", PermissionWorkbenchStoreRead},
		{"listingkit_viewer", PermissionWorkbenchSourceAccountRead},
		{"listingkit_operator", PermissionListingKitAdminRead},
		{"listingkit_operator", PermissionListingKitAdminWrite},
		{"listingkit_operator", PermissionProductSourcingWrite},
		{"listingkit_operator", PermissionLocalAgentWrite},
		{"listingkit_operator", PermissionImageAgentRead},
		{"listingkit_operator", PermissionImageAgentWrite},
		{"listingkit_operator", PermissionWorkbenchStoreRead},
		{"listingkit_operator", PermissionWorkbenchStoreCreate},
		{"listingkit_operator", PermissionWorkbenchStoreUpdate},
		{"listingkit_operator", PermissionWorkbenchStoreLifecycle},
		{"listingkit_operator", PermissionWorkbenchSourceAccountRead},
		{"listingkit_operator", PermissionWorkbenchSourceAccountManage},
		{"listingkit_operator", PermissionWorkbenchCommercialRead},
		{"listingkit_admin", PermissionListingKitAdminRead},
		{"listingkit_admin", PermissionListingKitAdminWrite},
		{"listingkit_admin", PermissionListingKitPromptWrite},
		{"listingkit_admin", PermissionProductSourcingWrite},
		{"listingkit_admin", PermissionLocalAgentWrite},
		{"listingkit_admin", PermissionImageAgentRead},
		{"listingkit_admin", PermissionImageAgentWrite},
		{"listingkit_admin", PermissionWorkbenchStoreRead},
		{"listingkit_admin", PermissionWorkbenchStoreCreate},
		{"listingkit_admin", PermissionWorkbenchStoreUpdate},
		{"listingkit_admin", PermissionWorkbenchStoreLifecycle},
		{"listingkit_admin", PermissionWorkbenchStoreDelete},
		{"listingkit_admin", PermissionWorkbenchSourceAccountRead},
		{"listingkit_admin", PermissionWorkbenchSourceAccountManage},
		{"listingkit_admin", PermissionWorkbenchCommercialRead},
		{"listingkit_admin", PermissionWorkbenchCommercialPurchase},
		{"listingkit_admin", PermissionWorkbenchCommercialWalletTopUp},
		{"platform_admin", PermissionListingKitAdminRead},
		{"platform_admin", PermissionListingKitAdminWrite},
		{"platform_admin", PermissionListingKitPromptWrite},
		{"platform_admin", PermissionListingKitPlatformAdm},
		{"platform_admin", PermissionProductSourcingWrite},
		{"platform_admin", PermissionLocalAgentWrite},
		{"platform_admin", PermissionImageAgentRead},
		{"platform_admin", PermissionImageAgentWrite},
		{"platform_admin", PermissionWorkbenchStoreRead},
		{"platform_admin", PermissionWorkbenchStoreCreate},
		{"platform_admin", PermissionWorkbenchStoreUpdate},
		{"platform_admin", PermissionWorkbenchStoreLifecycle},
		{"platform_admin", PermissionWorkbenchStoreDelete},
		{"platform_admin", PermissionWorkbenchSourceAccountRead},
		{"platform_admin", PermissionWorkbenchSourceAccountManage},
		{"platform_admin", PermissionWorkbenchCommercialRead},
		{"platform_admin", PermissionWorkbenchCommercialPurchase},
		{"platform_admin", PermissionWorkbenchCommercialWalletTopUp},
		{"admin", PermissionListingKitAdminRead},
		{"admin", PermissionListingKitPromptWrite},
		{"admin", PermissionLocalAgentWrite},
		{"admin", PermissionImageAgentRead},
		{"admin", PermissionImageAgentWrite},
	} {
		if _, err := enforcer.AddPolicy(policy); err != nil {
			return nil, err
		}
	}

	for _, role := range append([]string{"listingkit_viewer", "listingkit_operator", "listingkit_admin", "platform_admin"}, normalizeUnique(platformAdminRoles)...) {
		if _, e := enforcer.AddPolicy(role, "workbench.project.read"); e != nil {
			return nil, e
		}
		if role != "listingkit_viewer" {
			if _, e := enforcer.AddPolicy(role, "workbench.project.manage"); e != nil {
				return nil, e
			}
		}
	}
	for _, userID := range normalizeUnique(platformAdminUsers) {
		for _, permission := range []string{"workbench.project.read", "workbench.project.manage"} {
			if _, err := enforcer.AddPolicy(userSubject(userID), permission); err != nil {
				return nil, err
			}
		}
	}
	for _, policy := range ToolMarketPolicies() {
		if _, err := enforcer.AddPolicy(policy); err != nil {
			return nil, err
		}
	}
	for _, role := range append([]string{"listingkit_operator", "listingkit_admin", "platform_admin"}, normalizeUnique(platformAdminRoles)...) {
		for _, permission := range workbenchStoreObservationPermissions {
			if _, err := enforcer.AddPolicy(role, permission); err != nil {
				return nil, err
			}
		}
		for _, permission := range workbenchSupplyPermissions {
			if _, err := enforcer.AddPolicy(role, permission); err != nil {
				return nil, err
			}
		}
	}
	for _, permission := range []string{PermissionWorkbenchCollectionRead, PermissionWorkbenchSupplyRead, PermissionWorkbenchStoreProductsRead, PermissionWorkbenchStoreOrdersRead} {
		if _, err := enforcer.AddPolicy("listingkit_viewer", permission); err != nil {
			return nil, err
		}
	}
	for _, userID := range normalizeUnique(platformAdminUsers) {
		for _, permission := range workbenchSupplyPermissions {
			if _, err := enforcer.AddPolicy(userSubject(userID), permission); err != nil {
				return nil, err
			}
		}
		for _, permission := range workbenchStoreObservationPermissions {
			if _, err := enforcer.AddPolicy(userSubject(userID), permission); err != nil {
				return nil, err
			}
		}
	}
	for _, role := range normalizeUnique(platformAdminRoles) {
		for _, permission := range workbenchChatPermissions {
			if _, err := enforcer.AddPolicy(role, permission); err != nil {
				return nil, err
			}
		}
		// A configured platform-admin role with Chat use must pass the same
		// existing Product Agent gates as the built-in platform_admin role.
		for _, permission := range []string{PermissionWorkbenchAgentUse, PermissionListingKitAdminWrite} {
			if _, err := enforcer.AddPolicy(role, permission); err != nil {
				return nil, err
			}
		}
		for _, permission := range append(append([]string{}, workbenchKnowledgePermissions...), workbenchEcoservicesPermissions...) {
			if _, err := enforcer.AddPolicy(role, permission); err != nil {
				return nil, err
			}
		}
		for _, permission := range workbenchOrganizationMemberPermissions {
			if _, err := enforcer.AddPolicy(role, permission); err != nil {
				return nil, err
			}
		}
		if _, err := enforcer.AddPolicy(role, PermissionListingKitPlatformAdm); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(role, PermissionListingKitAdminRead); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(role, PermissionListingKitPromptWrite); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(role, PermissionProductSourcingWrite); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(role, PermissionLocalAgentWrite); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(role, PermissionImageAgentRead); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(role, PermissionImageAgentWrite); err != nil {
			return nil, err
		}
		for _, permission := range workbenchStorePermissions {
			if _, err := enforcer.AddPolicy(role, permission); err != nil {
				return nil, err
			}
		}
		for _, permission := range workbenchSourceAccountPermissions {
			if _, err := enforcer.AddPolicy(role, permission); err != nil {
				return nil, err
			}
		}
		for _, permission := range workbenchCommercialPermissions {
			if _, err := enforcer.AddPolicy(role, permission); err != nil {
				return nil, err
			}
		}
	}
	for _, userID := range normalizeUnique(platformAdminUsers) {
		subject := userSubject(userID)
		for _, permission := range workbenchChatPermissions {
			if _, err := enforcer.AddPolicy(subject, permission); err != nil {
				return nil, err
			}
		}
		for _, permission := range []string{PermissionWorkbenchAgentUse, PermissionListingKitAdminWrite} {
			if _, err := enforcer.AddPolicy(subject, permission); err != nil {
				return nil, err
			}
		}
		for _, permission := range append(append([]string{}, workbenchKnowledgePermissions...), workbenchEcoservicesPermissions...) {
			if _, err := enforcer.AddPolicy(subject, permission); err != nil {
				return nil, err
			}
		}
		for _, permission := range workbenchOrganizationMemberPermissions {
			if _, err := enforcer.AddPolicy(subject, permission); err != nil {
				return nil, err
			}
		}
		if _, err := enforcer.AddPolicy(subject, PermissionListingKitPlatformAdm); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(subject, PermissionListingKitAdminRead); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(subject, PermissionListingKitPromptWrite); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(subject, PermissionProductSourcingWrite); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(subject, PermissionLocalAgentWrite); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(subject, PermissionImageAgentRead); err != nil {
			return nil, err
		}
		if _, err := enforcer.AddPolicy(subject, PermissionImageAgentWrite); err != nil {
			return nil, err
		}
		for _, permission := range workbenchStorePermissions {
			if _, err := enforcer.AddPolicy(subject, permission); err != nil {
				return nil, err
			}
		}
		for _, permission := range workbenchSourceAccountPermissions {
			if _, err := enforcer.AddPolicy(subject, permission); err != nil {
				return nil, err
			}
		}
		for _, permission := range workbenchCommercialPermissions {
			if _, err := enforcer.AddPolicy(subject, permission); err != nil {
				return nil, err
			}
		}
	}

	moduleEnforcer, err := newModuleEnforcer()
	if err != nil {
		return nil, err
	}
	protected := map[string]bool{}
	for _, role := range normalizeUnique(platformAdminRoles) {
		protected[role] = true
	}
	return &ListingKitAuthorizer{enforcer: enforcer, moduleEnforcer: moduleEnforcer, protectedRoles: protected}, nil
}

func (a *ListingKitAuthorizer) Authorize(userID string, roles []string, permission string) bool {
	if a == nil || a.enforcer == nil {
		return false
	}
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return true
	}

	if subject := userSubject(userID); subject != "" {
		ok, err := a.enforcer.Enforce(subject, permission)
		if err == nil && ok {
			return true
		}
	}

	for _, role := range normalizeUnique(roles) {
		ok, err := a.enforcer.Enforce(role, permission)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func DefaultListingKitAuthorizer() *ListingKitAuthorizer {
	defaultListingKitAuthorizerOnce.Do(func() {
		defaultListingKitAuthorizer, _ = NewListingKitAuthorizer(nil, nil)
	})
	return defaultListingKitAuthorizer
}

func IsListingKitPlatformAdmin(userID string, roles []string) bool {
	return DefaultListingKitAuthorizer().Authorize(userID, roles, PermissionListingKitPlatformAdm)
}

// IsTenantAdmin reports whether the identity may bypass owner scope inside its
// already authenticated tenant. It intentionally uses this authorizer's
// configured users and roles instead of the process-wide default instance.
func (a *ListingKitAuthorizer) IsTenantAdmin(userID string, roles []string) bool {
	if a.Authorize(userID, roles, PermissionListingKitPlatformAdm) {
		return true
	}
	for _, role := range normalizeUnique(roles) {
		if role == "listingkit_admin" || role == "admin" {
			return true
		}
	}
	return false
}

// IsListingKitTenantAdmin reports whether the identity may manage all data in
// its current tenant. Platform administrators are included because they also
// have tenant-wide access, while listingkit_operator remains owner-scoped.
func IsListingKitTenantAdmin(userID string, roles []string) bool {
	return DefaultListingKitAuthorizer().IsTenantAdmin(userID, roles)
}

func userSubject(userID string) string {
	if value := strings.TrimSpace(userID); value != "" {
		return "user:" + value
	}
	return ""
}

func normalizeUnique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, item := range values {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
