package preparationpersistence

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/listing/preparation"
)

var Tables = []string{"listing_preparations", "listing_preparation_sources"}

func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return preparation.ErrUnavailable
	}
	return db.Exec(`CREATE TABLE IF NOT EXISTS listing_preparations (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 id uuid NOT NULL, command_key uuid NOT NULL, input_hash varchar(64) NOT NULL CHECK(length(input_hash)=64), input_json jsonb NOT NULL,
 source_batch_id uuid NOT NULL, source_revision bigint NOT NULL CHECK(source_revision>0), name varchar(200) NOT NULL,
 item_count bigint NOT NULL CHECK(item_count>=0), revision bigint NOT NULL CHECK(revision>0), created_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,actor_id,id), UNIQUE(organization_id,actor_id,command_key));
CREATE TABLE IF NOT EXISTS listing_preparation_sources (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 id uuid NOT NULL, preparation_id uuid NOT NULL, collection_item_id uuid NOT NULL, collection_revision bigint NOT NULL CHECK(collection_revision>0),
 product_key varchar(128) NOT NULL, publication_id varchar(128) NOT NULL, original_version bigint NOT NULL CHECK(original_version>0),
 source_kind varchar(32) NOT NULL CHECK(source_kind IN ('acquisition','own')), source_operation_id varchar(128) NOT NULL,
 PRIMARY KEY(organization_id,actor_id,id), UNIQUE(organization_id,actor_id,preparation_id,collection_item_id),
 FOREIGN KEY(organization_id,actor_id,preparation_id) REFERENCES listing_preparations(organization_id,actor_id,id),
 FOREIGN KEY(organization_id,actor_id,collection_item_id) REFERENCES product_collection_items(organization_id,actor_id,id),
 FOREIGN KEY(organization_id,product_key,original_version) REFERENCES product_snapshot_versions(tenant_id,product_key,version));
CREATE INDEX IF NOT EXISTS preparation_source_page ON listing_preparation_sources(organization_id,actor_id,preparation_id,id);`).Error
}

func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return preparation.ErrUnavailable
	}
	for _, query := range []string{
		"SELECT organization_id,actor_id,member_id,id,command_key,input_hash,input_json,source_batch_id,source_revision,name,item_count,revision,created_at FROM listing_preparations LIMIT 0",
		"SELECT organization_id,actor_id,member_id,id,preparation_id,collection_item_id,collection_revision,product_key,publication_id,original_version,source_kind,source_operation_id FROM listing_preparation_sources LIMIT 0",
	} {
		if db.WithContext(ctx).Exec(query).Error != nil {
			return preparation.ErrUnavailable
		}
	}
	var ready bool
	err := db.WithContext(ctx).Raw(`WITH expected(relation,kind,columns,reference,reference_columns) AS (VALUES
 ('listing_preparations','p',ARRAY['organization_id','actor_id','id'],NULL::text,NULL::text[]),
 ('listing_preparations','u',ARRAY['organization_id','actor_id','command_key'],NULL,NULL),
 ('listing_preparation_sources','p',ARRAY['organization_id','actor_id','id'],NULL,NULL),
 ('listing_preparation_sources','u',ARRAY['organization_id','actor_id','preparation_id','collection_item_id'],NULL,NULL),
 ('listing_preparation_sources','f',ARRAY['organization_id','actor_id','preparation_id'],'listing_preparations',ARRAY['organization_id','actor_id','id']),
 ('listing_preparation_sources','f',ARRAY['organization_id','actor_id','collection_item_id'],'product_collection_items',ARRAY['organization_id','actor_id','id']),
 ('listing_preparation_sources','f',ARRAY['organization_id','product_key','original_version'],'product_snapshot_versions',ARRAY['tenant_id','product_key','version'])
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
