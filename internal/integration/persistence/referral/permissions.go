package referral

import (
	"context"
	d "task-processor/internal/referral"
	"time"

	"gorm.io/gorm"
)

// VerifyPermissions measures effective privileges (including inherited grants),
// not the role's name or a declaration in configuration.
func VerifyPermissions(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil {
		return d.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var safe bool
	err := db.WithContext(ctx).Raw(`SELECT NOT (rolsuper OR rolcreaterole OR rolcreatedb OR rolbypassrls)
 AND NOT has_schema_privilege(current_user,'public','CREATE')
 AND NOT has_database_privilege(current_user,current_database(),'CREATE,TEMP')
 AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND pg_has_role(current_user,c.relowner,'MEMBER'))
 FROM pg_roles WHERE rolname=current_user`).Scan(&safe).Error
	if err != nil || !safe {
		return d.ErrUnavailable
	}
	for _, table := range []string{"referral_codes", "registration_intents", "referral_relations", "referral_receipts", "registration_admission_buckets", "referral_earning_claims", "referral_earnings_ledger", "referral_refund_operations", "referral_earnings_projection", "referral_withdrawals", "referral_withdrawal_operations", "referral_earnings_audit_events"} {
		for _, permission := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
			want := permission == "SELECT" || permission == "INSERT" || (table == "registration_admission_buckets" && (permission == "UPDATE" || permission == "DELETE")) || (table == "registration_intents" && permission == "UPDATE") || (table == "referral_earning_claims" && permission == "UPDATE") || (table == "referral_earnings_projection" && permission == "UPDATE") || (table == "referral_withdrawals" && permission == "UPDATE")
			var have bool
			if err = db.WithContext(ctx).Raw("SELECT has_table_privilege(current_user,?,?)", "public."+table, permission).Scan(&have).Error; err != nil || have != want {
				return d.ErrUnavailable
			}
		}
	}
	var invalid int64
	err = db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_attribute WHERE attrelid IN
	 ('public.registration_intents'::regclass,'public.referral_codes'::regclass,'public.referral_relations'::regclass,'public.referral_receipts'::regclass,'public.registration_admission_buckets'::regclass,'public.referral_earning_claims'::regclass,'public.referral_earnings_ledger'::regclass,'public.referral_refund_operations'::regclass,'public.referral_earnings_projection'::regclass,'public.referral_withdrawals'::regclass,'public.referral_withdrawal_operations'::regclass,'public.referral_earnings_audit_events'::regclass)
 AND attnum>0 AND NOT attisdropped AND
 (has_column_privilege(current_user,attrelid,attname,'REFERENCES') OR
 has_column_privilege(current_user,attrelid,attname,'UPDATE') <>
	 (attrelid='public.registration_admission_buckets'::regclass OR attrelid='public.referral_earning_claims'::regclass OR attrelid='public.referral_earnings_projection'::regclass OR attrelid='public.referral_withdrawals'::regclass OR (attrelid='public.registration_intents'::regclass AND attname IN ('state','ciphertext','lease_until'))))`).Scan(&invalid).Error
	if err != nil || invalid != 0 {
		return d.ErrUnavailable
	}
	return nil
}
