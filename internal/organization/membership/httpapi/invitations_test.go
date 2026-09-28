package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
	kernel "task-processor/internal/kernel/module"
	flow "task-processor/internal/organization/membership/inviteflow"
	"testing"
	"time"
)

type invitationStoreStub struct {
	flow.Store
	inv flow.Invitation
}

func (s *invitationStoreStub) Read(_ context.Context, id string) (flow.Invitation, error) {
	if id != s.inv.ID {
		return flow.Invitation{}, flow.ErrNotFound
	}
	return s.inv, nil
}
func TestInvitationRecipientRouteNeedsNoTargetOrganizationGrant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	email := "recipient@example.test"
	verified := true
	store := &invitationStoreStub{inv: flow.Invitation{ID: uuid.NewString(), ProjectID: "p", OrganizationID: "target-org", CreatorID: "creator", Contact: email, Role: "listingkit_viewer", State: flow.Pending, Revision: 1, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}}
	h := NewHandler(nil)
	h.ConfigureInvitations(func(*http.Request) (*flow.Service, error) {
		return flow.New("p", flow.Dependencies{Store: store, ReadSelf: func(ctx context.Context) (authidentity.SelfProfile, error) {
			identity, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
			return authidentity.SelfProfile{UserID: identity.UserID, Email: &email, EmailVerified: &verified}, nil
		}}), nil
	})
	reg := kernel.NewRegistry()
	if err := NewModule(h).Register(reg); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	var recipientRoutes int
	for _, route := range reg.Routes() {
		if route.OrganizationAccessPolicy == httproute.OrganizationAccessPolicyNone {
			recipientRoutes++
			if route.Permission != "" || route.AuthPolicy != httproute.AuthPolicyCurrentIdentity || route.OrganizationTargetResolver != nil {
				t.Fatal("recipient erroneously needs enterprise access")
			}
			router.Handle(route.Method, route.Path, route.Handler)
		}
	}
	if recipientRoutes != 3 {
		t.Fatal(recipientRoutes)
	}
	for _, good := range []bool{true, false} {
		verified = good
		r := httptest.NewRequest("GET", "/api/v1/account/invitations/"+store.inv.ID, nil)
		r = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), authidentity.AuthenticatedIdentity{UserID: "recipient", TokenExpiresAt: time.Now().Add(time.Hour)}))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		want := 403
		if good {
			want = 200
		}
		if w.Code != want {
			t.Fatal(w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("cacheable invitation")
		}
	}
}
