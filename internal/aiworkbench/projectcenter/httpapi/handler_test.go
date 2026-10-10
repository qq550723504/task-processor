package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	pc "task-processor/internal/aiworkbench/projectcenter"
	"testing"
)

type httpStore struct{ calls int }

func (s *httpStore) Replay(context.Context, pc.Scope, string, pc.Command) (pc.Receipt, bool, error) {
	return pc.Receipt{}, false, nil
}
func (s *httpStore) Commit(context.Context, pc.Scope, string, pc.Command, error) (pc.Receipt, error) {
	s.calls++
	return pc.Receipt{ID: "550e8400-e29b-41d4-a716-446655440000", Revision: 1}, nil
}
func (s *httpStore) Get(context.Context, pc.Scope, string) (pc.Project, []pc.Reference, error) {
	return pc.Project{}, nil, pc.ErrNotFound
}
func (s *httpStore) List(context.Context, pc.Scope, pc.Query) ([]pc.Project, string, error) {
	return []pc.Project{}, "", nil
}
func (s *httpStore) Templates(context.Context, pc.Scope, string) ([]pc.Template, string, error) {
	return []pc.Template{}, "", nil
}
func TestProjectHTTPBoundsAndCanonicalCommands(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &httpStore{}
	h := &Handler{Service: &pc.Service{Store: store, Authorize: func(context.Context, pc.Scope, bool) error { return nil }}, Bind: func(ctx context.Context, _ string) (context.Context, pc.Scope, error) {
		return ctx, pc.Scope{OrganizationID: "org-a", ActorID: "actor-a"}, nil
	}}
	router := gin.New()
	for _, r := range Routes(h) {
		require.NoError(t, ValidateDescriptor(r))
		router.Handle(r.Method, r.Path, r.Handler)
	}
	valid := `{"title":"Project","goal":"Goal","kind":"OTHER","dueDate":""}`
	for _, body := range []string{`{"title":"a","title":"b","goal":"g","kind":"OTHER","dueDate":""}`, `{"title":"a","goal":"g","kind":"OTHER","organizationId":"org-b"}`, strings.Repeat(" ", 16385) + valid, `{"title":"a","goal":"g","kind":"OTHER","dueDate":"2026-02-30"}`} {
		req := httptest.NewRequest("POST", Base, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, 400, rec.Code, rec.Body.String())
	}
	require.Zero(t, store.calls)
	req := httptest.NewRequest("POST", Base, strings.NewReader(valid))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, 1, store.calls)
	require.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
}
