package reportcenterpersistence

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
	rc "task-processor/internal/reportcenter"
	"time"
)

//go:embed schema.sql
var schemaSQL string

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) (*Store, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, rc.ErrUnavailable
	}
	return &Store{db}, nil
}

// InstallSchema is empty installation only. It never runs in a serving process.
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return rc.ErrUnavailable
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec(`DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='report_center_owner') THEN CREATE ROLE report_center_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS; END IF; END $$;`).Error; e != nil {
			return e
		}
		var safe bool
		if e := tx.Raw(`SELECT NOT(rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls) FROM pg_roles WHERE rolname='report_center_owner'`).Scan(&safe).Error; e != nil || !safe {
			return rc.ErrInvalid
		}
		for _, sql := range []string{`CREATE SCHEMA report_center AUTHORIZATION report_center_owner`, `SET LOCAL ROLE report_center_owner`, schemaSQL, `RESET ROLE`} {
			if e := tx.Exec(sql).Error; e != nil {
				return e
			}
		}
		return nil
	})
}

// VerifySchema refuses elevated runtime roles or missing read/write boundaries.
func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return rc.ErrUnavailable
	}
	var role string
	if db.WithContext(ctx).Raw("SELECT current_user").Scan(&role).Error != nil || roleSafe(db.WithContext(ctx), role) != nil {
		return rc.ErrUnavailable
	}
	var safe bool
	e := db.WithContext(ctx).Raw(`SELECT NOT (r.rolsuper OR r.rolcreatedb OR r.rolcreaterole OR r.rolbypassrls)
 AND NOT pg_has_role(current_user,'report_center_owner','MEMBER')
 AND NOT has_database_privilege(current_user,current_database(),'CREATE')
 AND NOT has_schema_privilege(current_user,'report_center','CREATE')
 AND has_schema_privilege(current_user,'report_center','USAGE')
 AND has_table_privilege(current_user,'report_center.saved_reports','SELECT')
 AND has_table_privilege(current_user,'report_center.saved_reports','INSERT')
 AND NOT has_any_column_privilege(current_user,'report_center.saved_reports','UPDATE')
 AND NOT has_table_privilege(current_user,'report_center.saved_reports','DELETE,TRUNCATE,TRIGGER')
 AND has_table_privilege(current_user,'report_center.commands','SELECT')
 AND has_table_privilege(current_user,'report_center.commands','INSERT')
 AND NOT has_any_column_privilege(current_user,'report_center.commands','UPDATE')
 AND NOT has_table_privilege(current_user,'report_center.commands','DELETE,TRUNCATE,TRIGGER')
 AND has_table_privilege(current_user,'report_center.favorites','SELECT')
 AND has_table_privilege(current_user,'report_center.favorites','INSERT')
 AND has_column_privilege(current_user,'report_center.favorites','favorite','UPDATE')
 AND has_column_privilege(current_user,'report_center.favorites','updated_at','UPDATE')
 AND NOT has_column_privilege(current_user,'report_center.favorites','actor_id','UPDATE')
 AND NOT has_column_privilege(current_user,'report_center.favorites','organization_id','UPDATE')
 AND NOT has_column_privilege(current_user,'report_center.favorites','report_id','UPDATE')
 AND NOT has_table_privilege(current_user,'report_center.favorites','DELETE,TRUNCATE,TRIGGER')
 FROM pg_roles r WHERE r.rolname=current_user`).Scan(&safe).Error
	if e != nil || !safe {
		return rc.ErrUnavailable
	}
	for _, q := range []string{`SELECT id,organization_id,actor_id,kind,source_id,source_version,title,product_key,store_id,source_at,captured_at,content,digest FROM report_center.saved_reports LIMIT 0`, `SELECT report_id,organization_id,actor_id,favorite,updated_at FROM report_center.favorites LIMIT 0`, `SELECT organization_id,actor_id,idempotency_key,fingerprint,report_id FROM report_center.commands LIMIT 0`} {
		if db.WithContext(ctx).Exec(q).Error != nil {
			return rc.ErrUnavailable
		}
	}
	return nil
}

type reportRow struct {
	ID, OrganizationID, ActorID, Kind, SourceID, SourceVersion, Title, ProductKey, StoreID string
	SourceAt                                                                               *time.Time
	CapturedAt                                                                             time.Time
	Content                                                                                []byte
	Digest                                                                                 string
}

func (reportRow) TableName() string { return "report_center.saved_reports" }

type favoriteRow struct {
	ReportID, OrganizationID, ActorID string
	Favorite                          bool
	UpdatedAt                         time.Time
}

func (favoriteRow) TableName() string { return "report_center.favorites" }

type commandRow struct{ OrganizationID, ActorID, IdempotencyKey, Fingerprint, ReportID string }

func (commandRow) TableName() string { return "report_center.commands" }
func scoped(db *gorm.DB, s rc.Scope) *gorm.DB {
	return db.Where("organization_id=? AND actor_id=?", s.OrganizationID, s.ActorID)
}
func lookup(db *gorm.DB, s rc.Scope, key, fp string) (string, error) {
	var row commandRow
	e := scoped(db, s).Where("idempotency_key=?", key).Take(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	if row.Fingerprint != fp {
		return "", rc.ErrConflict
	}
	return row.ReportID, nil
}
func (s *Store) Lookup(ctx context.Context, scope rc.Scope, key, fp string) (string, error) {
	if !scope.Valid() || !rc.UUID(key) {
		return "", rc.ErrInvalid
	}
	return lookup(s.db.WithContext(ctx), scope, key, fp)
}
func rowSummary(r reportRow, favorite bool) rc.ReportSummary {
	return rc.ReportSummary{ID: r.ID, SourceInfo: rc.SourceInfo{Ref: rc.SourceRef{Kind: r.Kind, ID: r.SourceID, Version: r.SourceVersion}, Title: r.Title, ProductKey: r.ProductKey, StoreID: r.StoreID, SourceAt: r.SourceAt}, CapturedAt: r.CapturedAt, Favorite: favorite}
}
func read(db *gorm.DB, scope rc.Scope, id string) (rc.Report, error) {
	var r reportRow
	if e := scoped(db, scope).Where("id=?", id).Take(&r).Error; e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return rc.Report{}, rc.ErrNotFound
		}
		return rc.Report{}, e
	}
	var fav favoriteRow
	if e := scoped(db, scope).Where("report_id=?", id).Take(&fav).Error; e != nil {
		return rc.Report{}, rc.ErrUnavailable
	}
	var content rc.Document
	sum := sha256.Sum256(r.Content)
	if hex.EncodeToString(sum[:]) != r.Digest || json.Unmarshal(r.Content, &content) != nil {
		return rc.Report{}, rc.ErrUnavailable
	}
	snap := rc.Snapshot{SourceInfo: rowSummary(r, fav.Favorite).SourceInfo, Content: content}
	_, digest, e := snap.Bytes()
	if e != nil || digest != r.Digest {
		return rc.Report{}, rc.ErrUnavailable
	}
	return rc.Report{ReportSummary: rowSummary(r, fav.Favorite), Content: content, Digest: r.Digest}, nil
}
func (s *Store) Read(ctx context.Context, scope rc.Scope, id string) (rc.Report, error) {
	if !scope.Valid() || !rc.UUID(id) {
		return rc.Report{}, rc.ErrInvalid
	}
	return read(s.db.WithContext(ctx), scope, id)
}
func (s *Store) Save(ctx context.Context, scope rc.Scope, key, fp string, snap rc.Snapshot, check func(context.Context) error) (rc.Result, error) {
	result := rc.Result{}
	body, digest, e := snap.Bytes()
	if e != nil {
		return result, e
	}
	if !scope.Valid() || !rc.UUID(key) || check == nil {
		return result, rc.ErrInvalid
	}
	e = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := check(ctx); e != nil {
			return e
		}
		id, e := lookup(tx, scope, key, fp)
		if e != nil {
			return e
		}
		if id != "" {
			result.Replayed = true
			result.Report, e = read(tx, scope, id)
			return e
		}
		r := reportRow{ID: uuid.NewString(), OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, Kind: snap.Ref.Kind, SourceID: snap.Ref.ID, SourceVersion: snap.Ref.Version, Title: snap.Title, ProductKey: snap.ProductKey, StoreID: snap.StoreID, SourceAt: snap.SourceAt, CapturedAt: time.Now().UTC(), Content: body, Digest: digest}
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&r).Error; e != nil {
			return e
		}
		var existing reportRow
		if e := scoped(tx, scope).Where("kind=? AND source_id=? AND source_version=?", r.Kind, r.SourceID, r.SourceVersion).Take(&existing).Error; e != nil {
			return e
		}
		if existing.Digest != digest || existing.Title != r.Title || existing.ProductKey != r.ProductKey || existing.StoreID != r.StoreID || !sameTime(existing.SourceAt, r.SourceAt) {
			return rc.ErrConflict
		}
		f := favoriteRow{ReportID: existing.ID, OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, UpdatedAt: existing.CapturedAt}
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&f).Error; e != nil {
			return e
		}
		cmd := commandRow{scope.OrganizationID, scope.ActorID, key, fp, existing.ID}
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&cmd).Error; e != nil {
			return e
		}
		winner, e := lookup(tx, scope, key, fp)
		if e != nil {
			return e
		}
		if winner != existing.ID {
			return rc.ErrConflict
		}
		result.Report, e = read(tx, scope, winner)
		return e
	})
	return result, e
}
func sameTime(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}
func (s *Store) Favorite(ctx context.Context, scope rc.Scope, key, fp, id string, target bool, check func(context.Context) error) (rc.Result, error) {
	result := rc.Result{}
	if !scope.Valid() || !rc.UUID(key) || !rc.UUID(id) || check == nil {
		return result, rc.ErrInvalid
	}
	e := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := check(ctx); e != nil {
			return e
		}
		var fav favoriteRow
		if e := scoped(tx, scope).Where("report_id=?", id).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&fav).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return rc.ErrNotFound
			}
			return e
		}
		winner, e := lookup(tx, scope, key, fp)
		if e != nil {
			return e
		}
		if winner != "" {
			if winner != id {
				return rc.ErrConflict
			}
			result.Replayed = true
			result.Report, e = read(tx, scope, id)
			return e
		}
		cmd := commandRow{scope.OrganizationID, scope.ActorID, key, fp, id}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&cmd)
		if created.Error != nil {
			return created.Error
		}
		winner, e = lookup(tx, scope, key, fp)
		if e != nil || winner != id {
			if e != nil {
				return e
			}
			return rc.ErrConflict
		}
		if created.RowsAffected == 1 {
			if e := scoped(tx.Model(&favoriteRow{}), scope).Where("report_id=?", id).Updates(map[string]any{"favorite": target, "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
		} else {
			result.Replayed = true
		}
		result.Report, e = read(tx, scope, id)
		return e
	})
	return result, e
}

type cursor struct {
	At time.Time
	ID string
}

func pageQuery(db *gorm.DB, scope rc.Scope, f rc.Filter) (*gorm.DB, error) {
	if !scope.Valid() || !f.Valid() {
		return nil, rc.ErrInvalid
	}
	q := db.Table("report_center.saved_reports AS r").Joins("JOIN report_center.favorites AS f ON f.report_id=r.id AND f.organization_id=r.organization_id AND f.actor_id=r.actor_id").Where("r.organization_id=? AND r.actor_id=?", scope.OrganizationID, scope.ActorID)
	if f.View == "favorites" {
		q = q.Where("f.favorite=TRUE")
	}
	if f.View == "recent" {
		q = q.Where("r.captured_at>=?", time.Now().UTC().AddDate(0, 0, -30))
	}
	if f.Kind != "" {
		q = q.Where("r.kind=?", f.Kind)
	}
	if f.Search != "" {
		term := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(f.Search)
		q = q.Where("(r.title ILIKE ? OR r.product_key ILIKE ?)", "%"+term+"%", "%"+term+"%")
	}
	if f.Cursor != "" {
		b, e := base64.RawURLEncoding.DecodeString(f.Cursor)
		var c cursor
		if e != nil || json.Unmarshal(b, &c) != nil || c.At.IsZero() || !rc.UUID(c.ID) {
			return nil, rc.ErrInvalid
		}
		q = q.Where("(r.captured_at,r.id)<(?,?)", c.At, c.ID)
	}
	return q, nil
}
func (s *Store) List(ctx context.Context, scope rc.Scope, f rc.Filter) (rc.Page, error) {
	result := rc.Page{Items: []rc.ReportSummary{}}
	q, e := pageQuery(s.db.WithContext(ctx), scope, f)
	if e != nil {
		return result, e
	}
	var rows []struct {
		Report   reportRow `gorm:"embedded"`
		Favorite bool
	}
	e = q.Select("r.id,r.kind,r.source_id,r.source_version,r.title,r.product_key,r.store_id,r.source_at,r.captured_at,f.favorite").Order("r.captured_at DESC,r.id DESC").Limit(f.Limit + 1).Scan(&rows).Error
	if e != nil {
		return result, e
	}
	if len(rows) > f.Limit {
		rows = rows[:f.Limit]
		last := rows[len(rows)-1]
		b, _ := json.Marshal(cursor{last.Report.CapturedAt, last.Report.ID})
		result.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	}
	for _, r := range rows {
		result.Items = append(result.Items, rowSummary(r.Report, r.Favorite))
	}
	return result, nil
}
func (s *Store) Summary(ctx context.Context, scope rc.Scope) (rc.Summary, error) {
	result := rc.Summary{}
	if !scope.Valid() {
		return result, rc.ErrInvalid
	}
	e := s.db.WithContext(ctx).Raw(`SELECT count(*) AS saved,count(*) FILTER(WHERE r.captured_at>=?) AS recent,count(*) FILTER(WHERE f.favorite) AS favorites,count(DISTINCT nullif(r.store_id,'')) AS stores FROM report_center.saved_reports r JOIN report_center.favorites f ON f.report_id=r.id AND f.organization_id=r.organization_id AND f.actor_id=r.actor_id WHERE r.organization_id=? AND r.actor_id=?`, time.Now().UTC().AddDate(0, 0, -30), scope.OrganizationID, scope.ActorID).Scan(&result).Error
	return result, e
}
