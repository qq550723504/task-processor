package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	k "task-processor/internal/knowledge"
)

type Repository struct{ db *gorm.DB }

var _ k.Repository = (*Repository)(nil)

func safe(err error) error {
	if err == nil {
		return nil
	}
	for _, known := range []error{k.ErrInvalid, k.ErrNotFound, k.ErrConflict, k.ErrInactive, k.ErrSourceLimit, k.ErrRevisionBusy, k.ErrNotReadable, k.ErrLeaseLost, k.ErrIntegrity, k.ErrContextTooLarge, k.ErrForbidden, k.ErrSelectionChanged} {
		if errors.Is(err, known) {
			return known
		}
	}
	return k.ErrUnavailable
}
func one(tx *gorm.DB, query string, dest any, args ...any) error {
	r := tx.Raw(query, args...).Scan(dest)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return k.ErrNotFound
	}
	return nil
}
func lockBase(tx *gorm.DB, org, id, mode string) (b k.Base, err error) {
	err = one(tx, "SELECT * FROM public.knowledge_bases WHERE organization_id=? AND id=? FOR "+mode, &b, org, id)
	return
}
func lockSource(tx *gorm.DB, org, id, mode string) (s k.Source, err error) {
	err = one(tx, "SELECT * FROM public.knowledge_sources WHERE organization_id=? AND id=? FOR "+mode, &s, org, id)
	return
}
func sourceBase(tx *gorm.DB, org, id string) (string, error) {
	var s k.Source
	err := one(tx, "SELECT base_id FROM public.knowledge_sources WHERE organization_id=? AND id=?", &s, org, id)
	return s.BaseID, err
}
func databaseNow(tx *gorm.DB) (time.Time, error) {
	var clock struct{ Now time.Time }
	err := tx.Raw("SELECT clock_timestamp() AS now").Scan(&clock).Error
	return clock.Now, err
}

func (r *Repository) Apply(ctx context.Context, c k.Command) (result k.Result, resultErr error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", c.Scope.OrganizationID+":"+c.Kind+":"+c.Key).Error; err != nil {
			return err
		}
		var existing struct {
			Fingerprint          string
			Result               []byte
			SourceID, RevisionID string
		}
		found := tx.Raw("SELECT fingerprint,result,source_id,revision_id FROM public.knowledge_ingest_operations WHERE organization_id=? AND kind=? AND key=?", c.Scope.OrganizationID, c.Kind, c.Key).Scan(&existing)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected > 0 {
			if existing.Fingerprint != c.Fingerprint {
				return k.ErrConflict
			}
			if existing.RevisionID != "" {
				source, err := readSource(tx, c.Scope.OrganizationID, existing.SourceID)
				if err != nil {
					return err
				}
				var rev k.Revision
				if err := one(tx, "SELECT * FROM public.knowledge_revisions WHERE organization_id=? AND id=?", &rev, c.Scope.OrganizationID, existing.RevisionID); err != nil {
					return err
				}
				result = k.Result{Source: &source, Revision: &rev}
				return nil
			}
			return json.Unmarshal(existing.Result, &result)
		}
		org, actor := c.Scope.OrganizationID, c.Scope.ActorID
		now, clockErr := databaseNow(tx)
		if clockErr != nil {
			return clockErr
		}
		if c.Kind == "base_create" {
			base := k.Base{ID: uuid.NewString(), OrganizationID: org, Name: c.Name, State: k.Active, Version: 1, FenceVersion: 1, CreatedBy: actor, UpdatedBy: actor, CreatedAt: now, UpdatedAt: now}
			if err := tx.Table("public.knowledge_bases").Create(&base).Error; err != nil {
				return err
			}
			result.Base = &base
		} else {
			baseID := c.BaseID
			if c.SourceID != "" {
				var err error
				baseID, err = sourceBase(tx, org, c.SourceID)
				if err != nil {
					return err
				}
			}
			base, err := lockBase(tx, org, baseID, "UPDATE")
			if err != nil {
				return err
			}
			if c.Kind == "base_update" || c.Kind == "base_disable" {
				if base.Version != c.Version {
					return k.ErrConflict
				}
				if c.Kind == "base_update" {
					if base.State != k.Active {
						return k.ErrInactive
					}
					base.Name = c.Name
				} else if base.State == k.Active {
					base.State, err = disableState(tx, org, base.ID, "", now)
					if err != nil {
						return err
					}
					base.FenceVersion++
				}
				base.Version++
				base.UpdatedBy = actor
				base.UpdatedAt = now
				if err := tx.Exec("UPDATE public.knowledge_bases SET name=?,state=?,version=?,fence_version=?,updated_by=?,updated_at=? WHERE organization_id=? AND id=?", base.Name, base.State, base.Version, base.FenceVersion, actor, now, org, base.ID).Error; err != nil {
					return err
				}
				result.Base = &base
			} else {
				if base.State != k.Active {
					return k.ErrInactive
				}
				var source k.Source
				if c.Kind == "source_create" {
					var count int64
					if err := tx.Table("public.knowledge_sources").Where("organization_id=? AND base_id=? AND state=?", org, base.ID, k.Active).Count(&count).Error; err != nil {
						return err
					}
					if count >= k.MaxActiveSources {
						return k.ErrSourceLimit
					}
					source = k.Source{ID: uuid.NewString(), OrganizationID: org, BaseID: base.ID, Name: c.Name, State: k.Active, Version: 1, FenceVersion: 1, CreatedBy: actor, UpdatedBy: actor, CreatedAt: now, UpdatedAt: now}
					if err := tx.Exec("INSERT INTO public.knowledge_sources(organization_id,id,base_id,name,state,version,fence_version,created_by,updated_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", org, source.ID, base.ID, c.Name, k.Active, 1, 1, actor, actor, now, now).Error; err != nil {
						return err
					}
				} else {
					source, err = lockSource(tx, org, c.SourceID, "UPDATE")
					if err != nil {
						return err
					}
					if source.Version != c.Version {
						return k.ErrConflict
					}
					if c.Kind == "revision_create" {
						if source.State != k.Active {
							return k.ErrInactive
						}
						var count int64
						if err := tx.Table("public.knowledge_revisions").Where("organization_id=? AND source_id=? AND state IN ?", org, source.ID, []k.ProcessingState{k.Admitted, k.ObjectStored, k.Processing}).Count(&count).Error; err != nil {
							return err
						}
						if count > 0 {
							return k.ErrRevisionBusy
						}
					}
					source.Version++
					source.UpdatedAt = now
					source.UpdatedBy = actor
				}
				if c.Kind == "source_disable" {
					if source.State == k.Active {
						source.State, err = disableState(tx, org, base.ID, source.ID, now)
						if err != nil {
							return err
						}
						source.FenceVersion++
					}
					if err := tx.Exec("UPDATE public.knowledge_sources SET state=?,version=?,fence_version=?,updated_by=?,updated_at=? WHERE organization_id=? AND id=?", source.State, source.Version, source.FenceVersion, actor, now, org, source.ID).Error; err != nil {
						return err
					}
					read, err := readSource(tx, org, source.ID)
					if err != nil {
						return err
					}
					result.Source = &read
				} else {
					if c.Upload == nil {
						return k.ErrInvalid
					}
					revision := *c.Upload
					revision.ID = uuid.NewString()
					revision.OrganizationID = org
					revision.SourceID = source.ID
					if err := tx.Raw("SELECT COALESCE(MAX(number),0)+1 FROM public.knowledge_revisions WHERE organization_id=? AND source_id=?", org, source.ID).Scan(&revision.Number).Error; err != nil {
						return err
					}
					revision.ObjectKey = fmt.Sprintf("knowledge/%s/%s", revision.ID, revision.SHA256)
					revision.State = k.Admitted
					revision.CreatedAt = now
					revision.UpdatedAt = now
					revision.NextAttemptAt = now.Add(30 * time.Second)
					if err := tx.Table("public.knowledge_revisions").Create(&revision).Error; err != nil {
						return err
					}
					source.LatestRevisionID = revision.ID
					if err := tx.Exec("UPDATE public.knowledge_sources SET latest_revision_id=?,version=?,updated_by=?,updated_at=? WHERE organization_id=? AND id=?", revision.ID, source.Version, actor, now, org, source.ID).Error; err != nil {
						return err
					}
					read, err := readSource(tx, org, source.ID)
					if err != nil {
						return err
					}
					result = k.Result{Source: &read, Revision: &revision}
				}
			}
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		var source, revision any
		if result.Source != nil {
			source = result.Source.ID
		}
		if result.Revision != nil {
			revision = result.Revision.ID
		}
		return tx.Exec("INSERT INTO public.knowledge_ingest_operations(organization_id,kind,key,actor_id,fingerprint,result,source_id,revision_id,created_at) VALUES(?,?,?,?,?,?::jsonb,?,?,?)", c.Scope.OrganizationID, c.Kind, c.Key, c.Scope.ActorID, c.Fingerprint, string(encoded), source, revision, now).Error
	})
	return result, safe(err)
}

func (r *Repository) ListBases(ctx context.Context, org string, page, size int) (bases []k.Base, total int64, err error) {
	db := r.db.WithContext(ctx).Table("public.knowledge_bases").Where("organization_id=?", org)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, safe(err)
	}
	bases = []k.Base{}
	err = db.Order("created_at DESC,id").Offset((page - 1) * size).Limit(size).Find(&bases).Error
	return bases, total, safe(err)
}
func (r *Repository) GetBase(ctx context.Context, org, id string) (b k.Base, err error) {
	err = one(r.db.WithContext(ctx), "SELECT * FROM public.knowledge_bases WHERE organization_id=? AND id=?", &b, org, id)
	return b, safe(err)
}
func readSource(tx *gorm.DB, org, id string) (s k.Source, err error) {
	err = one(tx, "SELECT * FROM public.knowledge_sources WHERE organization_id=? AND id=?", &s, org, id)
	if err != nil {
		return
	}
	err = tx.Raw("SELECT state FROM public.knowledge_bases WHERE organization_id=? AND id=?", org, s.BaseID).Scan(&s.BaseState).Error
	if err != nil {
		return
	}
	for _, ref := range []struct {
		id     string
		target **k.Revision
	}{{s.LatestRevisionID, &s.LatestRevision}, {s.CurrentReadableRevisionID, &s.CurrentReadableRevision}} {
		if ref.id != "" {
			rev := new(k.Revision)
			if err = one(tx, "SELECT id,organization_id,source_id,number,filename,content_type,size_bytes,state,failure,warning,created_at,updated_at FROM public.knowledge_revisions WHERE organization_id=? AND id=?", rev, org, ref.id); err != nil {
				return
			}
			*ref.target = rev
		}
	}
	return
}
func (r *Repository) GetSource(ctx context.Context, org, id string) (s k.Source, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		base, e := sourceBase(tx, org, id)
		if e != nil {
			return e
		}
		if _, e = lockBase(tx, org, base, "SHARE"); e != nil {
			return e
		}
		if _, e = lockSource(tx, org, id, "SHARE"); e != nil {
			return e
		}
		s, e = readSource(tx, org, id)
		return e
	})
	return s, safe(err)
}
func (r *Repository) ListSources(ctx context.Context, org, base string) (sources []k.Source, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, e := lockBase(tx, org, base, "SHARE"); e != nil {
			return e
		}
		var ids []string
		if e := tx.Raw("SELECT id FROM public.knowledge_sources WHERE organization_id=? AND base_id=? ORDER BY (state='ACTIVE') DESC,created_at DESC,id LIMIT 100", org, base).Scan(&ids).Error; e != nil {
			return e
		}
		sources = []k.Source{}
		for _, id := range ids {
			s, e := readSource(tx, org, id)
			if e != nil {
				return e
			}
			sources = append(sources, s)
		}
		return nil
	})
	return sources, safe(err)
}
func (r *Repository) Preview(ctx context.Context, org, source, revision string) (preview k.Preview, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		baseID, e := sourceBase(tx, org, source)
		if e != nil {
			return e
		}
		base, e := lockBase(tx, org, baseID, "SHARE")
		if e != nil {
			return e
		}
		s, e := lockSource(tx, org, source, "SHARE")
		if e != nil {
			return e
		}
		if base.State != k.Active || s.State != k.Active {
			return k.ErrInactive
		}
		if s.CurrentReadableRevisionID != revision {
			return k.ErrNotReadable
		}
		var rev k.Revision
		if e := one(tx, "SELECT id,text,state,warning FROM public.knowledge_revisions WHERE organization_id=? AND source_id=? AND id=?", &rev, org, source, revision); e != nil {
			return e
		}
		if rev.State != k.Available && rev.State != k.Partial {
			return k.ErrNotReadable
		}
		preview = k.Preview{RevisionID: rev.ID, Text: rev.Text, Warning: rev.Warning}
		return nil
	})
	return preview, safe(err)
}
