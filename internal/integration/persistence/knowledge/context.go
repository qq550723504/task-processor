package knowledge

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"task-processor/internal/agent"
	k "task-processor/internal/knowledge"
)

var _ k.ContextRepository = (*Repository)(nil)

type storedBundle struct {
	OrganizationID, ID, ActorID, ContextKind, ContextID, RequestKey string
	Fingerprint, Selection, PolicyVersion, BaseID, Digest           string
	BaseFenceVersion                                                int64
	Payload                                                         []byte
	CreatedAt                                                       time.Time
}
type storedEntry struct {
	OrganizationID, BundleID, BaseID, SourceID, RevisionID, CitationID, ContentDigest string
	SourceFenceVersion                                                                int64
}

func readBundle(tx *gorm.DB, org string, ref k.ContextSnapshotRef) (b storedBundle, err error) {
	err = one(tx, "SELECT * FROM public.knowledge_context_bundles WHERE organization_id=? AND id=?", &b, org, ref.ID)
	if err == nil && (ref.Kind != k.ContextKind || ref.Digest != b.Digest || k.Digest(b.Payload) != b.Digest) {
		err = k.ErrIntegrity
	}
	return
}

// lifecycleBundle locks the same hierarchy used by ingest/disable and permits.
// It checks the exact frozen revisions, not today's readable pointers.
func lifecycleBundle(tx *gorm.DB, b storedBundle) (k.ContextBundle, error) {
	base, err := lockBase(tx, b.OrganizationID, b.BaseID, "UPDATE")
	if err != nil {
		return k.ContextBundle{}, err
	}
	if base.State != k.Active {
		return k.ContextBundle{}, k.ErrInactive
	}
	if base.FenceVersion != b.BaseFenceVersion {
		return k.ContextBundle{}, k.ErrIntegrity
	}
	var entries []storedEntry
	if err := tx.Raw("SELECT * FROM public.knowledge_context_bundle_entries WHERE organization_id=? AND bundle_id=? ORDER BY source_id", b.OrganizationID, b.ID).Scan(&entries).Error; err != nil {
		return k.ContextBundle{}, err
	}
	var payload k.ContextBundle
	if len(b.Payload) > k.MaxContextPayloadBytes || k.Digest(b.Payload) != b.Digest || json.Unmarshal(b.Payload, &payload) != nil || payload.BaseID != b.BaseID || !payload.Binding.Valid() || len(payload.Entries) != len(entries) || len(entries) == 0 || len(entries) > k.MaxActiveSources {
		return k.ContextBundle{}, k.ErrIntegrity
	}
	if _, err := k.EncodeContextBundle(payload); err != nil {
		return k.ContextBundle{}, err
	}
	for i, e := range entries {
		source, err := lockSource(tx, b.OrganizationID, e.SourceID, "UPDATE")
		if err != nil {
			return k.ContextBundle{}, err
		}
		if source.State != k.Active {
			return k.ContextBundle{}, k.ErrInactive
		}
		if source.BaseID != b.BaseID || e.BaseID != b.BaseID || e.SourceFenceVersion != source.FenceVersion {
			return k.ContextBundle{}, k.ErrIntegrity
		}
		var revision k.Revision
		if err := one(tx, "SELECT id,source_id,text,state,warning FROM public.knowledge_revisions WHERE organization_id=? AND source_id=? AND id=?", &revision, b.OrganizationID, e.SourceID, e.RevisionID); err != nil {
			return k.ContextBundle{}, err
		}
		if revision.State != k.Available && revision.State != k.Partial {
			return k.ContextBundle{}, k.ErrNotReadable
		}
		p := payload.Entries[i]
		if p.SourceID != e.SourceID || p.RevisionID != e.RevisionID || p.ContentDigest != e.ContentDigest || k.Digest([]byte(revision.Text)) != e.ContentDigest || p.Text != revision.Text || p.State != revision.State || p.Warning != revision.Warning || p.Citation != (k.Citation{ID: e.CitationID, BaseID: b.BaseID, SourceID: e.SourceID, RevisionID: e.RevisionID, ContentDigest: e.ContentDigest, Location: "text"}) {
			return k.ContextBundle{}, k.ErrIntegrity
		}
	}
	return payload, nil
}

func (r *Repository) Materialize(ctx context.Context, req k.ContextRequest) (k.ContextSnapshotRef, error) {
	fingerprint, err := k.MaterializationFingerprint(req)
	if err != nil {
		return k.ContextSnapshotRef{}, err
	}
	var ref k.ContextSnapshotRef
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		identity, _ := json.Marshal([]string{req.Scope.OrganizationID, req.Scope.ActorID, req.Binding.ContextKind, req.Binding.ContextID, req.Key})
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", string(identity)).Error; err != nil {
			return err
		}
		var existing storedBundle
		found := tx.Raw("SELECT * FROM public.knowledge_context_bundles WHERE organization_id=? AND actor_id=? AND context_kind=? AND context_id=? AND request_key=?", req.Scope.OrganizationID, req.Scope.ActorID, req.Binding.ContextKind, req.Binding.ContextID, req.Key).Scan(&existing)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected != 0 {
			if existing.Fingerprint != fingerprint {
				return k.ErrConflict
			}
			if _, err := lifecycleBundle(tx, existing); err != nil {
				return err
			}
			ref = k.ContextSnapshotRef{Kind: k.ContextKind, ID: existing.ID, Digest: existing.Digest}
			return nil
		}
		baseID := strings.TrimPrefix(req.Selection, "knowledge-base:")
		base, err := lockBase(tx, req.Scope.OrganizationID, baseID, "UPDATE")
		if err != nil {
			return err
		}
		if base.State != k.Active {
			return k.ErrInactive
		}
		var sources []k.Source
		if err := tx.Raw("SELECT * FROM public.knowledge_sources WHERE organization_id=? AND base_id=? AND state='ACTIVE' ORDER BY id LIMIT 5 FOR UPDATE", req.Scope.OrganizationID, base.ID).Scan(&sources).Error; err != nil {
			return err
		}
		if len(sources) == 0 {
			return k.ErrNotReadable
		}
		if len(sources) > k.MaxActiveSources {
			return k.ErrContextTooLarge
		}
		bundle := k.ContextBundle{BaseID: base.ID, Binding: req.Binding, Entries: make([]k.ContextEntry, 0, len(sources))}
		for _, s := range sources {
			if s.CurrentReadableRevisionID == "" {
				return k.ErrNotReadable
			}
			var rev k.Revision
			if err := one(tx, "SELECT id,text,state,warning FROM public.knowledge_revisions WHERE organization_id=? AND source_id=? AND id=?", &rev, req.Scope.OrganizationID, s.ID, s.CurrentReadableRevisionID); err != nil {
				return err
			}
			if (rev.State != k.Available && rev.State != k.Partial) || rev.Text == "" {
				return k.ErrNotReadable
			}
			digest := k.Digest([]byte(rev.Text))
			bundle.Entries = append(bundle.Entries, k.ContextEntry{SourceID: s.ID, RevisionID: rev.ID, Name: s.Name, State: rev.State, Warning: rev.Warning, Text: rev.Text, ContentDigest: digest, Citation: k.Citation{ID: uuid.NewString(), BaseID: base.ID, SourceID: s.ID, RevisionID: rev.ID, ContentDigest: digest, Location: "text"}})
		}
		payload, err := k.EncodeContextBundle(bundle)
		if err != nil {
			return err
		}
		now, err := databaseNow(tx)
		if err != nil {
			return err
		}
		row := storedBundle{OrganizationID: req.Scope.OrganizationID, ID: uuid.NewString(), ActorID: req.Scope.ActorID, ContextKind: req.Binding.ContextKind, ContextID: req.Binding.ContextID, RequestKey: req.Key, Fingerprint: fingerprint, Selection: req.Selection, PolicyVersion: req.PolicyVersion, BaseID: base.ID, BaseFenceVersion: base.FenceVersion, Payload: payload, Digest: k.Digest(payload), CreatedAt: now}
		if err := tx.Table("public.knowledge_context_bundles").Create(&row).Error; err != nil {
			return err
		}
		for i, p := range bundle.Entries {
			e := storedEntry{OrganizationID: req.Scope.OrganizationID, BundleID: row.ID, BaseID: base.ID, SourceID: p.SourceID, RevisionID: p.RevisionID, CitationID: p.Citation.ID, ContentDigest: p.ContentDigest, SourceFenceVersion: sources[i].FenceVersion}
			if err := tx.Table("public.knowledge_context_bundle_entries").Create(&e).Error; err != nil {
				return err
			}
		}
		ref = k.ContextSnapshotRef{Kind: k.ContextKind, ID: row.ID, Digest: row.Digest}
		return nil
	})
	if err != nil {
		return k.ContextSnapshotRef{}, safe(err)
	}
	return ref, nil
}
func (r *Repository) ReadContext(ctx context.Context, scope k.Scope, ref k.ContextSnapshotRef) (k.ContextBundle, error) {
	if !ref.Valid() || scope.OrganizationID == "" || scope.ActorID == "" {
		return k.ContextBundle{}, k.ErrInvalid
	}
	var bundle k.ContextBundle
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		b, err := readBundle(tx, scope.OrganizationID, ref)
		if err != nil {
			return err
		}
		bundle, err = lifecycleBundle(tx, b)
		return err
	})
	if err != nil {
		return k.ContextBundle{}, safe(err)
	}
	return bundle, nil
}

func (r *Repository) AcquireDispatchPermit(ctx context.Context, scope k.Scope, ref k.ContextSnapshotRef, invocation string) (k.DispatchPermit, error) {
	if !ref.Valid() || !agent.ValidID(invocation) || scope.OrganizationID == "" || scope.ActorID == "" {
		return k.DispatchPermit{}, k.ErrInvalid
	}
	var permit k.DispatchPermit
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		b, err := readBundle(tx, scope.OrganizationID, ref)
		if err != nil {
			return err
		}
		if _, err = lifecycleBundle(tx, b); err != nil {
			return err
		}
		now, err := databaseNow(tx)
		if err != nil {
			return err
		}
		id := uuid.NewString()
		expiry := now.Add(k.DispatchPermitLease)
		// A terminal invocation retains its identity too: reacquisition never
		// grants a second transport handoff or becomes an AI retry mechanism.
		result := tx.Exec("INSERT INTO public.knowledge_dispatch_permits(organization_id,id,actor_id,invocation_id,bundle_id,digest,state,acquired_at,expires_at) VALUES(?,?,?,?,?,?,'ACTIVE',?,?) ON CONFLICT(organization_id,invocation_id) DO NOTHING", scope.OrganizationID, id, scope.ActorID, invocation, b.ID, b.Digest, now, expiry)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return k.ErrConflict
		}
		permit = k.DispatchPermit{ID: id, OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, InvocationID: invocation, Ref: ref, ExpiresAt: expiry}
		return nil
	})
	if err != nil {
		return k.DispatchPermit{}, safe(err)
	}
	return permit, nil
}
