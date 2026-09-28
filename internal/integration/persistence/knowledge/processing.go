package knowledge

import (
	"context"
	"time"

	"gorm.io/gorm"
	k "task-processor/internal/knowledge"
)

func (r *Repository) ClaimUpload(ctx context.Context, org, id, owner string) (revision k.Revision, claimed bool, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rev k.Revision
		if e := one(tx, "SELECT * FROM public.knowledge_revisions WHERE organization_id=? AND id=?", &rev, org, id); e != nil {
			return e
		}
		baseID, e := sourceBase(tx, org, rev.SourceID)
		if e != nil {
			return e
		}
		base, e := lockBase(tx, org, baseID, "UPDATE")
		if e != nil {
			return e
		}
		source, e := lockSource(tx, org, rev.SourceID, "UPDATE")
		if e != nil {
			return e
		}
		if e := one(tx, "SELECT * FROM public.knowledge_revisions WHERE organization_id=? AND id=? FOR UPDATE", &rev, org, id); e != nil {
			return e
		}
		revision = rev
		now, clockErr := databaseNow(tx)
		if clockErr != nil {
			return clockErr
		}
		if rev.State == k.Failed && rev.Failure == "UPLOAD_INCOMPLETE" {
			if source.LatestRevisionID != rev.ID {
				return k.ErrConflict
			}
		} else if rev.State != k.Admitted {
			return nil
		}
		// A completed same-key operation is read-only reconciliation, including
		// after disable. Only an actual upload/resume requires ACTIVE lifecycle.
		if base.State != k.Active || source.State != k.Active {
			return k.ErrInactive
		}
		if rev.LeaseUntil != nil && rev.LeaseUntil.After(now) {
			return nil
		}
		until := now.Add(30 * time.Second)
		rev.State = k.Admitted
		rev.Failure = ""
		rev.LeaseOwner = owner
		rev.LeaseUntil = &until
		if e := tx.Exec("UPDATE public.knowledge_revisions SET state=?,failure='',lease_owner=?,lease_until=?,updated_at=? WHERE organization_id=? AND id=?", rev.State, owner, until, now, org, id).Error; e != nil {
			return e
		}
		revision = rev
		claimed = true
		return nil
	})
	return revision, claimed, safe(err)
}
func (r *Repository) ClaimProcessing(ctx context.Context, owner string, max int) (revisions []k.Revision, err error) {
	if max < 1 || max > 4 {
		return nil, k.ErrInvalid
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, clockErr := databaseNow(tx)
		if clockErr != nil {
			return clockErr
		}
		if e := tx.Raw("SELECT * FROM public.knowledge_revisions WHERE state IN ('ADMITTED','OBJECT_STORED','PROCESSING') AND next_attempt_at<=? AND (lease_until IS NULL OR lease_until<=?) ORDER BY next_attempt_at,id LIMIT ? FOR UPDATE SKIP LOCKED", now, now, max).Scan(&revisions).Error; e != nil {
			return e
		}
		runnable := make([]k.Revision, 0, len(revisions))
		for i := range revisions {
			rev := &revisions[i]
			if rev.State != k.Admitted && rev.Attempts >= 3 {
				if e := tx.Exec("UPDATE public.knowledge_revisions SET state='FAILED',failure='PARSER_RETRIES_EXHAUSTED',lease_owner='',lease_until=NULL,updated_at=? WHERE organization_id=? AND id=?", now, rev.OrganizationID, rev.ID).Error; e != nil {
					return e
				}
				continue
			}
			until := now.Add(30 * time.Second)
			rev.LeaseOwner = owner
			rev.LeaseUntil = &until
			if rev.State != k.Admitted {
				rev.State = k.Processing
				rev.Attempts++
			}
			if e := tx.Exec("UPDATE public.knowledge_revisions SET state=?,lease_owner=?,lease_until=?,attempts=?,updated_at=? WHERE organization_id=? AND id=?", rev.State, owner, until, rev.Attempts, now, rev.OrganizationID, rev.ID).Error; e != nil {
				return e
			}
			runnable = append(runnable, *rev)
		}
		revisions = runnable
		return nil
	})
	return revisions, safe(err)
}
func (r *Repository) ConfirmObject(ctx context.Context, revision k.Revision) error {
	raw := r.db.WithContext(ctx).Exec("UPDATE public.knowledge_revisions SET state='OBJECT_STORED',failure='',lease_owner='',lease_until=NULL,next_attempt_at=now(),updated_at=now() WHERE organization_id=? AND id=? AND state='ADMITTED' AND lease_owner=? AND lease_until>now()", revision.OrganizationID, revision.ID, revision.LeaseOwner)
	if raw.Error != nil {
		return safe(raw.Error)
	}
	if raw.RowsAffected != 1 {
		return k.ErrLeaseLost
	}
	return nil
}
func (r *Repository) Finish(ctx context.Context, revision k.Revision, result k.ParseResult) error {
	return safe(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		org := revision.OrganizationID
		baseID, e := sourceBase(tx, org, revision.SourceID)
		if e != nil {
			return e
		}
		base, e := lockBase(tx, org, baseID, "UPDATE")
		if e != nil {
			return e
		}
		source, e := lockSource(tx, org, revision.SourceID, "UPDATE")
		if e != nil {
			return e
		}
		var current k.Revision
		if e := one(tx, "SELECT * FROM public.knowledge_revisions WHERE organization_id=? AND id=? FOR UPDATE", &current, org, revision.ID); e != nil {
			return e
		}
		now, clockErr := databaseNow(tx)
		if clockErr != nil {
			return clockErr
		}
		if current.LeaseOwner != revision.LeaseOwner || current.LeaseUntil == nil || !current.LeaseUntil.After(now) || (current.State != k.Admitted && current.State != k.Processing) {
			return k.ErrLeaseLost
		}
		state := k.Failed
		next := now
		if result.Transient && current.Attempts < 3 {
			state = k.ObjectStored
			next = now.Add(time.Duration(current.Attempts*5) * time.Second)
			result.Text = ""
			result.Warning = ""
		} else if result.Failure == "" {
			normalized := k.NormalizeText(result.Text, result.Warning)
			result = normalized
			if result.Failure == "" {
				state = k.Available
				if result.Warning != "" {
					state = k.Partial
				}
			}
		}
		if len(result.Failure) > 64 || len(result.Warning) > 64 {
			return k.ErrInvalid
		}
		if e := tx.Exec("UPDATE public.knowledge_revisions SET state=?,text=?,failure=?,warning=?,lease_owner='',lease_until=NULL,next_attempt_at=?,updated_at=? WHERE organization_id=? AND id=?", state, result.Text, result.Failure, result.Warning, next, now, org, revision.ID).Error; e != nil {
			return e
		}
		if (state == k.Available || state == k.Partial) && base.State == k.Active && source.State == k.Active && source.LatestRevisionID == revision.ID {
			return tx.Exec("UPDATE public.knowledge_sources SET current_readable_revision_id=?,version=version+1,updated_at=? WHERE organization_id=? AND id=? AND latest_revision_id=?", revision.ID, now, org, source.ID, revision.ID).Error
		}
		return nil
	}))
}
