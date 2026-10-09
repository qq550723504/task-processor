package money

import (
	"context"
	m "task-processor/internal/ledger/money"
	"time"

	"gorm.io/gorm"
)

// VerifyProviderTopUpRuntime verifies the serving role, independently of its
// configured name. Schema installation remains an explicit owner operation.
func VerifyProviderTopUpRuntime(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil {
		return m.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var safe bool
	if err := db.WithContext(ctx).Raw(`SELECT NOT (rolsuper OR rolcreaterole OR rolcreatedb OR rolbypassrls)
 AND NOT has_schema_privilege(current_user,'public','CREATE')
 AND NOT has_database_privilege(current_user,current_database(),'CREATE,TEMP')
 AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND pg_has_role(current_user,c.relowner,'MEMBER'))
 FROM pg_roles WHERE rolname=current_user`).Scan(&safe).Error; err != nil || !safe {
		return m.ErrUnavailable
	}
	mutable := map[string]bool{"ledger_organization_wallets": true, "ledger_organization_wallet_reservations": true, "ledger_provider_topup_claims": true, "ledger_topup_refund_holds": true, "ledger_service_payment_bindings": true, "ledger_service_operation_reservations": true}
	tables := []string{"ledger_payment_settlements", "ledger_refund_settlements", "ledger_chargeback_settlements", "ledger_organization_wallets", "ledger_organization_wallet_entries", "ledger_organization_wallet_reservations", "ledger_organization_wallet_reserve_decisions", "ledger_organization_topup_settlements", "ledger_organization_wallet_reversals", "ledger_provider_topup_claims", "ledger_topup_reversal_receipts", "ledger_topup_excess_reconciliations", "ledger_topup_refund_holds"}
	tables = append(tables, "ledger_channel_payment_claims", "ledger_service_payment_bindings", "ledger_service_operation_reservations", "ledger_service_effect_receipts", "ledger_service_refund_review_admissions", "ledger_service_fulfillment_admissions")
	for _, table := range tables {
		var valid bool
		err := db.WithContext(ctx).Raw(`SELECT has_table_privilege(current_user,?,'SELECT,INSERT') AND has_table_privilege(current_user,?,'SELECT') AND has_table_privilege(current_user,?,'INSERT') AND has_table_privilege(current_user,?,'UPDATE')=? AND NOT has_table_privilege(current_user,?,'DELETE,TRUNCATE,REFERENCES,TRIGGER')`, "public."+table, "public."+table, "public."+table, "public."+table, mutable[table], "public."+table).Scan(&valid).Error
		if err != nil || !valid {
			return m.ErrUnavailable
		}
	}
	// Column grants can bypass a table-level UPDATE check. Reject them on
	// immutable facts, and reject mutations outside the money owner's tables.
	var invalid int64
	err := db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relkind IN ('r','p') AND
 (has_any_column_privilege(current_user,c.oid,'REFERENCES')
 OR (c.relname NOT IN ? AND (has_any_column_privilege(current_user,c.oid,'INSERT,UPDATE') OR has_table_privilege(current_user,c.oid,'DELETE,TRUNCATE,TRIGGER')))
	 OR (c.relname IN ? AND has_any_column_privilege(current_user,c.oid,'UPDATE')))`, tables, []string{"ledger_payment_settlements", "ledger_refund_settlements", "ledger_chargeback_settlements", "ledger_organization_wallet_entries", "ledger_organization_wallet_reserve_decisions", "ledger_organization_topup_settlements", "ledger_organization_wallet_reversals", "ledger_topup_reversal_receipts", "ledger_topup_excess_reconciliations", "ledger_channel_payment_claims", "ledger_service_effect_receipts", "ledger_service_refund_review_admissions", "ledger_service_fulfillment_admissions"}).Scan(&invalid).Error
	if err != nil || invalid != 0 {
		return m.ErrUnavailable
	}
	return nil
}
