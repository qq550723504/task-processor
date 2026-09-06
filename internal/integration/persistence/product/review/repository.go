// Package reviewpersistence implements the title-review UoW on one PostgreSQL DB.
package reviewpersistence

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/review"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }
type proposalRow struct {
	Org, ID, Owner string
	Payload        []byte
}

func (proposalRow) TableName() string { return "product_title_proposals" }

type operationRow struct {
	Org, Actor, OperationKey, Fingerprint string
	Response                              []byte
}

func (operationRow) TableName() string { return "product_title_operations" }

type transaction struct {
	db        *gorm.DB
	op        review.Operation
	publisher *catalog.Publisher
	reader    catalog.VersionedSnapshotReader
}

func NewRepository(db *gorm.DB) (*Repository, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, review.ErrUnavailable
	}
	return &Repository{db}, nil
}
func scoped(db *gorm.DB, a review.Scope, id string) *gorm.DB {
	q := db.Where("org = ? AND id = ?", a.Org, id)
	if !a.Admin {
		q = q.Where("owner = ?", a.Actor)
	}
	return q
}
func load(db *gorm.DB, a review.Scope, id string, lock bool) (review.Record, error) {
	q := scoped(db, a, id).Select("org,id,owner,CASE WHEN octet_length(payload) <= ? THEN payload ELSE NULL END AS payload", review.MaxRecordBytes)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row proposalRow
	err := q.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return review.Record{}, review.ErrNotFound
	}
	if err != nil {
		return review.Record{}, err
	}
	var r review.Record
	if len(row.Payload) == 0 || json.Unmarshal(row.Payload, &r) != nil || r.Org != row.Org || r.ID != row.ID || r.Owner != row.Owner {
		return r, review.ErrUnavailable
	}
	return r, nil
}
func (r *Repository) Read(ctx context.Context, a review.Scope, id string) (review.Record, error) {
	return load(r.db.WithContext(ctx), a, id, false)
}
func replay(db *gorm.DB, op review.Operation) (review.View, bool, error) {
	var row operationRow
	err := db.Select("org,actor,operation_key,fingerprint,CASE WHEN octet_length(response) <= ? THEN response ELSE NULL END AS response", review.MaxRecordBytes).Where("org = ? AND actor = ? AND operation_key = ?", op.Scope.Org, op.Scope.Actor, op.Key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return review.View{}, false, nil
	}
	if err != nil {
		return review.View{}, false, err
	}
	if row.Fingerprint != op.Fingerprint {
		return review.View{}, false, review.ErrConflict
	}
	var v review.View
	if len(row.Response) == 0 || json.Unmarshal(row.Response, &v) != nil {
		return v, false, review.ErrUnavailable
	}
	return v, true, nil
}
func (r *Repository) FindOperation(ctx context.Context, op review.Operation) (review.View, bool, error) {
	return replay(r.db.WithContext(ctx), op)
}
func (r *Repository) Run(ctx context.Context, op review.Operation, fn func(review.Tx) (review.View, error)) (review.View, error) {
	var result review.View
	err := r.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		// Canonical length framing avoids concatenation collisions. Hash collisions
		// merely serialize unrelated operations; SQL primary keys remain authoritative.
		raw, _ := json.Marshal([]string{op.Scope.Org, op.Scope.Actor, op.Key})
		sum := sha256.Sum256(raw)
		if e := db.Exec("SELECT pg_advisory_xact_lock(?)", int64(binary.BigEndian.Uint64(sum[:8]))).Error; e != nil {
			return e
		}
		writer, e := catalogstore.NewTransactionWriter(db)
		if e != nil {
			return e
		}
		publisher, e := catalog.NewPublisher(writer)
		if e != nil {
			return e
		}
		reader, e := catalogstore.NewBoundedSnapshotReader(db, 2<<20)
		if e != nil {
			return e
		}
		result, e = fn(&transaction{db, op, publisher, reader})
		return e
	})
	if err != nil {
		return review.View{}, err
	}
	return result, nil
}
func (t *transaction) Load(id string) (review.Record, error) { return load(t.db, t.op.Scope, id, true) }
func (t *transaction) Replay() (review.View, bool, error)    { return replay(t.db, t.op) }
func (t *transaction) Save(r review.Record) error {
	if err := r.ValidateStorage(); err != nil {
		return err
	}
	if r.Org != t.op.Scope.Org || !t.op.Scope.Admin && r.Owner != t.op.Scope.Actor {
		return review.ErrForbidden
	}
	raw, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if len(raw) > review.MaxRecordBytes {
		return review.ErrTooLarge
	}
	row := proposalRow{r.Org, r.ID, r.Owner, raw}
	// Domain owns transitions; the caller holds the row lock before updating.
	if r.Revision == 1 && r.State == "pending" {
		return t.db.Create(&row).Error
	}
	q := scoped(t.db.Model(&proposalRow{}), t.op.Scope, r.ID).Update("payload", raw)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return review.ErrNotFound
	}
	return nil
}
func (t *transaction) Complete(v review.View) error {
	raw, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if len(raw) > review.MaxRecordBytes {
		return review.ErrTooLarge
	}
	return t.db.Create(&operationRow{t.op.Scope.Org, t.op.Scope.Actor, t.op.Key, t.op.Fingerprint, raw}).Error
}
func (t *transaction) Publisher() *catalog.Publisher           { return t.publisher }
func (t *transaction) Reader() catalog.VersionedSnapshotReader { return t.reader }
