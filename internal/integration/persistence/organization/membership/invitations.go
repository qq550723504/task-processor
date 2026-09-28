package membership

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"task-processor/internal/authidentity"
	flow "task-processor/internal/organization/membership/inviteflow"
	"time"
)

const invitationTable = "public.organization_member_invitations"

type InvitationRepository struct{ *Repository }

func (r *Repository) Invitations() *InvitationRepository { return &InvitationRepository{r} }

type invitationQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (r *InvitationRepository) read(ctx context.Context, q invitationQuery, id string, lock bool) (flow.Invitation, error) {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id || parsed == uuid.Nil {
		return flow.Invitation{}, flow.ErrInvalid
	}
	query := `SELECT organization_id,contact,state,revision,expires_at,payload FROM ` + invitationTable + ` WHERE project_id=$1 AND invitation_id=$2`
	if lock {
		query += ` FOR UPDATE`
	}
	var org, contact, state string
	var revision int64
	var expires time.Time
	var raw []byte
	err = q.QueryRowContext(ctx, query, r.projectID, id).Scan(&org, &contact, &state, &revision, &expires, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return flow.Invitation{}, flow.ErrNotFound
	}
	if err != nil {
		return flow.Invitation{}, flow.ErrUnavailable
	}
	var inv flow.Invitation
	var stored invitationPayload
	if len(raw) > 16384 || json.Unmarshal(raw, &stored) != nil {
		return inv, flow.ErrUnavailable
	}
	inv = stored.Invitation
	inv.ProjectID = stored.ProjectID
	inv.Fingerprint = stored.Fingerprint
	inv.DispatchID = stored.DispatchID
	inv.DeliveryAttempt = stored.DeliveryAttempt
	if inv.ID != id || inv.ProjectID != r.projectID || inv.OrganizationID != org || inv.Contact != contact || inv.State != state || inv.Revision != revision || !inv.ExpiresAt.Equal(expires) {
		return flow.Invitation{}, flow.ErrUnavailable
	}
	return inv, nil
}

// The JSON receipt contains hidden protocol facts as well as the bounded public view.
type invitationPayload struct {
	flow.Invitation
	ProjectID       string `json:"projectId"`
	Fingerprint     string `json:"fingerprint"`
	DispatchID      string `json:"dispatchId"`
	DeliveryAttempt string `json:"deliveryAttempt"`
}

func invitationJSON(inv flow.Invitation) ([]byte, error) {
	return json.Marshal(invitationPayload{Invitation: inv, ProjectID: inv.ProjectID, Fingerprint: inv.Fingerprint, DispatchID: inv.DispatchID, DeliveryAttempt: inv.DeliveryAttempt})
}
func (r *InvitationRepository) Read(ctx context.Context, id string) (flow.Invitation, error) {
	return r.read(ctx, r.db, id, false)
}
func (r *InvitationRepository) Create(ctx context.Context, inv flow.Invitation) (flow.Invitation, bool, error) {
	// PostgreSQL timestamptz stores microseconds. Keep indexed facts and the
	// immutable receipt identical so a read can validate both representations.
	inv.CreatedAt = inv.CreatedAt.UTC().Truncate(time.Microsecond)
	inv.ExpiresAt = inv.ExpiresAt.UTC().Truncate(time.Microsecond)
	if inv.ProjectID != r.projectID || !authidentity.IsBoundedIdentifier(inv.OrganizationID) || !fingerprintPattern.MatchString(inv.Fingerprint) || inv.State != flow.Pending || inv.Revision != 1 {
		return flow.Invitation{}, false, flow.ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return inv, false, flow.ErrUnavailable
	}
	defer tx.Rollback()
	// Serialize same-contact admissions, including expiration of pending receipts.
	_, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.projectID+"\x1f"+inv.OrganizationID+"\x1f"+inv.Contact)
	if err != nil {
		return inv, false, flow.ErrUnavailable
	}
	old, err := r.read(ctx, tx, inv.ID, true)
	if err == nil {
		if old.OrganizationID != inv.OrganizationID || old.CreatorID != inv.CreatorID || old.Fingerprint != inv.Fingerprint {
			return old, false, flow.ErrConflict
		}
		return old, true, tx.Commit()
	}
	if err != flow.ErrNotFound {
		return inv, false, err
	}
	// Expire only pending. An accepting/UNKNOWN receipt holds its contact forever.
	rows, err := tx.QueryContext(ctx, `SELECT invitation_id::text FROM `+invitationTable+` WHERE project_id=$1 AND organization_id=$2 AND contact=$3 AND state='pending' AND expires_at<=$4 FOR UPDATE`, r.projectID, inv.OrganizationID, inv.Contact, inv.CreatedAt)
	if err != nil {
		return inv, false, flow.ErrUnavailable
	}
	var expired []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil {
			rows.Close()
			return inv, false, flow.ErrUnavailable
		}
		expired = append(expired, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return inv, false, flow.ErrUnavailable
	}
	for _, id := range expired {
		old, err := r.read(ctx, tx, id, true)
		if err != nil {
			return inv, false, err
		}
		old, err = flow.Transition(old, flow.Change{State: flow.Expired, Now: inv.CreatedAt})
		if err != nil {
			return inv, false, err
		}
		if err = r.save(ctx, tx, old); err != nil {
			return inv, false, err
		}
	}
	raw, err := invitationJSON(inv)
	if err != nil {
		return inv, false, flow.ErrInvalid
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO `+invitationTable+`(project_id,organization_id,invitation_id,contact,state,revision,expires_at,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, r.projectID, inv.OrganizationID, inv.ID, inv.Contact, inv.State, inv.Revision, inv.ExpiresAt, raw)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return inv, false, flow.ErrConflict
		}
		return inv, false, flow.ErrUnavailable
	}
	return inv, false, tx.Commit()
}
func (r *InvitationRepository) save(ctx context.Context, tx *sql.Tx, inv flow.Invitation) error {
	raw, err := invitationJSON(inv)
	if err != nil {
		return flow.ErrInvalid
	}
	_, err = tx.ExecContext(ctx, `UPDATE `+invitationTable+` SET state=$3,revision=$4,payload=$5 WHERE project_id=$1 AND invitation_id=$2`, r.projectID, inv.ID, inv.State, inv.Revision, raw)
	if err != nil {
		return flow.ErrUnavailable
	}
	return nil
}
func (r *InvitationRepository) change(ctx context.Context, id string, f func(flow.Invitation) (flow.Invitation, error)) (flow.Invitation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return flow.Invitation{}, flow.ErrUnavailable
	}
	defer tx.Rollback()
	inv, err := r.read(ctx, tx, id, true)
	if err != nil {
		return inv, err
	}
	inv, err = f(inv)
	if err != nil {
		return inv, err
	}
	if err = r.save(ctx, tx, inv); err != nil {
		return inv, err
	}
	if err = tx.Commit(); err != nil {
		return inv, flow.ErrUnavailable
	}
	return inv, nil
}
func (r *InvitationRepository) Change(ctx context.Context, id string, revision int64, c flow.Change) (flow.Invitation, error) {
	return r.change(ctx, id, func(inv flow.Invitation) (flow.Invitation, error) {
		if revision != inv.Revision {
			return inv, flow.ErrConflict
		}
		return flow.Transition(inv, c)
	})
}
func (r *InvitationRepository) ClaimDelivery(ctx context.Context, id, attempt string, now time.Time) (flow.Invitation, error) {
	return r.change(ctx, id, func(inv flow.Invitation) (flow.Invitation, error) {
		if inv.State != flow.Pending || !now.Before(inv.ExpiresAt) || inv.DeliveryState == "sending" && inv.DeliveryUpdatedAt != nil && now.Sub(*inv.DeliveryUpdatedAt) < 30*time.Second {
			return inv, flow.ErrConflict
		}
		inv.DeliveryState = "sending"
		inv.DeliveryAttempt = attempt
		inv.DeliveryAttempts++
		inv.DeliveryUpdatedAt = &now
		return inv, nil
	})
}
func (r *InvitationRepository) FinishDelivery(ctx context.Context, id, attempt, status string, now time.Time) (flow.Invitation, error) {
	return r.change(ctx, id, func(inv flow.Invitation) (flow.Invitation, error) {
		if status != "mail_server_accepted" && status != "delivery_unknown" {
			return inv, flow.ErrInvalid
		}
		if inv.DeliveryAttempt != attempt || inv.DeliveryState != "sending" {
			return inv, flow.ErrConflict
		}
		inv.DeliveryState = status
		inv.DeliveryUpdatedAt = &now
		return inv, nil
	})
}
func (r *InvitationRepository) List(ctx context.Context, org string, limit, offset int) (flow.Page, error) {
	page := flow.Page{Items: []flow.Invitation{}}
	if !authidentity.IsBoundedIdentifier(org) || limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		return page, flow.ErrInvalid
	}
	if err := r.db.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE state='pending' AND expires_at>now()) FROM `+invitationTable+` WHERE project_id=$1 AND organization_id=$2`, r.projectID, org).Scan(&page.Total, &page.Pending); err != nil {
		return page, flow.ErrUnavailable
	}
	rows, err := r.db.QueryContext(ctx, `SELECT invitation_id::text FROM `+invitationTable+` WHERE project_id=$1 AND organization_id=$2 ORDER BY expires_at DESC,invitation_id DESC LIMIT $3 OFFSET $4`, r.projectID, org, limit, offset)
	if err != nil {
		return page, flow.ErrUnavailable
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil {
			rows.Close()
			return page, flow.ErrUnavailable
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, flow.ErrUnavailable
	}
	for _, id := range ids {
		inv, err := r.Read(ctx, id)
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, inv)
	}
	return page, nil
}
