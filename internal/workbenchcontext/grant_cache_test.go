package workbenchcontext

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"task-processor/internal/authidentity"
)

func TestGrantCacheKeysBySubjectProjectAndContractVersionWithoutBearerToken(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	client := &authorizationClientStub{grantsByProject: map[string][]authidentity.OrganizationGrant{
		"project-a": {validGrant("org-a")},
		"project-b": {validGrantForProject("org-b", "project-b")},
	}}
	cache := NewGrantCache(func() time.Time { return now })
	resolver := NewGrantResolver(client, cache)

	base := GrantRequest{BearerToken: "secret-bearer-token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(2 * time.Minute)}
	first, err := resolver.Grants(context.Background(), base)
	if err != nil || first.Source != GrantLive {
		t.Fatalf("first Grants() = (%+v, %v), want live result", first, err)
	}
	secondRequest := base
	secondRequest.BearerToken = "rotated-secret-bearer-token"
	second, err := resolver.Grants(context.Background(), secondRequest)
	if err != nil || second.Source != GrantReadCached {
		t.Fatalf("same subject/project/version Grants() = (%+v, %v), want cached result", second, err)
	}
	if calls := client.callCount(); calls != 1 {
		t.Fatalf("client calls after rotated bearer token = %d, want 1", calls)
	}
	if cacheText := fmt.Sprintf("%#v", cache); contains(cacheText, "secret-bearer-token") || contains(cacheText, "rotated-secret-bearer-token") {
		t.Fatalf("cache retained a bearer token: %s", cacheText)
	}

	for _, request := range []GrantRequest{
		{BearerToken: "secret-bearer-token", Subject: "subject-b", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(2 * time.Minute)},
		{BearerToken: "secret-bearer-token", Subject: "subject-a", ProjectID: "project-b", ContractVersion: "v1", TokenExpiresAt: now.Add(2 * time.Minute)},
		{BearerToken: "secret-bearer-token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v2", TokenExpiresAt: now.Add(2 * time.Minute)},
	} {
		result, err := resolver.Grants(context.Background(), request)
		if err != nil || result.Source != GrantLive {
			t.Fatalf("different cache key Grants() = (%+v, %v), want live result", result, err)
		}
	}
	if calls := client.callCount(); calls != 4 {
		t.Fatalf("client calls for distinct subject/project/version keys = %d, want 4", calls)
	}
}

func TestGrantResolverGrantLiveBypassesWarmCache(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	client := &authorizationClientStub{grants: []authidentity.OrganizationGrant{validGrant("org-cached")}}
	resolver := NewGrantResolver(client, NewGrantCache(func() time.Time { return now }))
	request := GrantRequest{BearerToken: "token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(time.Minute)}

	if _, err := resolver.Grants(context.Background(), request); err != nil {
		t.Fatalf("warm Grants() error = %v", err)
	}
	client.setGrants([]authidentity.OrganizationGrant{validGrant("org-live")})

	result, err := resolver.Load(context.Background(), GrantLive, request)
	if err != nil {
		t.Fatalf("live Load() error = %v", err)
	}
	if result.Source != GrantLive || len(result.Grants) != 1 || result.Grants[0].OrganizationID != "org-live" {
		t.Fatalf("live Load() = %+v, want fresh org-live grant", result)
	}
	if calls := client.callCount(); calls != 2 {
		t.Fatalf("client calls = %d, want 2 because GrantLive bypasses a warm cache", calls)
	}
}

func TestGrantResolverGrantLiveFailureDoesNotFallBackToWarmCache(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	client := &authorizationClientStub{grants: []authidentity.OrganizationGrant{validGrant("org-cached")}}
	resolver := NewGrantResolver(client, NewGrantCache(func() time.Time { return now }))
	request := GrantRequest{BearerToken: "token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(time.Minute)}

	if _, err := resolver.Grants(context.Background(), request); err != nil {
		t.Fatalf("warm Grants() error = %v", err)
	}
	client.setError(errors.New("ZITADEL unavailable"))

	_, err := resolver.Load(context.Background(), GrantLive, request)
	if !errors.Is(err, ErrAuthorizationDependencyUnavailable) {
		t.Fatalf("live Load() error = %v, want authorization dependency error", err)
	}
	if calls := client.callCount(); calls != 2 {
		t.Fatalf("client calls = %d, want 2 because live failure cannot use warm cache", calls)
	}
}

func TestGrantCacheUsesTokenBoundTTLAndRejectsExpiredEntries(t *testing.T) {
	base := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	now := base
	client := &authorizationClientStub{grantsByProject: map[string][]authidentity.OrganizationGrant{
		"project-a": {validGrant("org-a")},
		"project-b": {validGrantForProject("org-b", "project-b")},
	}}
	cache := NewGrantCache(func() time.Time { return now })
	resolver := NewGrantResolver(client, cache)
	request := GrantRequest{BearerToken: "token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: base.Add(90 * time.Second)}

	if _, err := resolver.Grants(context.Background(), request); err != nil {
		t.Fatalf("initial Grants() error = %v", err)
	}
	if cache.entries.Len() != 1 {
		t.Fatalf("cache entries = %d, want 1", cache.entries.Len())
	}
	for _, item := range cache.entries.Items() {
		entry := item.Value()
		if !entry.expiresAt.Equal(base.Add(60 * time.Second)) {
			t.Fatalf("cache expiry = %s, want %s", entry.expiresAt, base.Add(60*time.Second))
		}
	}

	now = base.Add(60 * time.Second)
	result, err := resolver.Grants(context.Background(), request)
	if err != nil || result.Source != GrantLive {
		t.Fatalf("expired Grants() = (%+v, %v), want live result", result, err)
	}
	if calls := client.callCount(); calls != 2 {
		t.Fatalf("client calls after cache expiry = %d, want 2", calls)
	}
}

func TestGrantCacheDoesNotCacheTokensWithLessThanOneSecondRemaining(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	client := &authorizationClientStub{grantsByProject: map[string][]authidentity.OrganizationGrant{
		"project-a": {validGrant("org-a")},
		"project-b": {validGrantForProject("org-b", "project-b")},
	}}
	cache := NewGrantCache(func() time.Time { return now })
	resolver := NewGrantResolver(client, cache)
	request := GrantRequest{BearerToken: "token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(999 * time.Millisecond)}

	for attempt := 0; attempt < 2; attempt++ {
		result, err := resolver.Grants(context.Background(), request)
		if err != nil || result.Source != GrantLive {
			t.Fatalf("Grants() attempt %d = (%+v, %v), want live result", attempt, result, err)
		}
	}
	if calls := client.callCount(); calls != 2 {
		t.Fatalf("client calls = %d, want 2 when token has under one second left", calls)
	}
	if cache.entries.Len() != 0 {
		t.Fatalf("cache entries = %d, want 0", cache.entries.Len())
	}
}

func TestGrantCacheEnforcesHardCapacityByEvictingLeastRecentlyUsedEntry(t *testing.T) {
	now := time.Now().UTC()
	client := &authorizationClientStub{grants: []authidentity.OrganizationGrant{validGrant("org-a")}}
	cache := newGrantCache(func() time.Time { return now }, 2)
	resolver := NewGrantResolver(client, cache)
	request := GrantRequest{BearerToken: "token", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(time.Minute)}

	for _, subject := range []string{"subject-1", "subject-2"} {
		request.Subject = subject
		if _, err := resolver.Grants(context.Background(), request); err != nil {
			t.Fatalf("warm %s: %v", subject, err)
		}
	}
	request.Subject = "subject-1"
	if _, err := resolver.Grants(context.Background(), request); err != nil {
		t.Fatalf("touch subject-1: %v", err)
	}
	request.Subject = "subject-3"
	if _, err := resolver.Grants(context.Background(), request); err != nil {
		t.Fatalf("insert subject-3: %v", err)
	}

	if got := cache.entries.Len(); got != 2 {
		t.Fatalf("cache entries = %d, want hard capacity 2", got)
	}
	if cache.entries.Get(grantCacheKey{subject: "subject-2", projectID: "project-a", contractVersion: "v1"}) != nil {
		t.Fatal("least-recently-used subject-2 entry survived capacity eviction")
	}
	if cache.entries.Get(grantCacheKey{subject: "subject-1", projectID: "project-a", contractVersion: "v1"}) == nil {
		t.Fatal("recently accessed subject-1 entry was evicted")
	}
}

func TestGrantCachePutGloballyRemovesExpiredOneOffSubjectEntries(t *testing.T) {
	now := time.Now().UTC()
	cache := newGrantCache(nil, 8)
	expiredKey := grantCacheKey{subject: "one-off-subject", projectID: "project-a", contractVersion: "v1"}
	cache.entries.Set(expiredKey, grantCacheEntry{expiresAt: now.Add(5 * time.Millisecond)}, 5*time.Millisecond)
	time.Sleep(25 * time.Millisecond)

	cache.put(GrantRequest{
		Subject: "active-subject", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: time.Now().Add(time.Minute),
	}, []authidentity.OrganizationGrant{validGrant("org-a")})

	if got := cache.entries.Len(); got != 1 {
		t.Fatalf("cache entries after global expiry cleanup = %d, want 1", got)
	}
	if cache.entries.Get(expiredKey) != nil {
		t.Fatal("expired one-off subject entry survived unrelated put lifecycle cleanup")
	}
}

func TestGrantCacheInvalidatesEveryContractVersionForSubjectAndProject(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	client := &authorizationClientStub{grantsByProject: map[string][]authidentity.OrganizationGrant{
		"project-a": {validGrant("org-a")},
		"project-b": {validGrantForProject("org-b", "project-b")},
	}}
	cache := NewGrantCache(func() time.Time { return now })
	resolver := NewGrantResolver(client, cache)
	requests := []GrantRequest{
		{BearerToken: "token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(time.Minute)},
		{BearerToken: "token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v2", TokenExpiresAt: now.Add(time.Minute)},
		{BearerToken: "token", Subject: "subject-a", ProjectID: "project-b", ContractVersion: "v1", TokenExpiresAt: now.Add(time.Minute)},
	}
	for _, request := range requests {
		if _, err := resolver.Grants(context.Background(), request); err != nil {
			t.Fatalf("warm Grants() error = %v", err)
		}
	}
	cache.Invalidate("subject-a", "project-a")

	for _, request := range requests[:2] {
		result, err := resolver.Grants(context.Background(), request)
		if err != nil || result.Source != GrantLive {
			t.Fatalf("invalidated Grants() = (%+v, %v), want live result", result, err)
		}
	}
	result, err := resolver.Grants(context.Background(), requests[2])
	if err != nil || result.Source != GrantReadCached {
		t.Fatalf("unrelated Grants() = (%+v, %v), want cached result", result, err)
	}
}

func TestGrantCacheNormalizesAndProtectsCachedGrants(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	client := &authorizationClientStub{grants: []authidentity.OrganizationGrant{
		{OrganizationID: "org-b", OrganizationName: " Beta ", ProjectID: " project-a ", Roles: []string{"viewer", " admin ", "viewer"}},
		{OrganizationID: "org-a", OrganizationName: " Alpha ", ProjectID: "project-a", Roles: []string{"operator", "admin"}},
	}}
	resolver := NewGrantResolver(client, NewGrantCache(func() time.Time { return now }))
	request := GrantRequest{BearerToken: "token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(time.Minute)}

	first, err := resolver.Grants(context.Background(), request)
	if err != nil {
		t.Fatalf("initial Grants() error = %v", err)
	}
	if got, want := first.Grants, []authidentity.OrganizationGrant{
		{OrganizationID: "org-a", OrganizationName: "Alpha", ProjectID: "project-a", Roles: []string{"admin", "operator"}},
		{OrganizationID: "org-b", OrganizationName: "Beta", ProjectID: "project-a", Roles: []string{"admin", "viewer"}},
	}; !equalGrants(got, want) {
		t.Fatalf("initial Grants() grants = %#v, want %#v", got, want)
	}
	first.Grants[0].OrganizationID = "mutated"
	first.Grants[0].Roles[0] = "mutated"

	second, err := resolver.Grants(context.Background(), request)
	if err != nil || second.Source != GrantReadCached {
		t.Fatalf("cached Grants() = (%+v, %v), want cached normalized result", second, err)
	}
	if got, want := second.Grants[0], (authidentity.OrganizationGrant{OrganizationID: "org-a", OrganizationName: "Alpha", ProjectID: "project-a", Roles: []string{"admin", "operator"}}); !equalGrants([]authidentity.OrganizationGrant{got}, []authidentity.OrganizationGrant{want}) {
		t.Fatalf("cached grant = %#v, want %#v", got, want)
	}
}

func TestGrantCacheRejectsConflictingDuplicateOrganizationNamesWithoutCaching(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	request := GrantRequest{BearerToken: "token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(time.Minute)}
	first := authidentity.OrganizationGrant{OrganizationID: "org-a", OrganizationName: " Alpha ", ProjectID: "project-a", Roles: []string{"admin"}}
	second := authidentity.OrganizationGrant{OrganizationID: "org-a", OrganizationName: "Beta", ProjectID: "project-a", Roles: []string{"viewer"}}

	for _, testCase := range []struct {
		name   string
		grants []authidentity.OrganizationGrant
	}{
		{name: "alpha then beta", grants: []authidentity.OrganizationGrant{first, second}},
		{name: "beta then alpha", grants: []authidentity.OrganizationGrant{second, first}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			client := &authorizationClientStub{grants: testCase.grants}
			resolver := NewGrantResolver(client, NewGrantCache(func() time.Time { return now }))
			for attempt := 0; attempt < 2; attempt++ {
				_, err := resolver.Grants(context.Background(), request)
				if !errors.Is(err, ErrAuthorizationDependencyUnavailable) {
					t.Fatalf("Grants() attempt %d error = %v, want authorization dependency error", attempt, err)
				}
			}
			if calls := client.callCount(); calls != 2 {
				t.Fatalf("client calls = %d, want 2 because conflicting names must not cache", calls)
			}
		})
	}
}

func TestGrantCacheDoesNotCacheDependencyFailuresOrMalformedGrants(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	request := GrantRequest{BearerToken: "token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(time.Minute)}

	testCases := []struct {
		name   string
		client *authorizationClientStub
	}{
		{name: "dependency failure", client: &authorizationClientStub{err: errors.New("ZITADEL unavailable")}},
		{name: "blank organization id", client: &authorizationClientStub{grants: []authidentity.OrganizationGrant{{OrganizationID: " ", ProjectID: "project-a", Roles: []string{"viewer"}}}}},
		{name: "blank project id", client: &authorizationClientStub{grants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: " ", Roles: []string{"viewer"}}}}},
		{name: "empty roles", client: &authorizationClientStub{grants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project-a"}}}},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewGrantResolver(tt.client, NewGrantCache(func() time.Time { return now }))
			for attempt := 0; attempt < 2; attempt++ {
				_, err := resolver.Grants(context.Background(), request)
				if !errors.Is(err, ErrAuthorizationDependencyUnavailable) {
					t.Fatalf("Grants() error = %v, want authorization dependency error", err)
				}
			}
			if calls := tt.client.callCount(); calls != 2 {
				t.Fatalf("client calls = %d, want 2 because invalid result must not cache", calls)
			}
		})
	}
}

func TestGrantCacheCachesAuthoritativeEmptyResultAndSelectionFailsClosed(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	client := &authorizationClientStub{grants: []authidentity.OrganizationGrant{}}
	resolver := NewGrantResolver(client, NewGrantCache(func() time.Time { return now }))
	request := GrantRequest{BearerToken: "token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(time.Minute)}

	first, err := resolver.Grants(context.Background(), request)
	if err != nil || first.Source != GrantLive || len(first.Grants) != 0 {
		t.Fatalf("initial Grants() = (%+v, %v), want authoritative live empty grants", first, err)
	}
	if _, err := SelectOrganization("", "org-a", first.Grants); !errors.Is(err, ErrOrganizationAccessRevoked) {
		t.Fatalf("SelectOrganization(empty grants) error = %v, want revoked access", err)
	}
	second, err := resolver.Grants(context.Background(), request)
	if err != nil || second.Source != GrantReadCached || len(second.Grants) != 0 {
		t.Fatalf("cached Grants() = (%+v, %v), want cached authoritative empty grants", second, err)
	}
	if calls := client.callCount(); calls != 1 {
		t.Fatalf("client calls = %d, want 1", calls)
	}
}

func TestGrantCacheSupportsConcurrentCachedReads(t *testing.T) {
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	client := &authorizationClientStub{grants: []authidentity.OrganizationGrant{validGrant("org-a")}}
	resolver := NewGrantResolver(client, NewGrantCache(func() time.Time { return now }))
	request := GrantRequest{BearerToken: "token", Subject: "subject-a", ProjectID: "project-a", ContractVersion: "v1", TokenExpiresAt: now.Add(time.Minute)}
	if _, err := resolver.Grants(context.Background(), request); err != nil {
		t.Fatalf("warm Grants() error = %v", err)
	}

	var group sync.WaitGroup
	errors := make(chan error, 24)
	for worker := 0; worker < 24; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := resolver.Grants(context.Background(), request)
			if err != nil {
				errors <- err
				return
			}
			if result.Source != GrantReadCached || len(result.Grants) != 1 || result.Grants[0].OrganizationID != "org-a" {
				errors <- fmt.Errorf("unexpected concurrent result: %+v", result)
			}
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if calls := client.callCount(); calls != 1 {
		t.Fatalf("client calls = %d, want 1", calls)
	}
}

type authorizationClientStub struct {
	mu              sync.Mutex
	grants          []authidentity.OrganizationGrant
	grantsByProject map[string][]authidentity.OrganizationGrant
	err             error
	calls           int
}

func (stub *authorizationClientStub) ListOwnProjectAuthorizations(_ context.Context, _ string, _ string, projectID string) ([]authidentity.OrganizationGrant, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.calls++
	if stub.err != nil {
		return nil, stub.err
	}
	if stub.grantsByProject != nil {
		return cloneGrants(stub.grantsByProject[projectID]), nil
	}
	return cloneGrants(stub.grants), nil
}

func (stub *authorizationClientStub) callCount() int {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.calls
}

func (stub *authorizationClientStub) setGrants(grants []authidentity.OrganizationGrant) {
	stub.mu.Lock()
	stub.grants = cloneGrants(grants)
	stub.err = nil
	stub.mu.Unlock()
}

func (stub *authorizationClientStub) setError(err error) {
	stub.mu.Lock()
	stub.err = err
	stub.mu.Unlock()
}

func validGrant(organizationID string) authidentity.OrganizationGrant {
	return validGrantForProject(organizationID, "project-a")
}

func validGrantForProject(organizationID string, projectID string) authidentity.OrganizationGrant {
	return authidentity.OrganizationGrant{OrganizationID: organizationID, OrganizationName: organizationID + " name", ProjectID: projectID, Roles: []string{"viewer"}}
}

func equalGrants(left, right []authidentity.OrganizationGrant) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].OrganizationID != right[index].OrganizationID || left[index].OrganizationName != right[index].OrganizationName || left[index].ProjectID != right[index].ProjectID || len(left[index].Roles) != len(right[index].Roles) {
			return false
		}
		for roleIndex := range left[index].Roles {
			if left[index].Roles[roleIndex] != right[index].Roles[roleIndex] {
				return false
			}
		}
	}
	return true
}

func contains(value, fragment string) bool {
	for start := 0; start+len(fragment) <= len(value); start++ {
		if value[start:start+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
