package knowledge

import (
	"context"
	"time"

	"gorm.io/gorm"
	k "task-processor/internal/knowledge"
)

// livePermit is called with the lifecycle Base lock held. The bounded lease,
// rather than a still-ACTIVE row alone, determines whether disable must drain.
func livePermit(tx *gorm.DB, org, base, source string, now time.Time) (bool, error) {
	var live bool
	query := `SELECT EXISTS(SELECT 1 FROM public.knowledge_dispatch_permits p JOIN public.knowledge_context_bundles b ON b.organization_id=p.organization_id AND b.id=p.bundle_id WHERE p.organization_id=? AND b.base_id=? AND p.state='ACTIVE' AND p.expires_at>?)`
	args := []any{org, base, now}
	if source != "" {
		query = `SELECT EXISTS(SELECT 1 FROM public.knowledge_dispatch_permits p JOIN public.knowledge_context_bundle_entries e ON e.organization_id=p.organization_id AND e.bundle_id=p.bundle_id WHERE p.organization_id=? AND e.base_id=? AND p.state='ACTIVE' AND p.expires_at>? AND e.source_id=?)`
		args = append(args, source)
	}
	err := tx.Raw(query, args...).Scan(&live).Error
	return live, err
}
func disableState(tx *gorm.DB, org, base, source string, now time.Time) (k.State, error) {
	live, err := livePermit(tx, org, base, source, now)
	if err != nil {
		return "", err
	}
	if live {
		return k.Disabling, nil
	}
	return k.Disabled, nil
}
func lockBaseSources(tx *gorm.DB, org, base string) ([]k.Source, error) {
	var sources []k.Source
	err := tx.Raw("SELECT * FROM public.knowledge_sources WHERE organization_id=? AND base_id=? ORDER BY id FOR UPDATE", org, base).Scan(&sources).Error
	return sources, err
}

// finalizeDisable is called after Base -> canonical Sources -> Permit locks.
func finalizeDisable(tx *gorm.DB, base k.Base, sources []k.Source, now time.Time) error {
	for _, s := range sources {
		if s.State != k.Disabling {
			continue
		}
		state, err := disableState(tx, base.OrganizationID, base.ID, s.ID, now)
		if err != nil {
			return err
		}
		if state == k.Disabled {
			if err := tx.Exec("UPDATE public.knowledge_sources SET state='DISABLED',version=version+1,updated_at=? WHERE organization_id=? AND id=?", now, base.OrganizationID, s.ID).Error; err != nil {
				return err
			}
		}
	}
	if base.State == k.Disabling {
		state, err := disableState(tx, base.OrganizationID, base.ID, "", now)
		if err != nil {
			return err
		}
		if state == k.Disabled {
			return tx.Exec("UPDATE public.knowledge_bases SET state='DISABLED',version=version+1,updated_at=? WHERE organization_id=? AND id=?", now, base.OrganizationID, base.ID).Error
		}
	}
	return nil
}
func (r *Repository) ReleaseDispatchPermit(ctx context.Context, p k.DispatchPermit) error {
	if !k.ValidID(p.ID) || !p.Ref.Valid() || p.OrganizationID == "" || p.ActorID == "" || p.InvocationID == "" {
		return k.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var target struct{ BaseID string }
		if err := one(tx, `SELECT b.base_id FROM public.knowledge_dispatch_permits p JOIN public.knowledge_context_bundles b ON b.organization_id=p.organization_id AND b.id=p.bundle_id WHERE p.organization_id=? AND p.id=? AND p.actor_id=? AND p.invocation_id=? AND p.bundle_id=? AND p.digest=?`, &target, p.OrganizationID, p.ID, p.ActorID, p.InvocationID, p.Ref.ID, p.Ref.Digest); err != nil {
			return err
		}
		base, err := lockBase(tx, p.OrganizationID, target.BaseID, "UPDATE")
		if err != nil {
			return err
		}
		sources, err := lockBaseSources(tx, p.OrganizationID, base.ID)
		if err != nil {
			return err
		}
		now, err := databaseNow(tx)
		if err != nil {
			return err
		}
		if err := tx.Exec("UPDATE public.knowledge_dispatch_permits SET state=CASE WHEN expires_at<=? THEN 'EXPIRED' ELSE 'RELEASED' END WHERE organization_id=? AND id=? AND state='ACTIVE'", now, p.OrganizationID, p.ID).Error; err != nil {
			return err
		}
		return finalizeDisable(tx, base, sources, now)
	})
	return safe(err)
}

// RecoverExpiredDispatchPermits runs in the existing Knowledge processor.
// Each bounded transaction owns only one Base's content-use lifecycle. There
// is no provider/model, AI ledger, retry or cross-database dependency here.
func (r *Repository) RecoverExpiredDispatchPermits(ctx context.Context) error {
	var targets []struct{ OrganizationID, ID string }
	err := r.db.WithContext(ctx).Raw(`SELECT b.organization_id,b.id FROM public.knowledge_bases b WHERE b.state='DISABLING' OR EXISTS(SELECT 1 FROM public.knowledge_sources s WHERE s.organization_id=b.organization_id AND s.base_id=b.id AND s.state='DISABLING') OR EXISTS(SELECT 1 FROM public.knowledge_dispatch_permits p JOIN public.knowledge_context_bundles c ON c.organization_id=p.organization_id AND c.id=p.bundle_id WHERE c.organization_id=b.organization_id AND c.base_id=b.id AND p.state='ACTIVE' AND p.expires_at<=clock_timestamp()) ORDER BY b.organization_id,b.id LIMIT 32`).Scan(&targets).Error
	if err != nil {
		return safe(err)
	}
	for _, target := range targets {
		if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			base, err := lockBase(tx, target.OrganizationID, target.ID, "UPDATE")
			if err != nil {
				return err
			}
			sources, err := lockBaseSources(tx, target.OrganizationID, target.ID)
			if err != nil {
				return err
			}
			now, err := databaseNow(tx)
			if err != nil {
				return err
			}
			if err := tx.Exec(`UPDATE public.knowledge_dispatch_permits p SET state='EXPIRED' FROM public.knowledge_context_bundles b WHERE b.organization_id=p.organization_id AND b.id=p.bundle_id AND b.organization_id=? AND b.base_id=? AND p.state='ACTIVE' AND p.expires_at<=?`, target.OrganizationID, target.ID, now).Error; err != nil {
				return err
			}
			return finalizeDisable(tx, base, sources, now)
		}); err != nil {
			return safe(err)
		}
	}
	return nil
}
