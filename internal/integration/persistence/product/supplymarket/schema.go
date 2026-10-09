package supplymarketpersistence

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/product/supplymarket"
)

var Tables = []string{"supply_market_records", "supply_market_events", "supply_market_releases", "supply_market_commands", "supply_market_private_uploads"}

// InstallSchema is for a new empty installation. Runtime never alters an
// existing database or translates legacy supply records.
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return supplymarket.ErrUnavailable
	}
	return db.Exec(`CREATE TABLE IF NOT EXISTS supply_market_records (
 id uuid PRIMARY KEY, organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 kind varchar(32) NOT NULL CHECK(kind IN ('official','selected','connection')),
 stage varchar(32) NOT NULL CHECK(stage IN ('DRAFT','SUBMITTED','EVALUATING','SUPPLEMENT_REQUIRED','APPROVED','REJECTED','PLAN_CONFIRMED','CLOSED')),
 revision bigint NOT NULL CHECK(revision>0), record_json jsonb NOT NULL CHECK(octet_length(record_json::text)<=2097152), created_at timestamptz NOT NULL);
 CREATE TABLE IF NOT EXISTS supply_market_events (
 id uuid PRIMARY KEY, record_id uuid NOT NULL REFERENCES supply_market_records(id), event_json jsonb NOT NULL CHECK(octet_length(event_json::text)<=65536), created_at timestamptz NOT NULL);
 CREATE TABLE IF NOT EXISTS supply_market_releases (
 id uuid PRIMARY KEY, record_id uuid NOT NULL REFERENCES supply_market_records(id), organization_id varchar(128) NOT NULL,actor_id varchar(128) NOT NULL,member_id varchar(128) NOT NULL,
 channel varchar(32) NOT NULL CHECK(channel IN ('official','selected')), revision bigint NOT NULL CHECK(revision>0), active boolean NOT NULL,
 release_json jsonb NOT NULL CHECK(octet_length(release_json::text)<=2097152), source_json jsonb NOT NULL CHECK(octet_length(source_json::text)<=65536), published_at timestamptz NOT NULL);
 CREATE UNIQUE INDEX IF NOT EXISTS supply_release_active_record ON supply_market_releases(record_id) WHERE active;
 CREATE TABLE IF NOT EXISTS supply_market_commands (
 principal varchar(300) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL, command_key uuid NOT NULL,
 operation_id uuid NOT NULL UNIQUE, input_hash varchar(64) NOT NULL CHECK(length(input_hash)=64), receipt_json jsonb NOT NULL CHECK(octet_length(receipt_json::text)<=65536),created_at timestamptz NOT NULL,
 PRIMARY KEY(principal,actor_id,command_key));
 CREATE TABLE IF NOT EXISTS supply_market_private_uploads (
 id uuid PRIMARY KEY,organization_id varchar(128) NOT NULL,actor_id varchar(128) NOT NULL,member_id varchar(128) NOT NULL,
 object_key varchar(1024) NOT NULL UNIQUE, content_type varchar(64) NOT NULL CHECK(content_type IN ('image/jpeg','image/png','application/pdf')),
 digest varchar(64) NOT NULL CHECK(length(digest)=64),size bigint NOT NULL CHECK(size>0 AND size<=20971520),created_at timestamptz NOT NULL);
 CREATE INDEX IF NOT EXISTS supply_record_owner ON supply_market_records(organization_id,actor_id,id);
 CREATE INDEX IF NOT EXISTS supply_event_record ON supply_market_events(record_id,id);
 CREATE INDEX IF NOT EXISTS supply_release_visible ON supply_market_releases(channel,id) WHERE active;`).Error
}
func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return supplymarket.ErrUnavailable
	}
	for _, query := range []string{
		"SELECT id,organization_id,actor_id,member_id,kind,stage,revision,record_json,created_at FROM supply_market_records LIMIT 0",
		"SELECT id,record_id,event_json,created_at FROM supply_market_events LIMIT 0",
		"SELECT id,record_id,organization_id,actor_id,member_id,channel,revision,active,release_json,source_json,published_at FROM supply_market_releases LIMIT 0",
		"SELECT principal,actor_id,member_id,command_key,operation_id,input_hash,receipt_json,created_at FROM supply_market_commands LIMIT 0",
		"SELECT id,organization_id,actor_id,member_id,object_key,content_type,digest,size,created_at FROM supply_market_private_uploads LIMIT 0",
	} {
		if db.WithContext(ctx).Exec(query).Error != nil {
			return supplymarket.ErrUnavailable
		}
	}
	var ready bool
	if db.WithContext(ctx).Raw(`SELECT
 EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('supply_market_commands') AND contype='p' AND convalidated AND NOT condeferrable)
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('supply_market_releases') AND contype='f' AND confrelid=to_regclass('supply_market_records') AND convalidated AND NOT condeferrable)
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('supply_market_events') AND contype='f' AND confrelid=to_regclass('supply_market_records') AND convalidated AND NOT condeferrable)
 AND EXISTS(SELECT 1 FROM pg_index WHERE indexrelid=to_regclass('supply_release_active_record') AND indisunique AND indisvalid AND indpred IS NOT NULL)`).Scan(&ready).Error != nil || !ready {
		return supplymarket.ErrUnavailable
	}
	return nil
}
