package membership

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/text/cases"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	domain "task-processor/internal/organization/membership"
	"time"
)

// InstallRoleSlotsTx is called only by explicit native-verified initialization.
// Runtime credentials cannot insert or update inventory.
func InstallRoleSlotsTx(ctx context.Context, tx *sql.Tx, project, organization string) error {
	if tx == nil || !authidentity.IsBoundedIdentifier(project) || !authidentity.IsBoundedIdentifier(organization) {
		return domain.ErrInvalidRequest
	}
	for slot := 1; slot <= authz.EnterpriseRoleCapacity; slot++ {
		key := authz.EnterpriseRoleKey(organization, slot)
		if _, err := tx.ExecContext(ctx, `INSERT INTO public.organization_role_slots(project_id,organization_id,role_key,slot) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, project, organization, key, slot); err != nil {
			return domain.ErrUnavailable
		}
		var existing string
		if err := tx.QueryRowContext(ctx, `SELECT role_key FROM public.organization_role_slots WHERE project_id=$1 AND organization_id=$2 AND slot=$3`, project, organization, slot).Scan(&existing); err != nil || existing != key {
			return domain.ErrConflict
		}
	}
	return nil
}

func (r *Repository) Roles(ctx context.Context, organization string) ([]domain.RoleDefinition, int, error) {
	if r == nil || r.db == nil || !authidentity.IsBoundedIdentifier(r.projectID) || !authidentity.IsBoundedIdentifier(organization) {
		return nil, 0, domain.ErrInvalidRequest
	}
	rows, err := r.db.QueryContext(ctx, `SELECT role_key,name,modules,revision FROM public.organization_roles WHERE project_id=$1 AND organization_id=$2 ORDER BY created_at,role_key LIMIT 65`, r.projectID, organization)
	if err != nil {
		return nil, 0, domain.ErrUnavailable
	}
	defer rows.Close()
	items := []domain.RoleDefinition{}
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil || !authz.IsEnterpriseRoleKey(organization, role.ID) || len(items) >= 64 {
			return nil, 0, domain.ErrUnavailable
		}
		items = append(items, role)
	}
	if rows.Err() != nil || rows.Close() != nil {
		return nil, 0, domain.ErrUnavailable
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM public.organization_role_slots WHERE project_id=$1 AND organization_id=$2`, r.projectID, organization).Scan(&total); err != nil || total > 64 || total < len(items) {
		return nil, 0, domain.ErrUnavailable
	}
	return items, total - len(items), nil
}

type roleScanner interface{ Scan(...any) error }

func scanRole(row roleScanner) (domain.RoleDefinition, error) {
	var role domain.RoleDefinition
	var modules []byte
	if err := row.Scan(&role.ID, &role.Name, &modules, &role.Version); err != nil {
		return role, err
	}
	if json.Unmarshal(modules, &role.Modules) != nil || role.Modules == nil || !authz.ValidModuleIDs(role.Modules) || role.Version < 1 || role.Version > 1000000000 {
		return role, domain.ErrUnavailable
	}
	return role, nil
}

func (r *Repository) MutateRole(ctx context.Context, scope domain.OperationScope, key string, input domain.RoleMutation) (domain.RoleDefinition, error) {
	if r == nil || r.db == nil || !validScope(scope) || scope.ProjectID != r.projectID || !validKey(key) {
		return domain.RoleDefinition{}, domain.ErrInvalidRequest
	}
	input, err := input.Normalize(scope.OrganizationID)
	if err != nil {
		return domain.RoleDefinition{}, err
	}
	payload, _ := json.Marshal([]any{input.RoleID, input.Name, input.Modules, input.ExpectedVersion})
	digest := sha256.Sum256(payload)
	fingerprint := hex.EncodeToString(digest[:])
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.RoleDefinition{}, domain.ErrUnavailable
	}
	defer tx.Rollback()
	lock := sha256.Sum256([]byte("roles\x00" + scope.ProjectID + "\x00" + scope.OrganizationID))
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(binary.BigEndian.Uint64(lock[:8]))); err != nil {
		return domain.RoleDefinition{}, domain.ErrUnavailable
	}
	var oldFingerprint string
	var result []byte
	err = tx.QueryRowContext(ctx, `SELECT fingerprint,result FROM public.organization_role_mutations WHERE project_id=$1 AND organization_id=$2 AND actor_id=$3 AND operation_key=$4`, scope.ProjectID, scope.OrganizationID, scope.ActorID, key).Scan(&oldFingerprint, &result)
	if err == nil {
		if oldFingerprint != fingerprint {
			return domain.RoleDefinition{}, domain.ErrConflict
		}
		var role domain.RoleDefinition
		if json.Unmarshal(result, &role) != nil || !authz.IsEnterpriseRoleKey(scope.OrganizationID, role.ID) || !authz.ValidModuleIDs(role.Modules) || role.System {
			return domain.RoleDefinition{}, domain.ErrUnavailable
		}
		return role, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.RoleDefinition{}, domain.ErrUnavailable
	}
	var role domain.RoleDefinition
	modules, _ := json.Marshal(input.Modules)
	if input.RoleID == "" {
		var nameExists bool
		nameKey := cases.Fold().String(input.Name)
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM public.organization_roles WHERE project_id=$1 AND organization_id=$2 AND name_key=$3)`, scope.ProjectID, scope.OrganizationID, nameKey).Scan(&nameExists); err != nil {
			return role, domain.ErrUnavailable
		}
		if nameExists {
			return role, domain.ErrConflict
		}
		var slot string
		err = tx.QueryRowContext(ctx, `SELECT s.role_key FROM public.organization_role_slots s WHERE s.project_id=$1 AND s.organization_id=$2 AND NOT EXISTS(SELECT 1 FROM public.organization_roles r WHERE r.project_id=s.project_id AND r.organization_id=s.organization_id AND r.role_key=s.role_key) ORDER BY s.slot LIMIT 1 FOR UPDATE OF s`, scope.ProjectID, scope.OrganizationID).Scan(&slot)
		if errors.Is(err, sql.ErrNoRows) {
			return role, domain.ErrConflict
		}
		if err != nil || !authz.IsEnterpriseRoleKey(scope.OrganizationID, slot) {
			return role, domain.ErrUnavailable
		}
		role, err = scanRole(tx.QueryRowContext(ctx, `INSERT INTO public.organization_roles(project_id,organization_id,role_key,name,name_key,modules,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,1,$7,$7) RETURNING role_key,name,modules,revision`, scope.ProjectID, scope.OrganizationID, slot, input.Name, nameKey, modules, time.Now().UTC()))
	} else {
		role, err = scanRole(tx.QueryRowContext(ctx, `UPDATE public.organization_roles SET modules=$4,revision=revision+1,updated_at=$5 WHERE project_id=$1 AND organization_id=$2 AND role_key=$3 AND revision=$6 RETURNING role_key,name,modules,revision`, scope.ProjectID, scope.OrganizationID, input.RoleID, modules, time.Now().UTC(), input.ExpectedVersion))
	}
	if errors.Is(err, sql.ErrNoRows) {
		return role, domain.ErrConflict
	}
	if err != nil {
		return role, domain.ErrUnavailable
	}
	result, err = json.Marshal(role)
	if err != nil {
		return domain.RoleDefinition{}, domain.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO public.organization_role_mutations(project_id,organization_id,actor_id,operation_key,fingerprint,result,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, scope.ProjectID, scope.OrganizationID, scope.ActorID, key, fingerprint, result, time.Now().UTC()); err != nil {
		return domain.RoleDefinition{}, domain.ErrUnavailable
	}
	if ctx.Err() != nil || tx.Commit() != nil {
		return domain.RoleDefinition{}, domain.ErrUnavailable
	}
	return role, nil
}

func (r *Repository) RoleModules(ctx context.Context, organization string, keys []string) (map[string][]string, error) {
	if len(keys) > 64 {
		return nil, domain.ErrInvalidRequest
	}
	items, _, err := r.Roles(ctx, organization)
	if err != nil {
		return nil, err
	}
	result := map[string][]string{}
	for _, item := range items {
		for _, key := range keys {
			if item.ID == key {
				result[key] = append([]string{}, item.Modules...)
				break
			}
		}
	}
	return result, nil
}
