package notificationcenterpersistence

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"task-processor/internal/notificationcenter"
)

//go:embed schema.sql
var schema string

type Store struct{ db *gorm.DB }

func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return notificationcenter.ErrUnavailable
	}
	return db.Exec(schema).Error
}
func New(db *gorm.DB) (*Store, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, notificationcenter.ErrUnavailable
	}
	return &Store{db}, nil
}
func GrantRuntime(db *gorm.DB, role string) error {
	if !regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`).MatchString(role) {
		return notificationcenter.ErrInvalid
	}
	q := `"` + role + `"`
	for _, sql := range []string{"GRANT USAGE ON SCHEMA notification_center TO " + q, "GRANT SELECT, INSERT ON notification_center.reading_receipts, notification_center.commands TO " + q, "GRANT SELECT, INSERT, DELETE ON notification_center.snapshots TO " + q, "GRANT SELECT, INSERT ON notification_center.announcements TO " + q, "GRANT UPDATE(revision,state,withdrawn_at) ON notification_center.announcements TO " + q} {
		if e := db.Exec(sql).Error; e != nil {
			return e
		}
	}
	return nil
}
func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return notificationcenter.ErrUnavailable
	}
	var unsafe bool
	if e := db.WithContext(ctx).Raw(`SELECT rolsuper OR rolcreaterole OR rolcreatedb OR rolreplication OR rolbypassrls FROM pg_roles WHERE rolname=current_user`).Scan(&unsafe).Error; e != nil || unsafe {
		return notificationcenter.ErrUnavailable
	}
	if e := db.WithContext(ctx).Raw(`SELECT EXISTS(SELECT 1 FROM pg_auth_members WHERE member=(SELECT oid FROM pg_roles WHERE rolname=current_user)) OR has_schema_privilege(current_user,'notification_center','CREATE')`).Scan(&unsafe).Error; e != nil || unsafe {
		return notificationcenter.ErrUnavailable
	}
	if e := db.WithContext(ctx).Raw(`SELECT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' AND c.relkind IN ('r','p','v','m','f') AND (c.relowner=(SELECT oid FROM pg_roles WHERE rolname=current_user) OR ((n.nspname<>'notification_center' OR c.relname NOT IN ('announcements','reading_receipts','commands','snapshots')) AND (has_table_privilege(current_user,c.oid,'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER') OR has_any_column_privilege(current_user,c.oid,'SELECT,INSERT,UPDATE,REFERENCES')))))`).Scan(&unsafe).Error; e != nil || unsafe {
		return notificationcenter.ErrUnavailable
	}
	if e := db.WithContext(ctx).Raw(`SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname NOT IN ('pg_catalog','information_schema') AND nspname NOT LIKE 'pg_toast%' AND has_schema_privilege(current_user,oid,'CREATE'))`).Scan(&unsafe).Error; e != nil || unsafe {
		return notificationcenter.ErrUnavailable
	}
	for _, name := range []string{"announcements", "reading_receipts", "commands", "snapshots"} {
		var valid bool
		if e := db.WithContext(ctx).Raw("SELECT to_regclass(?) IS NOT NULL AND has_table_privilege(current_user,?,'SELECT') AND has_table_privilege(current_user,?,'INSERT')", "notification_center."+name, "notification_center."+name, "notification_center."+name).Scan(&valid).Error; e != nil || !valid {
			return notificationcenter.ErrUnavailable
		}
		if e := db.WithContext(ctx).Raw(`SELECT has_table_privilege(current_user,?,'UPDATE,TRUNCATE,REFERENCES,TRIGGER') OR (?<>'snapshots' AND has_table_privilege(current_user,?,'DELETE'))`, "notification_center."+name, name, "notification_center."+name).Scan(&unsafe).Error; e != nil || unsafe {
			return notificationcenter.ErrUnavailable
		}
	}
	var valid bool
	if e := db.WithContext(ctx).Raw(`SELECT has_table_privilege(current_user,'notification_center.snapshots','DELETE') AND has_column_privilege(current_user,'notification_center.announcements','revision','UPDATE') AND has_column_privilege(current_user,'notification_center.announcements','state','UPDATE') AND has_column_privilege(current_user,'notification_center.announcements','withdrawn_at','UPDATE')`).Scan(&valid).Error; e != nil || !valid {
		return notificationcenter.ErrUnavailable
	}
	if e := db.WithContext(ctx).Raw(`SELECT EXISTS(SELECT 1 FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='notification_center' AND a.attnum>0 AND NOT a.attisdropped AND (c.relname<>'announcements' OR a.attname NOT IN ('revision','state','withdrawn_at')) AND has_column_privilege(current_user,c.oid,a.attnum,'UPDATE,REFERENCES'))`).Scan(&unsafe).Error; e != nil || unsafe {
		return notificationcenter.ErrUnavailable
	}
	return nil
}
func unavailable(e error) error {
	if e == nil {
		return nil
	}
	for _, v := range []error{notificationcenter.ErrConflict, notificationcenter.ErrStale, notificationcenter.ErrNotFound, notificationcenter.ErrCapacity, notificationcenter.ErrInvalid} {
		if errors.Is(e, v) {
			return v
		}
	}
	return notificationcenter.ErrUnavailable
}

type commandRow struct {
	ScopeKey, CommandKey, Operation, Fingerprint, ResultID string
	CommittedAt                                            time.Time
}

func (commandRow) TableName() string { return "notification_center.commands" }
func command(r commandRow) notificationcenter.Command {
	return notificationcenter.Command{Key: r.CommandKey, Operation: r.Operation, Fingerprint: r.Fingerprint, ResultID: r.ResultID, CommittedAt: r.CommittedAt}
}
func (s *Store) Replay(ctx context.Context, scope notificationcenter.Scope, key, op, fingerprint string) (notificationcenter.Command, bool, error) {
	var row commandRow
	e := s.db.WithContext(ctx).Where("scope_key=? AND command_key=?", notificationcenter.ScopeKey(scope), key).Take(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return notificationcenter.Command{}, false, nil
	}
	if e != nil {
		return notificationcenter.Command{}, false, unavailable(e)
	}
	if row.Operation != op || row.Fingerprint != fingerprint {
		return notificationcenter.Command{}, false, notificationcenter.ErrConflict
	}
	return command(row), true, nil
}
func (s *Store) Command(ctx context.Context, scope notificationcenter.Scope, key string) (notificationcenter.Command, error) {
	if !notificationcenter.ValidKey(key) {
		return notificationcenter.Command{}, notificationcenter.ErrInvalid
	}
	var row commandRow
	e := s.db.WithContext(ctx).Where("scope_key=? AND command_key=?", notificationcenter.ScopeKey(scope), key).Take(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return notificationcenter.Command{}, notificationcenter.ErrNotFound
	}
	return command(row), unavailable(e)
}
func (s *Store) transact(ctx context.Context, scope notificationcenter.Scope, c notificationcenter.Command, work func(*gorm.DB) error) (notificationcenter.Command, error) {
	out := c
	e := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", notificationcenter.ScopeKey(scope)).Error; e != nil {
			return e
		}
		local := &Store{tx}
		saved, replay, e := local.Replay(ctx, scope, c.Key, c.Operation, c.Fingerprint)
		if e != nil {
			return e
		}
		if replay {
			out = saved
			return nil
		}
		if e := work(tx); e != nil {
			return e
		}
		row := commandRow{ScopeKey: notificationcenter.ScopeKey(scope), CommandKey: c.Key, Operation: c.Operation, Fingerprint: c.Fingerprint, ResultID: out.ResultID, CommittedAt: c.CommittedAt}
		return tx.Create(&row).Error
	})
	return out, unavailable(e)
}
func (s *Store) ReadStates(ctx context.Context, scope notificationcenter.Scope, refs []notificationcenter.Ref) (map[string]bool, error) {
	result := map[string]bool{}
	if len(refs) == 0 {
		return result, nil
	}
	keys := make([]string, len(refs))
	for i, r := range refs {
		keys[i] = notificationcenter.RefKey(r)
	}
	var rows []struct{ ReferenceKey string }
	e := s.db.WithContext(ctx).Table("notification_center.reading_receipts").Select("reference_key").Where("realm=? AND subject=? AND reference_key IN ?", scope.Realm, scope.Subject, keys).Find(&rows).Error
	for _, r := range rows {
		result[r.ReferenceKey] = true
	}
	return result, unavailable(e)
}
func (s *Store) CommitRead(ctx context.Context, scope notificationcenter.Scope, c notificationcenter.Command, refs []notificationcenter.Ref, snapshotID string) (notificationcenter.Command, error) {
	return s.transact(ctx, scope, c, func(tx *gorm.DB) error {
		if snapshotID != "" {
			var alive bool
			if e := tx.Raw("SELECT EXISTS(SELECT 1 FROM notification_center.snapshots WHERE id=? AND scope_key=? AND expires_at>clock_timestamp())", snapshotID, notificationcenter.ScopeKey(scope)).Scan(&alive).Error; e != nil {
				return e
			}
			if !alive {
				return notificationcenter.ErrStale
			}
		}
		for _, ref := range refs {
			raw, _ := json.Marshal(ref)
			if e := tx.Exec("INSERT INTO notification_center.reading_receipts(realm,subject,reference_key,reference,read_at) VALUES(?,?,?,?::jsonb,?) ON CONFLICT DO NOTHING", scope.Realm, scope.Subject, notificationcenter.RefKey(ref), string(raw), c.CommittedAt).Error; e != nil {
				return e
			}
		}
		return nil
	})
}
func (s *Store) Snapshot(ctx context.Context, scope notificationcenter.Scope, id string) (notificationcenter.Snapshot, error) {
	var row struct {
		ID, ScopeKey, Fingerprint string
		Refs                      json.RawMessage
		ExpiresAt                 time.Time
	}
	e := s.db.WithContext(ctx).Table("notification_center.snapshots").Where("id=? AND scope_key=?", id, notificationcenter.ScopeKey(scope)).Take(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return notificationcenter.Snapshot{}, notificationcenter.ErrNotFound
	}
	if e != nil {
		return notificationcenter.Snapshot{}, unavailable(e)
	}
	out := notificationcenter.Snapshot{ID: row.ID, Fingerprint: row.Fingerprint, ExpiresAt: row.ExpiresAt}
	if json.Unmarshal(row.Refs, &out.Refs) != nil || len(out.Refs) > notificationcenter.MaxItems || notificationcenter.Digest(struct {
		Scope notificationcenter.Scope
		Refs  []notificationcenter.Ref
	}{scope, out.Refs}) != out.Fingerprint {
		return notificationcenter.Snapshot{}, notificationcenter.ErrUnavailable
	}
	out.Count = len(out.Refs)
	return out, nil
}
func (s *Store) CreateSnapshot(ctx context.Context, scope notificationcenter.Scope, c notificationcenter.Command, snap notificationcenter.Snapshot) (notificationcenter.Snapshot, error) {
	raw, _ := json.Marshal(snap.Refs)
	committed, e := s.transact(ctx, scope, c, func(tx *gorm.DB) error {
		sk := notificationcenter.ScopeKey(scope)
		if e := tx.Exec("DELETE FROM notification_center.snapshots WHERE scope_key=? AND expires_at<=clock_timestamp()", sk).Error; e != nil {
			return e
		}
		var count int64
		if e := tx.Table("notification_center.snapshots").Where("scope_key=?", sk).Count(&count).Error; e != nil {
			return e
		}
		if count >= 4 {
			return notificationcenter.ErrCapacity
		}
		return tx.Exec("INSERT INTO notification_center.snapshots(id,scope_key,fingerprint,refs,created_at,expires_at) VALUES(?,?,?,?::jsonb,?,?)", snap.ID, sk, snap.Fingerprint, string(raw), c.CommittedAt, snap.ExpiresAt).Error
	})
	if e != nil {
		return notificationcenter.Snapshot{}, e
	}
	return s.Snapshot(ctx, scope, committed.ResultID)
}
func (s *Store) Publish(ctx context.Context, scope notificationcenter.Scope, c notificationcenter.Command, input notificationcenter.AnnouncementInput) (notificationcenter.Command, error) {
	if !notificationcenter.ValidAnnouncement(input) {
		return notificationcenter.Command{}, notificationcenter.ErrInvalid
	}
	c.ResultID = uuid.NewString()
	raw, _ := json.Marshal(input)
	return s.transact(ctx, scope, c, func(tx *gorm.DB) error {
		return tx.Exec("INSERT INTO notification_center.announcements(id,realm,publisher,revision,state,content,published_at) VALUES(?,?,?,1,'PUBLISHED',?::jsonb,?)", c.ResultID, scope.Realm, scope.Subject, string(raw), c.CommittedAt).Error
	})
}
func (s *Store) Withdraw(ctx context.Context, scope notificationcenter.Scope, c notificationcenter.Command, id string, revision int64) (notificationcenter.Command, error) {
	c.ResultID = id
	return s.transact(ctx, scope, c, func(tx *gorm.DB) error {
		result := tx.Exec("UPDATE notification_center.announcements SET state='WITHDRAWN',revision=2,withdrawn_at=? WHERE realm=? AND id=? AND revision=? AND state='PUBLISHED'", c.CommittedAt, scope.Realm, id, revision)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return notificationcenter.ErrConflict
		}
		return nil
	})
}

// Announcements is a source backed only by notification-owned content.
type Announcements struct{ Store *Store }

func (Announcements) Name() string     { return "official" }
func (Announcements) Category() string { return notificationcenter.Official }
func (Announcements) Personal() bool   { return true }

type announcementRow struct {
	ID, Realm, Publisher, State string
	Revision                    int64
	Content                     json.RawMessage
	PublishedAt                 time.Time
}

func (a Announcements) item(r announcementRow) (notificationcenter.Item, error) {
	var input notificationcenter.AnnouncementInput
	if r.State != "PUBLISHED" || r.Revision != 1 || json.Unmarshal(r.Content, &input) != nil || !notificationcenter.ValidAnnouncement(input) {
		return notificationcenter.Item{}, notificationcenter.ErrUnavailable
	}
	return notificationcenter.Item{Ref: notificationcenter.Ref{Source: "official", EntityID: r.ID, Type: input.Category, Revision: strconv.FormatInt(r.Revision, 10)}, Category: notificationcenter.Official, Title: input.Title, Summary: input.Summary, Paragraphs: input.Paragraphs, OccurredAt: &r.PublishedAt, Target: input.Target}, nil
}
func (a Announcements) ListVisible(ctx context.Context, scope notificationcenter.Scope, after string, limit int) (notificationcenter.SourcePage, error) {
	q := a.Store.db.WithContext(ctx).Table("notification_center.announcements").Where("realm=? AND state='PUBLISHED'", scope.Realm)
	if after != "" {
		if !notificationcenter.ValidKey(after) {
			return notificationcenter.SourcePage{}, notificationcenter.ErrInvalid
		}
		q = q.Where("id>?", after)
	}
	var rows []announcementRow
	if e := q.Order("id ASC").Limit(limit + 1).Find(&rows).Error; e != nil {
		return notificationcenter.SourcePage{}, unavailable(e)
	}
	page := notificationcenter.SourcePage{Items: []notificationcenter.Item{}}
	if len(rows) > limit {
		rows = rows[:limit]
		page.Next = rows[len(rows)-1].ID
	}
	for _, r := range rows {
		item, e := a.item(r)
		if e != nil {
			return notificationcenter.SourcePage{}, e
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}
func (a Announcements) ReadVisible(ctx context.Context, scope notificationcenter.Scope, ref notificationcenter.Ref) (notificationcenter.Item, error) {
	if !notificationcenter.ValidKey(ref.EntityID) {
		return notificationcenter.Item{}, notificationcenter.ErrInvalid
	}
	var row announcementRow
	e := a.Store.db.WithContext(ctx).Table("notification_center.announcements").Where("realm=? AND id=? AND state='PUBLISHED'", scope.Realm, ref.EntityID).Take(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return notificationcenter.Item{}, notificationcenter.ErrNotFound
	}
	if e != nil {
		return notificationcenter.Item{}, unavailable(e)
	}
	return a.item(row)
}

func (a Announcements) ReadVisibleRefs(ctx context.Context, scope notificationcenter.Scope, refs []notificationcenter.Ref) ([]notificationcenter.Item, error) {
	if len(refs) > notificationcenter.MaxItems {
		return nil, notificationcenter.ErrCapacity
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		if !notificationcenter.ValidKey(ref.EntityID) {
			return nil, notificationcenter.ErrInvalid
		}
		ids = append(ids, ref.EntityID)
	}
	items := make([]notificationcenter.Item, 0, len(refs))
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		var rows []announcementRow
		if err := a.Store.db.WithContext(ctx).Table("notification_center.announcements").Where("realm=? AND id IN ? AND state='PUBLISHED'", scope.Realm, ids[start:end]).Limit(100).Find(&rows).Error; err != nil {
			return nil, unavailable(err)
		}
		if len(rows) != end-start {
			return nil, notificationcenter.ErrStale
		}
		for _, row := range rows {
			item, err := a.item(row)
			if err != nil {
				return nil, err
			}
			item.Paragraphs = []string{}
			items = append(items, item)
		}
	}
	return items, nil
}

var _ notificationcenter.Repository = (*Store)(nil)
