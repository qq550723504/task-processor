package preparationpersistence

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/listing/preparation"
)

var OperationTables = []string{"listing_preparation_operations", "listing_preparation_operation_items"}

func InstallOperationSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return preparation.ErrUnavailable
	}
	return db.Exec(`CREATE TABLE IF NOT EXISTS listing_preparation_operations (
 organization_id varchar(128) NOT NULL,actor_id varchar(128) NOT NULL,member_id varchar(128) NOT NULL,
 id uuid NOT NULL,command_key uuid NOT NULL,input_hash varchar(64) NOT NULL CHECK(length(input_hash)=64),input_json jsonb NOT NULL,
 preparation_id uuid NOT NULL,store_id uuid NOT NULL,action varchar(16) NOT NULL CHECK(action IN ('adapt','upload','optimize')),
 item_count bigint NOT NULL CHECK(item_count>0),completed_count bigint NOT NULL DEFAULT 0 CHECK(completed_count>=0 AND completed_count<=item_count),
 status varchar(16) NOT NULL CHECK(status IN ('pending','running','completed','cancelled')),
 execution varchar(16) NOT NULL CHECK(execution IN ('pending','started','start_unknown')),created_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,actor_id,id),UNIQUE(organization_id,actor_id,command_key),UNIQUE(organization_id,id),
 FOREIGN KEY(organization_id,actor_id,preparation_id) REFERENCES listing_preparations(organization_id,actor_id,id));
CREATE TABLE IF NOT EXISTS listing_preparation_operation_items (
 organization_id varchar(128) NOT NULL,actor_id varchar(128) NOT NULL,member_id varchar(128) NOT NULL,
 operation_id uuid NOT NULL,source_id uuid NOT NULL,record_id uuid,record_revision bigint,
 status varchar(16) NOT NULL CHECK(status IN ('pending','running','succeeded','missing','review','unknown','denied','failed','cancelled')),
 result_reference varchar(128) NOT NULL DEFAULT '',note varchar(300) NOT NULL DEFAULT '',
 PRIMARY KEY(organization_id,actor_id,operation_id,source_id),
 FOREIGN KEY(organization_id,actor_id,operation_id) REFERENCES listing_preparation_operations(organization_id,actor_id,id),
 FOREIGN KEY(organization_id,actor_id,source_id) REFERENCES listing_preparation_sources(organization_id,actor_id,id),
 CHECK((record_id IS NULL AND record_revision IS NULL) OR (record_id IS NOT NULL AND record_revision>0)));
CREATE INDEX IF NOT EXISTS supply_operation_source_page ON listing_preparation_operation_items(organization_id,actor_id,operation_id,source_id);`).Error
}
func VerifyOperationSchema(ctx context.Context, db *gorm.DB) error {
	if VerifySchema(ctx, db) != nil {
		return preparation.ErrUnavailable
	}
	for _, query := range []string{
		"SELECT organization_id,actor_id,member_id,id,command_key,input_hash,input_json,preparation_id,store_id,action,item_count,completed_count,status,execution,created_at FROM listing_preparation_operations LIMIT 0",
		"SELECT organization_id,actor_id,member_id,operation_id,source_id,record_id,record_revision,status,result_reference,note FROM listing_preparation_operation_items LIMIT 0",
	} {
		if db.WithContext(ctx).Exec(query).Error != nil {
			return preparation.ErrUnavailable
		}
	}
	var ready bool
	err := db.WithContext(ctx).Raw(`WITH expected(relation,kind,columns,reference,reference_columns) AS (VALUES
 ('listing_preparation_operations','p',ARRAY['organization_id','actor_id','id'],NULL::text,NULL::text[]),
 ('listing_preparation_operations','u',ARRAY['organization_id','actor_id','command_key'],NULL,NULL),
 ('listing_preparation_operations','u',ARRAY['organization_id','id'],NULL,NULL),
 ('listing_preparation_operations','f',ARRAY['organization_id','actor_id','preparation_id'],'listing_preparations',ARRAY['organization_id','actor_id','id']),
 ('listing_preparation_operation_items','p',ARRAY['organization_id','actor_id','operation_id','source_id'],NULL,NULL),
 ('listing_preparation_operation_items','f',ARRAY['organization_id','actor_id','operation_id'],'listing_preparation_operations',ARRAY['organization_id','actor_id','id']),
 ('listing_preparation_operation_items','f',ARRAY['organization_id','actor_id','source_id'],'listing_preparation_sources',ARRAY['organization_id','actor_id','id'])
 ) SELECT NOT EXISTS(SELECT 1 FROM expected e WHERE NOT EXISTS(SELECT 1 FROM pg_constraint c
 WHERE c.conrelid=to_regclass(e.relation) AND c.contype::text=e.kind AND c.convalidated AND NOT c.condeferrable
 AND ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY AS k(id,position)
 JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.id ORDER BY k.position)=e.columns
 AND (e.reference IS NULL OR (c.confrelid=to_regclass(e.reference)
 AND ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY AS k(id,position)
 JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.id ORDER BY k.position)=e.reference_columns))))`).Scan(&ready).Error
	if err != nil || !ready {
		return preparation.ErrUnavailable
	}
	return nil
}
