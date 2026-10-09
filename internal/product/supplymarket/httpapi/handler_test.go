package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/product/supplymarket"
	"testing"
)

type fixtureService struct {
	Service
	calls int
	input supplymarket.Mutation
}

func (s *fixtureService) ExecuteMember(_ context.Context, _ string, i supplymarket.Mutation) (supplymarket.Receipt, error) {
	s.calls++
	s.input = i
	return supplymarket.Receipt{OperationID: uuid.NewString(), Revision: 1}, nil
}
func TestCommandsRejectAmbiguousKeysDuplicateFieldsAndMissingPreconditions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fixtureService{}
	router := gin.New()
	for _, r := range Routes(service, nil, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }) {
		router.Handle(r.Method, r.Path, r.Handler)
	}
	id, key := uuid.NewString(), uuid.NewString()
	for _, test := range []struct {
		body, key, match string
		extra            bool
		status           int
	}{
		{`{"action":"supplement","action":"approve"}`, key, "", false, 400},
		{`{"action":"submit_connection","organizationId":"org-b"}`, key, "", false, 400},
		{`{"action":"supplement","id":"` + id + `","expectedRevision":1,"note":"补充"}`, key, "", false, 400},
		{`{"action":"supplement","id":"` + id + `","expectedRevision":1,"note":"补充"}`, key, `"01"`, false, 400},
		{`{"action":"supplement","id":"` + id + `","expectedRevision":1,"note":"补充"}`, key, `"1"`, true, 400},
		{`{"action":"supplement","id":"` + id + `","expectedRevision":1,"note":"补充"}`, key, `"1"`, false, 200},
	} {
		request := httptest.NewRequest(http.MethodPost, BasePath+"/commands", strings.NewReader(test.body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", test.key)
		if test.match != "" {
			request.Header.Set("If-Match", test.match)
		}
		if test.extra {
			request.Header.Add("Idempotency-Key", uuid.NewString())
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, test.status, response.Code, response.Body.String())
	}
	require.Equal(t, 1, service.calls)
}
func TestPlatformRoutesUseGlobalIdentityAndNeverAnOrganizationGrant(t *testing.T) {
	for _, r := range Routes(nil, nil, nil) {
		if strings.HasPrefix(r.Path, AdminPath) {
			require.Equal(t, httproute.AuthPolicyCurrentIdentityWithVerifiedRoles, r.AuthPolicy)
			require.Equal(t, httproute.OrganizationAccessPolicyNone, r.OrganizationAccessPolicy)
			require.Equal(t, authz.PermissionListingKitPlatformAdm, r.Permission)
		}
	}
}
