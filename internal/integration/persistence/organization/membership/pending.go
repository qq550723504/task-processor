package membership

import (
	"context"
	"database/sql"

	domain "task-processor/internal/organization/membership"
)

func (r *Repository) ListPending(ctx context.Context, scope domain.OperationScope, page domain.PendingPageRequest) (domain.PendingPage, error) {
	if !validScope(scope) || scope.ProjectID != r.projectID || !page.Valid() {
		return domain.PendingPage{}, domain.ErrInvalidRequest
	}
	rows, err := r.db.QueryContext(ctx, `SELECT operation_key::text,payload,revision,target_user_id,fingerprint,active,invite_email FROM `+table+`
 WHERE project_id=$1 AND organization_id=$2 AND actor_id=$3 AND active
 AND ($4='' OR operation_key>NULLIF($4,'')::uuid) ORDER BY operation_key LIMIT $5`, scope.ProjectID, scope.OrganizationID, scope.ActorID, page.After, page.Limit+1)
	if err != nil {
		return domain.PendingPage{}, domain.ErrUnavailable
	}
	defer rows.Close()
	result := domain.PendingPage{Scope: scope, Items: []domain.Operation{}}
	for rows.Next() {
		var key, target, fingerprint string
		var payload []byte
		var revision int64
		var active bool
		var email sql.NullString
		if rows.Scan(&key, &payload, &revision, &target, &fingerprint, &active, &email) != nil {
			return domain.PendingPage{}, domain.ErrUnavailable
		}
		op, err := decodeOperation(scope, key, payload, revision, target, fingerprint, active, email)
		if err != nil || !domain.PendingPhase(op.Phase) {
			return domain.PendingPage{}, domain.ErrUnavailable
		}
		if len(result.Items) == page.Limit {
			result.Next = result.Items[len(result.Items)-1].Key
			break
		}
		result.Items = append(result.Items, op)
	}
	if rows.Err() != nil || ctx.Err() != nil {
		return domain.PendingPage{}, domain.ErrUnavailable
	}
	return result, nil
}
