package toolmarketpersistence

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strconv"
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

func TestLargeProgressHistoryRemainsReadableAndFullyPageable(t *testing.T) {
	_, store := fixture(t)
	ctx := context.Background()
	cmd := create()
	cmd.Scope.OrganizationID = strings.Repeat("a", 128)
	cmd.Demand.Kind = "AUTOMATION"
	cmd.Demand.Title = strings.Repeat("<", 120)
	cmd.Demand.Description = strings.Repeat("<", 4000)
	created, err := store.Execute(ctx, cmd, allow)
	require.NoError(t, err)
	for revision := int64(1); revision < 100; revision++ {
		_, err = store.Execute(ctx, tm.Command{
			Scope: tm.Scope{ActorID: "specialist"}, Platform: true, Key: uuid.NewString(),
			Operation: "progress", ID: created.ID, Expected: revision,
			Progress: tm.Progress{Stage: "SUBMITTED", Note: strings.Repeat("<", 2000)},
		}, allow)
		require.NoError(t, err)
	}
	gin.SetMode(gin.TestMode)
	authorize := func(_ context.Context, _ string, platform bool) (tm.Scope, error) {
		if platform {
			return tm.Scope{ActorID: "specialist"}, nil
		}
		return cmd.Scope, nil
	}
	h := &api.Handler{Repository: store, Authorize: authorize, ReadAuthorize: authorize}
	router := gin.New()
	for _, r := range api.Routes(h) {
		router.Handle(r.Method, r.Path, r.Handler)
	}
	for _, base := range []string{api.Base, api.AdminBase} {
		cursor := ""
		seen := map[string]bool{}
		for {
			path := base + "/requests/" + created.ID
			if cursor != "" {
				path += "?eventsBefore=" + cursor
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			require.Equal(t, 200, w.Code, w.Body.String())
			require.Less(t, w.Body.Len(), 256<<10)
			var detail struct {
				Request          tm.Request `json:"request"`
				Events           []tm.Event `json:"events"`
				NextEventsBefore string     `json:"nextEventsBefore"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
			require.Equal(t, "100", detail.Request.Revision, "current write revision stays available on every history page")
			require.NotEmpty(t, detail.Events)
			require.LessOrEqual(t, len(detail.Events), 16)
			previous := int64(0)
			for _, event := range detail.Events {
				n, err := strconv.ParseInt(event.Revision, 10, 64)
				require.NoError(t, err)
				require.Greater(t, n, previous, "each page is chronological")
				if cursor != "" {
					before, _ := strconv.ParseInt(cursor, 10, 64)
					require.Less(t, n, before, "exclusive cursor cannot overlap the last page")
				}
				require.False(t, seen[event.Revision], "history must neither skip nor duplicate facts")
				seen[event.Revision] = true
				previous = n
			}
			if cursor == "" {
				require.Equal(t, "100", detail.Events[len(detail.Events)-1].Revision, "latest progress is visible first")
			}
			if detail.NextEventsBefore == "" {
				break
			}
			require.Equal(t, detail.Events[0].Revision, detail.NextEventsBefore)
			cursor = detail.NextEventsBefore
		}
		require.Len(t, seen, 100)
		for _, query := range []string{"eventsBefore=0", "eventsBefore=01", "eventsBefore=-1", "eventsBefore=9223372036854775808", "eventsBefore=80&eventsBefore=60", "cursor=" + uuid.NewString(), "eventsBefore=80&organizationId=foreign"} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", base+"/requests/"+created.ID+"?"+query, nil))
			require.Equal(t, 400, w.Code, query)
		}
	}
	_, err = store.Detail(ctx, tm.Scope{ActorID: "other", OrganizationID: "foreign"}, false, created.ID, "80")
	require.ErrorIs(t, err, tm.ErrNotFound, "history cursor cannot bypass request tenant scope")
}
