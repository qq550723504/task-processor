package httpapi

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"task-processor/internal/authidentity"
	domain "task-processor/internal/subjectverification"
)

type callbackStub struct{}

func (callbackStub) Parse([]byte, string) (domain.Event, bool, error) {
	return domain.Event{}, false, nil
}
func TestPostRoutesInterruptSlowRequestBodies(t *testing.T) {
	for _, path := range []string{BasePath + "/applications", CallbackPath} {
		t.Run(path, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Millisecond)
				defer cancel()
				c.Request = c.Request.WithContext(ctx)
				c.Next()
			})
			h := Handler{Service: &serviceSpy{}, Callback: callbackStub{}}
			for _, route := range h.Routes(nil) {
				router.Handle(route.Method, route.Path, func(c *gin.Context) {
					c.Request = c.Request.WithContext(authidentity.WithAuthenticatedIdentity(c.Request.Context(), authidentity.AuthenticatedIdentity{UserID: "user", EffectiveOrganizationID: "org"}))
					route.Handler(c)
				})
			}
			server := httptest.NewServer(router)
			defer server.Close()
			conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, err = fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{", path)
			if err != nil {
				t.Fatal(err)
			}
			response, err := http.ReadResponse(bufio.NewReader(conn), nil)
			if err != nil {
				t.Fatalf("slow body was not interrupted: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != 400 && response.StatusCode != 408 {
				t.Fatalf("unexpected timeout status %d", response.StatusCode)
			}
		})
	}
}

type profileStub struct{ profile authidentity.SelfProfile }

func (p profileStub) ReadSelf(context.Context, string, string) (authidentity.SelfProfile, error) {
	return p.profile, nil
}

type serviceSpy struct {
	calls int
	actor domain.Actor
}

func (s *serviceSpy) Start(_ context.Context, a domain.Actor, _ domain.Input) (domain.Application, error) {
	s.calls++
	s.actor = a
	return domain.Application{State: domain.Unknown}, nil
}
func (s *serviceSpy) Read(context.Context, domain.Actor) (domain.Application, string, error) {
	return domain.Application{}, "", domain.ErrNotFound
}
func (s *serviceSpy) Observe(context.Context, domain.Event) error { return nil }
func TestStartRequiresCurrentVerifiedMainlandPhone(t *testing.T) {
	phone := "+8613800000001"
	yes := true
	no := false
	for _, tc := range []struct {
		name, user string
		verified   *bool
		want       int
	}{{"verified", "user", &yes, 200}, {"unverified", "user", &no, 409}, {"wrong subject", "other", &yes, 409}, {"unknown", "user", nil, 409}} {
		t.Run(tc.name, func(t *testing.T) {
			s := &serviceSpy{}
			h := Handler{Service: s, Profile: profileStub{authidentity.SelfProfile{UserID: tc.user, PhoneNumber: &phone, PhoneNumberVerified: tc.verified}}}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			r := httptest.NewRequest("POST", BasePath+"/applications", strings.NewReader(`{"companyName":"企业","creditCode":"91310000MA00000001","consent":true,"idempotencyKey":"35e86bfb-9c42-4c85-9647-b5a1702c57e9"}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer token")
			c.Request = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), authidentity.AuthenticatedIdentity{UserID: "user", EffectiveOrganizationID: "org"}))
			h.start(c)
			if w.Code != tc.want {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if tc.want != 200 && s.calls != 0 {
				t.Fatal("provider called without verified identity")
			}
			if tc.want == 200 && s.actor.VerifiedPhone != phone {
				t.Fatal("untrusted phone")
			}
			if strings.Contains(w.Body.String(), phone) {
				t.Fatal("raw phone disclosed")
			}
		})
	}
}
func TestStartRejectsClientSuppliedSubjectAndOversize(t *testing.T) {
	for _, body := range []string{`{"organizationId":"other"}`, `{"phone":"13800000002"}`, strings.Repeat(" ", 8193) + `{}`} {
		s := &serviceSpy{}
		h := Handler{Service: s}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		r := httptest.NewRequest("POST", BasePath+"/applications", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		c.Request = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), authidentity.AuthenticatedIdentity{UserID: "user", EffectiveOrganizationID: "org"}))
		h.start(c)
		if w.Code != 400 || s.calls != 0 {
			t.Fatalf("untrusted body accepted: %d", w.Code)
		}
	}
}
