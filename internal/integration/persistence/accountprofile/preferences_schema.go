package accountprofile

import (
	"context"
	"database/sql"
	"errors"
	"gorm.io/gorm"
)

func InstallPreferencesTx(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return errors.New("preferences schema transaction unavailable")
	}
	_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS public.account_user_preferences (
 user_id varchar(128) PRIMARY KEY CHECK (octet_length(user_id) BETWEEN 1 AND 128 AND user_id=btrim(user_id)),
 country varchar(128) NOT NULL DEFAULT '',
 province varchar(128) NOT NULL DEFAULT '',
 city varchar(128) NOT NULL DEFAULT '',
 updated_at timestamptz NOT NULL
 )`)
	return err
}

func VerifyPreferencesSchema(ctx context.Context, db *gorm.DB) error {
	var count int64
	err := db.WithContext(ctx).Raw(`SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='account_user_preferences' AND is_nullable='NO' AND ((column_name IN ('user_id','country','province','city') AND data_type='character varying' AND character_maximum_length=128) OR (column_name='updated_at' AND data_type='timestamp with time zone'))`).Scan(&count).Error
	if err != nil || count != 5 {
		return errors.New("preferences schema unavailable")
	}
	var primary string
	err = db.WithContext(ctx).Raw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='public.account_user_preferences'::regclass AND contype='p'`).Scan(&primary).Error
	if err != nil || primary != "PRIMARY KEY (user_id)" {
		return errors.New("preferences schema key invalid")
	}
	return nil
}
