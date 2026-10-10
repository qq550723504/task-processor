package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	c "task-processor/internal/operationscockpit"
	"testing"
)

type repoFixture struct {
	c.Repository
	writes int
	scope  c.Scope
	reads  int
}

func (r *repoFixture) Facts(context.Context, c.Scope, string, int) ([]c.Record, error) {
	r.reads++
	return []c.Record{}, nil
}
func (r *repoFixture) Snapshot(context.Context, c.Scope, c.Query) (c.Snapshot, error) {
	r.reads++
	return c.Snapshot{Stores: map[string]c.StoreAggregate{}, GoalStores: map[string]c.StoreAggregate{}}, nil
}

func TestGoalEvidenceIsBoundedAndKeepsRevisionAndTotals(t *testing.T) {
	id := "123e4567-e89b-42d3-a456-426614174000"
	a := c.StoreAggregate{StoreID: id, Period: c.Period{Start: "2026-10-01", End: "2026-10-09"}, Complete: true, Totals: c.Totals{NetProfit: 42}}
	for i := 0; i < 25; i++ {
		a.Records = append(a.Records, c.RecordEvidence{ID: id, Revision: 2, Period: a.Period})
	}
	basis := goalEvidence(c.Snapshot{Goal: &c.GoalVersion{Config: c.GoalConfig{StoreIDs: []string{id}}}, GoalStores: map[string]c.StoreAggregate{id: a}})
	if len(basis) != 1 || basis[0].RecordCount != 25 || len(basis[0].Records) != 20 || basis[0].Records[0].Revision != 2 || basis[0].Totals.NetProfit != 42 {
		t.Fatalf("unbounded or rewritten evidence: %+v", basis)
	}
}

func TestFactsRejectAmbiguousStoreAndLateRuleGrantRevocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &repoFixture{}
	revoked := false
	h := &Handler{Repository: repo, Bind: func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }, Scope: func(context.Context) (c.Scope, error) { return c.Scope{}, nil }, Current: func(context.Context, c.Scope) (c.Access, error) {
		return c.Access{StoresRead: true, AdviceRead: true}, nil
	}, Directory: func(context.Context, c.Scope) ([]c.StoreReference, error) {
		return []c.StoreReference{{ID: "123e4567-e89b-42d3-a456-426614174000"}}, nil
	}, Revalidate: func(context.Context, c.Scope, []string) error {
		if revoked {
			return c.ErrForbidden
		}
		return nil
	}, Observations: func(context.Context, c.Scope, []c.StoreReference) (c.ObservationProjection, error) {
		revoked = true
		return c.ObservationProjection{}, nil
	}}
	routes, err := BuildRoutes(h)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	for _, r := range routes {
		router.Handle(r.Method, r.Path, r.Handler)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", Base+"/facts?storeId=123e4567-e89b-42d3-a456-426614174000&storeId=123e4567-e89b-42d3-a456-426614174001", nil))
	if w.Code != 400 || repo.reads != 0 {
		t.Fatalf("ambiguous selection consumed: %d reads=%d", w.Code, repo.reads)
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", Base+"/advice?startDate=2026-10-01&endDate=2026-10-02", nil))
	if w.Code != 403 {
		t.Fatalf("late revoked grant leaked projection: %d %s", w.Code, w.Body.String())
	}
}

func (r *repoFixture) Execute(_ context.Context, cmd c.Command) (c.Receipt, error) {
	r.writes++
	r.scope = cmd.Scope
	return c.Receipt{CommandID: cmd.Key, ID: cmd.ID, Revision: "1"}, nil
}
func TestHTTPFinancialInputRequiresExplicitZeroAndStrictBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &repoFixture{}
	h := &Handler{Repository: repo, Bind: func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }, Scope: func(context.Context) (c.Scope, error) {
		return c.Scope{OrganizationID: "org-real", ActorID: "actor-real"}, nil
	}, Current: func(context.Context, c.Scope) (c.Access, error) {
		return c.Access{StoresRead: true, FactsWrite: true}, nil
	}, Directory: func(context.Context, c.Scope) ([]c.StoreReference, error) { return []c.StoreReference{}, nil }}
	h.Revalidate = func(context.Context, c.Scope, []string) error { return nil }
	routes, err := BuildRoutes(h)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	for _, r := range routes {
		router.Handle(r.Method, r.Path, r.Handler)
	}
	valid := `{"id":"` + uuid.NewString() + `","storeId":"` + uuid.NewString() + `","expectedRevision":"0","fact":{"period":{"startDate":"2026-10-01","endDate":"2026-10-01"},"amounts":{"revenue":100,"refunds":0,"procurement":0,"logistics":0,"platform":0,"advertising":0,"other":0},"note":""}}`
	for _, body := range []string{strings.Replace(valid, `"refunds":0,`, "", 1), strings.Replace(valid, `"refunds":0`, `"refunds":0,"refunds":1`, 1), strings.Replace(valid, `"expectedRevision":"0"`, `"expectedRevision":"0","organizationId":"org-forged"`, 1), strings.Replace(valid, `"revenue":100`, `"revenue":1.5`, 1)} {
		req := httptest.NewRequest(http.MethodPost, Base+"/facts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", uuid.NewString())
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != 400 || repo.writes != 0 {
			t.Fatalf("invalid intent consumed: %d %s", response.Code, response.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, Base+"/facts", strings.NewReader(valid))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", uuid.NewString())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 200 || repo.writes != 1 || repo.scope.OrganizationID != "org-real" {
		t.Fatalf("valid trusted intent failed: %d %s", response.Code, response.Body.String())
	}
	for _, route := range Routes(nil) {
		if err := ValidateDescriptor(route); err != nil {
			t.Fatal(err)
		}
		route.OrganizationAccessPolicy = "cached_read"
		if ValidateDescriptor(route) == nil {
			t.Fatal("stale route policy accepted")
		}
	}
}
