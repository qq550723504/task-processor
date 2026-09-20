package accountprofile

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"gorm.io/gorm"
)

var installationStatements = []string{`CREATE TABLE public.account_business_profiles (
    user_id VARCHAR(128) NOT NULL,
    user_role VARCHAR(128) NOT NULL DEFAULT '',
    shop_situation VARCHAR(128) NOT NULL DEFAULT '',
    factory_situation VARCHAR(128) NOT NULL DEFAULT '',
    platforms TEXT NOT NULL DEFAULT '[]',
    sites TEXT NOT NULL DEFAULT '[]',
    shop_type VARCHAR(128) NOT NULL DEFAULT '',
    services TEXT NOT NULL DEFAULT '[]',
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT account_business_profiles_pkey PRIMARY KEY (user_id),
    CONSTRAINT account_business_profiles_user_id_check CHECK (octet_length(user_id) BETWEEN 1 AND 128 AND user_id = btrim(user_id)),
    CONSTRAINT account_business_profiles_text_check CHECK (octet_length(user_role) <= 128 AND octet_length(shop_situation) <= 128 AND octet_length(factory_situation) <= 128 AND octet_length(shop_type) <= 128),
    CONSTRAINT account_business_profiles_json_check CHECK (platforms IS JSON AND sites IS JSON AND services IS JSON)
)`}

func InstallSchemaTx(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return fmt.Errorf("account profile schema transaction is nil")
	}
	for _, statement := range installationStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("install account profile schema: %w", err)
		}
	}
	return nil
}

func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("account profile database is nil")
	}
	sqlDB, err := db.DB()
	if err != nil { return fmt.Errorf("get account profile schema database: %w", err) }
	columns, err := readColumns(ctx, sqlDB)
	if err != nil { return fmt.Errorf("inspect account profile schema: %w", err) }
	if !slices.Equal(columns, expectedColumns()) { return fmt.Errorf("account profile schema mismatch: got %v", columns) }
	return nil
}

type schemaQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func expectedColumns() []string {
	return []string{"user_id|character varying|128|NO", "user_role|character varying|128|NO", "shop_situation|character varying|128|NO", "factory_situation|character varying|128|NO", "platforms|text||NO", "sites|text||NO", "shop_type|character varying|128|NO", "services|text||NO", "updated_at|timestamp with time zone||NO"}
}

func readColumns(ctx context.Context, query schemaQuery) ([]string, error) {
	rows, err := query.QueryContext(ctx, `SELECT column_name, data_type, COALESCE(character_maximum_length::text, ''), is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 ORDER BY ordinal_position`, TableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var name, kind, length, nullable string
		if err := rows.Scan(&name, &kind, &length, &nullable); err != nil {
			return nil, err
		}
		result = append(result, strings.Join([]string{name, kind, length, nullable}, "|"))
	}
	return result, rows.Err()
}
