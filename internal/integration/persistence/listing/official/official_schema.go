package officialpersistence

import (
	"context"
	"gorm.io/gorm"
	recordstore "task-processor/internal/integration/persistence/listing/record"
	submissionstore "task-processor/internal/integration/persistence/listing/submission"
	"task-processor/internal/listing/submission"
)

var OfficialTables = []string{"listing_submission_official_intents", "listing_submission_official_receipts"}

func InstallOfficialSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return submission.ErrExecutionUnavailable
	}
	return db.Exec(`CREATE TABLE IF NOT EXISTS listing_submission_official_intents (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 intent_key varchar(128) NOT NULL, record_id uuid NOT NULL, kind varchar(16) NOT NULL CHECK(kind IN ('publish','image')),
 input_hash varchar(64) NOT NULL CHECK(input_hash ~ '^[0-9a-f]{64}$'), body_hash varchar(64) NOT NULL CHECK(body_hash ~ '^[0-9a-f]{64}$'), body_json jsonb NOT NULL,
 PRIMARY KEY(organization_id,intent_key),
 FOREIGN KEY(organization_id,actor_id,record_id) REFERENCES listing_target_records(organization_id,actor_id,id));
CREATE TABLE IF NOT EXISTS listing_submission_official_receipts (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 attempt_id uuid NOT NULL, intent_key varchar(128) NOT NULL, store_id uuid NOT NULL, subject_id varchar(128) NOT NULL,
 kind varchar(16) NOT NULL CHECK(kind IN ('publish','image')), body_hash varchar(64) NOT NULL CHECK(body_hash ~ '^[0-9a-f]{64}$'), body_json jsonb NOT NULL,
 PRIMARY KEY(organization_id,attempt_id), UNIQUE(organization_id,store_id,subject_id),
 FOREIGN KEY(organization_id,intent_key) REFERENCES listing_submission_official_intents(organization_id,intent_key),
 FOREIGN KEY(organization_id,attempt_id) REFERENCES listing_submission_execution_attempts(organization_id,attempt_id));`).Error
}
func VerifyOfficialSchema(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" || submissionstore.VerifySchema(ctx, db) != nil || recordstore.VerifySchema(ctx, db) != nil {
		return submission.ErrExecutionUnavailable
	}
	for _, query := range []string{
		"SELECT organization_id,actor_id,member_id,intent_key,record_id,kind,input_hash,body_hash,body_json FROM listing_submission_official_intents LIMIT 0",
		"SELECT organization_id,actor_id,member_id,attempt_id,intent_key,store_id,subject_id,kind,body_hash,body_json FROM listing_submission_official_receipts LIMIT 0",
	} {
		if db.WithContext(ctx).Exec(query).Error != nil {
			return submission.ErrExecutionUnavailable
		}
	}
	var ready bool
	err := db.WithContext(ctx).Raw(`WITH expected(relation,kind,columns,reference,reference_columns) AS (VALUES
 ('listing_submission_official_intents','p',ARRAY['organization_id','intent_key'],NULL::text,NULL::text[]),
 ('listing_submission_official_intents','f',ARRAY['organization_id','actor_id','record_id'],'listing_target_records',ARRAY['organization_id','actor_id','id']),
 ('listing_submission_official_receipts','p',ARRAY['organization_id','attempt_id'],NULL,NULL),
 ('listing_submission_official_receipts','u',ARRAY['organization_id','store_id','subject_id'],NULL,NULL),
 ('listing_submission_official_receipts','f',ARRAY['organization_id','intent_key'],'listing_submission_official_intents',ARRAY['organization_id','intent_key']),
 ('listing_submission_official_receipts','f',ARRAY['organization_id','attempt_id'],'listing_submission_execution_attempts',ARRAY['organization_id','attempt_id'])
 ) SELECT NOT EXISTS(SELECT 1 FROM expected e WHERE NOT EXISTS(SELECT 1 FROM pg_constraint c
 WHERE c.conrelid=to_regclass(e.relation) AND c.contype::text=e.kind AND c.convalidated AND NOT c.condeferrable
 AND ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY AS k(id,position)
 JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.id ORDER BY k.position)=e.columns
 AND (e.reference IS NULL OR (c.confrelid=to_regclass(e.reference)
 AND ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY AS k(id,position)
 JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.id ORDER BY k.position)=e.reference_columns))))`).Scan(&ready).Error
	if err != nil || !ready {
		return submission.ErrExecutionUnavailable
	}
	return nil
}
