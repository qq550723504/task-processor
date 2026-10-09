package recordpersistence

import (
	"context"
	"gorm.io/gorm"
	record "task-processor/internal/listing/record/target"
)

var Tables = []string{"listing_target_records", "listing_preparation_targets", "listing_target_record_commands"}

// InstallSchema belongs to explicit initialization of the Product owner pool.
// Serving construction only verifies these facts and never runs DDL.
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return record.ErrUnavailable
	}
	return db.Exec(`CREATE TABLE IF NOT EXISTS listing_target_records (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 id uuid NOT NULL, target_id uuid NOT NULL, source_id uuid NOT NULL, store_id uuid NOT NULL,
 site varchar(32) NOT NULL CHECK(site='shein-us'), revision bigint NOT NULL CHECK(revision>0),
 input_hash varchar(64) NOT NULL CHECK(length(input_hash)=64), body_hash varchar(64) NOT NULL CHECK(length(body_hash)=64),
 body_json jsonb NOT NULL, created_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,actor_id,id), UNIQUE(organization_id,actor_id,target_id,id,revision),
 FOREIGN KEY(organization_id,actor_id,source_id) REFERENCES listing_preparation_sources(organization_id,actor_id,id));
CREATE TABLE IF NOT EXISTS listing_preparation_targets (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 id uuid NOT NULL, source_id uuid NOT NULL, store_id uuid NOT NULL, site varchar(32) NOT NULL CHECK(site='shein-us'),
 current_record_id uuid NOT NULL, revision bigint NOT NULL CHECK(revision>0),
 PRIMARY KEY(organization_id,actor_id,id), UNIQUE(organization_id,actor_id,source_id,store_id,site),
 FOREIGN KEY(organization_id,actor_id,source_id) REFERENCES listing_preparation_sources(organization_id,actor_id,id),
 FOREIGN KEY(organization_id,actor_id,id,current_record_id,revision) REFERENCES listing_target_records(organization_id,actor_id,target_id,id,revision));
CREATE TABLE IF NOT EXISTS listing_target_record_commands (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 command_key uuid NOT NULL, record_id uuid NOT NULL, input_hash varchar(64) NOT NULL CHECK(length(input_hash)=64),
 PRIMARY KEY(organization_id,actor_id,command_key),
 FOREIGN KEY(organization_id,actor_id,record_id) REFERENCES listing_target_records(organization_id,actor_id,id));`).Error
}

func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return record.ErrUnavailable
	}
	for _, query := range []string{
		"SELECT organization_id,actor_id,member_id,id,target_id,source_id,store_id,site,revision,input_hash,body_hash,body_json,created_at FROM listing_target_records LIMIT 0",
		"SELECT organization_id,actor_id,member_id,id,source_id,store_id,site,current_record_id,revision FROM listing_preparation_targets LIMIT 0",
		"SELECT organization_id,actor_id,member_id,command_key,record_id,input_hash FROM listing_target_record_commands LIMIT 0",
	} {
		if db.WithContext(ctx).Exec(query).Error != nil {
			return record.ErrUnavailable
		}
	}
	var ready bool
	err := db.WithContext(ctx).Raw(`WITH expected(relation,kind,columns,reference,reference_columns) AS (VALUES
 ('listing_target_records','p',ARRAY['organization_id','actor_id','id'],NULL::text,NULL::text[]),
 ('listing_target_records','u',ARRAY['organization_id','actor_id','target_id','id','revision'],NULL,NULL),
 ('listing_target_records','f',ARRAY['organization_id','actor_id','source_id'],'listing_preparation_sources',ARRAY['organization_id','actor_id','id']),
 ('listing_preparation_targets','p',ARRAY['organization_id','actor_id','id'],NULL,NULL),
 ('listing_preparation_targets','u',ARRAY['organization_id','actor_id','source_id','store_id','site'],NULL,NULL),
 ('listing_preparation_targets','f',ARRAY['organization_id','actor_id','source_id'],'listing_preparation_sources',ARRAY['organization_id','actor_id','id']),
 ('listing_preparation_targets','f',ARRAY['organization_id','actor_id','id','current_record_id','revision'],'listing_target_records',ARRAY['organization_id','actor_id','target_id','id','revision']),
 ('listing_target_record_commands','p',ARRAY['organization_id','actor_id','command_key'],NULL,NULL),
 ('listing_target_record_commands','f',ARRAY['organization_id','actor_id','record_id'],'listing_target_records',ARRAY['organization_id','actor_id','id'])
 ) SELECT NOT EXISTS(SELECT 1 FROM expected e WHERE NOT EXISTS(SELECT 1 FROM pg_constraint c
 WHERE c.conrelid=to_regclass(e.relation) AND c.contype::text=e.kind AND c.convalidated AND NOT c.condeferrable
 AND ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY AS k(id,position)
 JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.id ORDER BY k.position)=e.columns
 AND (e.reference IS NULL OR (c.confrelid=to_regclass(e.reference)
 AND ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY AS k(id,position)
 JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.id ORDER BY k.position)=e.reference_columns))))`).Scan(&ready).Error
	if err != nil || !ready {
		return record.ErrUnavailable
	}
	return nil
}
