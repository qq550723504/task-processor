package ownerreconcile

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const legacyOrganizationMetadataQuery = `SELECT org_id::text, convert_from(value, 'UTF8') AS legacy_tenant_id FROM projections.org_metadata2 WHERE key = 'yudao_tenant_id' AND owner_removed = false`

const legacyUserMetadataQuery = `SELECT user_id::text, resource_owner::text, max(convert_from(value, 'UTF8')) FILTER (WHERE key = 'yudao_user_id') AS legacy_user_id, max(convert_from(value, 'UTF8')) FILTER (WHERE key = 'yudao_tenant_id') AS legacy_tenant_id FROM projections.user_metadata5 WHERE key IN ('yudao_user_id', 'yudao_tenant_id') GROUP BY user_id, resource_owner`

func LoadLegacyIdentities(ctx context.Context, queryer Queryer) ([]LegacyIdentity, error) {
	if queryer == nil {
		return nil, errors.New("legacy identity metadata database is unavailable")
	}
	organizations, err := loadLegacyOrganizations(ctx, queryer)
	if err != nil {
		return nil, err
	}
	rows, err := queryer.QueryContext(ctx, legacyUserMetadataQuery)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, errors.New("query legacy user metadata failed")
	}
	defer rows.Close()

	identities := make([]LegacyIdentity, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		var subject, resourceOwner, legacyUserID, legacyTenantID string
		if err := rows.Scan(&subject, &resourceOwner, &legacyUserID, &legacyTenantID); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, errors.New("scan legacy user metadata failed")
		}
		subject = strings.TrimSpace(subject)
		resourceOwner = strings.TrimSpace(resourceOwner)
		legacyUserID = strings.TrimSpace(legacyUserID)
		legacyTenantID = strings.TrimSpace(legacyTenantID)
		if subject == "" || resourceOwner == "" || legacyUserID == "" || legacyTenantID == "" {
			return nil, errors.New("legacy user metadata contains an incomplete mapping")
		}
		if organizations[resourceOwner] != legacyTenantID {
			return nil, errors.New("legacy user metadata contains a tenant mismatch")
		}
		key := legacyIdentityKey(legacyTenantID, legacyUserID)
		if _, exists := seen[key]; exists {
			return nil, errors.New("legacy user metadata contains a duplicate mapping")
		}
		seen[key] = struct{}{}
		identities = append(identities, LegacyIdentity{TenantID: legacyTenantID, LegacyUserID: legacyUserID, Subject: subject})
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, errors.New("iterate legacy user metadata failed")
	}
	return identities, nil
}

func loadLegacyOrganizations(ctx context.Context, queryer Queryer) (map[string]string, error) {
	rows, err := queryer.QueryContext(ctx, legacyOrganizationMetadataQuery)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, errors.New("query legacy organization metadata failed")
	}
	defer rows.Close()
	organizations := make(map[string]string)
	legacyTenantOrganizations := make(map[string]string)
	for rows.Next() {
		var organizationID, legacyTenantID string
		if err := rows.Scan(&organizationID, &legacyTenantID); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, errors.New("scan legacy organization metadata failed")
		}
		organizationID = strings.TrimSpace(organizationID)
		legacyTenantID = strings.TrimSpace(legacyTenantID)
		if organizationID == "" || legacyTenantID == "" {
			return nil, errors.New("legacy organization metadata contains an incomplete mapping")
		}
		if previous, exists := organizations[organizationID]; exists && previous != legacyTenantID {
			return nil, fmt.Errorf("legacy organization metadata contains a duplicate organization mapping")
		}
		if previous, exists := legacyTenantOrganizations[legacyTenantID]; exists && previous != organizationID {
			return nil, errors.New("legacy organization metadata contains an ambiguous tenant mapping")
		}
		organizations[organizationID] = legacyTenantID
		legacyTenantOrganizations[legacyTenantID] = organizationID
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, errors.New("iterate legacy organization metadata failed")
	}
	return organizations, nil
}
