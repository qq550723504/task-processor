package membership

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	d "task-processor/internal/organization/membership"
	flow "task-processor/internal/organization/membership/inviteflow"
)

func (r *InvitationRepository) ListNoticeInvitations(ctx context.Context, q flow.NoticeQuery) ([]flow.Invitation, string, error) {
	if q.Limit < 1 || q.Limit > 100 || q.After != "" && uuid.Validate(q.After) != nil || q.OrganizationID == "" && q.Contact == "" || q.OrganizationID != "" && q.Contact != "" {
		return nil, "", flow.ErrInvalid
	}
	rows, e := r.db.QueryContext(ctx, `SELECT invitation_id::text FROM `+invitationTable+` WHERE project_id=$1 AND ($2='' OR organization_id=$2) AND ($3='' OR contact=$3) AND ($4='' OR invitation_id>NULLIF($4,'')::uuid) ORDER BY invitation_id LIMIT $5`, r.projectID, q.OrganizationID, q.Contact, q.After, q.Limit+1)
	if e != nil {
		return nil, "", flow.ErrUnavailable
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil {
			rows.Close()
			return nil, "", flow.ErrUnavailable
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, "", flow.ErrUnavailable
	}
	next := ""
	if len(ids) > q.Limit {
		ids = ids[:q.Limit]
		next = ids[len(ids)-1]
	}
	result := []flow.Invitation{}
	for _, id := range ids {
		inv, e := r.Read(ctx, id)
		if e != nil {
			return nil, "", e
		}
		if q.Contact != "" && inv.RecipientID != "" && inv.RecipientID != q.RecipientID {
			continue
		}
		result = append(result, inv)
	}
	return result, next, nil
}
func (r *Repository) ListNoticeOperations(ctx context.Context, scope d.OperationScope, after string, limit int) ([]d.Operation, string, error) {
	if !validScope(scope) || scope.ProjectID != r.projectID || limit < 1 || limit > 100 || after != "" && uuid.Validate(after) != nil {
		return nil, "", d.ErrInvalidRequest
	}
	rows, e := r.db.QueryContext(ctx, `SELECT operation_key::text,payload,revision,target_user_id,fingerprint,active,invite_email FROM `+table+` WHERE project_id=$1 AND organization_id=$2 AND actor_id=$3 AND ($4='' OR operation_key>NULLIF($4,'')::uuid) ORDER BY operation_key LIMIT $5`, scope.ProjectID, scope.OrganizationID, scope.ActorID, after, limit+1)
	if e != nil {
		return nil, "", d.ErrUnavailable
	}
	defer rows.Close()
	result := []d.Operation{}
	next := ""
	for rows.Next() {
		var key, target, fingerprint string
		var payload []byte
		var revision int64
		var active bool
		var email sql.NullString
		if rows.Scan(&key, &payload, &revision, &target, &fingerprint, &active, &email) != nil {
			return nil, "", d.ErrUnavailable
		}
		op, e := decodeOperation(scope, key, payload, revision, target, fingerprint, active, email)
		if e != nil {
			return nil, "", e
		}
		if len(result) == limit {
			next = result[len(result)-1].Key
			break
		}
		result = append(result, op)
	}
	if rows.Err() != nil {
		return nil, "", d.ErrUnavailable
	}
	return result, next, nil
}
