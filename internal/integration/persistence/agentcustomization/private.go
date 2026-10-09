package agentcustomizationpersistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	d "task-processor/internal/agentcustomization"
	"task-processor/internal/product/quality"
	"time"
)

type rowReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readDelivery(ctx context.Context, db rowReader, scope d.Scope, id string) (d.Delivery, error) {
	var v d.Delivery
	var raw []byte
	e := db.QueryRowContext(ctx, "SELECT payload FROM agent_customization.deliveries WHERE id=$1 AND organization_id=$2", id, scope.OrganizationID).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return v, d.ErrNotFound
	}
	if e != nil {
		return v, e
	}
	if json.Unmarshal(raw, &v) != nil || v.ID != id || !validDelivery(v, scope) {
		return v, d.ErrUnavailable
	}
	return v, nil
}
func validDelivery(v d.Delivery, scope d.Scope) bool {
	return !scope.Platform && v.OrganizationID == scope.OrganizationID && v.Definition == d.QualityDefinition && v.Version == d.QualityVersion && d.UUID(v.ID) && d.UUID(v.RequestID)
}
func (s *Store) Delivery(ctx context.Context, scope d.Scope, id string) (d.Delivery, error) {
	return readDelivery(ctx, s.db, scope, id)
}
func (s *Store) Deliveries(ctx context.Context, scope d.Scope, cursor string) (d.DeliveryPage, error) {
	out := d.DeliveryPage{Items: []d.Delivery{}}
	rows, e := s.db.QueryContext(ctx, "SELECT payload FROM agent_customization.deliveries WHERE organization_id=$1 AND ($2='' OR id<NULLIF($2,'')::uuid) ORDER BY id DESC LIMIT 21", scope.OrganizationID, cursor)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v d.Delivery
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			return out, e
		}
		if json.Unmarshal(raw, &v) != nil || !validDelivery(v, scope) {
			return out, d.ErrUnavailable
		}
		out.Items = append(out.Items, v)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if len(out.Items) > 20 {
		out.Items = out.Items[:20]
		out.NextCursor = out.Items[19].ID
	}
	return out, nil
}
func (s *Store) RunQuality(ctx context.Context, c d.RunCommand) (d.QualityRun, error) {
	var out d.QualityRun
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	identity, _ := json.Marshal([]string{c.Scope.OrganizationID, c.Scope.ActorID, c.Key})
	if _, e = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "quality:"+string(identity)); e != nil {
		return out, e
	}
	// Current delivery authorization is checked before every receipt replay.
	delivery, e := readDelivery(ctx, tx, c.Scope, c.DeliveryID)
	if e != nil {
		return out, e
	}
	var raw []byte
	var fingerprint string
	e = tx.QueryRowContext(ctx, "SELECT fingerprint,payload FROM agent_customization.quality_runs WHERE organization_id=$1 AND actor_id=$2 AND key=$3", c.Scope.OrganizationID, c.Scope.ActorID, c.Key).Scan(&fingerprint, &raw)
	if e == nil {
		if fingerprint != c.Fingerprint {
			return out, d.ErrConflict
		}
		if json.Unmarshal(raw, &out) != nil || out.OrganizationID != c.Scope.OrganizationID || out.ActorID != c.Scope.ActorID || out.Key != c.Key || out.DeliveryID != delivery.ID {
			return out, d.ErrUnavailable
		}
		return out, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return out, e
	}
	report, e := quality.Check(c.Input)
	if e != nil {
		return out, d.ErrInvalid
	}
	out = d.QualityRun{ID: uuid.NewSHA1(uuid.NameSpaceOID, append([]byte("private-quality-run:"), identity...)).String(), DeliveryID: delivery.ID, OrganizationID: c.Scope.OrganizationID, ActorID: c.Scope.ActorID, Key: c.Key, Definition: delivery.Definition, Version: delivery.Version, Input: c.Input, Report: report, CreatedAt: time.Now().UTC()}
	raw, e = json.Marshal(out)
	if e != nil {
		return out, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO agent_customization.quality_runs(id,delivery_id,organization_id,actor_id,key,fingerprint,payload) VALUES($1,$2,$3,$4,$5,$6,$7)", out.ID, out.DeliveryID, out.OrganizationID, out.ActorID, out.Key, c.Fingerprint, raw); e != nil {
		return out, e
	}
	if e = tx.Commit(); e != nil {
		return d.QualityRun{}, e
	}
	return out, nil
}
func (s *Store) QualityRuns(ctx context.Context, scope d.Scope, id, cursor string) (d.QualityRunPage, error) {
	out := d.QualityRunPage{Items: []d.QualityRun{}}
	if _, e := s.Delivery(ctx, scope, id); e != nil {
		return out, e
	}
	rows, e := s.db.QueryContext(ctx, "SELECT payload FROM agent_customization.quality_runs WHERE organization_id=$1 AND delivery_id=$2 AND ($3='' OR id<NULLIF($3,'')::uuid) ORDER BY id DESC LIMIT 21", scope.OrganizationID, id, cursor)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v d.QualityRun
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			return out, e
		}
		if json.Unmarshal(raw, &v) != nil || v.OrganizationID != scope.OrganizationID || v.DeliveryID != id || v.Definition != d.QualityDefinition || v.Version != d.QualityVersion {
			return out, d.ErrUnavailable
		}
		out.Items = append(out.Items, v)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if len(out.Items) > 20 {
		out.Items = out.Items[:20]
		out.NextCursor = out.Items[19].ID
	}
	return out, nil
}
