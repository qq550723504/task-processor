package projectcenterpersistence

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	pc "task-processor/internal/aiworkbench/projectcenter"
	"time"
)

//go:embed schema.sql
var schema string

type Store struct{ db *gorm.DB }
type projectRow struct {
	ID, OrganizationID, ActorID, Title, Goal, Kind, StoreID, DueDate string
	Archived                                                         bool
	Revision                                                         uint64
	CreatedAt, UpdatedAt                                             time.Time
}

func (projectRow) TableName() string { return "ai_workbench_projects.projects" }

type referenceRow struct {
	SlotID, ProjectID, OrganizationID, ActorID, Kind, TargetID string
	Active                                                     bool
}

func (referenceRow) TableName() string { return "ai_workbench_projects.references" }

type templateRow struct {
	ID, OrganizationID, ActorID, Name, Title, Goal, Kind string
	Archived                                             bool
	Revision                                             uint64
	CreatedAt                                            time.Time
}

func (templateRow) TableName() string { return "ai_workbench_projects.templates" }

type commandRow struct {
	OrganizationID, ActorID, Key, Operation, Fingerprint, EntityID string
	Revision                                                       uint64
	CreatedAt                                                      time.Time
}

func (commandRow) TableName() string { return "ai_workbench_projects.commands" }

type auditRow struct {
	ID, OrganizationID, ActorID, EntityID, Operation string
	Revision                                         uint64
	CreatedAt                                        time.Time
}

func (auditRow) TableName() string { return "ai_workbench_projects.audit" }

type visitRow struct {
	ProjectID, OrganizationID, ActorID string
	LastVisitedAt                      time.Time
}

func (visitRow) TableName() string { return "ai_workbench_projects.visits" }
func New(db *gorm.DB) (*Store, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, pc.ErrUnavailable
	}
	return &Store{db}, nil
}
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return pc.ErrInvalid
	}
	return db.Exec(schema).Error
}
func project(r projectRow) pc.Project {
	return pc.Project{ID: r.ID, Scope: pc.Scope{OrganizationID: r.OrganizationID, ActorID: r.ActorID}, Fields: pc.Fields{Title: r.Title, Goal: r.Goal, Kind: r.Kind, DueDate: r.DueDate}, StoreID: r.StoreID, Archived: r.Archived, Revision: r.Revision, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}
func fingerprint(c pc.Command) string {
	raw, _ := json.Marshal(c)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func scoped(db *gorm.DB, s pc.Scope) *gorm.DB {
	return db.Where("organization_id=? AND actor_id=?", s.OrganizationID, s.ActorID)
}
func databaseError(e error) error {
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return pc.ErrNotFound
	}
	if e != nil {
		return pc.ErrUnavailable
	}
	return nil
}
func lookupReceipt(db *gorm.DB, s pc.Scope, key string, c pc.Command) (pc.Receipt, bool, error) {
	var r commandRow
	e := scoped(db, s).Where("key=?", key).Take(&r).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return pc.Receipt{}, false, nil
	}
	if e != nil {
		return pc.Receipt{}, false, pc.ErrUnavailable
	}
	if r.Operation != c.Operation || r.Fingerprint != fingerprint(c) {
		return pc.Receipt{}, false, pc.ErrConflict
	}
	return pc.Receipt{ID: r.EntityID, Revision: r.Revision, Replayed: true}, true, nil
}
func (s *Store) Replay(ctx context.Context, scope pc.Scope, key string, c pc.Command) (pc.Receipt, bool, error) {
	return lookupReceipt(s.db.WithContext(ctx), scope, key, c)
}
func (s *Store) Commit(ctx context.Context, scope pc.Scope, key string, c pc.Command, validation error) (pc.Receipt, error) {
	var out pc.Receipt
	e := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// JSON scopes prevent concatenation ambiguities, and no source I/O occurs under this lock.
		lock, _ := json.Marshal([]string{scope.OrganizationID, scope.ActorID, key})
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", string(lock)).Error; e != nil {
			return e
		}
		replay, found, e := lookupReceipt(tx, scope, key, c)
		if e != nil {
			return e
		}
		if found {
			out = replay
			return nil
		}
		if validation != nil {
			return validation
		}
		now := time.Now().UTC()
		entity, revision, e := mutate(tx, scope, c, now)
		if e != nil {
			return e
		}
		if e = tx.Create(&commandRow{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, Key: key, Operation: c.Operation, Fingerprint: fingerprint(c), EntityID: entity, Revision: revision, CreatedAt: now}).Error; e != nil {
			return e
		}
		if e = tx.Create(&auditRow{ID: uuid.NewString(), OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, EntityID: entity, Operation: c.Operation, Revision: revision, CreatedAt: now}).Error; e != nil {
			return e
		}
		out = pc.Receipt{ID: entity, Revision: revision}
		return nil
	})
	if e != nil {
		for _, known := range []error{pc.ErrConflict, pc.ErrNotFound, pc.ErrRevision, pc.ErrArchived, pc.ErrInvalid, pc.ErrUnavailable} {
			if errors.Is(e, known) {
				return pc.Receipt{}, known
			}
		}
		return pc.Receipt{}, pc.ErrUnavailable
	}
	return out, nil
}
func mutate(tx *gorm.DB, s pc.Scope, c pc.Command, now time.Time) (string, uint64, error) {
	if c.Operation == "create" {
		f := *c.Fields
		r := projectRow{ID: uuid.NewString(), OrganizationID: s.OrganizationID, ActorID: s.ActorID, Title: f.Title, Goal: f.Goal, Kind: f.Kind, DueDate: f.DueDate, Revision: 1, CreatedAt: now, UpdatedAt: now}
		if c.StoreID != nil {
			r.StoreID = *c.StoreID
		}
		return r.ID, 1, tx.Create(&r).Error
	}
	if c.Operation == "template_archive" {
		var r templateRow
		if e := scoped(tx, s).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", c.ID).Take(&r).Error; e != nil {
			return "", 0, databaseError(e)
		}
		if r.Revision != c.Expected {
			return "", 0, pc.ErrRevision
		}
		e := scoped(tx.Model(&templateRow{}), s).Where("id=?", r.ID).Updates(map[string]any{"archived": true, "revision": r.Revision + 1}).Error
		return r.ID, r.Revision + 1, e
	}
	var r projectRow
	if e := scoped(tx, s).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", c.ID).Take(&r).Error; e != nil {
		return "", 0, databaseError(e)
	}
	if c.Operation == "visit" {
		v := visitRow{ProjectID: r.ID, OrganizationID: s.OrganizationID, ActorID: s.ActorID, LastVisitedAt: now}
		return r.ID, r.Revision, tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "project_id"}}, DoUpdates: clause.AssignmentColumns([]string{"last_visited_at"})}).Create(&v).Error
	}
	if r.Revision != c.Expected {
		return "", 0, pc.ErrRevision
	}
	if c.Operation == "template_save" {
		t := templateRow{ID: uuid.NewString(), OrganizationID: s.OrganizationID, ActorID: s.ActorID, Name: c.Name, Title: r.Title, Goal: r.Goal, Kind: r.Kind, Revision: 1, CreatedAt: now}
		return t.ID, 1, tx.Create(&t).Error
	}
	if r.Archived && c.Operation != "restore" {
		return "", 0, pc.ErrArchived
	}
	changes := map[string]any{"revision": r.Revision + 1, "updated_at": now}
	switch c.Operation {
	case "edit":
		f := *c.Fields
		changes["title"] = f.Title
		changes["goal"] = f.Goal
		changes["kind"] = f.Kind
		changes["due_date"] = f.DueDate
		if c.StoreID != nil {
			changes["store_id"] = *c.StoreID
		}
	case "archive":
		changes["archived"] = true
	case "restore":
		if !r.Archived {
			return "", 0, pc.ErrInvalid
		}
		changes["archived"] = false
	case "add":
		var count int64
		if e := scoped(tx.Model(&referenceRow{}), s).Where("project_id=? AND active", r.ID).Count(&count).Error; e != nil {
			return "", 0, e
		}
		if count >= 100 {
			return "", 0, pc.ErrInvalid
		}
		var duplicate int64
		if e := scoped(tx.Model(&referenceRow{}), s).Where("project_id=? AND kind=? AND target_id=? AND active", r.ID, c.Reference.Kind, c.Reference.TargetID).Count(&duplicate).Error; e != nil {
			return "", 0, e
		}
		if duplicate > 0 {
			return "", 0, pc.ErrConflict
		}
		ref := referenceRow{SlotID: uuid.NewString(), ProjectID: r.ID, OrganizationID: s.OrganizationID, ActorID: s.ActorID, Kind: c.Reference.Kind, TargetID: c.Reference.TargetID, Active: true}
		if e := tx.Create(&ref).Error; e != nil {
			return "", 0, e
		}
	case "remove":
		result := scoped(tx.Model(&referenceRow{}), s).Where("project_id=? AND slot_id=? AND active", r.ID, c.SlotID).Update("active", false)
		if result.Error != nil {
			return "", 0, result.Error
		}
		if result.RowsAffected != 1 {
			return "", 0, pc.ErrNotFound
		}
	default:
		return "", 0, pc.ErrInvalid
	}
	return r.ID, r.Revision + 1, scoped(tx.Model(&projectRow{}), s).Where("id=?", r.ID).Updates(changes).Error
}
func (s *Store) Get(ctx context.Context, scope pc.Scope, id string) (pc.Project, []pc.Reference, error) {
	var row projectRow
	if e := scoped(s.db.WithContext(ctx), scope).Where("id=?", id).Take(&row).Error; e != nil {
		return pc.Project{}, nil, databaseError(e)
	}
	var refs []referenceRow
	if e := scoped(s.db.WithContext(ctx), scope).Where("project_id=? AND active", id).Order("slot_id").Limit(101).Find(&refs).Error; e != nil || len(refs) > 100 {
		return pc.Project{}, nil, pc.ErrUnavailable
	}
	out := []pc.Reference{}
	for _, r := range refs {
		out = append(out, pc.Reference{SlotID: r.SlotID, Kind: r.Kind, TargetID: r.TargetID})
	}
	return project(row), out, nil
}

type cursor struct {
	Time time.Time `json:"time"`
	ID   string    `json:"id"`
}

func encodeCursor(t time.Time, id string) string {
	raw, _ := json.Marshal(cursor{t, id})
	return base64.RawURLEncoding.EncodeToString(raw)
}
func pageAfter(db *gorm.DB, after, column string) (*gorm.DB, error) {
	if after == "" {
		return db, nil
	}
	raw, e := base64.RawURLEncoding.DecodeString(after)
	var c cursor
	if e != nil || json.Unmarshal(raw, &c) != nil || !pc.ValidID(c.ID) || c.Time.IsZero() {
		return nil, pc.ErrInvalid
	}
	return db.Where("("+column+",p.id) < (?,?)", c.Time, c.ID), nil
}
func (s *Store) List(ctx context.Context, scope pc.Scope, q pc.Query) ([]pc.Project, string, error) {
	db := s.db.WithContext(ctx).Table("ai_workbench_projects.projects p").Where("p.organization_id=? AND p.actor_id=? AND p.archived=?", scope.OrganizationID, scope.ActorID, q.Mode == "archived")
	column := "p.updated_at"
	if q.Mode == "recent" {
		db = db.Joins("JOIN ai_workbench_projects.visits v ON v.project_id=p.id AND v.organization_id=p.organization_id AND v.actor_id=p.actor_id")
		column = "v.last_visited_at"
	}
	if q.Kind != "" {
		db = db.Where("p.kind=?", q.Kind)
	}
	if q.WorkScope == "store" {
		db = db.Where("p.store_id<>''")
	}
	if q.WorkScope == "general" {
		db = db.Where("p.store_id=''")
	}
	if q.StoreID != "" {
		db = db.Where("p.store_id=?", q.StoreID)
	}
	if q.Search != "" {
		escaped := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q.Search)
		db = db.Where("p.title ILIKE ?", "%"+escaped+"%")
	}
	db, e := pageAfter(db, q.After, column)
	if e != nil {
		return nil, "", e
	}
	type listRow struct {
		Project projectRow `gorm:"embedded"`
		SortAt  time.Time
	}
	var rows []listRow
	if e := db.Select("p.*, " + column + " AS sort_at").Order(column + " DESC,p.id DESC").Limit(21).Scan(&rows).Error; e != nil {
		return nil, "", pc.ErrUnavailable
	}
	next := ""
	if len(rows) > 20 {
		rows = rows[:20]
		last := rows[19]
		next = encodeCursor(last.SortAt, last.Project.ID)
	}
	out := []pc.Project{}
	for _, r := range rows {
		out = append(out, project(r.Project))
	}
	return out, next, nil
}
func (s *Store) Templates(ctx context.Context, scope pc.Scope, after string) ([]pc.Template, string, error) {
	db := s.db.WithContext(ctx).Table("ai_workbench_projects.templates AS p").Where("p.organization_id=? AND p.actor_id=? AND NOT p.archived", scope.OrganizationID, scope.ActorID)
	db, e := pageAfter(db, after, "p.created_at")
	if e != nil {
		return nil, "", e
	}
	var rows []templateRow
	if e := db.Order("p.created_at DESC,p.id DESC").Limit(21).Find(&rows).Error; e != nil {
		return nil, "", pc.ErrUnavailable
	}
	next := ""
	if len(rows) > 20 {
		rows = rows[:20]
		next = encodeCursor(rows[19].CreatedAt, rows[19].ID)
	}
	out := []pc.Template{}
	for _, r := range rows {
		out = append(out, pc.Template{ID: r.ID, Scope: scope, Name: r.Name, Fields: pc.Fields{Title: r.Title, Goal: r.Goal, Kind: r.Kind}, Revision: r.Revision, Archived: r.Archived, CreatedAt: r.CreatedAt})
	}
	return out, next, nil
}
