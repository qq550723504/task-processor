// Package reviewpersistence implements the title-review UoW on one PostgreSQL DB.
package reviewpersistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/review"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:embed schema.sql
var schemaSQL string

type TransactionSourceReaderFactory func(*gorm.DB) (review.SourcePublicationReader, error)

type Repository struct {
	db                  *gorm.DB
	sourceReaderFactory TransactionSourceReaderFactory
	beforeOperationLock func()
}
type proposalRow struct {
	Org, ID, Owner, State string
	Payload               []byte
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
	source    review.SourcePublicationReader
}

func NewRepository(db *gorm.DB, sourceReaderFactory TransactionSourceReaderFactory) (*Repository, error) {
	if db == nil || db.Dialector.Name() != "postgres" || sourceReaderFactory == nil {
		return nil, review.ErrUnavailable
	}
	return &Repository{db: db, sourceReaderFactory: sourceReaderFactory}, nil
}

// InstallSchema initializes the Review-owned tables for an empty task schema.
// Application construction and request handling never execute DDL.
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return review.ErrUnavailable
	}
	return db.Exec(schemaSQL).Error
}
func scoped(db *gorm.DB, a review.Scope, id string) *gorm.DB {
	q := db.Where("org = ? AND id = ?", a.Org, id)
	if !a.Admin {
		q = q.Where("owner = ?", a.Actor)
	}
	return q
}
func load(db *gorm.DB, a review.Scope, id string, lock bool) (review.Record, error) {
	q := scoped(db, a, id).Select("org,id,owner,state,CASE WHEN octet_length(payload) <= ? THEN payload ELSE NULL END AS payload", review.MaxRecordBytes)
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
	return decodeProposalRow(row)
}
func (r *Repository) Read(ctx context.Context, a review.Scope, id string) (review.Record, error) {
	return load(r.db.WithContext(ctx), a, id, false)
}

func decodeProposalRow(row proposalRow) (review.Record, error) {
	var record review.Record
	if len(row.Payload) == 0 || json.Unmarshal(row.Payload, &record) != nil || record.Org != row.Org || record.ID != row.ID || record.Owner != row.Owner || record.State != row.State || review.ValidateStoredRecord(record) != nil {
		return record, review.ErrUnavailable
	}
	return record, nil
}

func (r *Repository) List(ctx context.Context, scope review.Scope, request review.PageRequest) (review.Page, error) {
	if err := request.Validate(); err != nil {
		return review.Page{}, err
	}
	query := r.db.WithContext(ctx).Where("org = ? AND state IN ?", scope.Org, []string{"pending", "accepted"})
	if !scope.Admin {
		query = query.Where("owner = ?", scope.Actor)
	}
	if request.Cursor != nil {
		query = query.Where("id > ?", request.Cursor.ID)
	}
	var rows []proposalRow
	err := query.Select("org,id,owner,state,CASE WHEN octet_length(payload) <= ? THEN payload ELSE NULL END AS payload", review.MaxRecordBytes).Order("id ASC").Limit(request.Limit + 1).Find(&rows).Error
	if err != nil {
		return review.Page{}, err
	}
	page := review.Page{Items: make([]review.CollectionItem, 0, min(len(rows), request.Limit))}
	for index, row := range rows {
		if index == request.Limit {
			break
		}
		record, decodeErr := decodeProposalRow(row)
		if decodeErr != nil {
			return review.Page{}, decodeErr
		}
		item, itemErr := review.CollectionItemForPersistence(record)
		if itemErr != nil {
			return review.Page{}, itemErr
		}
		page.Items = append(page.Items, item)
	}
	if len(rows) > request.Limit {
		page.NextCursor = &review.PageCursor{ID: page.Items[len(page.Items)-1].ID}
	}
	return page, nil
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
		if r.beforeOperationLock != nil {
			r.beforeOperationLock()
		}
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
		source, e := r.sourceReaderFactory(db)
		if e != nil {
			return e
		}
		result, e = fn(&transaction{db, op, publisher, reader, source})
		return e
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return review.View{}, err
	}
	return result, nil
}
func (t *transaction) Load(id string) (review.Record, error) { return load(t.db, t.op.Scope, id, true) }
func (t *transaction) Replay() (review.View, bool, error)    { return replay(t.db, t.op) }
func (t *transaction) Save(r review.Record) error {
	if err := review.ValidateStoredRecord(r); err != nil {
		return err
	}
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
	row := proposalRow{Org: r.Org, ID: r.ID, Owner: r.Owner, State: r.State, Payload: raw}
	// Domain owns transitions; the caller holds the row lock before updating.
	if r.Revision == 1 && r.State == "pending" {
		return t.db.Create(&row).Error
	}
	q := scoped(t.db.Model(&proposalRow{}), t.op.Scope, r.ID).Updates(map[string]any{"payload": raw, "state": r.State})
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
func (t *transaction) Publisher() *catalog.Publisher                { return t.publisher }
func (t *transaction) Reader() catalog.VersionedSnapshotReader      { return t.reader }
func (t *transaction) SourceReader() review.SourcePublicationReader { return t.source }
