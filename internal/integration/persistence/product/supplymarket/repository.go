package supplymarketpersistence

import (
	"context"
	"encoding/json"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/supplymarket"

	"gorm.io/gorm"
)

type DisclosureVerifier interface {
	Verify(context.Context, supplymarket.SourceReference, supplymarket.PublicProduct) error
}
type FileVerifier interface {
	Verify(context.Context, collection.Scope, []string) error
}
type Receiver interface {
	Receive(context.Context, collection.Scope, string, supplymarket.Release) (collection.Receipt, error)
}
type Dependencies struct {
	Disclosure func(*gorm.DB) (DisclosureVerifier, error)
	Files      func(*gorm.DB) (FileVerifier, error)
	Receiver   func(*gorm.DB) (Receiver, error)
}
type Repository struct {
	db          *gorm.DB
	deps        Dependencies
	afterCommit func() error
}

func NewRepository(ctx context.Context, db *gorm.DB, deps Dependencies) (*Repository, error) {
	if deps.Disclosure == nil || deps.Files == nil || deps.Receiver == nil || VerifySchema(ctx, db) != nil {
		return nil, supplymarket.ErrUnavailable
	}
	return &Repository{db: db, deps: deps}, nil
}
func principal(scope collection.Scope, platform string) (string, string, string, error) {
	if platform != "" {
		if scope != (collection.Scope{}) || !authidentity.IsBoundedIdentifier(platform) {
			return "", "", "", supplymarket.ErrForbidden
		}
		return "platform", platform, "", nil
	}
	if scope.Validate() != nil {
		return "", "", "", supplymarket.ErrForbidden
	}
	return "organization:" + scope.OrganizationID, scope.ActorID, scope.MemberID, nil
}
func (r *Repository) Execute(ctx context.Context, c supplymarket.Command, guard supplymarket.Guard) (receipt supplymarket.Receipt, err error) {
	p, actor, member, err := principal(c.Scope, c.PlatformActor)
	if err != nil {
		return receipt, err
	}
	if ctx == nil || r == nil || guard == nil || !collection.ValidID(c.Key) || !collection.ValidID(c.OperationID) || len(c.InputHash) != 64 {
		return receipt, supplymarket.ErrInvalid
	}
	err = r.write(ctx, func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", collection.Digest([]string{p, actor, c.Key})).Error; err != nil {
			return err
		}
		var op struct {
			OperationID, MemberID, InputHash string
			ReceiptJSON                      []byte
		}
		found := tx.Raw("SELECT operation_id,member_id,input_hash,receipt_json FROM supply_market_commands WHERE principal=? AND actor_id=? AND command_key=?", p, actor, c.Key).Scan(&op)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected == 1 {
			if op.InputHash != c.InputHash || op.OperationID != c.OperationID || op.MemberID != member {
				return supplymarket.ErrConflict
			}
			if json.Unmarshal(op.ReceiptJSON, &receipt) != nil || receipt.OperationID != c.OperationID {
				return supplymarket.ErrUnavailable
			}
			receipt.Replayed = true
			return guard(ctx)
		}
		receipt = supplymarket.Receipt{OperationID: c.OperationID}
		var verifyDisclosure func() error
		now := time.Now().UTC().Truncate(time.Microsecond)
		i := c.Input
		switch i.Action {
		case "create_official_draft", "submit_selected", "submit_connection":
			if c.PlatformActor != "" || c.Scope.Validate() != nil {
				return supplymarket.ErrForbidden
			}
			record := supplymarket.Record{ID: collection.StableID(c.OperationID, "record"), Owner: c.Scope, Revision: 1, CreatedAt: now, Stage: supplymarket.Submitted, Supply: i.Supply, Connection: i.Connection, FileIDs: i.FileIDs, Disclosure: i.Disclosure}
			if i.Action == "submit_connection" {
				record.Kind = "connection"
			} else {
				if c.Select == nil {
					return supplymarket.ErrForbidden
				}
				selected, err := c.Select(ctx)
				if err != nil {
					return err
				}
				if selected.Source.Scope != c.Scope || !record.Disclosure || record.Supply == nil || record.Supply.Validate() != nil {
					return supplymarket.ErrInvalid
				}
				record.Source = &selected.Source
				record.Product = &selected.Product
				verifyDisclosure = func() error { return r.verify(tx, ctx, *record.Source, *record.Product) }
				if i.Action == "create_official_draft" {
					record.Kind = "official"
					record.Stage = supplymarket.Draft
				} else {
					record.Kind = "selected"
					if selected.Source.ApplyID == "" || selected.Source.Version <= selected.Source.OriginalVersion || len(i.FileIDs) == 0 {
						return supplymarket.ErrInvalid
					}
				}
			}
			if len(i.FileIDs) > 0 {
				if err := r.files(tx, ctx, c.Scope, i.FileIDs); err != nil {
					return err
				}
			}
			raw, err := json.Marshal(record)
			if err != nil {
				return supplymarket.ErrInvalid
			}
			if err := tx.Exec("INSERT INTO supply_market_records(id,organization_id,actor_id,member_id,kind,stage,revision,record_json,created_at) VALUES(?,?,?,?,?,?,1,?::jsonb,?)", record.ID, c.Scope.OrganizationID, c.Scope.ActorID, c.Scope.MemberID, record.Kind, record.Stage, string(raw), now).Error; err != nil {
				return err
			}
			if err := appendEvent(tx, record, c.OperationID, actor, i.Action, i.Note, i.FileIDs, now); err != nil {
				return err
			}
			receipt.RecordID, receipt.Revision = record.ID, 1
		case "select_release":
			if c.PlatformActor != "" {
				return supplymarket.ErrForbidden
			}
			release, err := readRelease(tx, i.ID, true)
			if err != nil {
				return err
			}
			if release.Revision != i.ExpectedRevision {
				return supplymarket.ErrConflict
			}
			verifyDisclosure = func() error { return r.verify(tx, ctx, release.Source, release.Product) }
			receiver, err := r.deps.Receiver(tx)
			if err != nil || receiver == nil {
				return supplymarket.ErrUnavailable
			}
			received, err := receiver.Receive(ctx, c.Scope, c.OperationID, release)
			if err != nil {
				return err
			}
			receipt.ReleaseID, receipt.Collection, receipt.Revision = release.ID, &received, release.Revision
		case "revoke":
			if c.PlatformActor == "" {
				return supplymarket.ErrForbidden
			}
			// Use the same record -> release lock order as publication. The first
			// immutable release reference only identifies the record to lock.
			release, err := readRelease(tx, i.ID, false)
			if err != nil {
				return err
			}
			record, err := readRecord(tx, collection.Scope{}, c.PlatformActor, release.RecordID, true)
			if err != nil {
				return err
			}
			release, err = readRelease(tx, i.ID, true)
			if err != nil {
				return err
			}
			if release.Revision != i.ExpectedRevision {
				return supplymarket.ErrConflict
			}
			release.Active = false
			release.Revision++
			raw, err := json.Marshal(release)
			if err != nil {
				return err
			}
			if err := tx.Exec("UPDATE supply_market_releases SET active=false,revision=?,release_json=?::jsonb WHERE id=?", release.Revision, string(raw), release.ID).Error; err != nil {
				return err
			}
			if err := appendEvent(tx, record, c.OperationID, actor, i.Action, i.Note, nil, now); err != nil {
				return err
			}
			receipt.ReleaseID, receipt.Revision = release.ID, release.Revision
		default:
			record, err := readRecord(tx, c.Scope, c.PlatformActor, i.ID, true)
			if err != nil {
				return err
			}
			if record.Revision != i.ExpectedRevision {
				return supplymarket.ErrConflict
			}
			if i.Action == "publish" {
				if c.PlatformActor == "" || record.Kind == "connection" || !record.Disclosure || record.Source == nil || record.Product == nil || record.Supply == nil {
					return supplymarket.ErrForbidden
				}
				if record.Kind == "official" {
					if c.PlatformActor != record.Owner.ActorID {
						return supplymarket.ErrForbidden
					}
					if record.Stage != supplymarket.Draft {
						return supplymarket.ErrConflict
					}
				} else if record.Stage != supplymarket.Approved || !record.CooperationConfirmed {
					return supplymarket.ErrConflict
				}
				verifyDisclosure = func() error { return r.verify(tx, ctx, *record.Source, *record.Product) }
				var active int64
				if err := tx.Raw("SELECT count(*) FROM supply_market_releases WHERE record_id=? AND active", record.ID).Scan(&active).Error; err != nil {
					return err
				}
				if active != 0 {
					return supplymarket.ErrConflict
				}
				release := supplymarket.Release{ID: collection.StableID(c.OperationID, "release"), RecordID: record.ID, Channel: record.Kind, OriginalOwner: record.Owner, Source: *record.Source, Product: *record.Product, Supply: *record.Supply, Revision: 1, Active: true, PublishedAt: now}
				raw, err := json.Marshal(release)
				if err != nil {
					return err
				}
				source, err := json.Marshal(release.Source)
				if err != nil {
					return err
				}
				if err := tx.Exec("INSERT INTO supply_market_releases(id,record_id,organization_id,actor_id,member_id,channel,revision,active,release_json,source_json,published_at) VALUES(?,?,?,?,?,?,1,true,?::jsonb,?::jsonb,?)", release.ID, record.ID, record.Owner.OrganizationID, record.Owner.ActorID, record.Owner.MemberID, record.Kind, string(raw), string(source), now).Error; err != nil {
					return err
				}
				receipt.ReleaseID = release.ID
			} else {
				if i.Action == "supplement" {
					if c.PlatformActor != "" {
						return supplymarket.ErrForbidden
					}
					if err := r.files(tx, ctx, record.Owner, i.FileIDs); err != nil {
						return err
					}
				} else if c.PlatformActor == "" {
					return supplymarket.ErrForbidden
				}
				next, err := supplymarket.NextStage(record.Kind, record.Stage, supplymarket.Evaluation{Action: i.Action, Note: i.Note, CooperationConfirmed: i.CooperationConfirmed})
				if err != nil {
					return err
				}
				record.Stage = next
				if next == supplymarket.Approved {
					record.CooperationConfirmed = true
				}
			}
			record.Revision++
			raw, err := json.Marshal(record)
			if err != nil {
				return err
			}
			if err := tx.Exec("UPDATE supply_market_records SET stage=?,revision=?,record_json=?::jsonb WHERE id=?", record.Stage, record.Revision, string(raw), record.ID).Error; err != nil {
				return err
			}
			if err := appendEvent(tx, record, c.OperationID, actor, i.Action, i.Note, i.FileIDs, now); err != nil {
				return err
			}
			receipt.RecordID, receipt.Revision = record.ID, record.Revision
		}
		raw, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO supply_market_commands(principal,actor_id,member_id,command_key,operation_id,input_hash,receipt_json,created_at) VALUES(?,?,?,?,?,?,?::jsonb,?)", p, actor, member, c.Key, c.OperationID, c.InputHash, string(raw), now).Error; err != nil {
			return err
		}
		if verifyDisclosure != nil {
			if err := verifyDisclosure(); err != nil {
				return err
			}
		}
		return guard(ctx)
	})
	if err != nil {
		return supplymarket.Receipt{}, err
	}
	return receipt, nil
}
func (r *Repository) verify(tx *gorm.DB, ctx context.Context, source supplymarket.SourceReference, product supplymarket.PublicProduct) error {
	v, err := r.deps.Disclosure(tx)
	if err != nil || v == nil {
		return supplymarket.ErrUnavailable
	}
	return v.Verify(ctx, source, product)
}
func (r *Repository) files(tx *gorm.DB, ctx context.Context, scope collection.Scope, ids []string) error {
	v, err := r.deps.Files(tx)
	if err != nil || v == nil {
		return supplymarket.ErrUnavailable
	}
	return v.Verify(ctx, scope, ids)
}
func appendEvent(tx *gorm.DB, r supplymarket.Record, op, actor, action, note string, files []string, now time.Time) error {
	event := supplymarket.Event{ID: collection.StableID(op, "event"), RecordID: r.ID, ActorID: actor, Action: action, Note: note, FileIDs: files, Stage: r.Stage, CreatedAt: now}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return tx.Exec("INSERT INTO supply_market_events(id,record_id,event_json,created_at) VALUES(?,?,?::jsonb,?)", event.ID, r.ID, string(raw), now).Error
}
func (r *Repository) write(ctx context.Context, fn func(*gorm.DB) error) (err error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return supplymarket.ErrUnavailable
	}
	committing := false
	defer func() {
		if caught := recover(); caught != nil {
			if !committing {
				_ = tx.Rollback().Error
			}
			panic(caught)
		}
		if err != nil && !committing {
			_ = tx.Rollback().Error
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	committing = true
	if tx.Commit().Error != nil {
		return supplymarket.ErrUnknown
	}
	if r.afterCommit != nil && r.afterCommit() != nil {
		return supplymarket.ErrUnknown
	}
	return nil
}
