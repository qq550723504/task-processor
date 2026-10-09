package agentcustomizationpersistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"math"
	"strconv"
	d "task-processor/internal/agentcustomization"
	"time"
)

type Store struct{ db *sql.DB }

func New(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, d.ErrUnavailable
	}
	return &Store{db: db}, nil
}
func (s *Store) Execute(ctx context.Context, c d.Command) (d.Receipt, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return d.Receipt{}, e
	}
	defer tx.Rollback()
	// Serialize the full identity even before its immutable receipt exists.
	identity, _ := json.Marshal([]any{c.Scope.Platform, c.Scope.OrganizationID, c.Scope.ActorID, c.Key})
	if _, e = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", string(identity)); e != nil {
		return d.Receipt{}, e
	}
	kind := "enterprise"
	if c.Scope.Platform {
		kind = "platform"
	}
	var fingerprint string
	var raw []byte
	e = tx.QueryRowContext(ctx, "SELECT fingerprint,receipt FROM agent_customization.commands WHERE scope_kind=$1 AND organization_id=$2 AND actor_id=$3 AND key=$4", kind, c.Scope.OrganizationID, c.Scope.ActorID, c.Key).Scan(&fingerprint, &raw)
	if e == nil {
		if fingerprint != c.Fingerprint {
			return d.Receipt{}, d.ErrConflict
		}
		var receipt d.Receipt
		if json.Unmarshal(raw, &receipt) != nil {
			return receipt, d.ErrUnavailable
		}
		return receipt, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return d.Receipt{}, e
	}
	now := time.Now().UTC()
	var r d.Request
	if c.Operation == "submit" {
		id := uuid.NewSHA1(uuid.NameSpaceOID, append([]byte("agent-customization:"), identity...)).String()
		input := c.Input
		input.Files = nil
		r = d.Request{ID: id, OrganizationID: c.Scope.OrganizationID, CreatedBy: c.Scope.ActorID, Input: input, Stage: d.Submitted, Version: "1", ConsentVersion: d.ConsentVersion, Attachments: []d.Attachment{}, CreatedAt: now, UpdatedAt: now}
		for i, f := range c.Input.Files {
			mime, e := d.FileType(f.Data)
			if e != nil {
				return d.Receipt{}, e
			}
			r.Attachments = append(r.Attachments, d.Attachment{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(id+":"+strconv.Itoa(i))).String(), Name: f.Name, ContentType: mime, Size: len(f.Data)})
		}
		raw, e = json.Marshal(r)
		if e != nil {
			return d.Receipt{}, e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO agent_customization.requests(id,organization_id,version,payload) VALUES($1,$2,1,$3)", r.ID, r.OrganizationID, raw); e != nil {
			return d.Receipt{}, e
		}
		for i, f := range c.Input.Files {
			digest := sha256.Sum256(f.Data)
			if _, e = tx.ExecContext(ctx, "INSERT INTO agent_customization.attachments(id,request_id,digest,data) VALUES($1,$2,$3,$4)", r.Attachments[i].ID, r.ID, hex.EncodeToString(digest[:]), f.Data); e != nil {
				return d.Receipt{}, e
			}
		}
	} else {
		if !c.Scope.Platform {
			return d.Receipt{}, d.ErrForbidden
		}
		var version int64
		e = tx.QueryRowContext(ctx, "SELECT payload,version FROM agent_customization.requests WHERE id=$1 FOR UPDATE", c.ID).Scan(&raw, &version)
		if errors.Is(e, sql.ErrNoRows) {
			return d.Receipt{}, d.ErrNotFound
		}
		if e != nil {
			return d.Receipt{}, e
		}
		if version != c.Expected || version == math.MaxInt64 {
			return d.Receipt{}, d.ErrRevision
		}
		if json.Unmarshal(raw, &r) != nil || r.ID != c.ID || r.Version != strconv.FormatInt(version, 10) {
			return d.Receipt{}, d.ErrUnavailable
		}
		if e = d.ApplyUpdate(&r, c.Update, c.Scope.Platform); e != nil {
			return d.Receipt{}, e
		}
		if c.Update.DeliverQualityAgent {
			deliveryID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("private-quality:"+r.ID+":"+d.QualityVersion)).String()
			if r.DeliveryID == "" {
				v := d.Delivery{ID: deliveryID, RequestID: r.ID, OrganizationID: r.OrganizationID, Definition: d.QualityDefinition, Version: d.QualityVersion, Name: "平台草稿资料质检", CreatedBy: c.Scope.ActorID, CreatedAt: now}
				payload, err := json.Marshal(v)
				if err != nil {
					return d.Receipt{}, err
				}
				if _, e = tx.ExecContext(ctx, "INSERT INTO agent_customization.deliveries(id,request_id,organization_id,payload) VALUES($1,$2,$3,$4)", v.ID, v.RequestID, v.OrganizationID, payload); e != nil {
					return d.Receipt{}, e
				}
				r.DeliveryID = deliveryID
			} else if r.DeliveryID != deliveryID {
				return d.Receipt{}, d.ErrConflict
			}
		}
		r.Version = strconv.FormatInt(version+1, 10)
		r.UpdatedAt = now
		raw, e = json.Marshal(r)
		if e != nil {
			return d.Receipt{}, e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE agent_customization.requests SET version=$2,payload=$3 WHERE id=$1", r.ID, version+1, raw); e != nil {
			return d.Receipt{}, e
		}
	}
	update := c.Update
	if c.Operation == "submit" {
		update = d.Update{Stage: d.Submitted, Note: "需求已提交；提交不产生费用，等待平台专员评估。"}
	}
	event := d.Event{Version: r.Version, ActorID: c.Scope.ActorID, Update: update, At: now}
	eventRaw, e := json.Marshal(event)
	if e != nil {
		return d.Receipt{}, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO agent_customization.events(request_id,version,payload) VALUES($1,$2,$3)", r.ID, r.Version, eventRaw); e != nil {
		return d.Receipt{}, e
	}
	receipt := d.Receipt{RequestID: r.ID, Key: c.Key, Version: r.Version, Stage: r.Stage, At: now}
	receiptRaw, e := json.Marshal(receipt)
	if e != nil {
		return d.Receipt{}, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO agent_customization.commands(scope_kind,organization_id,actor_id,key,fingerprint,receipt) VALUES($1,$2,$3,$4,$5,$6)", kind, c.Scope.OrganizationID, c.Scope.ActorID, c.Key, c.Fingerprint, receiptRaw); e != nil {
		return d.Receipt{}, e
	}
	if e = tx.Commit(); e != nil {
		return d.Receipt{}, e
	}
	return receipt, nil
}
func (s *Store) List(ctx context.Context, scope d.Scope, cursor string) (d.Page, error) {
	out := d.Page{Items: []d.Request{}}
	rows, e := s.db.QueryContext(ctx, "SELECT payload FROM agent_customization.requests WHERE ($1 OR organization_id=$2) AND ($3='' OR id<NULLIF($3,'')::uuid) ORDER BY id DESC LIMIT 21", scope.Platform, scope.OrganizationID, cursor)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var r d.Request
		if e = rows.Scan(&raw); e != nil {
			return out, e
		}
		if json.Unmarshal(raw, &r) != nil || !scope.Platform && r.OrganizationID != scope.OrganizationID {
			return out, d.ErrUnavailable
		}
		out.Items = append(out.Items, r)
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
func (s *Store) Read(ctx context.Context, scope d.Scope, id string, after int64) (d.Detail, error) {
	out := d.Detail{Events: []d.Event{}}
	var raw []byte
	e := s.db.QueryRowContext(ctx, "SELECT payload FROM agent_customization.requests WHERE id=$1 AND ($2 OR organization_id=$3)", id, scope.Platform, scope.OrganizationID).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return out, d.ErrNotFound
	}
	if e != nil {
		return out, e
	}
	if json.Unmarshal(raw, &out.Request) != nil || out.Request.ID != id || !scope.Platform && out.Request.OrganizationID != scope.OrganizationID {
		return out, d.ErrUnavailable
	}
	rows, e := s.db.QueryContext(ctx, "SELECT payload FROM agent_customization.events WHERE request_id=$1 AND version>$2 AND version<=$3 ORDER BY version LIMIT 51", id, after, out.Request.Version)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var event d.Event
		if e = rows.Scan(&raw); e != nil {
			return out, e
		}
		if json.Unmarshal(raw, &event) != nil {
			return out, d.ErrUnavailable
		}
		out.Events = append(out.Events, event)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if len(out.Events) > 50 {
		out.Events = out.Events[:50]
		out.NextEventVersion = out.Events[49].Version
	}
	return out, nil
}
func (s *Store) Download(ctx context.Context, scope d.Scope, id, file string) (d.Attachment, []byte, error) {
	var meta d.Attachment
	detail, e := s.Read(ctx, scope, id, 0)
	if e != nil {
		return meta, nil, e
	}
	found := false
	for _, v := range detail.Request.Attachments {
		if v.ID == file {
			meta = v
			found = true
			break
		}
	}
	if !found {
		return meta, nil, d.ErrNotFound
	}
	var digest string
	var data []byte
	e = s.db.QueryRowContext(ctx, "SELECT digest,data FROM agent_customization.attachments WHERE id=$1 AND request_id=$2", file, id).Scan(&digest, &data)
	if e != nil {
		return meta, nil, e
	}
	sum := sha256.Sum256(data)
	if len(data) != meta.Size || hex.EncodeToString(sum[:]) != digest {
		return meta, nil, d.ErrUnavailable
	}
	return meta, data, nil
}
