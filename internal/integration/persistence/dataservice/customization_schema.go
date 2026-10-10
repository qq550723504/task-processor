package dataservicepersistence

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/dataservice"
)

var CustomTables = []string{"data_service_custom_requests", "data_service_custom_events", "data_service_custom_commands"}

func InstallCustomSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return dataservice.ErrUnavailable
	}
	return db.Exec(`CREATE TABLE IF NOT EXISTS data_service_custom_requests (
 organization_id varchar(128) NOT NULL,actor_id varchar(128) NOT NULL,member_id varchar(128) NOT NULL,id uuid NOT NULL,
 input_json jsonb NOT NULL CHECK(octet_length(input_json::text)<=65536),state varchar(24) NOT NULL CHECK(state IN ('SUBMITTED','EVALUATING','SPEC_CONFIRMED','PREPARING','DELIVERED','CLOSED')),
 revision bigint NOT NULL CHECK(revision>0),spec_json jsonb CHECK(octet_length(spec_json::text)<=65536),spec_revision bigint NOT NULL DEFAULT 0 CHECK(spec_revision>=0),
 batch_id uuid,delivered_rows integer NOT NULL DEFAULT 0 CHECK(delivered_rows BETWEEN 0 AND 200),created_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,actor_id,id),UNIQUE(id),
 CHECK((state='DELIVERED' AND batch_id IS NOT NULL AND delivered_rows>0) OR (state<>'DELIVERED' AND batch_id IS NULL AND delivered_rows=0)),
 CHECK(state NOT IN ('SPEC_CONFIRMED','PREPARING','DELIVERED') OR (spec_json IS NOT NULL AND spec_revision>0)));
CREATE TABLE IF NOT EXISTS data_service_custom_events (
 organization_id varchar(128) NOT NULL,actor_id varchar(128) NOT NULL,request_id uuid NOT NULL,revision bigint NOT NULL CHECK(revision>0),
 state varchar(24) NOT NULL,operator_id varchar(128) NOT NULL,note varchar(2000) NOT NULL,created_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,actor_id,request_id,revision),FOREIGN KEY(organization_id,actor_id,request_id) REFERENCES data_service_custom_requests(organization_id,actor_id,id));
CREATE TABLE IF NOT EXISTS data_service_custom_commands (
 operator_id varchar(128) NOT NULL,command_key uuid NOT NULL,request_id uuid NOT NULL,input_hash varchar(64) NOT NULL CHECK(input_hash ~ '^[0-9a-f]{64}$'),
 action varchar(24) NOT NULL,revision bigint NOT NULL CHECK(revision>0),created_at timestamptz NOT NULL,
 PRIMARY KEY(operator_id,command_key),FOREIGN KEY(request_id) REFERENCES data_service_custom_requests(id));
CREATE INDEX IF NOT EXISTS data_service_custom_actor ON data_service_custom_requests(organization_id,actor_id,created_at DESC,id);
CREATE INDEX IF NOT EXISTS data_service_custom_state ON data_service_custom_requests(state,id);
`).Error
}
func VerifyCustomSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return dataservice.ErrUnavailable
	}
	for _, q := range []string{
		"SELECT organization_id,actor_id,member_id,id,input_json,state,revision,spec_json,spec_revision,batch_id,delivered_rows,created_at FROM data_service_custom_requests LIMIT 0",
		"SELECT organization_id,actor_id,request_id,revision,state,operator_id,note,created_at FROM data_service_custom_events LIMIT 0",
		"SELECT operator_id,command_key,request_id,input_hash,action,revision,created_at FROM data_service_custom_commands LIMIT 0",
	} {
		if db.WithContext(ctx).Exec(q).Error != nil {
			return dataservice.ErrUnavailable
		}
	}
	var ready bool
	err := db.WithContext(ctx).Raw(`WITH expected(relation,kind,columns,reference,reference_columns) AS (VALUES
 ('data_service_custom_requests','p',ARRAY['organization_id','actor_id','id'],NULL::text,NULL::text[]),
 ('data_service_custom_requests','u',ARRAY['id'],NULL,NULL),
 ('data_service_custom_events','p',ARRAY['organization_id','actor_id','request_id','revision'],NULL,NULL),
 ('data_service_custom_events','f',ARRAY['organization_id','actor_id','request_id'],'data_service_custom_requests',ARRAY['organization_id','actor_id','id']),
 ('data_service_custom_commands','p',ARRAY['operator_id','command_key'],NULL,NULL),
 ('data_service_custom_commands','f',ARRAY['request_id'],'data_service_custom_requests',ARRAY['id'])
 ) SELECT NOT EXISTS(SELECT 1 FROM expected e WHERE NOT EXISTS(
 SELECT 1 FROM pg_constraint c WHERE c.conrelid=to_regclass(e.relation) AND c.contype::text=e.kind AND c.convalidated AND NOT c.condeferrable
 AND ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY AS k(id,position) JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.id ORDER BY k.position)=e.columns
 AND (e.reference IS NULL OR (c.confrelid=to_regclass(e.reference)
 AND ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY AS k(id,position) JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.id ORDER BY k.position)=e.reference_columns))))`).Scan(&ready).Error
	if err != nil || !ready {
		return dataservice.ErrUnavailable
	}
	return nil
}
