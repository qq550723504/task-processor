package skillpersistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"task-processor/internal/agent"
	d "task-processor/internal/skill"
)

// InstallOfficial is a bounded release command. Its identity is global, not
// actor-derived. Existing exact releases, including disabled/revoked or edited
// heads, are never overwritten, upgraded, or re-enabled by installation.
func (s *Store) InstallOfficial(ctx context.Context, p d.Principal, resolve d.ResolvePrincipal) (d.OfficialInstallResult, error) {
	out := d.OfficialInstallResult{Installed: []agent.SkillVersionRef{}, Existing: []agent.SkillVersionRef{}}
	if !p.PlatformMaintainer || !agent.ValidID(p.UserID) || resolve == nil {
		return out, d.ErrForbidden
	}
	packages, err := d.OfficialPackages()
	if err != nil {
		return out, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		live, err := resolve(ctx)
		if err != nil {
			return err
		}
		if live != p || !live.PlatformMaintainer {
			return d.ErrForbidden
		}
		if err = tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "skill:official-method-v1").Error; err != nil {
			return err
		}
		for _, release := range packages {
			id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("skill:"+release.Key)).String()
			existing, readErr := readVersion(tx, id, 1, true)
			if readErr == nil {
				if existing.Metadata.Owner != (d.Owner{Kind: d.PlatformOwner, ID: d.PlatformID}) || existing.Package.Digest != release.Package.Digest || existing.Source != nil {
					return d.ErrConflict
				}
				out.Existing = append(out.Existing, existing.Ref())
				continue
			}
			if !errors.Is(readErr, d.ErrNotFound) {
				return readErr
			}
			// A row without its original version is a conflicting identity too.
			var count int64
			if err = tx.Model(&skillRow{}).Where("id=?", id).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				return d.ErrConflict
			}
			now := time.Now().UTC()
			v := d.Version{Metadata: d.Metadata{ID: id, Owner: d.Owner{Kind: d.PlatformOwner, ID: d.PlatformID}, Creator: p.UserID, HeadVersion: 1, Revision: 1, CreatedAt: now, UpdatedAt: now}, Number: 1, Package: release.Package, State: d.Draft, Visibility: d.Private, AllowCopy: true, AllowExport: true, CreatedAt: now}
			for _, action := range []string{"publish", "enable"} {
				v.State, v.Visibility, err = d.Transition(live, v, action)
				if err != nil {
					return err
				}
				v.Metadata.Revision++
			}
			row := skillRow{ID: id, OwnerKind: d.PlatformOwner, OwnerID: d.PlatformID, Creator: p.UserID, HeadVersion: 1, Revision: v.Metadata.Revision, CreatedAt: now, UpdatedAt: now}
			if err = tx.Create(&row).Error; err != nil {
				return err
			}
			if err = insertVersion(tx, v); err != nil {
				return err
			}
			out.Installed = append(out.Installed, v.Ref())
		}
		return nil
	})
	if err != nil {
		return d.OfficialInstallResult{}, err
	}
	return out, nil
}
