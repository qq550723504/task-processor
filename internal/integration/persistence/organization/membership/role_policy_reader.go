package membership

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
)

const RolePolicyReaderRole = "organization_role_policy_reader"

func NewRolePolicyReader(ctx context.Context, db *gorm.DB, project string) (*Repository, error) {
	if db == nil || !authidentity.IsBoundedIdentifier(project) {
		return nil, errors.New("role policy reader unavailable")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.New("role policy reader unavailable")
	}
	var user string
	var permitted bool
	err = sqlDB.QueryRowContext(ctx, `SELECT current_user,
 has_table_privilege(current_user,'public.organization_roles','SELECT')
 AND has_table_privilege(current_user,'public.organization_role_slots','SELECT')
 AND NOT has_database_privilege(current_user,current_database(),'CREATE')
 AND NOT has_database_privilege(current_user,current_database(),'TEMPORARY')
 AND NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=current_user AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls))
 AND NOT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname<>'information_schema' AND has_schema_privilege(current_user,oid,'CREATE'))
 AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL aclexplode(acldefault('r',c.relowner)) p
 WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND c.relkind IN ('r','p','v','m','f')
 AND CASE WHEN p.privilege_type IN ('SELECT','INSERT','UPDATE','REFERENCES') THEN has_any_column_privilege(current_user,c.oid,p.privilege_type) ELSE has_table_privilege(current_user,c.oid,p.privilege_type) END
 AND NOT(n.nspname='public' AND c.relname IN ('organization_role_slots','organization_roles') AND p.privilege_type='SELECT'))`).Scan(&user, &permitted)
	if err != nil || user != RolePolicyReaderRole || !permitted {
		return nil, errors.New("role policy reader privilege boundary unavailable")
	}
	if err := verifyRoleReadSchema(ctx, sqlDB); err != nil {
		return nil, errors.New("role policy schema unavailable")
	}
	return &Repository{db: sqlDB, projectID: project}, nil
}
