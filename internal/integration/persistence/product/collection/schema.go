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
 PRIMARY KEY(organization_id,actor_id,id), CHECK(name<>''), CHECK(kind IN ('acquisition','own','manual')));
CREATE TABLE IF NOT EXISTS product_collection_items (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, member_id varchar(128) NOT NULL,
 id uuid NOT NULL, batch_id uuid NOT NULL, product_key varchar(128) NOT NULL,
 publication_id varchar(128) NOT NULL, original_version bigint NOT NULL CHECK(original_version>0),
 source_kind varchar(32) NOT NULL CHECK(source_kind IN ('acquisition','own')), source_operation_id varchar(128) NOT NULL,
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
	err := db.WithContext(ctx).Raw(`SELECT
 (SELECT count(*)=3 FROM pg_constraint WHERE conrelid IN ('product_collection_batches'::regclass,'product_collection_items'::regclass,'product_collection_operations'::regclass) AND contype='p' AND convalidated AND NOT condeferrable)
 AND (SELECT count(*)=2 FROM pg_constraint WHERE conrelid='product_collection_items'::regclass AND contype='f' AND convalidated AND NOT condeferrable)
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='product_collection_items'::regclass AND contype='u' AND convalidated AND NOT condeferrable)
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='product_collection_operations'::regclass AND contype='u' AND convalidated AND NOT condeferrable)`).Scan(&ready).Error
	if err != nil || !ready {
		return collection.ErrUnavailable
	}
	return nil
}
