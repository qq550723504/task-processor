package workbenchcontext

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
)

type grantLoaderCall struct {
	source  GrantSource
	request GrantRequest
}

type grantLoaderStub struct {
	result        GrantResult
	err           error
	calls         []grantLoaderCall
	invalidations [][2]string
	events        []string
}

func (stub *grantLoaderStub) Load(_ context.Context, source GrantSource, request GrantRequest) (GrantResult, error) {
	stub.calls = append(stub.calls, grantLoaderCall{source: source, request: request})
	stub.events = append(stub.events, "load")
	return stub.result, stub.err
}

func (stub *grantLoaderStub) Invalidate(subject string, projectID string) {
	stub.invalidations = append(stub.invalidations, [2]string{subject, projectID})
	stub.events = append(stub.events, "invalidate")
}

func TestResolverLivePoliciesBypassCacheAndSwitchInvalidatesBeforeAndAfterLookup(t *testing.T) {
	now := time.Date(2026, time.August, 30, 15, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name              string
		policy            httproute.OrganizationAccessPolicy
		wantEvents        []string
		wantInvalidations int
	}{
		{name: "live write", policy: httproute.OrganizationAccessPolicyLiveWrite, wantEvents: []string{"load"}},
		{name: "live switch", policy: httproute.OrganizationAccessPolicyLiveSwitch, wantEvents: []string{"invalidate", "load", "invalidate"}, wantInvalidations: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			loader := &grantLoaderStub{result: GrantResult{Grants: []authidentity.OrganizationGrant{
				{OrganizationID: "org-b", ProjectID: "project-1", Roles: []string{"listingkit_viewer"}},
			}}}
			resolver := NewResolver(loader, "project-1", "v1", nil, WithResolverClock(func() time.Time { return now }))

			_, err := resolver.Resolve(context.Background(), tt.policy, ResolveInput{
				Identity:    authidentity.AuthenticatedIdentity{UserID: "user-1", HomeOrganizationID: "org-a", TokenExpiresAt: now.Add(time.Minute)},
				BearerToken: "secret-token", RequestedOrganizationID: "org-b",
			})

			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if len(loader.calls) != 1 || loader.calls[0].source != GrantLive {
				t.Fatalf("grant calls = %+v, want one live load", loader.calls)
			}
			if !equalStrings(loader.events, tt.wantEvents) {
				t.Fatalf("events = %v, want %v", loader.events, tt.wantEvents)
			}
			if len(loader.invalidations) != tt.wantInvalidations {
				t.Fatalf("invalidations = %v, want %d", loader.invalidations, tt.wantInvalidations)
			}
			for _, invalidation := range loader.invalidations {
				if invalidation != [2]string{"user-1", "project-1"} {
					t.Fatalf("invalidation = %v, want verified subject/project", invalidation)
				}
			}
		})
	}
}

func TestResolverLiveSwitchInvalidatesAfterFailedLookup(t *testing.T) {
	now := time.Date(2026, time.August, 30, 15, 0, 0, 0, time.UTC)
	loader := &grantLoaderStub{err: errors.New("ZITADEL unavailable")}
	resolver := NewResolver(loader, "project-1", "v1", nil, WithResolverClock(func() time.Time { return now }))

	_, err := resolver.Resolve(context.Background(), httproute.OrganizationAccessPolicyLiveSwitch, ResolveInput{
		Identity:    authidentity.AuthenticatedIdentity{UserID: "user-1", TokenExpiresAt: now.Add(time.Minute)},
		BearerToken: "secret-token", RequestedOrganizationID: "org-b",
	})

	if err == nil {
		t.Fatal("Resolve() error = nil, want live dependency failure")
	}
	if !equalStrings(loader.events, []string{"invalidate", "load", "invalidate"}) {
		t.Fatalf("events = %v, want pre/load/post invalidation on failure", loader.events)
	}
}

func TestResolverRejectsExpiredOrIncompleteAuthenticationBeforeGrantLookup(t *testing.T) {
	now := time.Date(2026, time.August, 30, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		input ResolveInput
	}{
		{
			name:  "token expires exactly now",
			input: ResolveInput{Identity: authidentity.AuthenticatedIdentity{UserID: "user-1", TokenExpiresAt: now}, BearerToken: "secret-token"},
		},
		{
			name:  "token already expired",
			input: ResolveInput{Identity: authidentity.AuthenticatedIdentity{UserID: "user-1", TokenExpiresAt: now.Add(-time.Nanosecond)}, BearerToken: "secret-token"},
		},
		{
			name:  "missing verified subject",
			input: ResolveInput{Identity: authidentity.AuthenticatedIdentity{TokenExpiresAt: now.Add(time.Minute)}, BearerToken: "secret-token"},
		},
		{
			name:  "missing current bearer",
			input: ResolveInput{Identity: authidentity.AuthenticatedIdentity{UserID: "user-1", TokenExpiresAt: now.Add(time.Minute)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &grantLoaderStub{}
			resolver := NewResolver(loader, "project-1", "v1", nil, WithResolverClock(func() time.Time { return now }))

			_, err := resolver.Resolve(context.Background(), httproute.OrganizationAccessPolicyCachedRead, tt.input)

			if !errors.Is(err, ErrAuthenticationRequired) {
				t.Fatalf("Resolve() error = %v, want authentication required", err)
			}
			if len(loader.events) != 0 {
				t.Fatalf("grant events = %v, want none before authentication rejection", loader.events)
			}
		})
	}
}

func TestResolverAppliesConfiguredOrganizationSuspensionAsDenyOnlyOverlay(t *testing.T) {
	now := time.Date(2026, time.August, 30, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		checker   *organizationSuspensionCheckerStub
		wantError error
	}{
		{name: "active", checker: &organizationSuspensionCheckerStub{}},
		{name: "suspended", checker: &organizationSuspensionCheckerStub{suspended: true}, wantError: ErrOrganizationSuspended},
		{name: "checker unavailable", checker: &organizationSuspensionCheckerStub{err: errors.New("directory unavailable")}, wantError: ErrAuthorizationDependencyUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &grantLoaderStub{result: GrantResult{Grants: []authidentity.OrganizationGrant{
				{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_viewer"}},
			}}}
			resolver := NewResolver(loader, "project-1", "v1", tt.checker, WithResolverClock(func() time.Time { return now }))

			got, err := resolver.Resolve(context.Background(), httproute.OrganizationAccessPolicyCachedRead, ResolveInput{
				Identity:    authidentity.AuthenticatedIdentity{UserID: "user-1", HomeOrganizationID: "org-a", TokenExpiresAt: now.Add(time.Minute)},
				BearerToken: "secret-token",
			})

			if tt.wantError != nil {
				if !errors.Is(err, tt.wantError) {
					t.Fatalf("Resolve() error = %v, want %v", err, tt.wantError)
				}
			} else if err != nil || got.EffectiveOrganizationID != "org-a" {
				t.Fatalf("Resolve() = (%+v, %v), want active org-a", got, err)
			}
			if len(tt.checker.calls) != 1 || tt.checker.calls[0] != "org-a" {
				t.Fatalf("checker calls = %v, want selected org-a only", tt.checker.calls)
			}
		})
	}
}

func TestResolverMapsGrantDependencyFailureWithoutExposingProviderError(t *testing.T) {
	now := time.Date(2026, time.August, 30, 15, 0, 0, 0, time.UTC)
	loader := &grantLoaderStub{err: errors.New("ZITADEL response mentioned secret-token and org-other")}
	resolver := NewResolver(loader, "project-1", "v1", nil, WithResolverClock(func() time.Time { return now }))

	_, err := resolver.Resolve(context.Background(), httproute.OrganizationAccessPolicyCachedRead, ResolveInput{
		Identity:    authidentity.AuthenticatedIdentity{UserID: "user-1", TokenExpiresAt: now.Add(time.Minute)},
		BearerToken: "secret-token",
	})

	if !errors.Is(err, ErrAuthorizationDependencyUnavailable) {
		t.Fatalf("Resolve() error = %v, want dependency unavailable", err)
	}
	if strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "org-other") {
		t.Fatalf("sanitized error leaked provider detail: %v", err)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

type organizationSuspensionCheckerStub struct {
	suspended bool
	err       error
	calls     []string
}

func (stub *organizationSuspensionCheckerStub) IsOrganizationSuspended(_ context.Context, organizationID string) (bool, error) {
	stub.calls = append(stub.calls, organizationID)
	return stub.suspended, stub.err
}

func TestResolverTreatsRequestedOrganizationAsCandidateAndScopesRolesToVerifiedGrant(t *testing.T) {
	now := time.Date(2026, time.August, 30, 15, 0, 0, 0, time.UTC)
	loader := &grantLoaderStub{result: GrantResult{Grants: []authidentity.OrganizationGrant{
		{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_admin"}},
		{OrganizationID: "org-b", ProjectID: "project-1", Roles: []string{"listingkit_viewer"}},
	}}}
	resolver := NewResolver(loader, "project-1", "v1", nil, WithResolverClock(func() time.Time { return now }))
	input := ResolveInput{
		Identity: authidentity.AuthenticatedIdentity{
			TenantID:           "org-a",
			UserID:             "user-1",
			Roles:              []string{"listingkit_admin"},
			HomeOrganizationID: "org-a",
			TokenExpiresAt:     now.Add(time.Minute),
		},
		BearerToken:             "secret-token",
		RequestedOrganizationID: "org-b",
	}

	got, err := resolver.Resolve(context.Background(), httproute.OrganizationAccessPolicyCachedRead, input)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.EffectiveOrganizationID != "org-b" || got.TenantID != "org-b" {
		t.Fatalf("resolved organization = (%q, %q), want org-b", got.EffectiveOrganizationID, got.TenantID)
	}
	if len(got.Roles) != 1 || got.Roles[0] != "listingkit_viewer" {
		t.Fatalf("resolved roles = %v, want only org-b viewer", got.Roles)
	}
	if len(got.OrganizationGrants) != 2 || got.OrganizationGrants[0].OrganizationID != "org-a" {
		t.Fatalf("resolved grants = %+v, want normalized detached grants", got.OrganizationGrants)
	}
	got.OrganizationGrants[0].Roles[0] = "mutated"
	if loader.result.Grants[0].Roles[0] != "listingkit_admin" {
		t.Fatal("Resolve() returned shared grant role storage")
	}
	if len(loader.calls) != 1 || loader.calls[0].source != GrantReadCached {
		t.Fatalf("grant calls = %+v, want one cached-read load", loader.calls)
	}
	if loader.calls[0].request.BearerToken != "secret-token" || loader.calls[0].request.Subject != "user-1" {
		t.Fatalf("grant request = %+v, want current bearer and verified subject", loader.calls[0].request)
	}

	input.RequestedOrganizationID = "org-forged"
	_, err = resolver.Resolve(context.Background(), httproute.OrganizationAccessPolicyCachedRead, input)
	if !errors.Is(err, ErrOrganizationAccessDenied) {
		t.Fatalf("forged organization error = %v, want access denied", err)
	}
}

func TestResolverContextReadAllowsSelectionAndZeroGrantStatesWithoutBusinessScope(t *testing.T) {
	now := time.Date(2026, time.August, 30, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		grants []authidentity.OrganizationGrant
	}{
		{
			name: "selection required",
			grants: []authidentity.OrganizationGrant{
				{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_admin"}},
				{OrganizationID: "org-b", ProjectID: "project-1", Roles: []string{"listingkit_viewer"}},
			},
		},
		{name: "authoritative zero grants", grants: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &grantLoaderStub{result: GrantResult{Grants: tt.grants}}
			resolver := NewResolver(loader, "project-1", "v1", nil, WithResolverClock(func() time.Time { return now }))

			got, err := resolver.Resolve(context.Background(), httproute.OrganizationAccessPolicyContextRead, ResolveInput{
				Identity: authidentity.AuthenticatedIdentity{
					TenantID:                "org-home",
					UserID:                  "user-1",
					Roles:                   []string{"legacy_admin"},
					HomeOrganizationID:      "org-home",
					EffectiveOrganizationID: "org-home",
					TokenExpiresAt:          now.Add(time.Minute),
				},
				BearerToken: "secret-token",
			})

			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got.TenantID != "" || got.EffectiveOrganizationID != "" || len(got.Roles) != 0 {
				t.Fatalf("no-selection business scope = tenant %q effective %q roles %v, want empty", got.TenantID, got.EffectiveOrganizationID, got.Roles)
			}
			if got.UserID != "user-1" || got.HomeOrganizationID != "org-home" {
				t.Fatalf("authenticated identity = %+v, want verified user and home organization retained", got)
			}
			if len(got.OrganizationGrants) != len(tt.grants) {
				t.Fatalf("grants = %+v, want %d verified grants", got.OrganizationGrants, len(tt.grants))
			}
			if len(loader.calls) != 1 || loader.calls[0].source != GrantReadCached {
				t.Fatalf("grant calls = %+v, want cached read", loader.calls)
			}
		})
	}
}

func TestResolverBusinessReadsRequireSelectedOrganization(t *testing.T) {
	now := time.Date(2026, time.August, 30, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		policy    httproute.OrganizationAccessPolicy
		requested string
		grants    []authidentity.OrganizationGrant
		wantError error
	}{
		{
			name: "cached read requires selection", policy: httproute.OrganizationAccessPolicyCachedRead,
			grants: []authidentity.OrganizationGrant{
				{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_admin"}},
				{OrganizationID: "org-b", ProjectID: "project-1", Roles: []string{"listingkit_viewer"}},
			},
			wantError: ErrOrganizationSelectionRequired,
		},
		{
			name: "context read does not forgive explicit wrong organization", policy: httproute.OrganizationAccessPolicyContextRead,
			requested: "org-forged",
			grants:    []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_viewer"}}},
			wantError: ErrOrganizationAccessDenied,
		},
		{
			name: "context read does not forgive explicit organization after revocation", policy: httproute.OrganizationAccessPolicyContextRead,
			requested: "org-a", wantError: ErrOrganizationAccessRevoked,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &grantLoaderStub{result: GrantResult{Grants: tt.grants}}
			resolver := NewResolver(loader, "project-1", "v1", nil, WithResolverClock(func() time.Time { return now }))

			_, err := resolver.Resolve(context.Background(), tt.policy, ResolveInput{
				Identity:    authidentity.AuthenticatedIdentity{UserID: "user-1", TokenExpiresAt: now.Add(time.Minute)},
				BearerToken: "secret-token", RequestedOrganizationID: tt.requested,
			})

			if !errors.Is(err, tt.wantError) {
				t.Fatalf("Resolve() error = %v, want %v", err, tt.wantError)
			}
		})
	}
}
