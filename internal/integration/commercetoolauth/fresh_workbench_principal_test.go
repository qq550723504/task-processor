package commercetoolauth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"task-processor/internal/httproute"
	"task-processor/internal/workbenchcontext"
)

type freshAuthorizationFixture struct {
	grants []authidentity.OrganizationGrant
	err    error
	calls  int
}

func (f *freshAuthorizationFixture) ListOwnProjectAuthorizations(ctx context.Context, _, _, _ string) ([]authidentity.OrganizationGrant, error) {
	f.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return f.grants, f.err
}

type freshAuditFixture struct{ records []commercetool.AuditRecord }

func (f *freshAuditFixture) RecordToolCall(_ context.Context, record commercetool.AuditRecord) error {
	f.records = append(f.records, record)
	return nil
}

func TestFreshWorkbenchActualResolverInvocation(t *testing.T) {
	for _, scenario := range []string{"two-calls", "permission-revoked", "organization-revoked", "provider-failed", "expired", "canceled", "selected-org-drift"} {
		t.Run(scenario, func(t *testing.T) {
			now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
			client := &freshAuthorizationFixture{grants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project", Roles: []string{"listingkit_operator"}}}}
			grants := workbenchcontext.NewGrantResolver(client, workbenchcontext.NewGrantCache(func() time.Time { return now }))
			owner := workbenchcontext.NewResolver(grants, "project", "v1", nil, workbenchcontext.WithResolverClock(func() time.Time { return now }))
			request := OrganizationRequest{Identity: authidentity.AuthenticatedIdentity{UserID: "actor", HomeOrganizationID: "org-home", TokenExpiresAt: now.Add(time.Minute)}, BearerToken: "secret-fixture-bearer", RequestedOrganizationID: "org-a"}
			// Warm the actual CachedRead cache first. No Invalidate is used below.
			if _, err := owner.Resolve(context.Background(), httproute.OrganizationAccessPolicyCachedRead, workbenchcontext.ResolveInput{Identity: request.Identity, BearerToken: request.BearerToken, RequestedOrganizationID: request.RequestedOrganizationID}); err != nil {
				t.Fatal(err)
			}
			resolver, err := NewFreshWorkbenchPrincipalResolver(FreshOrganizationResolverFunc(func(ctx context.Context, request OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
				return owner.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: request.Identity, BearerToken: request.BearerToken, RequestedOrganizationID: request.RequestedOrganizationID})
			}), func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			policy, err := authz.NewListingKitAuthorizer(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			authorizer, err := NewCasbinAuthorizer(policy)
			if err != nil {
				t.Fatal(err)
			}
			readerCalls := 0
			definition := commercetool.Definition{
				Ref: commercetool.ToolRef{ID: "fixture.facts.read", Version: "v1.0.0"}, Capability: "fixture.facts", Owner: "fixture", Description: "Read controlled facts.",
				InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`), OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`),
				Risk: commercetool.RiskRead, Permission: commercetool.PermissionRequirement{Permission: authz.PermissionListingKitAdminRead},
				SideEffects: commercetool.SideEffectPolicy{Mode: commercetool.SideEffectNone}, Idempotency: commercetool.IdempotencyPolicy{Mode: commercetool.IdempotencyDeterministic},
				Timeout: commercetool.TimeoutPolicy{Duration: time.Second}, Retry: commercetool.RetryPolicy{Owner: commercetool.RetryOwnerCaller}, Usage: commercetool.UsagePolicy{Owner: commercetool.UsageOwnerUnmetered},
			}
			registry, err := commercetool.NewRegistry(commercetool.Tool{Definition: definition, Executor: commercetool.ExecutorFunc(func(_ context.Context, envelope commercetool.ExecutionEnvelope, _ json.RawMessage) (commercetool.ExecutionResult, error) {
				readerCalls++
				if envelope.Principal().TenantID != "org-a" || envelope.Principal().UserID != "actor" {
					t.Error("wrong actor/org reached domain")
				}
				return commercetool.ExecutionResult{Output: json.RawMessage(`{}`)}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			audits := &freshAuditFixture{}
			bound, err := registry.Bind(commercetool.AgentDefinition{ID: "fixture.agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{definition.Ref}}, commercetool.InvocationDependencies{PrincipalResolver: resolver, Authorizer: authorizer, Recorder: audits, Tracer: otel.Tracer("fresh-test"), Now: func() time.Time { return now }, AuditTimeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			invoke := func(ctx context.Context) error {
				_, err := bound.Invoke(WithOrganizationRequest(ctx, request), commercetool.Call{Tool: definition.Ref, Metadata: commercetool.CallMetadata{CallID: "call", AgentID: "fixture.agent", AgentVersion: "v1.0.0", AgentRunID: "run", BusinessTaskID: "correlation-only"}, Arguments: json.RawMessage(`{}`)})
				return err
			}
			if err := invoke(context.Background()); err != nil {
				t.Fatalf("first read: %v", err)
			}
			if readerCalls != 1 || client.calls != 2 {
				t.Fatalf("first read not fresh: reader=%d provider=%d", readerCalls, client.calls)
			}
			ctx := context.Background()
			want := commercetool.ErrorIdentityIntegrity
			switch scenario {
			case "two-calls":
				want = ""
			case "permission-revoked":
				client.grants[0].Roles = []string{"listingkit_viewer"}
				want = commercetool.ErrorPermissionDenied
			case "organization-revoked":
				client.grants = []authidentity.OrganizationGrant{{OrganizationID: "org-home", ProjectID: "project", Roles: []string{"listingkit_operator"}}}
			case "provider-failed":
				client.err = errors.New("secret-fixture-bearer provider failed")
			case "expired":
				now = request.Identity.TokenExpiresAt
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "selected-org-drift":
				request.RequestedOrganizationID = "org-other"
			}
			err = invoke(ctx)
			if want == "" {
				if err != nil || readerCalls != 2 || client.calls != 3 {
					t.Fatalf("second fresh read: %v reader=%d provider=%d", err, readerCalls, client.calls)
				}
			} else if err == nil || commercetool.CodeOf(err) != want || readerCalls != 1 {
				t.Fatalf("next invocation did not fail closed: %v reader=%d", err, readerCalls)
			}
			encoded, _ := json.Marshal(audits.records)
			if strings.Contains(string(encoded), "secret-fixture-bearer") || (err != nil && strings.Contains(err.Error(), "secret-fixture-bearer")) {
				t.Fatal("credential leaked")
			}
		})
	}
}

func TestFreshWorkbenchRejectsInvalidConstructionAndRequest(t *testing.T) {
	var nilFunc FreshOrganizationResolverFunc
	for _, dependency := range []FreshOrganizationResolver{nil, nilFunc} {
		if _, err := NewFreshWorkbenchPrincipalResolver(dependency, nil); err == nil {
			t.Fatal("nil dependency accepted")
		}
	}
	now := time.Now()
	for _, scenario := range []string{"missing", "actor-drift", "org-drift", "extended-expiry", "canceled-after-resolution"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			request := OrganizationRequest{Identity: authidentity.AuthenticatedIdentity{UserID: "actor", TokenExpiresAt: now.Add(time.Minute)}, BearerToken: "secret", RequestedOrganizationID: "org-a"}
			resolver, _ := NewFreshWorkbenchPrincipalResolver(FreshOrganizationResolverFunc(func(context.Context, OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
				identity := authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "org-a", EffectiveOrganizationID: "org-a", TokenExpiresAt: request.Identity.TokenExpiresAt, Roles: []string{"listingkit_operator"}}
				switch scenario {
				case "actor-drift":
					identity.UserID = "other"
				case "org-drift":
					identity.TenantID = "other"
					identity.EffectiveOrganizationID = "other"
				case "extended-expiry":
					identity.TokenExpiresAt = now.Add(time.Hour)
				case "canceled-after-resolution":
					cancel()
				}
				return identity, nil
			}), func() time.Time { return now })
			if scenario != "missing" {
				ctx = WithOrganizationRequest(ctx, request)
			}
			if principal, err := resolver.ResolveFreshPrincipal(ctx); err == nil || principal.UserID != "" {
				t.Fatalf("invalid request accepted: %+v %v", principal, err)
			}
		})
	}
}

func TestFreshWorkbenchConcurrentRequestIdentityIsolation(t *testing.T) {
	now := time.Now()
	resolver, err := NewFreshWorkbenchPrincipalResolver(FreshOrganizationResolverFunc(func(ctx context.Context, request OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
		if err := ctx.Err(); err != nil {
			return authidentity.AuthenticatedIdentity{}, err
		}
		return authidentity.AuthenticatedIdentity{UserID: request.Identity.UserID, TenantID: request.RequestedOrganizationID, EffectiveOrganizationID: request.RequestedOrganizationID, TokenExpiresAt: request.Identity.TokenExpiresAt, Roles: []string{"listingkit_operator"}}, nil
	}), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for index := range 40 {
		group.Add(1)
		go func() {
			defer group.Done()
			actor, org := "actor-a", "org-a"
			if index%2 != 0 {
				actor, org = "actor-b", "org-b"
			}
			ctx := WithOrganizationRequest(context.Background(), OrganizationRequest{Identity: authidentity.AuthenticatedIdentity{UserID: actor, TokenExpiresAt: now.Add(time.Minute)}, RequestedOrganizationID: org, BearerToken: "request-only"})
			for range 5 {
				principal, err := resolver.ResolveFreshPrincipal(ctx)
				if err != nil || principal.UserID != actor || principal.TenantID != org {
					t.Errorf("request crossed identities: %+v %v", principal, err)
					return
				}
			}
		}()
	}
	group.Wait()
}
