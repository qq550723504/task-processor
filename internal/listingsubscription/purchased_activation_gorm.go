package listingsubscription

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *GormRepository) ResolvePurchasablePlan(ctx context.Context, planCode string) (PurchasedPlanSnapshot, error) {
	if r == nil || r.db == nil || strings.TrimSpace(planCode) == "" {
		return PurchasedPlanSnapshot{}, ErrPurchasablePlanUnavailable
	}
	var snapshot PurchasedPlanSnapshot
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		bundle, found, err := purchasedPlanBundle(tx, strings.TrimSpace(planCode))
		if err != nil {
			return err
		}
		if !found || !bundle.Plan.Active {
			return ErrPurchasablePlanUnavailable
		}
		snapshot = PurchasedPlanSnapshot{PlanCode: bundle.Plan.Code, DisplayName: bundle.Plan.Name, Fingerprint: planSemanticFingerprint(bundle.Plan, bundle.Modules)}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrPurchasablePlanUnavailable) {
			return PurchasedPlanSnapshot{}, err
		}
		return PurchasedPlanSnapshot{}, ErrPurchasedPlanActivationUnavailable
	}
	return snapshot, nil
}

func (r *GormRepository) ReadPurchasedSubscriptionState(ctx context.Context, organizationID string, at time.Time) (PurchasedSubscriptionState, error) {
	if r == nil || r.db == nil || !boundedPurchasedIdentifier(organizationID) || at.IsZero() {
		return PurchasedSubscriptionState{}, ErrPurchasedPlanActivationConflict
	}
	var row tenantSubscriptionRow
	err := r.db.WithContext(ctx).Where("tenant_id = ?", organizationID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PurchasedSubscriptionState{}, nil
	}
	if err != nil {
		return PurchasedSubscriptionState{}, ErrPurchasedPlanActivationUnavailable
	}
	if row.Status != StatusActive && row.Status != StatusTrialing {
		return PurchasedSubscriptionState{}, nil
	}
	at = at.UTC()
	if row.ExpiresAt != nil && !at.Before(row.ExpiresAt.UTC()) {
		return PurchasedSubscriptionState{}, nil
	}
	return PurchasedSubscriptionState{PlanCode: row.PlanCode, BlocksPurchase: true}, nil
}

func (r *GormRepository) ActivatePurchasedPlan(ctx context.Context, input PurchasedPlanActivationInput, decidedAt time.Time) (PurchasedPlanActivationResult, error) {
	if r == nil || r.db == nil || validatePurchasedPlanActivationInput(input) != nil || decidedAt.IsZero() {
		return PurchasedPlanActivationResult{}, ErrPurchasedPlanActivationConflict
	}
	input = normalizePurchasedPlanActivationInput(input)
	decidedAt = decidedAt.UTC().Truncate(time.Microsecond)
	requestFingerprint := purchasedActivationRequestFingerprint(input)
	var result PurchasedPlanActivationResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fence := purchasedPlanActivationFenceRow{OrganizationID: input.OrganizationID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&fence).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ?", input.OrganizationID).Take(&fence).Error; err != nil {
			return err
		}

		existing, found, err := readPurchasedActivationBySource(tx, input.SourceType, input.SourceID)
		if err != nil {
			return err
		}
		if found {
			if existing.OperationID != input.OperationID || existing.OrganizationID != input.OrganizationID || existing.RequestFingerprint != requestFingerprint {
				return ErrPurchasedPlanActivationConflict
			}
			result = purchasedActivationResult(existing, true)
			return nil
		}
		var operationCollision purchasedPlanActivationRow
		if err := tx.Where("operation_id = ?", input.OperationID).Take(&operationCollision).Error; err == nil {
			return ErrPurchasedPlanActivationConflict
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if active, err := hasBlockingPurchasedSubscription(tx, input.OrganizationID, decidedAt); err != nil {
			return err
		} else if active {
			row := rejectedPurchasedActivationRow(input, requestFingerprint, PurchasedPlanActivationActiveSubscriptionExists, decidedAt)
			if err := createPurchasedActivationDecision(tx, row, input.ActorID); err != nil {
				return err
			}
			result = purchasedActivationResult(row, false)
			return nil
		}

		bundle, found, err := purchasedPlanBundle(tx, input.PlanCode)
		if err != nil {
			return err
		}
		if !found || !bundle.Plan.Active || planSemanticFingerprint(bundle.Plan, bundle.Modules) != input.PlanFingerprint {
			row := rejectedPurchasedActivationRow(input, requestFingerprint, PurchasedPlanActivationPlanChanged, decidedAt)
			if err := createPurchasedActivationDecision(tx, row, input.ActorID); err != nil {
				return err
			}
			result = purchasedActivationResult(row, false)
			return nil
		}

		startsAt := decidedAt
		expiresAt := decidedAt.AddDate(0, input.TermMonths, 0)
		subscription := tenantSubscriptionRow{TenantID: input.OrganizationID, PlanCode: input.PlanCode, Status: StatusActive, StartsAt: &startsAt, ExpiresAt: &expiresAt}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "tenant_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"plan_code": input.PlanCode, "status": StatusActive, "starts_at": startsAt, "expires_at": expiresAt, "updated_at": decidedAt,
			}),
		}).Create(&subscription).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ?", input.OrganizationID).Take(&subscription).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ?", input.OrganizationID).Delete(&tenantEntitlementRow{}).Error; err != nil {
			return err
		}
		for _, module := range bundle.Modules {
			limitsJSON, err := marshalLimits(module.Limits)
			if err != nil {
				return err
			}
			entitlement := tenantEntitlementRow{TenantID: input.OrganizationID, ModuleCode: module.ModuleCode, Status: StatusActive, StartsAt: &startsAt, ExpiresAt: &expiresAt, LimitsJSON: limitsJSON, CreatedAt: decidedAt, UpdatedAt: decidedAt}
			if err := tx.Create(&entitlement).Error; err != nil {
				return err
			}
		}
		entitlementFingerprint := entitlementSetFingerprint(input.OrganizationID, input.PlanCode, startsAt, expiresAt, bundle.Modules)
		subscriptionID := subscription.ID
		row := purchasedPlanActivationRow{
			OperationID: input.OperationID, OrganizationID: input.OrganizationID, SourceType: input.SourceType, SourceID: input.SourceID,
			RequestFingerprint: requestFingerprint, PlanCode: input.PlanCode, PlanFingerprint: input.PlanFingerprint, TermMonths: input.TermMonths,
			Outcome: string(PurchasedPlanActivationActivated), SubscriptionID: &subscriptionID, StartsAt: &startsAt, ExpiresAt: &expiresAt,
			EntitlementSetFingerprint: entitlementFingerprint, DecidedAt: decidedAt,
		}
		if err := createPurchasedActivationDecision(tx, row, input.ActorID); err != nil {
			return err
		}
		result = purchasedActivationResult(row, false)
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrPurchasedPlanActivationConflict) {
			return PurchasedPlanActivationResult{}, err
		}
		return PurchasedPlanActivationResult{}, ErrPurchasedPlanActivationUnavailable
	}
	return result, nil
}

func (r *GormRepository) ReadPurchasedPlanActivation(ctx context.Context, organizationID, sourceID string) (PurchasedPlanActivationResult, error) {
	if r == nil || r.db == nil || !boundedPurchasedIdentifier(organizationID) || !boundedPurchasedIdentifier(sourceID) {
		return PurchasedPlanActivationResult{}, ErrPurchasedPlanActivationConflict
	}
	row, found, err := readPurchasedActivationBySource(r.db.WithContext(ctx), PurchasedPlanSourceCommercialOrder, sourceID)
	if err != nil {
		return PurchasedPlanActivationResult{}, ErrPurchasedPlanActivationUnavailable
	}
	if !found || row.OrganizationID != organizationID {
		return PurchasedPlanActivationResult{}, ErrPurchasedPlanActivationNotFound
	}
	return purchasedActivationResult(row, true), nil
}

func purchasedPlanBundle(tx *gorm.DB, planCode string) (PlanBundle, bool, error) {
	var planRow subscriptionPlanRow
	if err := tx.Where("code = ?", planCode).Take(&planRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PlanBundle{}, false, nil
		}
		return PlanBundle{}, false, err
	}
	var rows []subscriptionPlanModuleRow
	if err := tx.Where("plan_code = ?", planCode).Order("module_code asc").Find(&rows).Error; err != nil {
		return PlanBundle{}, false, err
	}
	modules := make([]PlanModule, 0, len(rows))
	for _, row := range rows {
		limits, err := unmarshalLimits(row.LimitsJSON)
		if err != nil {
			return PlanBundle{}, false, err
		}
		modules = append(modules, PlanModule{PlanCode: row.PlanCode, ModuleCode: row.ModuleCode, Limits: limits, SortOrder: row.SortOrder})
	}
	return PlanBundle{Plan: Plan{Code: planRow.Code, Name: planRow.Name, Description: planRow.Description, SortOrder: planRow.SortOrder, Active: planRow.Active, CreatedAt: planRow.CreatedAt, UpdatedAt: planRow.UpdatedAt}, Modules: modules}, true, nil
}

func hasBlockingPurchasedSubscription(tx *gorm.DB, organizationID string, now time.Time) (bool, error) {
	var row tenantSubscriptionRow
	err := tx.Where("tenant_id = ?", organizationID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if row.Status != StatusActive && row.Status != StatusTrialing {
		return false, nil
	}
	return row.ExpiresAt == nil || now.Before(row.ExpiresAt.UTC()), nil
}

func readPurchasedActivationBySource(tx *gorm.DB, sourceType, sourceID string) (purchasedPlanActivationRow, bool, error) {
	var row purchasedPlanActivationRow
	err := tx.Where("source_type = ? AND source_id = ?", sourceType, sourceID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return purchasedPlanActivationRow{}, false, nil
	}
	if err != nil {
		return purchasedPlanActivationRow{}, false, err
	}
	return row, true, nil
}

func rejectedPurchasedActivationRow(input PurchasedPlanActivationInput, requestFingerprint string, failure PurchasedPlanActivationFailureCode, decidedAt time.Time) purchasedPlanActivationRow {
	return purchasedPlanActivationRow{
		OperationID: input.OperationID, OrganizationID: input.OrganizationID, SourceType: input.SourceType, SourceID: input.SourceID,
		RequestFingerprint: requestFingerprint, PlanCode: input.PlanCode, PlanFingerprint: input.PlanFingerprint, TermMonths: input.TermMonths,
		Outcome: string(PurchasedPlanActivationRejected), FailureCode: string(failure), DecidedAt: decidedAt,
	}
}

func createPurchasedActivationDecision(tx *gorm.DB, row purchasedPlanActivationRow, actorID string) error {
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		SourceType                string     `json:"source_type"`
		SourceID                  string     `json:"source_id"`
		OperationID               string     `json:"operation_id"`
		PlanCode                  string     `json:"plan_code"`
		PlanFingerprint           string     `json:"plan_fingerprint"`
		RequestFingerprint        string     `json:"request_fingerprint"`
		StartsAt                  *time.Time `json:"starts_at,omitempty"`
		ExpiresAt                 *time.Time `json:"expires_at,omitempty"`
		EntitlementSetFingerprint string     `json:"entitlement_set_fingerprint,omitempty"`
		Outcome                   string     `json:"outcome"`
		FailureCode               string     `json:"failure_code,omitempty"`
	}{row.SourceType, row.SourceID, row.OperationID, row.PlanCode, row.PlanFingerprint, row.RequestFingerprint, row.StartsAt, row.ExpiresAt, row.EntitlementSetFingerprint, row.Outcome, row.FailureCode})
	if err != nil {
		return err
	}
	action := "purchased_plan_activation"
	if row.Outcome == string(PurchasedPlanActivationRejected) {
		action = "purchased_plan_activation_rejected"
	}
	return tx.Create(&auditLogRow{TenantID: row.OrganizationID, Action: action, ActorID: actorID, Reason: row.SourceID, Payload: string(payload), CreatedAt: row.DecidedAt}).Error
}

func purchasedActivationResult(row purchasedPlanActivationRow, existing bool) PurchasedPlanActivationResult {
	return PurchasedPlanActivationResult{
		OperationID: row.OperationID, OrganizationID: row.OrganizationID, SourceType: row.SourceType, SourceID: row.SourceID,
		PlanCode: row.PlanCode, PlanFingerprint: row.PlanFingerprint, ActivationRequestFingerprint: row.RequestFingerprint,
		Outcome: PurchasedPlanActivationOutcome(row.Outcome), FailureCode: PurchasedPlanActivationFailureCode(row.FailureCode),
		SubscriptionID: dereferenceInt64(row.SubscriptionID), StartsAt: clonePurchasedTime(row.StartsAt), ExpiresAt: clonePurchasedTime(row.ExpiresAt),
		EntitlementSetFingerprint: row.EntitlementSetFingerprint, DecidedAt: row.DecidedAt.UTC(), Existing: existing,
	}
}

func dereferenceInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func clonePurchasedTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
