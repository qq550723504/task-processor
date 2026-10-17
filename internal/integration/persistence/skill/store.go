// Package skillpersistence implements the one Skill owner on the existing RunDB.
package skillpersistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"math"
	"task-processor/internal/agent"
	d "task-processor/internal/skill"
	"time"
)

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) (*Store, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, d.ErrUnavailable
	}
	return &Store{db: db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})}, nil
}

type skillRow struct {
	ID, OwnerKind, OwnerID, Creator string
	HeadVersion, Revision           uint64
	CreatedAt, UpdatedAt            time.Time
}

func (skillRow) TableName() string { return "skill_capability.skills" }

type versionRow struct {
	SkillID                   string
	Version                   uint64
	Files                     []byte
	Digest, State, Visibility string
	AllowCopy, AllowExport    bool
	SourceRef                 []byte
	CreatedAt                 time.Time
}

func (versionRow) TableName() string { return "skill_capability.versions" }
func idOK(value string) bool {
	id, e := uuid.Parse(value)
	return e == nil && id != uuid.Nil && id.String() == value
}
func readVersion(db *gorm.DB, id string, version uint64, lock bool) (d.Version, error) {
	var out d.Version
	var row skillRow
	sql := "SELECT * FROM skill_capability.skills WHERE id=?"
	if lock {
		sql += " FOR UPDATE"
	}
	result := db.Raw(sql, id).Scan(&row)
	if result.Error != nil {
		return out, result.Error
	}
	if result.RowsAffected != 1 {
		return out, d.ErrNotFound
	}
	if version == 0 {
		version = row.HeadVersion
	}
	var vr versionRow
	result = db.Raw("SELECT * FROM skill_capability.versions WHERE skill_id=? AND version=?", id, version).Scan(&vr)
	if result.Error != nil {
		return out, result.Error
	}
	if result.RowsAffected != 1 {
		return out, d.ErrNotFound
	}
	var files []d.File
	if json.Unmarshal(vr.Files, &files) != nil {
		return out, d.ErrUnavailable
	}
	pkg, e := d.ValidatePackage(files)
	if e != nil || pkg.Digest != vr.Digest {
		return out, d.ErrUnavailable
	}
	out = d.Version{Metadata: d.Metadata{ID: row.ID, Owner: d.Owner{Kind: row.OwnerKind, ID: row.OwnerID}, Creator: row.Creator, HeadVersion: row.HeadVersion, Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, Number: version, Package: pkg, State: vr.State, Visibility: vr.Visibility, AllowCopy: vr.AllowCopy, AllowExport: vr.AllowExport, CreatedAt: vr.CreatedAt}
	if len(vr.SourceRef) > 0 {
		var ref agent.SkillVersionRef
		if json.Unmarshal(vr.SourceRef, &ref) != nil || !ref.Valid() {
			return d.Version{}, d.ErrUnavailable
		}
		out.Source = &ref
	}
	return out, nil
}
func (s *Store) Read(ctx context.Context, p d.Principal, o d.Owner, id string, version uint64) (d.Version, error) {
	if !o.Valid() || !idOK(id) || version >= 1<<63 {
		return d.Version{}, d.ErrInvalid
	}
	v, e := readVersion(s.db.WithContext(ctx), id, version, false)
	if e != nil {
		return d.Version{}, e
	}
	if v.Metadata.Owner != o || !d.Authorized(p, v, d.Read) {
		return d.Version{}, d.ErrNotFound
	}
	return v, nil
}
func commandFingerprint(c d.Command) string {
	raw, _ := json.Marshal([]any{c.Owner, c.Operation, c.ID, c.Expected, c.Version, c.Package.Digest, c.AllowCopy, c.AllowExport, c.Source})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func (s *Store) Execute(ctx context.Context, c d.Command, resolve d.ResolvePrincipal) (d.Receipt, error) {
	var out d.Receipt
	if !c.Owner.Valid() || !idOK(c.Key) || resolve == nil || !agent.ValidID(c.Principal.UserID) || c.Expected >= math.MaxInt64 || c.Version >= math.MaxInt64 {
		return out, d.ErrInvalid
	}
	if c.Operation == "create" || c.Operation == "save-version" {
		p, e := d.ValidatePackage(c.Package.Files)
		if e != nil || p.Digest != c.Package.Digest || p.Name != c.Package.Name || p.Description != c.Package.Description || p.Target != c.Package.Target || p.DisplayName != c.Package.DisplayName {
			return out, d.ErrInvalid
		}
	}
	fp := commandFingerprint(c)
	e := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		principal, e := resolve(ctx)
		if e != nil {
			return e
		}
		if principal != c.Principal {
			return d.ErrForbidden
		}
		identity, _ := json.Marshal([]string{c.Owner.Kind, c.Owner.ID, principal.UserID, c.Key})
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", string(identity)).Error; e != nil {
			return e
		}
		var old struct {
			Fingerprint string
			Receipt     []byte
		}
		q := tx.Raw("SELECT fingerprint,receipt FROM skill_capability.commands WHERE owner_kind=? AND owner_id=? AND actor_id=? AND key=?", c.Owner.Kind, c.Owner.ID, principal.UserID, c.Key).Scan(&old)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected == 1 {
			if old.Fingerprint != fp {
				return d.ErrConflict
			}
			if json.Unmarshal(old.Receipt, &out) != nil {
				return d.ErrUnavailable
			}
			return nil
		}
		now := time.Now().UTC()
		var current d.Version
		if c.Operation == "create" || c.Operation == "copy" {
			if c.ID != "" || c.Expected != 0 {
				return d.ErrInvalid
			}
			id := uuid.NewSHA1(uuid.NameSpaceOID, append([]byte("skill:"), identity...)).String()
			current = d.Version{Metadata: d.Metadata{ID: id, Owner: c.Owner, Creator: principal.UserID, HeadVersion: 1, Revision: 1, CreatedAt: now, UpdatedAt: now}, Number: 1, State: d.Draft, Visibility: d.Private, AllowCopy: c.AllowCopy, AllowExport: c.AllowExport, Package: c.Package, CreatedAt: now}
			if !d.Authorized(principal, current, d.Edit) {
				return d.ErrForbidden
			}
			if c.Operation == "copy" {
				if c.Source == nil || !c.Source.Valid() {
					return d.ErrInvalid
				}
				source, e := readVersion(tx, c.Source.ID, c.Source.Version, true)
				if e != nil {
					return e
				}
				if source.Ref() != *c.Source || !d.Authorized(principal, source, d.Copy) {
					return d.ErrForbidden
				}
				current.Package = source.Package
				current.Source = c.Source
			} else if c.Source != nil {
				return d.ErrInvalid
			}
			row := skillRow{ID: id, OwnerKind: c.Owner.Kind, OwnerID: c.Owner.ID, Creator: principal.UserID, HeadVersion: 1, Revision: 1, CreatedAt: now, UpdatedAt: now}
			if e := tx.Create(&row).Error; e != nil {
				return e
			}
			if e := insertVersion(tx, current); e != nil {
				return e
			}
		} else {
			if !idOK(c.ID) || c.Expected == 0 {
				return d.ErrInvalid
			}
			current, e = readVersion(tx, c.ID, c.Version, true)
			if e != nil {
				return e
			}
			if current.Metadata.Owner != c.Owner {
				return d.ErrNotFound
			}
			if current.Metadata.Revision != c.Expected {
				return d.ErrRevision
			}
			if current.Metadata.HeadVersion >= math.MaxInt64 {
				return d.ErrRevision
			}
			if c.Operation == "save-version" {
				if !d.Authorized(principal, current, d.Edit) {
					return d.ErrForbidden
				}
				current.Number = current.Metadata.HeadVersion + 1
				current.Metadata.HeadVersion = current.Number
				current.Package = c.Package
				current.State, current.Visibility = d.Draft, d.Private
				current.AllowCopy, current.AllowExport = c.AllowCopy, c.AllowExport
				current.CreatedAt = now
				if c.Source != nil {
					return d.ErrInvalid
				}
				if e := insertVersion(tx, current); e != nil {
					return e
				}
			} else {
				state, visibility, e := d.Transition(principal, current, c.Operation)
				if e != nil {
					return e
				}
				if e := tx.Exec("UPDATE skill_capability.versions SET state=?,visibility=? WHERE skill_id=? AND version=?", state, visibility, current.Metadata.ID, current.Number).Error; e != nil {
					return e
				}
			}
			current.Metadata.Revision++
			current.Metadata.UpdatedAt = now
			if e := tx.Exec("UPDATE skill_capability.skills SET head_version=?,revision=?,updated_at=? WHERE id=?", current.Metadata.HeadVersion, current.Metadata.Revision, now, current.Metadata.ID).Error; e != nil {
				return e
			}
		}
		out = d.Receipt{ID: current.Metadata.ID, Version: current.Number, Revision: current.Metadata.Revision, Operation: c.Operation, At: now}
		raw, _ := json.Marshal(out)
		return tx.Exec("INSERT INTO skill_capability.commands(owner_kind,owner_id,actor_id,key,fingerprint,receipt) VALUES(?,?,?,?,?,?)", c.Owner.Kind, c.Owner.ID, principal.UserID, c.Key, fp, raw).Error
	})
	return out, e
}
func insertVersion(tx *gorm.DB, v d.Version) error {
	raw, e := json.Marshal(v.Package.Files)
	if e != nil || len(raw) > d.MaxPackageBytes {
		return d.ErrInvalid
	}
	var source []byte
	if v.Source != nil {
		source, e = json.Marshal(v.Source)
		if e != nil {
			return e
		}
	}
	row := versionRow{SkillID: v.Metadata.ID, Version: v.Number, Files: raw, Digest: v.Package.Digest, State: v.State, Visibility: v.Visibility, AllowCopy: v.AllowCopy, AllowExport: v.AllowExport, SourceRef: source, CreatedAt: v.CreatedAt}
	return tx.Create(&row).Error
}
