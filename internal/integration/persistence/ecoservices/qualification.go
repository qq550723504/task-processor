package ecoservices

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"time"
)

// VerifyQualificationRuntime refuses to detach any retained merchant or
// financial work from its original channel and recovery consumers.
func (r *Repository) VerifyQualificationRuntime(ctx context.Context) error {
	if r == nil || r.db == nil || ctx == nil {
		return errors.New("ecoservices qualification owner unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, check := range []struct{ table, predicate string }{
		{"ecoservices_listings", ""}, {"ecoservices_requests", ""}, {"ecoservices_financial_commands", ""},
		{"ecoservices_merchant_intents", ""}, {"ecoservices_merchant_bindings", ""}, {"ecoservices_merchant_progress", ""},
		{"ecoservices_files", "parent_kind <> 'APPLICATION' OR parent_kind IS NULL"},
		{"ecoservices_operations", "kind NOT IN ('application_submit','application_review','application_reject','agreement_accept') OR kind IS NULL"},
		{"ecoservices_versions", "kind <> 'APPLICATION' OR kind IS NULL"},
	} {
		q := r.db.WithContext(ctx).Table(check.table).Select("1").Limit(1)
		if check.predicate != "" {
			q = q.Where(check.predicate)
		}
		var found int
		if err := q.Scan(&found).Error; err != nil {
			return err
		}
		if found != 0 {
			return errors.New("ecoservices qualification mode cannot consume retained merchant or service facts")
		}
	}
	var rows []applicationRow
	return r.db.WithContext(ctx).FindInBatches(&rows, 100, func(_ *gorm.DB, _ int) error {
		for _, row := range rows {
			app, err := applicationFact(row)
			if err != nil {
				return err
			}
			if app.State != row.State || app.State != "SUBMITTED" && app.State != "APPROVED" && app.State != "REJECTED" || app.MerchantID != "" || app.OnboardingState != "NOT_STARTED" || app.CurrentMerchantRevisionID != "" || app.CurrentMerchantRevisionVersion != 0 {
				return errors.New("ecoservices qualification mode requires original unsubmitted merchant state")
			}
		}
		return nil
	}).Error
}
