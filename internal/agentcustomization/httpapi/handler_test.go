package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	d "task-processor/internal/agentcustomization"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"testing"
)

type capture struct {
	calls int
	cmd   d.Command
}

func (s *capture) Execute(_ context.Context, c d.Command) (d.Receipt, error) {
	s.calls++
	s.cmd = c
	return d.Receipt{RequestID: uuid.NewString(), Key: c.Key, Version: "1", Stage: d.Submitted}, nil
}
func (s *capture) List(context.Context, d.Scope, string) (d.Page, error) {
	s.calls++
	return d.Page{Items: []d.Request{}}, nil
}

type mustNotReadBody struct{}

func (mustNotReadBody) Read([]byte) (int, error) { panic("GET body must not be read") }
func (mustNotReadBody) Close() error             { return nil }
func TestGETRejectsDeclaredBodyWithoutReadingOrCallingOwner(t *testing.T) {
	s := &capture{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authidentity.WithAuthenticatedIdentity(c.Request.Context(), authidentity.AuthenticatedIdentity{UserID: "actor", EffectiveOrganizationID: "org"}))
	})
	for _, r := range Routes(&Handler{Service: s}) {
		router.Handle(r.Method, r.Path, r.Handler)
	}
	request := httptest.NewRequest("GET", Base, nil)
	request.Body = mustNotReadBody{}
	request.ContentLength = 1
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Zero(t, s.calls)
	require.Equal(t, "close", response.Header().Get("Connection"))
}
func (*capture) Read(context.Context, d.Scope, string, int64) (d.Detail, error) {
	return d.Detail{}, d.ErrNotFound
}
func (*capture) Download(context.Context, d.Scope, string, string) (d.Attachment, []byte, error) {
	return d.Attachment{}, nil, d.ErrNotFound
}
func TestDescriptorsKeepIndependentPlatformBoundary(t *testing.T) {
	for _, r := range Routes(&Handler{}) {
		require.True(t, r.RejectUnreadRequestBody == (r.Method == "GET"))
		if strings.Contains(r.Path, "/admin/") {
			require.Equal(t, httproute.AuthPolicyCurrentIdentityWithVerifiedRoles, r.AuthPolicy)
			require.Equal(t, httproute.OrganizationAccessPolicyNone, r.OrganizationAccessPolicy)
			require.Equal(t, authz.PermissionListingKitPlatformAdm, r.Permission)
		} else {
			require.Equal(t, httproute.AuthPolicyCurrentIdentity, r.AuthPolicy)
			require.Equal(t, httproute.OrganizationAccessPolicyLiveWrite, r.OrganizationAccessPolicy)
		}
		require.NoError(t, ValidateDescriptor(r))
	}
}
func TestPayloadCannotSupplyAuthorityAndDuplicateJSONIsRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &capture{}
	h := &Handler{Service: s}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authidentity.WithAuthenticatedIdentity(c.Request.Context(), authidentity.AuthenticatedIdentity{UserID: "actor", EffectiveOrganizationID: "org"}))
	})
	for _, r := range Routes(h) {
		router.Handle(r.Method, r.Path, r.Handler)
	}
	valid := `{"name":"需求","scenario":"商品","direction":"OTHER","description":"说明","contactName":"测试","contactMethod":"test","consent":true}`
	for _, body := range []string{strings.Replace(valid, `"consent":true`, `"consent":true,"platform":true`, 1), strings.Replace(valid, `"consent":true`, `"consent":true,"consent":false`, 1), valid + valid} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", Base, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", uuid.NewString())
		router.ServeHTTP(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Zero(t, s.calls)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", Base, strings.NewReader(valid))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", uuid.NewString())
	router.ServeHTTP(w, r)
	require.Equal(t, 200, w.Code)
	require.Equal(t, "org", s.cmd.Scope.OrganizationID)
	require.Equal(t, "actor", s.cmd.Scope.ActorID)
	require.False(t, s.cmd.Scope.Platform)
}
