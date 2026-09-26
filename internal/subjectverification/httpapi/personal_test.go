package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
	domain "task-processor/internal/subjectverification"
	"testing"
)

type personalSpy struct {
	actor domain.Actor
	calls int
	err   error
}

func (s *personalSpy) Start(_ context.Context, a domain.Actor, _ domain.PersonalInput) error {
	s.actor = a
	s.calls++
	return s.err
}
func (s *personalSpy) Read(_ context.Context, a domain.Actor) (domain.PersonalView, error) {
	return domain.PersonalView{UserID: a.UserID, State: "NOT_STARTED"}, nil
}
func (s *personalSpy) Refresh(_ context.Context, a domain.Actor, _ string) error {
	s.actor = a
	s.calls++
	return s.err
}
func TestPersonalAccountBoundaryAndLimits(t *testing.T) {
	phone := "+8613800000001"
	yes := true
	for _, tc := range []struct {
		name, user, body string
		err              error
		status           int
	}{{"lifetime", "user", `{}`, &domain.PersonalLimitError{Code: "VERIFICATION_TOTAL_LIMIT"}, 429}, {"cooldown", "user", `{}`, &domain.PersonalLimitError{Code: "VERIFICATION_COOLDOWN", RetryAfter: 60}, 429}, {"foreign phone", "other", `{}`, nil, 409}, {"client identity", "user", `{"userId":"other"}`, nil, 400}, {"large", "user", strings.Repeat(" ", 16385), nil, 400}} {
		t.Run(tc.name, func(t *testing.T) {
			s := &personalSpy{err: tc.err}
			h := PersonalHandler{Service: s, Profile: profileStub{authidentity.SelfProfile{UserID: tc.user, PhoneNumber: &phone, PhoneNumberVerified: &yes}}}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			r := httptest.NewRequest("POST", PersonalBasePath+"/applications", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer token")
			c.Request = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), authidentity.AuthenticatedIdentity{UserID: "user"}))
			h.start(c)
			if w.Code != tc.status {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if tc.status != 429 && s.calls != 0 {
				t.Fatal("unauthorized dispatch")
			}
			if tc.status == 429 && (s.actor.UserID != "user" || s.actor.OrganizationID != "" || s.actor.VerifiedPhone != phone) {
				t.Fatal("account binding lost")
			}
			if strings.Contains(w.Body.String(), phone) {
				t.Fatal("phone leaked")
			}
			if tc.name == "cooldown" && w.Header().Get("Retry-After") != "60" {
				t.Fatal("cooldown header missing")
			}
		})
	}
	for _, r := range (PersonalHandler{}).Routes() {
		if r.AuthPolicy != httproute.AuthPolicyCurrentIdentity || r.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyNone || r.Permission != "" {
			t.Fatal("personal auth changed")
		}
	}
}
