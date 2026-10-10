package collectionpersistence

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/product/collection"
)

var Tables = []string{"product_collection_batches", "product_collection_items", "product_collection_operations"}

func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return collection.ErrUnavailable
	}
	return db.Exec(`CREATE TABLE IF NOT EXISTS product_collection_batches (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 id uuid NOT NULL, name varchar(200) NOT NULL, kind varchar(32) NOT NULL,
 revision bigint NOT NULL CHECK(revision>0), created_at timestamptz NOT NULL, archived_at timestamptz,
 PRIMARY KEY(organization_id,actor_id,id), CHECK(name<>''), CHECK(kind IN ('acquisition','own','manual','amazon_data','custom_dataset')));
CREATE TABLE IF NOT EXISTS product_collection_items (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 id uuid NOT NULL, batch_id uuid NOT NULL, product_key varchar(128) NOT NULL,
 publication_id varchar(128) NOT NULL, original_version bigint NOT NULL CHECK(original_version>0),
 source_kind varchar(32) NOT NULL CHECK(source_kind IN ('acquisition','own','amazon_data','custom_dataset')), source_operation_id varchar(128) NOT NULL,
 revision bigint NOT NULL CHECK(revision>0), created_at timestamptz NOT NULL, archived_at timestamptz,
 PRIMARY KEY(organization_id,actor_id,id), UNIQUE(organization_id,actor_id,publication_id),
 FOREIGN KEY(organization_id,actor_id,batch_id) REFERENCES product_collection_batches(organization_id,actor_id,id),
 FOREIGN KEY(organization_id,product_key,original_version) REFERENCES product_snapshot_versions(tenant_id,product_key,version));
CREATE TABLE IF NOT EXISTS product_collection_operations (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 id uuid NOT NULL, command_key uuid NOT NULL, input_hash varchar(64) NOT NULL CHECK(length(input_hash)=64),
 receipt_json jsonb NOT NULL, created_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,actor_id,id), UNIQUE(organization_id,actor_id,command_key));
CREATE INDEX IF NOT EXISTS collection_batch_visible ON product_collection_batches(organization_id,actor_id,id) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS collection_item_visible ON product_collection_items(organization_id,actor_id,batch_id,id) WHERE archived_at IS NULL;`).Error
}
func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return collection.ErrUnavailable
	}
	for _, query := range []string{
		"SELECT organization_id,actor_id,member_id,id,name,kind,revision,created_at,archived_at FROM product_collection_batches LIMIT 0",
		"SELECT organization_id,actor_id,member_id,id,batch_id,product_key,publication_id,original_version,source_kind,source_operation_id,revision,created_at,archived_at FROM product_collection_items LIMIT 0",
		"SELECT organization_id,actor_id,member_id,id,command_key,input_hash,receipt_json,created_at FROM product_collection_operations LIMIT 0",
	} {
		if err := db.WithContext(ctx).Exec(query).Error; err != nil {
			return collection.ErrUnavailable
		}
	}
	var ready bool
	err := db.WithContext(ctx).Raw(`WITH expected(relation,kind,columns,reference,reference_columns) AS (VALUES
 ('product_collection_batches','p',ARRAY['organization_id','actor_id','id'],NULL::text,NULL::text[]),
 ('product_collection_items','p',ARRAY['organization_id','actor_id','id'],NULL,NULL),
 ('product_collection_operations','p',ARRAY['organization_id','actor_id','id'],NULL,NULL),
 ('product_collection_items','u',ARRAY['organization_id','actor_id','publication_id'],NULL,NULL),
 ('product_collection_operations','u',ARRAY['organization_id','actor_id','command_key'],NULL,NULL),
 ('product_collection_items','f',ARRAY['organization_id','actor_id','batch_id'],'product_collection_batches',ARRAY['organization_id','actor_id','id']),
 ('product_collection_items','f',ARRAY['organization_id','product_key','original_version'],'product_snapshot_versions',ARRAY['tenant_id','product_key','version'])
 ) SELECT NOT EXISTS(SELECT 1 FROM expected e WHERE NOT EXISTS(
 SELECT 1 FROM pg_constraint c WHERE c.conrelid=to_regclass(e.relation)
 AND c.contype::text=e.kind AND c.convalidated AND NOT c.condeferrable
 AND ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY AS k(id,position)
 JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.id ORDER BY k.position)=e.columns
 AND (e.reference IS NULL OR (c.confrelid=to_regclass(e.reference)
 AND ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY AS k(id,position)
 JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.id ORDER BY k.position)=e.reference_columns))))`).Scan(&ready).Error
	if err != nil || !ready {
		return collection.ErrUnavailable
	}
	// Structural keys alone cannot prove that the installation admits the new
	// producer references. Every check on the source-kind column must include
	// the declared kinds; stale/restrictive or unvalidated checks fail closed.
	err = db.WithContext(ctx).Raw(`WITH expected(relation,column_name,kinds) AS (VALUES
 ('product_collection_batches','kind',ARRAY['acquisition','own','manual','amazon_data','custom_dataset']),
 ('product_collection_items','source_kind',ARRAY['acquisition','own','amazon_data','custom_dataset'])
 ) SELECT NOT EXISTS(SELECT 1 FROM expected e WHERE
 NOT EXISTS(SELECT 1 FROM pg_constraint c JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attname=e.column_name
 WHERE c.conrelid=to_regclass(e.relation) AND c.contype='c' AND a.attnum=ANY(c.conkey))
 OR EXISTS(SELECT 1 FROM pg_constraint c JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attname=e.column_name
 WHERE c.conrelid=to_regclass(e.relation) AND c.contype='c' AND a.attnum=ANY(c.conkey)
 AND (NOT c.convalidated OR EXISTS(SELECT 1 FROM unnest(e.kinds) AS k(value)
 WHERE position(quote_literal(k.value) IN pg_get_constraintdef(c.oid))=0))))`).Scan(&ready).Error
	if err != nil || !ready {
		return collection.ErrUnavailable
	}
	return nil
}
