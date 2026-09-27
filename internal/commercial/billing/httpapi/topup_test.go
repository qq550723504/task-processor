package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/httproute"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTopUpRoutesHaveSeparateTrustBoundaries(t *testing.T) {
	routes, err := Routes(NewHandler(nil))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, r := range routes {
		switch r.Path {
		case AlipayNotifyPath, WeChatNotifyPath:
			seen[r.Path] = true
			if r.AuthPolicy != httproute.AuthPolicyPublic || r.Permission != "" || r.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyNone || r.OrganizationTargetResolver != nil || r.RejectUnreadRequestBody {
				t.Fatal("callback entered browser authentication pipeline")
			}
		case TopUpRefundPath:
			seen[r.Path] = true
			if r.AuthPolicy != httproute.AuthPolicyCurrentIdentityWithVerifiedRoles || r.Permission != authz.PermissionListingKitPlatformAdm || r.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyNone || r.OrganizationTargetResolver != nil {
				t.Fatal("refund authority is not global")
			}
		}
	}
	if len(seen) != 3 {
		t.Fatal("missing external routes")
	}
}
func TestTopUpIntentRejectsBrowserOwnedPaymentFacts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, extra := range []string{`"organization_id":"other"`, `"actor_id":"other"`, `"merchant_id":"other"`, `"expires_at":"tomorrow"`, `"paid":true`, `"notify_url":"https://other.example"`} {
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		r := httptest.NewRequest(http.MethodPost, topUpIntentPath, strings.NewReader(`{"provider":"ALIPAY","amount_minor":"10000",`+extra+`}`))
		r.Header.Set("Idempotency-Key", "key")
		c.Request = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), authidentity.AuthenticatedIdentity{UserID: "actor-1", TenantID: "org-1", EffectiveOrganizationID: "org-1"}))
		NewHandler(&billing.Service{}).TopUpIntent(c)
		if response.Code != 400 {
			t.Fatalf("accepted browser fact %s: %d", extra, response.Code)
		}
	}
}

type validNotification struct{}

func (validNotification) VerifyNotification(*http.Request) (billing.ProviderObservation, error) {
	return billing.ProviderObservation{}, nil
}
func TestCallbackDoesNotAcknowledgeFailedDurableInbox(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, ali := range []bool{false, true} {
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request = httptest.NewRequest("POST", AlipayNotifyPath, strings.NewReader("fixture"))
		h := NewHandler(&billing.Service{})
		h.paymentNotify(c, validNotification{}, ali)
		c.Writer.WriteHeaderNow()
		if response.Code != 503 || strings.Contains(response.Body.String(), "success") {
			t.Fatal("ACK without durable evidence")
		}
	}
}
