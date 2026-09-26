package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	resourceadapter "task-processor/internal/integration/orgresource"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/organization/membership"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type pointLimitDirectory struct {
	member membership.Member
	calls  int
}

func TestCurrentMemberPointLimitRoutesPreserveLiveBoundaryAfterBillingRoutes(t *testing.T) {
	var routes []httproute.Descriptor
	for _, route := range currentWorkbenchApplicationRoutes {
		routes = append(routes, httproute.Descriptor{Method: route.Method, Path: route.Path})
	}
	for _, route := range currentCommercialBillingApplicationRoutes {
		routes = append(routes, httproute.Descriptor{Method: route.Method, Path: route.Path})
	}
	routes = append(routes, (memberPointLimitModule{}).routes()...)
	validate := func(r []httproute.Descriptor) error {
		return validateCurrentApplicationRoutesInternal(r, false, false, false, false, false, false, false, false, true)
	}
	require.NoError(t, validate(routes))
	for _, mode := range []string{"auth", "permission", "live", "target", "body", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			changed := append([]httproute.Descriptor(nil), routes...)
			value := &changed[len(changed)-1]
			switch mode {
			case "auth":
				value.AuthPolicy = httproute.AuthPolicyVerifiedIdentity
			case "permission":
				value.Permission = authz.PermissionWorkbenchOrganizationMemberRead
			case "live":
				value.OrganizationAccessPolicy = ""
			case "target":
				value.OrganizationTargetResolver = nil
			case "body":
				value.RejectUnreadRequestBody = false
			case "timeout":
				value.RequestTimeout = 0
			}
			require.Error(t, validate(changed))
		})
	}
}

func TestMemberPointLimitHTTPPersistsLimitWithoutCreatingBalance(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "points.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, resourceadapter.AutoMigrate(db))
	repository, err := resourceadapter.NewGormMemberLimitRepository(db, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	directory := &pointLimitDirectory{member: membership.Member{ID: "member-1", UserID: "user-1", DisplayName: "Member One", LoginName: "member@example.test", Roles: []string{"listingkit_operator"}, OrganizationID: "org-1", ProjectID: "project-1", State: "active"}}
	gate := memberPointLimitAuthorizer{authorizer: authorizer, directory: directory, projectID: "project-1"}
	service, err := orgresource.NewMemberLimitService(repository, gate)
	require.NoError(t, err)
	module := memberPointLimitModule{service: service, gate: gate}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authidentity.WithAuthenticatedIdentity(c.Request.Context(), authidentity.AuthenticatedIdentity{TenantID: "org-1", EffectiveOrganizationID: "org-1", EffectiveMemberID: "admin-grant", UserID: "admin-1", Roles: []string{"listingkit_admin"}}))
		c.Next()
	})
	for _, route := range module.routes() {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	request := func(method, path, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-Requested-Organization-ID", "org-1")
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	before := request(http.MethodGet, memberPointLimitBase, "", "")
	require.Equal(t, http.StatusOK, before.Code)
	require.Contains(t, before.Body.String(), `"configured":false`)
	require.Contains(t, before.Body.String(), `"displayName":"Member One"`)
	require.Contains(t, before.Body.String(), `"roles":["listingkit_operator"]`)
	set := request(http.MethodPut, memberPointLimitBase+"/member-1", `{"target":"25","expectedVersion":"0"}`, "set-1")
	require.Equal(t, http.StatusOK, set.Code, set.Body.String())
	require.Contains(t, set.Body.String(), `"monthlyLimit":"25"`)
	replay := request(http.MethodPut, memberPointLimitBase+"/member-1", `{"target":"25","expectedVersion":"0"}`, "set-1")
	require.Equal(t, http.StatusOK, replay.Code)
	require.Equal(t, set.Body.String(), replay.Body.String())
	conflict := request(http.MethodPut, memberPointLimitBase+"/member-1", `{"target":"26","expectedVersion":"0"}`, "set-1")
	require.Equal(t, http.StatusConflict, conflict.Code)
	read := request(http.MethodGet, memberPointLimitBase, "", "")
	require.Equal(t, http.StatusOK, read.Code)
	require.Contains(t, read.Body.String(), `"resourceType":"ai_point"`)
	require.Contains(t, read.Body.String(), `"timezone":"UTC"`)
	require.Contains(t, read.Body.String(), `"configured":true`)
	for _, body := range []string{`{"target":"26","expectedVersion":"1","month":"2030-01"}`, `{"target":26,"expectedVersion":"1"}`, `{"target":"-1","expectedVersion":"1"}`, `{"target":"9223372036854775808","expectedVersion":"1"}`, `{"target":"26","expectedVersion":"1"} {}`} {
		bad := request(http.MethodPut, memberPointLimitBase+"/member-1", body, "bad-set")
		require.Equal(t, http.StatusBadRequest, bad.Code, bad.Body.String())
	}
	wrongOrg := httptest.NewRequest(http.MethodPut, memberPointLimitBase+"/member-1", strings.NewReader(`{"target":"26","expectedVersion":"1"}`))
	wrongOrg.Header.Set("X-Requested-Organization-ID", "org-2")
	wrongOrg.Header.Set("Content-Type", "application/json")
	wrongOrg.Header.Set("Idempotency-Key", "wrong-org")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, wrongOrg)
	require.Equal(t, http.StatusForbidden, w.Code)
	var count int64
	require.NoError(t, db.Table("saas_organization_resource_operations").Count(&count).Error)
	require.EqualValues(t, 1, count, "replay/conflicting/malformed/other-org commands cannot add operations")
	require.NoError(t, db.Table("saas_organization_resource_buckets").Count(&count).Error)
	require.Zero(t, count, "monthly limit does not mint an enterprise balance")
}

func (d *pointLimitDirectory) Read(context.Context, string, string) (membership.Member, error) {
	d.calls++
	return d.member, nil
}
func (d *pointLimitDirectory) List(context.Context, string, membership.PageRequest) (membership.Page, error) {
	return membership.Page{Items: []membership.Member{d.member}, Total: 1}, nil
}

func TestMemberPointLimitUsesCurrentAdminAndExactActiveMember(t *testing.T) {
	for _, mode := range []string{"allowed", "operator", "other_org", "other_actor", "missing_canonical_actor", "target_inactive", "target_other_org", "target_other_id", "target_other_project"} {
		t.Run(mode, func(t *testing.T) {
			authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
			require.NoError(t, err)
			identity := authidentity.AuthenticatedIdentity{TenantID: "org-1", EffectiveOrganizationID: "org-1", EffectiveMemberID: "admin-grant", UserID: "admin-1", Roles: []string{"listingkit_admin"}}
			directory := &pointLimitDirectory{member: membership.Member{ID: "member-1", OrganizationID: "org-1", ProjectID: "project-1", UserID: "user-1", State: "active"}}
			switch mode {
			case "operator":
				identity.Roles = []string{"listingkit_operator"}
			case "other_org":
				identity.EffectiveOrganizationID = "org-2"
			case "other_actor":
				identity.UserID = "admin-2"
			case "missing_canonical_actor":
				identity.EffectiveMemberID = ""
			case "target_inactive":
				directory.member.State = "removed"
			case "target_other_org":
				directory.member.OrganizationID = "org-2"
			case "target_other_id":
				directory.member.ID = "member-2"
			case "target_other_project":
				directory.member.ProjectID = "project-2"
			}
			gate := memberPointLimitAuthorizer{authorizer: authorizer, directory: directory, projectID: "project-1"}
			err = gate.AuthorizeMemberLimitWrite(authidentity.WithAuthenticatedIdentity(context.Background(), identity), orgresource.Principal{ID: "admin-1", Kind: orgresource.PrincipalTenantHuman}, "org-1", "member-1")
			if mode == "allowed" {
				require.NoError(t, err)
				require.Equal(t, 1, directory.calls)
			} else {
				require.ErrorIs(t, err, orgresource.ErrForbidden)
			}
		})
	}
}
