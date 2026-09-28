package orgresourceadapter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/ledger/orgresource"
)

var _ orgresource.BalanceReader = (*GormRepository)(nil)

func (r *GormRepository) ReadBalances(ctx context.Context, organization string) (orgresource.Balances, error) {
	if ctx == nil || organization == "" || len(organization) > 128 || strings.TrimSpace(organization) != organization {
		return orgresource.Balances{}, orgresource.ErrInvalidInput
	}
	// One statement observes a consistent bucket/debt snapshot without write locks.
	// The independent debt join also detects debt whose bucket is missing.
	var rows []struct {
		ResourceType                  orgresource.ResourceType
		BucketOrganization            *string
		Available, Reserved, Consumed *int64
		DebtOrganization              *string
		Amount                        *int64
		UpdatedAt                     *time.Time
	}
	err := r.db.WithContext(ctx).Raw(`
 WITH resource_types(resource_type) AS (VALUES ('store_renewal_period'),('ai_point'),('data_row'))
 SELECT t.resource_type, b.organization_id AS bucket_organization,
 b.available, b.reserved, b.consumed, b.updated_at,
 d.organization_id AS debt_organization, d.amount
 FROM resource_types t
 LEFT JOIN saas_organization_resource_buckets b ON b.organization_id = ? AND b.resource_type = t.resource_type
 LEFT JOIN saas_organization_resource_debts d ON d.organization_id = ? AND d.resource_type = t.resource_type`, organization, organization).Scan(&rows).Error
	if err != nil {
		return orgresource.Balances{}, fmt.Errorf("%w: %w", orgresource.ErrBalanceUnavailable, err)
	}
	if len(rows) != 3 {
		return orgresource.Balances{}, orgresource.ErrBalanceUnavailable
	}
	indexed := make(map[orgresource.ResourceType]orgresource.Balance, 3)
	for _, row := range rows {
		unit := ""
		switch row.ResourceType {
		case orgresource.ResourceStoreRenewalPeriod:
			unit = "period"
		case orgresource.ResourceAIPoint:
			unit = "point"
		case orgresource.ResourceDataRow:
			unit = "row"
		default:
			return orgresource.Balances{}, orgresource.ErrBalanceUnavailable
		}
		if _, exists := indexed[row.ResourceType]; exists {
			return orgresource.Balances{}, orgresource.ErrBalanceUnavailable
		}
		balance := orgresource.Balance{ResourceType: row.ResourceType, Unit: unit, State: "not_recorded"}
		if row.BucketOrganization == nil {
			if row.DebtOrganization != nil {
				return orgresource.Balances{}, orgresource.ErrBalanceUnavailable
			}
		} else {
			if *row.BucketOrganization != organization || row.Available == nil || row.Reserved == nil || row.Consumed == nil || *row.Available < 0 || *row.Reserved < 0 || *row.Consumed < 0 || row.UpdatedAt == nil || row.UpdatedAt.IsZero() || row.UpdatedAt.Year() < 1 || row.UpdatedAt.Year() > 9999 {
				return orgresource.Balances{}, orgresource.ErrBalanceUnavailable
			}
			debt := int64(0)
			if row.DebtOrganization != nil {
				if *row.DebtOrganization != organization || row.Amount == nil || *row.Amount < 0 {
					return orgresource.Balances{}, orgresource.ErrBalanceUnavailable
				}
				debt = *row.Amount
			}
			if debt > 0 && *row.Available > 0 {
				return orgresource.Balances{}, orgresource.ErrBalanceUnavailable
			}
			available, reserved, consumed, amount := strconv.FormatInt(*row.Available, 10), strconv.FormatInt(*row.Reserved, 10), strconv.FormatInt(*row.Consumed, 10), strconv.FormatInt(debt, 10)
			updated := row.UpdatedAt.UTC()
			balance.State = "recorded"
			balance.Available = &available
			balance.Reserved = &reserved
			balance.Consumed = &consumed
			balance.Debt = &amount
			balance.UpdatedAt = &updated
		}
		indexed[row.ResourceType] = balance
	}
	result := orgresource.Balances{SchemaVersion: "organization-resource-balances-v1", OrganizationID: organization, ObservedAt: time.Now().UTC()}
	for _, kind := range []orgresource.ResourceType{orgresource.ResourceStoreRenewalPeriod, orgresource.ResourceAIPoint, orgresource.ResourceDataRow} {
		result.Resources = append(result.Resources, indexed[kind])
	}
	return result, nil
}
