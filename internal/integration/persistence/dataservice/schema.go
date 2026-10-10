package dataservicepersistence

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"task-processor/internal/dataservice"
)

var Tables = []string{"data_service_credentials", "data_service_commands", "data_service_quota"}

// InstallSchema is an explicit empty-install owner operation, never serving DDL.
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return dataservice.ErrUnavailable
	}
	return db.Exec(`CREATE TABLE IF NOT EXISTS data_service_credentials (
 id uuid PRIMARY KEY, organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 digest varchar(64) NOT NULL CHECK(digest ~ '^[0-9a-f]{64}$'), suffix varchar(4) NOT NULL,
 state varchar(16) NOT NULL CHECK(state IN ('ACTIVE','DISABLED','REVOKED')), revision bigint NOT NULL CHECK(revision>0),
 config_json jsonb NOT NULL CHECK(octet_length(config_json::text)<=8192), expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL,
 UNIQUE(organization_id,actor_id,id));
CREATE TABLE IF NOT EXISTS data_service_commands (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 command_key uuid NOT NULL, input_hash varchar(64) NOT NULL CHECK(input_hash ~ '^[0-9a-f]{64}$'),
 action varchar(32) NOT NULL, target_id uuid NOT NULL, receipt_json jsonb NOT NULL CHECK(octet_length(receipt_json::text)<=8192),
 created_at timestamptz NOT NULL, PRIMARY KEY(organization_id,actor_id,command_key));
CREATE TABLE IF NOT EXISTS data_service_quota (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, key_id uuid NOT NULL,
 window_kind varchar(8) NOT NULL CHECK(window_kind IN ('day','month')), window_start date NOT NULL,
 consumed_rows bigint NOT NULL DEFAULT 0 CHECK(consumed_rows>=0), reserved_rows bigint NOT NULL DEFAULT 0 CHECK(reserved_rows>=0),
 consumed_fen bigint NOT NULL DEFAULT 0 CHECK(consumed_fen>=0), reserved_fen bigint NOT NULL DEFAULT 0 CHECK(reserved_fen>=0),
 PRIMARY KEY(organization_id,actor_id,key_id,window_kind,window_start),
 FOREIGN KEY(organization_id,actor_id,key_id) REFERENCES data_service_credentials(organization_id,actor_id,id));
CREATE INDEX IF NOT EXISTS data_service_actor_keys ON data_service_credentials(organization_id,actor_id,id);
`).Error
}
func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return dataservice.ErrUnavailable
	}
	for _, q := range []string{
		"SELECT id,organization_id,actor_id,member_id,digest,suffix,state,revision,config_json,expires_at,created_at FROM data_service_credentials LIMIT 0",
		"SELECT organization_id,actor_id,member_id,command_key,input_hash,action,target_id,receipt_json,created_at FROM data_service_commands LIMIT 0",
		"SELECT organization_id,actor_id,key_id,window_kind,window_start,consumed_rows,reserved_rows,consumed_fen,reserved_fen FROM data_service_quota LIMIT 0",
	} {
		if err := db.WithContext(ctx).Exec(q).Error; err != nil {
			return fmt.Errorf("%w: schema query: %v", dataservice.ErrUnavailable, err)
		}
	}
	var ready bool
	err := db.WithContext(ctx).Raw(`WITH expected(relation,kind,columns,reference,reference_columns) AS (VALUES
 ('data_service_credentials','p',ARRAY['id'],NULL::text,NULL::text[]),
 ('data_service_credentials','u',ARRAY['organization_id','actor_id','id'],NULL,NULL),
 ('data_service_commands','p',ARRAY['organization_id','actor_id','command_key'],NULL,NULL),
 ('data_service_quota','p',ARRAY['organization_id','actor_id','key_id','window_kind','window_start'],NULL,NULL),
 ('data_service_quota','f',ARRAY['organization_id','actor_id','key_id'],'data_service_credentials',ARRAY['organization_id','actor_id','id'])
 ) SELECT NOT EXISTS(SELECT 1 FROM expected e WHERE NOT EXISTS(
 SELECT 1 FROM pg_constraint c WHERE c.conrelid=to_regclass(e.relation)
 AND c.contype::text=e.kind AND c.convalidated AND NOT c.condeferrable
 AND ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY AS k(id,position)
 JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.id ORDER BY k.position)=e.columns
 AND (e.reference IS NULL OR (c.confrelid=to_regclass(e.reference)
 AND ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY AS k(id,position)
 JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.id ORDER BY k.position)=e.reference_columns))))`).Scan(&ready).Error
	if err != nil || !ready {
		return fmt.Errorf("%w: constraints ready=%t: %v", dataservice.ErrUnavailable, ready, err)
	}
	return nil
}
