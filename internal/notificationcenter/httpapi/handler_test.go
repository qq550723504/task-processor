package notificationhttp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"task-processor/internal/authidentity"
	n "task-processor/internal/notificationcenter"
	"testing"
	"time"
)

type readRepo struct {
	n.Repository
	reads int
	scope n.Scope
}
type categorySource struct {
	n.ReaderSource
	category string
}

func (s categorySource) Category() string { return s.category }

func (*readRepo) ReadStates(context.Context, n.Scope, []n.Ref) (map[string]bool, error) {
	return map[string]bool{}, nil
}
func (*readRepo) Replay(context.Context, n.Scope, string, string, string) (n.Command, bool, error) {
	return n.Command{}, false, nil
}
func (r *readRepo) CommitRead(_ context.Context, s n.Scope, c n.Command, _ []n.Ref, _ string) (n.Command, error) {
	r.reads++
	r.scope = s
	return c, nil
}
func TestConsoleDefaultAllFilterUsesRealHTTPService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, category := range []string{n.Official, n.Business} {
		t.Run(category, func(t *testing.T) {
			name, base := "official", "/api/v1/notifications/official"
			if category == n.Business {
				name, base = "personal-verification", "/api/v1/notifications/personal"
			}
			ref := n.Ref{Source: name, EntityID: "native-fact", Type: "SYSTEM", Revision: "1"}
			item := n.Item{Ref: ref, Category: category, Title: "真实接口回归", Summary: "默认全部模式可读取", Target: n.Target{Kind: "home"}}
			source := n.NewReaderSource(name, true, func(context.Context, n.Scope, string, int) (n.SourcePage, error) {
				return n.SourcePage{Items: []n.Item{item}}, nil
			})
			h := &Handler{Realm: "http://identity.local", Service: &n.Service{Repository: &readRepo{}, Sources: []n.Source{categorySource{source, category}}}}
			router := gin.New()
			for _, route := range Routes(h) {
				router.Handle(route.Method, route.Path, route.Handler)
			}
			for _, query := range []string{"", "?filter=all&limit=20"} {
				req := httptest.NewRequest("GET", base+query, nil)
				req = req.WithContext(authidentity.WithAuthenticatedIdentity(req.Context(), authidentity.AuthenticatedIdentity{UserID: "actor", TokenExpiresAt: time.Now().Add(time.Hour)}))
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				var list n.List
				if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &list) != nil || len(list.Items) != 1 || list.Count != 1 || !list.Exact {
					t.Fatalf("Console default query %q: status=%d body=%s", query, response.Code, response.Body)
				}
			}
		})
	}
}
func TestStrictJSONAndPersonalReceiptScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &readRepo{}
	ref := n.Ref{Source: "personal-verification", EntityID: "application-a", Type: "VERIFIED", Revision: "1"}
	item := n.Item{Ref: ref, Category: n.Business, Title: "认证完成", Summary: "查看原认证页面", Target: n.Target{Kind: "personal-verification"}}
	source := n.NewReaderSource(ref.Source, true, func(context.Context, n.Scope, string, int) (n.SourcePage, error) {
		return n.SourcePage{Items: []n.Item{item}}, nil
	})
	handler := &Handler{Realm: "http://identity.local", Service: &n.Service{Repository: repo, Sources: []n.Source{source}}}
	router := gin.New()
	for _, route := range Routes(handler) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	makeRequest := func(body string) *http.Request {
		req := httptest.NewRequest("POST", "/api/v1/notifications/personal/read", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "4841d296-ef14-4c16-8d25-a7667e534feb")
		return req.WithContext(authidentity.WithAuthenticatedIdentity(req.Context(), authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "current-org", TokenExpiresAt: time.Now().Add(time.Hour)}))
	}
	for _, body := range []string{`{"ref":"a","ref":"b"}`, `{"ref":"a","subject":"other"}`, `{"ref":"a"} {}`, `{"ref":"a",}`} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, makeRequest(body))
		if response.Code != 400 {
			t.Fatalf("ambiguous body accepted %q: %d", body, response.Code)
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, makeRequest(`{"ref":"`+n.Token(ref)+`"}`))
	if response.Code != 200 || repo.reads != 1 || repo.scope.Subject != "actor" || repo.scope.OrganizationID != "" {
		t.Fatalf("personal read bound to enterprise: %d %+v", response.Code, repo)
	}
	expired := makeRequest(`{}`)
	expired = expired.WithContext(authidentity.WithAuthenticatedIdentity(expired.Context(), authidentity.AuthenticatedIdentity{UserID: "actor", TokenExpiresAt: time.Now().Add(-time.Minute)}))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, expired)
	if response.Code != 403 || repo.reads != 1 {
		t.Fatal("expired identity accessed receipts")
	}
}

type blockedBody struct {
	done chan struct{}
	once sync.Once
}

func (b *blockedBody) Read([]byte) (int, error) { <-b.done; return 0, errors.New("closed") }
func (b *blockedBody) Close() error             { b.once.Do(func() { close(b.done) }); return nil }

var _ io.ReadCloser = (*blockedBody)(nil)

func TestPOSTBodyTimeoutInterruptsBlockedRead(t *testing.T) {
	handler := &Handler{Realm: "http://identity.local", Service: &n.Service{Repository: &readRepo{}}}
	router := gin.New()
	for _, route := range Routes(handler) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	req := httptest.NewRequest("POST", "/api/v1/notifications/official/snapshot", nil)
	req.Body = &blockedBody{done: make(chan struct{})}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "4841d296-ef14-4c16-8d25-a7667e534feb")
	req = req.WithContext(authidentity.WithAuthenticatedIdentity(req.Context(), authidentity.AuthenticatedIdentity{UserID: "actor", TokenExpiresAt: time.Now().Add(time.Hour)}))
	done := make(chan struct{})
	response := httptest.NewRecorder()
	go func() { router.ServeHTTP(response, req); close(done) }()
	select {
	case <-done:
		if response.Code != 400 {
			t.Fatal(response.Code)
		}
	case <-time.After(7 * time.Second):
		_ = req.Body.Close()
		t.Fatal("POST body read exceeded bounded close")
	}
}
func TestDescriptorAdmissionRejectsWeakenedPlatformAndOrganizationGates(t *testing.T) {
	for _, route := range Routes(nil) {
		if err := ValidateDescriptor(route); err != nil {
			t.Fatal(err)
		}
		altered := route
		altered.RequestTimeout = 0
		if ValidateDescriptor(altered) == nil {
			t.Fatal("unbounded route admitted")
		}
		if route.Permission != "" {
			altered = route
			altered.Permission = ""
			if ValidateDescriptor(altered) == nil {
				t.Fatal("platform permission removed")
			}
		}
	}
}
