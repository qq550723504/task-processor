package podpersistence

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/product/pod"
)

var Tables = []string{"product_pod_operations", "product_pod_fences", "product_pod_commands"}

// Fresh-install only. Runtime verifies its exact required constraints, never DDL.
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return pod.ErrUnavailable
	}
	return db.Exec(`
 CREATE TABLE IF NOT EXISTS product_pod_operations(id uuid PRIMARY KEY,organization_id varchar(128) NOT NULL,actor_id varchar(128) NOT NULL,member_id varchar(128) NOT NULL,operation_json jsonb NOT NULL CHECK(octet_length(operation_json::text)<=2097152),created_at timestamptz NOT NULL,next_observation_at timestamptz NOT NULL);
 CREATE TABLE IF NOT EXISTS product_pod_fences(fence_key varchar(64) PRIMARY KEY CHECK(length(fence_key)=64),operation_id uuid NOT NULL UNIQUE REFERENCES product_pod_operations(id));
 CREATE TABLE IF NOT EXISTS product_pod_commands(organization_id varchar(128) NOT NULL,actor_id varchar(128) NOT NULL,member_id varchar(128) NOT NULL,command_key uuid NOT NULL,input_hash varchar(64) NOT NULL CHECK(length(input_hash)=64),kind varchar(16) NOT NULL CHECK(kind IN ('design','template','finished')),operation_id uuid,receipt_json jsonb NOT NULL CHECK(octet_length(receipt_json::text)<=65536),PRIMARY KEY(organization_id,actor_id,command_key));
 CREATE INDEX IF NOT EXISTS product_pod_owner ON product_pod_operations(organization_id,actor_id,id);`).Error
}
func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return pod.ErrUnavailable
	}
	for _, q := range []string{"SELECT id,organization_id,actor_id,member_id,operation_json,created_at,next_observation_at FROM product_pod_operations LIMIT 0", "SELECT fence_key,operation_id FROM product_pod_fences LIMIT 0", "SELECT organization_id,actor_id,member_id,command_key,input_hash,kind,operation_id,receipt_json FROM product_pod_commands LIMIT 0"} {
		if db.WithContext(ctx).Exec(q).Error != nil {
			return pod.ErrUnavailable
		}
	}
	var ready bool
	e := db.WithContext(ctx).Raw(`SELECT
 EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('product_pod_commands') AND contype='p' AND convalidated AND NOT condeferrable AND pg_get_constraintdef(oid)='PRIMARY KEY (organization_id, actor_id, command_key)')
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('product_pod_operations') AND contype='p' AND convalidated AND NOT condeferrable AND pg_get_constraintdef(oid)='PRIMARY KEY (id)')
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('product_pod_fences') AND contype='p' AND convalidated AND NOT condeferrable AND pg_get_constraintdef(oid)='PRIMARY KEY (fence_key)')
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('product_pod_fences') AND contype='u' AND convalidated AND NOT condeferrable AND pg_get_constraintdef(oid)='UNIQUE (operation_id)')
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('product_pod_fences') AND contype='f' AND convalidated AND NOT condeferrable AND confrelid=to_regclass('product_pod_operations') AND pg_get_constraintdef(oid)='FOREIGN KEY (operation_id) REFERENCES product_pod_operations(id)')`).Scan(&ready).Error
	if e != nil || !ready {
		return pod.ErrUnavailable
	}
	return nil
}
