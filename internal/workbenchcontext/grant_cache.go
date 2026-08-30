package workbenchcontext

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"task-processor/internal/authidentity"
)

const grantCacheMaximumTTL = 60 * time.Second

// GrantSource identifies whether verified grants were read from the local
// cache or obtained live from ZITADEL.
type GrantSource uint8

const (
	GrantReadCached GrantSource = iota + 1
	GrantLive
)

// AuthorizationClient is the Task 3 ZITADEL authorization boundary used to
// obtain the current subject's organization-scoped grants.
type AuthorizationClient interface {
	ListOwnProjectAuthorizations(context.Context, string, string, string) ([]authidentity.OrganizationGrant, error)
}

// GrantRequest contains request-local authorization data. BearerToken is used
// only for a live lookup and is deliberately excluded from cache state.
type GrantRequest struct {
	BearerToken     string
	Subject         string
	ProjectID       string
	ContractVersion string
	TokenExpiresAt  time.Time
}

// GrantResult contains a detached snapshot of verified organization grants.
type GrantResult struct {
	Grants []authidentity.OrganizationGrant
	Source GrantSource
}

// GrantResolver obtains verified grants with an in-process, token-bound cache.
type GrantResolver struct {
	client AuthorizationClient
	cache  *GrantCache
}

// NewGrantResolver constructs the reusable grant source used by the effective
// organization resolver. A nil cache creates a default in-process cache.
func NewGrantResolver(client AuthorizationClient, cache *GrantCache) *GrantResolver {
	if cache == nil {
		cache = NewGrantCache(nil)
	}
	return &GrantResolver{client: client, cache: cache}
}

// Grants reads a detached cached snapshot when it remains valid, otherwise it
// gets a live result from the authorization client. Errors and malformed live
// results never enter the cache.
func (resolver *GrantResolver) Grants(ctx context.Context, request GrantRequest) (GrantResult, error) {
	request = normalizeRequest(request)
	if err := validateRequest(request); err != nil {
		return GrantResult{}, err
	}
	if resolver == nil || resolver.client == nil {
		return GrantResult{}, fmt.Errorf("%w: client is not configured", ErrAuthorizationDependencyUnavailable)
	}
	if resolver.cache == nil {
		return GrantResult{}, fmt.Errorf("%w: cache is not configured", ErrAuthorizationDependencyUnavailable)
	}
	if grants, ok := resolver.cache.get(request); ok {
		return GrantResult{Grants: grants, Source: GrantReadCached}, nil
	}

	grants, err := resolver.client.ListOwnProjectAuthorizations(ctx, request.BearerToken, request.Subject, request.ProjectID)
	if err != nil {
		return GrantResult{}, fmt.Errorf("%w: live grant lookup failed", ErrAuthorizationDependencyUnavailable)
	}
	normalized, err := normalizeGrants(grants, request.ProjectID)
	if err != nil {
		return GrantResult{}, err
	}
	resolver.cache.put(request, normalized)
	return GrantResult{Grants: cloneGrants(normalized), Source: GrantLive}, nil
}

// Invalidate removes every cached contract-version entry for the subject and
// project. It is safe to call concurrently with reads.
func (resolver *GrantResolver) Invalidate(subject string, projectID string) {
	if resolver == nil || resolver.cache == nil {
		return
	}
	resolver.cache.Invalidate(subject, projectID)
}

type grantCacheKey struct {
	subject         string
	projectID       string
	contractVersion string
}

type grantCacheEntry struct {
	grants    []authidentity.OrganizationGrant
	expiresAt time.Time
}

// GrantCache stores only immutable authorization grants and their expiration;
// cache keys and values never contain bearer tokens.
type GrantCache struct {
	mu      sync.RWMutex
	now     func() time.Time
	entries map[grantCacheKey]grantCacheEntry
}

// NewGrantCache creates a concurrency-safe in-process grant cache. The clock
// parameter is injectable for deterministic expiry tests.
func NewGrantCache(now func() time.Time) *GrantCache {
	if now == nil {
		now = time.Now
	}
	return &GrantCache{now: now, entries: make(map[grantCacheKey]grantCacheEntry)}
}

func (cache *GrantCache) get(request GrantRequest) ([]authidentity.OrganizationGrant, bool) {
	key := cacheKey(request)
	now := cache.now()
	cache.mu.RLock()
	entry, ok := cache.entries[key]
	if ok && now.Before(entry.expiresAt) {
		grants := cloneGrants(entry.grants)
		cache.mu.RUnlock()
		return grants, true
	}
	cache.mu.RUnlock()
	if ok {
		cache.mu.Lock()
		entry, stillPresent := cache.entries[key]
		if stillPresent && !now.Before(entry.expiresAt) {
			delete(cache.entries, key)
		}
		cache.mu.Unlock()
	}
	return nil, false
}

func (cache *GrantCache) put(request GrantRequest, grants []authidentity.OrganizationGrant) {
	now := cache.now()
	if request.TokenExpiresAt.IsZero() {
		return
	}
	ttl := request.TokenExpiresAt.Sub(now)
	if ttl < time.Second {
		return
	}
	if ttl > grantCacheMaximumTTL {
		ttl = grantCacheMaximumTTL
	}
	expiresAt := now.Add(ttl)
	cache.mu.Lock()
	cache.entries[cacheKey(request)] = grantCacheEntry{grants: cloneGrants(grants), expiresAt: expiresAt}
	cache.mu.Unlock()
}

// Invalidate removes all contract versions belonging to one subject/project.
func (cache *GrantCache) Invalidate(subject string, projectID string) {
	if cache == nil {
		return
	}
	subject = strings.TrimSpace(subject)
	projectID = strings.TrimSpace(projectID)
	cache.mu.Lock()
	for key := range cache.entries {
		if key.subject == subject && key.projectID == projectID {
			delete(cache.entries, key)
		}
	}
	cache.mu.Unlock()
}

func normalizeRequest(request GrantRequest) GrantRequest {
	request.BearerToken = strings.TrimSpace(request.BearerToken)
	request.Subject = strings.TrimSpace(request.Subject)
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	request.ContractVersion = strings.TrimSpace(request.ContractVersion)
	if !request.TokenExpiresAt.IsZero() {
		request.TokenExpiresAt = request.TokenExpiresAt.Round(0).UTC()
	}
	return request
}

func validateRequest(request GrantRequest) error {
	if request.BearerToken == "" || request.Subject == "" || request.ProjectID == "" || request.ContractVersion == "" {
		return fmt.Errorf("%w: live grant lookup requires bearer token, subject, project ID, and contract version", ErrAuthorizationDependencyUnavailable)
	}
	return nil
}

func cacheKey(request GrantRequest) grantCacheKey {
	return grantCacheKey{subject: request.Subject, projectID: request.ProjectID, contractVersion: request.ContractVersion}
}

func normalizeGrants(grants []authidentity.OrganizationGrant, expectedProjectID string) ([]authidentity.OrganizationGrant, error) {
	byOrganization := make(map[string]authidentity.OrganizationGrant, len(grants))
	roleSets := make(map[string]map[string]struct{}, len(grants))
	for _, grant := range grants {
		grant.OrganizationID = strings.TrimSpace(grant.OrganizationID)
		grant.OrganizationName = strings.TrimSpace(grant.OrganizationName)
		grant.ProjectID = strings.TrimSpace(grant.ProjectID)
		if grant.OrganizationID == "" || grant.ProjectID == "" || grant.ProjectID != expectedProjectID {
			return nil, fmt.Errorf("%w: malformed live organization grant", ErrAuthorizationDependencyUnavailable)
		}
		roles := roleSets[grant.OrganizationID]
		if roles == nil {
			roles = make(map[string]struct{}, len(grant.Roles))
			roleSets[grant.OrganizationID] = roles
		}
		for _, role := range grant.Roles {
			if role = strings.TrimSpace(role); role != "" {
				roles[role] = struct{}{}
			}
		}
		if existing, ok := byOrganization[grant.OrganizationID]; !ok || (existing.OrganizationName == "" && grant.OrganizationName != "") {
			byOrganization[grant.OrganizationID] = authidentity.OrganizationGrant{
				OrganizationID:   grant.OrganizationID,
				OrganizationName: grant.OrganizationName,
				ProjectID:        grant.ProjectID,
			}
		}
	}

	organizationIDs := make([]string, 0, len(byOrganization))
	for organizationID := range byOrganization {
		if len(roleSets[organizationID]) == 0 {
			return nil, fmt.Errorf("%w: malformed live organization grant", ErrAuthorizationDependencyUnavailable)
		}
		organizationIDs = append(organizationIDs, organizationID)
	}
	sort.Strings(organizationIDs)
	normalized := make([]authidentity.OrganizationGrant, 0, len(organizationIDs))
	for _, organizationID := range organizationIDs {
		grant := byOrganization[organizationID]
		grant.Roles = make([]string, 0, len(roleSets[organizationID]))
		for role := range roleSets[organizationID] {
			grant.Roles = append(grant.Roles, role)
		}
		sort.Strings(grant.Roles)
		normalized = append(normalized, grant)
	}
	return normalized, nil
}

func cloneGrants(grants []authidentity.OrganizationGrant) []authidentity.OrganizationGrant {
	if grants == nil {
		return nil
	}
	cloned := make([]authidentity.OrganizationGrant, len(grants))
	for index, grant := range grants {
		cloned[index] = cloneGrant(grant)
	}
	return cloned
}
