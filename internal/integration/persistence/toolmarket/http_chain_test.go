package toolmarketpersistence

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"task-processor/internal/authz"
	tm "task-processor/internal/toolmarket"
	api "task-processor/internal/toolmarket/httpapi"
	"testing"
)

// Controlled trusted identity exercises the real HTTP decoder and PostgreSQL
// consumer chain. Live ZITADEL/current-installation acceptance is separate.
func TestEnterpriseDemandAndSpecialistProgressHTTPChain(t *testing.T) {
	_, store := fixture(t)
	gin.SetMode(gin.TestMode)
	current := tm.Scope{OrganizationID: "org-a", ActorID: "admin-a"}
	specialist := false
	h := &api.Handler{Repository: store, Readiness: tm.Readiness{LocalCapture: true}, Authorize: func(_ context.Context, p string, platform bool) (tm.Scope, error) {
		if platform {
			if !specialist || p != authz.PermissionListingKitPlatformAdm {
				return tm.Scope{}, tm.ErrForbidden
			}
			return tm.Scope{ActorID: "specialist"}, nil
		}
		return current, nil
	}}
	h.ReadAuthorize = h.Authorize // Controlled HTTP fixture, not the runtime adapter.
	router := gin.New()
	for _, r := range api.Routes(h) {
		router.Handle(r.Method, r.Path, r.Handler)
	}
	send := func(method, path, body, revision string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if method != "GET" {
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Idempotency-Key", uuid.NewString())
			if revision == "" {
				r.Header.Set("If-None-Match", "*")
			} else {
				r.Header.Set("If-Match", `"`+revision+`"`)
			}
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	enabled := send("PUT", api.Base+"/activations/"+tm.AcquisitionID, `{"enabled":true}`, "")
	require.Equal(t, 200, enabled.Code)
	mine := send("GET", api.Base+"/mine", "", "")
	require.Equal(t, 200, mine.Code)
	require.Contains(t, mine.Body.String(), `"enabled":true`)
	create := send("POST", api.Base+"/requests", `{"kind":"DATA","title":"商品资料需求","description":"保存已授权商品资料"}`, "")
	require.Equal(t, 200, create.Code)
	var receipt tm.Receipt
	require.NoError(t, json.Unmarshal(create.Body.Bytes(), &receipt))
	denied := send("GET", api.AdminBase+"/requests/"+receipt.ID, "", "")
	require.Equal(t, 403, denied.Code)
	specialist = true
	progress := send("POST", api.AdminBase+"/requests/"+receipt.ID+"/progress", `{"stage":"EVALUATING","note":"专员正在核实采集范围"}`, "1")
	require.Equal(t, 200, progress.Code)
	detail := send("GET", api.Base+"/requests/"+receipt.ID, "", "")
	require.Equal(t, 200, detail.Code)
	require.Contains(t, detail.Body.String(), "专员正在核实采集范围")
	current.OrganizationID = "org-b"
	detail = send("GET", api.Base+"/requests/"+receipt.ID, "", "")
	require.Equal(t, 404, detail.Code)
}
func TestCommittedActivationReplaysWhenCapabilityGoesOffline(t *testing.T) {
	_, store := fixture(t)
	gin.SetMode(gin.TestMode)
	h := &api.Handler{Repository: store, Readiness: tm.Readiness{LocalCapture: true}, Authorize: func(context.Context, string, bool) (tm.Scope, error) {
		return tm.Scope{ActorID: "admin-a", OrganizationID: "org-a"}, nil
	}}
	h.ReadAuthorize = h.Authorize // Controlled HTTP fixture, not the runtime adapter.
	router := gin.New()
	for _, r := range api.Routes(h) {
		router.Handle(r.Method, r.Path, r.Handler)
	}
	send := func(key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PUT", api.Base+"/activations/"+tm.AcquisitionID, strings.NewReader(`{"enabled":true}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", key)
		r.Header.Set("If-None-Match", "*")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	key := uuid.NewString()
	first := send(key)
	require.Equal(t, 200, first.Code)
	h.Readiness = tm.Readiness{}
	replay := send(key)
	require.Equal(t, 200, replay.Code)
	require.Equal(t, first.Body.String(), replay.Body.String())
	require.Equal(t, 503, send(uuid.NewString()).Code)
}
