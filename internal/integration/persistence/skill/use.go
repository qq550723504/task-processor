package skillpersistence

import (
	"context"
	"gorm.io/gorm"
	"sync"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	d "task-processor/internal/skill"
)

func (s *Store) UsesPool(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	a, e := s.db.DB()
	b, be := db.DB()
	return e == nil && be == nil && a == b
}

// CheckUseInTransaction joins the existing configuration/Run Claim transaction.
// All callers resolve live identity/grants before supplying this trusted principal.
func (s *Store) CheckUseInTransaction(ctx context.Context, tx *gorm.DB, p d.Principal, ref agent.SkillVersionRef) (d.Package, error) {
	return s.checkUseForTarget(ctx, tx, p, ref, d.TitleTarget)
}

func (s *Store) checkUseForTarget(ctx context.Context, tx *gorm.DB, p d.Principal, ref agent.SkillVersionRef, target string) (d.Package, error) {
	agentID := agentconfig.ContentAgentID
	if target == d.ImageTarget {
		agentID = agentconfig.ImageAgentID
	} else if target != d.TitleTarget {
		return d.Package{}, d.ErrInvalid
	}
	if !ref.Valid() || !agent.ValidID(p.UserID) || !agent.ValidID(p.OrganizationID) || !agent.ValidID(p.MemberID) || !p.UseOrganization {
		return d.Package{}, d.ErrForbidden
	}
	var activation string
	q := tx.WithContext(ctx).Raw("SELECT activation FROM agent_configuration.organization_agents WHERE organization_id=? AND agent_id=? FOR SHARE", p.OrganizationID, agentID).Scan(&activation)
	if q.Error != nil {
		return d.Package{}, d.ErrUnavailable
	}
	if q.RowsAffected != 1 || activation != "ENABLED" {
		return d.Package{}, d.ErrNotEnabled
	}
	if ref.OwnerKind == d.UserOwner {
		if target == d.ImageTarget {
			return d.Package{}, d.ErrForbidden
		}
		var allow bool
		if e := tx.WithContext(ctx).Raw("SELECT allow_personal FROM agent_configuration.skill_admissions WHERE organization_id=? AND agent_id='product.content.agent'", p.OrganizationID).Scan(&allow).Error; e != nil {
			return d.Package{}, d.ErrUnavailable
		}
		if !allow {
			return d.Package{}, d.ErrForbidden
		}
	}
	var locked string
	if e := tx.WithContext(ctx).Raw("SELECT id FROM skill_capability.skills WHERE id=? FOR SHARE", ref.ID).Scan(&locked).Error; e != nil {
		return d.Package{}, d.ErrUnavailable
	}
	v, e := readVersion(tx.WithContext(ctx), ref.ID, ref.Version, false)
	if e != nil {
		return d.Package{}, e
	}
	if v.Ref() != ref {
		return d.Package{}, d.ErrConflict
	}
	if v.State != d.Enabled {
		return d.Package{}, d.ErrNotEnabled
	}
	if !d.Authorized(p, v, d.Use) {
		return d.Package{}, d.ErrForbidden
	}
	if !v.Package.Supports(target) {
		return d.Package{}, d.ErrForbidden
	}
	return v.Package, nil
}

// AcquireUse holds the availability fence until the caller's existing effect
// owner returns. Rollback is idempotent; cancellation also releases the PG tx.
func (s *Store) AcquireUse(ctx context.Context, p d.Principal, ref agent.SkillVersionRef) (d.Package, func(), error) {
	return s.AcquireUseForTarget(ctx, p, ref, d.TitleTarget)
}

func (s *Store) AcquireUseForTarget(ctx context.Context, p d.Principal, ref agent.SkillVersionRef, target string) (d.Package, func(), error) {
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return d.Package{}, nil, d.ErrUnavailable
	}
	material, e := s.checkUseForTarget(ctx, tx, p, ref, target)
	if e != nil {
		_ = tx.Rollback().Error
		return d.Package{}, nil, e
	}
	var once sync.Once
	release := func() { once.Do(func() { _ = tx.Rollback().Error }) }
	return material, release, nil
}
func (s *Store) ReadForUse(ctx context.Context, p d.Principal, ref agent.SkillVersionRef) (d.Package, error) {
	material, release, e := s.AcquireUse(ctx, p, ref)
	if e != nil {
		return d.Package{}, e
	}
	release()
	return material, nil
}
