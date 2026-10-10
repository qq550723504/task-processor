package operationscockpitpersistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	c "task-processor/internal/operationscockpit"
)

// Boundary is mandatory current IAM/module/Store authorization. LockStores
// must borrow this exact transaction through the current Store owner, locking
// sorted Store/grant rows until commit, including administrator Store rows.
// No unlocked Get fallback or SQL against Store tables is provided here.
type Boundary interface {
	Current(context.Context, c.Scope) (c.Access, error)
	ReadStores(context.Context, c.Scope, []string) error
	LockStores(context.Context, *gorm.DB, c.Scope, []string) error
}

type Store struct {
	db       *gorm.DB
	boundary Boundary
	now      func() time.Time
}

func New(ctx context.Context, db *gorm.DB, boundary Boundary, now func() time.Time) (*Store, error) {
	if boundary == nil || now == nil {
		return nil, c.ErrUnavailable
	}
	if err := VerifyRuntime(ctx, db); err != nil {
		return nil, err
	}
	return &Store{db: db, boundary: boundary, now: now}, nil
}

type goalRow struct {
	OrganizationID, GoalID, CreatorID string
	Revision                          int64
	Config                            []byte `gorm:"type:jsonb"`
	UpdatedBy                         string
	UpdatedAt                         time.Time
}

func (goalRow) TableName() string { return "operations_cockpit.goal_heads" }
func (g goalRow) head() c.GoalHead {
	return c.GoalHead{OrganizationID: g.OrganizationID, ID: g.GoalID, CreatorID: g.CreatorID, Revision: g.Revision}
}
func (g goalRow) view() (c.GoalVersion, error) {
	var config c.GoalConfig
	if json.Unmarshal(g.Config, &config) != nil || !config.Valid() {
		return c.GoalVersion{}, c.ErrUnavailable
	}
	return c.GoalVersion{ID: g.GoalID, CreatorID: g.CreatorID, Revision: g.Revision, Config: config, UpdatedBy: g.UpdatedBy, UpdatedAt: g.UpdatedAt}, nil
}

type goalVersionRow struct {
	OrganizationID, GoalID string
	Revision               int64
	Config                 []byte `gorm:"type:jsonb"`
	UpdatedBy              string
	UpdatedAt              time.Time
}

func (goalVersionRow) TableName() string { return "operations_cockpit.goal_versions" }

type factRow struct {
	OrganizationID, RecordID, StoreID string
	Revision                          int64
	StartDate, EndDate                string
	Amounts                           []byte `gorm:"type:jsonb"`
	Note, UpdatedBy                   string
	UpdatedAt                         time.Time
}

func (factRow) TableName() string { return "operations_cockpit.facts" }

type commandRow struct {
	OrganizationID, ActorID, IdempotencyKey, Fingerprint string
	StoreIDs                                             []byte `gorm:"type:jsonb"`
	Receipt                                              []byte `gorm:"type:jsonb"`
}

func (commandRow) TableName() string { return "operations_cockpit.commands" }

func lock(tx *gorm.DB, parts ...string) error {
	key, err := json.Marshal(parts)
	if err != nil {
		return err
	}
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", string(key)).Error
}
func fingerprint(command c.Command) (string, error) {
	command.Key = ""
	if command.Goal != nil {
		copy := *command.Goal
		copy.StoreIDs = append([]string(nil), copy.StoreIDs...)
		sort.Strings(copy.StoreIDs)
		command.Goal = &copy
	}
	data, err := json.Marshal(command)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
func findGoal(tx *gorm.DB, org string, locked bool) (*goalRow, error) {
	query := tx.Where("organization_id=?", org)
	if locked {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row goalRow
	err := query.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

func (s *Store) Execute(ctx context.Context, command c.Command) (c.Receipt, error) {
	now := s.now()
	if !command.Valid(now) {
		return c.Receipt{}, c.ErrInvalid
	}
	access, err := s.boundary.Current(ctx, command.Scope)
	if err != nil {
		return c.Receipt{}, err
	}
	digest, err := fingerprint(command)
	if err != nil {
		return c.Receipt{}, err
	}
	var result c.Receipt
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lock(tx, "cockpit/command", command.Scope.OrganizationID, command.Scope.ActorID, command.Key); err != nil {
			return err
		}
		goalCommand := strings.HasPrefix(command.Operation, "goal_")
		var head *goalRow
		if goalCommand {
			if err := lock(tx, "cockpit/goal", command.Scope.OrganizationID); err != nil {
				return err
			}
			var err error
			head, err = findGoal(tx, command.Scope.OrganizationID, true)
			if err != nil {
				return err
			}
		}
		var existing commandRow
		lookup := tx.Where("organization_id=? AND actor_id=? AND idempotency_key=?", command.Scope.OrganizationID, command.Scope.ActorID, command.Key).Take(&existing).Error
		if lookup == nil {
			if existing.Fingerprint != digest {
				return c.ErrConflict
			}
			if goalCommand && (head == nil || head.GoalID != command.ID || !head.head().CanMaintain(command.Scope, access)) {
				return c.ErrForbidden
			}
			if !goalCommand && (!access.StoresRead || !access.FactsWrite) {
				return c.ErrForbidden
			}
			var ids []string
			if json.Unmarshal(existing.StoreIDs, &ids) != nil || json.Unmarshal(existing.Receipt, &result) != nil || len(ids) == 0 || len(ids) > 50 || result.CommandID != command.Key || result.ID != command.ID || result.Operation != command.Operation {
				return c.ErrUnavailable
			}
			sort.Strings(ids)
			if err := s.boundary.LockStores(ctx, tx, command.Scope, ids); err != nil {
				return err
			}
			return s.finalCheck(ctx, command, head, false)
		}
		if !errors.Is(lookup, gorm.ErrRecordNotFound) {
			return lookup
		}
		ids := []string{command.StoreID}
		var config *c.GoalConfig
		if goalCommand {
			var identity *c.GoalHead
			if head != nil {
				h := head.head()
				identity = &h
				if h.ID != command.ID {
					return c.ErrNotFound
				}
			}
			operation := strings.TrimPrefix(command.Operation, "goal_")
			if err := c.AuthorizeGoalCommand(identity, command.Scope, access, operation, command.Expected); err != nil {
				return err
			}
			config = command.Goal
			if operation == "restore" {
				var previous goalVersionRow
				if err := tx.Where("organization_id=? AND goal_id=? AND revision=?", command.Scope.OrganizationID, command.ID, command.SourceRevision).Take(&previous).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return c.ErrNotFound
					}
					return err
				}
				var restored c.GoalConfig
				if json.Unmarshal(previous.Config, &restored) != nil || !restored.Valid() {
					return c.ErrUnavailable
				}
				config = &restored
			}
			ids = append([]string(nil), config.StoreIDs...)
		} else if !access.StoresRead || !access.FactsWrite {
			return c.ErrForbidden
		}
		sort.Strings(ids)
		if err := s.boundary.LockStores(ctx, tx, command.Scope, ids); err != nil {
			return err
		}
		var revision int64
		if goalCommand {
			revision, head, err = s.applyGoal(tx, command, head, *config, now)
		} else {
			revision, err = s.applyFact(tx, command, now)
		}
		if err != nil {
			return err
		}
		result = c.Receipt{CommandID: command.Key, Operation: command.Operation, ID: command.ID, Revision: strconv.FormatInt(revision, 10), CommittedAt: now}
		receipt, err := json.Marshal(result)
		if err != nil {
			return err
		}
		scopeStores, err := json.Marshal(ids)
		if err != nil {
			return err
		}
		if err := tx.Create(&commandRow{OrganizationID: command.Scope.OrganizationID, ActorID: command.Scope.ActorID, IdempotencyKey: command.Key, Fingerprint: digest, StoreIDs: scopeStores, Receipt: receipt}).Error; err != nil {
			return err
		}
		return s.finalCheck(ctx, command, head, true)
	})
	if err != nil {
		return c.Receipt{}, err
	}
	return result, nil
}

func (s *Store) finalCheck(ctx context.Context, command c.Command, head *goalRow, newlyApplied bool) error {
	access, err := s.boundary.Current(ctx, command.Scope)
	if err != nil {
		return err
	}
	if newlyApplied && command.Operation == "goal_create" && !access.GoalsCreate {
		return c.ErrForbidden
	}
	if head != nil {
		if head.GoalID != command.ID || !head.head().CanMaintain(command.Scope, access) {
			return c.ErrForbidden
		}
	} else if !access.StoresRead || !access.FactsWrite {
		return c.ErrForbidden
	}
	return ctx.Err()
}

func (s *Store) applyGoal(tx *gorm.DB, command c.Command, head *goalRow, config c.GoalConfig, now time.Time) (int64, *goalRow, error) {
	config.StoreIDs = append([]string(nil), config.StoreIDs...)
	sort.Strings(config.StoreIDs)
	payload, err := json.Marshal(config)
	if err != nil {
		return 0, nil, err
	}
	revision := int64(1)
	if head == nil {
		head = &goalRow{OrganizationID: command.Scope.OrganizationID, GoalID: command.ID, CreatorID: command.Scope.ActorID, Revision: revision, Config: payload, UpdatedBy: command.Scope.ActorID, UpdatedAt: now}
		if err := tx.Create(head).Error; err != nil {
			return 0, nil, err
		}
	} else {
		if head.Revision != command.Expected || head.Revision == math.MaxInt64 {
			return 0, nil, c.ErrRevision
		}
		revision = head.Revision + 1
		update := tx.Model(&goalRow{}).Where("organization_id=? AND goal_id=? AND revision=?", command.Scope.OrganizationID, command.ID, command.Expected).Updates(map[string]any{"revision": revision, "config": payload, "updated_by": command.Scope.ActorID, "updated_at": now})
		if update.Error != nil {
			return 0, nil, update.Error
		}
		if update.RowsAffected != 1 {
			return 0, nil, c.ErrRevision
		}
		head.Revision = revision
		head.Config = payload
		head.UpdatedBy = command.Scope.ActorID
		head.UpdatedAt = now
	}
	version := goalVersionRow{OrganizationID: head.OrganizationID, GoalID: head.GoalID, Revision: revision, Config: payload, UpdatedBy: command.Scope.ActorID, UpdatedAt: now}
	return revision, head, tx.Create(&version).Error
}

func (s *Store) applyFact(tx *gorm.DB, command c.Command, now time.Time) (int64, error) {
	if err := lock(tx, "cockpit/facts", command.Scope.OrganizationID, command.StoreID); err != nil {
		return 0, err
	}
	var previous factRow
	lookup := tx.Where("organization_id=? AND record_id=?", command.Scope.OrganizationID, command.ID).Take(&previous).Error
	if lookup != nil && !errors.Is(lookup, gorm.ErrRecordNotFound) {
		return 0, lookup
	}
	if command.Operation == "fact_create" && lookup == nil {
		return 0, c.ErrRevision
	}
	if command.Operation == "fact_update" && (lookup != nil || previous.StoreID != command.StoreID) {
		return 0, c.ErrNotFound
	}
	if lookup == nil && (previous.Revision != command.Expected || previous.Revision == math.MaxInt64) {
		return 0, c.ErrRevision
	}
	var overlap int64
	period := command.Fact.Period
	if err := tx.Model(&factRow{}).Where("organization_id=? AND store_id=? AND record_id<>? AND start_date<=? AND end_date>=?", command.Scope.OrganizationID, command.StoreID, command.ID, period.End, period.Start).Count(&overlap).Error; err != nil {
		return 0, err
	}
	if overlap > 0 {
		return 0, c.ErrOverlap
	}
	payload, err := json.Marshal(command.Fact.Amounts)
	if err != nil {
		return 0, err
	}
	row := factRow{OrganizationID: command.Scope.OrganizationID, RecordID: command.ID, StoreID: command.StoreID, Revision: command.Expected + 1, StartDate: period.Start, EndDate: period.End, Amounts: payload, Note: command.Fact.Note, UpdatedBy: command.Scope.ActorID, UpdatedAt: now}
	if lookup != nil {
		err = tx.Create(&row).Error
	} else {
		update := tx.Model(&factRow{}).Where("organization_id=? AND record_id=? AND store_id=? AND revision=?", row.OrganizationID, row.RecordID, row.StoreID, command.Expected).Updates(map[string]any{"revision": row.Revision, "start_date": row.StartDate, "end_date": row.EndDate, "amounts": row.Amounts, "note": row.Note, "updated_by": row.UpdatedBy, "updated_at": now})
		err = update.Error
		if err == nil && update.RowsAffected != 1 {
			err = c.ErrRevision
		}
	}
	if err != nil {
		return 0, err
	}
	return row.Revision, tx.Table("operations_cockpit.fact_versions").Create(&row).Error
}

func (s *Store) Goal(ctx context.Context, scope c.Scope) (c.GoalVersion, error) {
	if !scope.Valid() {
		return c.GoalVersion{}, c.ErrForbidden
	}
	access, err := s.boundary.Current(ctx, scope)
	if err != nil {
		return c.GoalVersion{}, err
	}
	if !access.GoalsRead {
		return c.GoalVersion{}, c.ErrForbidden
	}
	head, err := findGoal(s.db.WithContext(ctx), scope.OrganizationID, false)
	if err != nil {
		return c.GoalVersion{}, err
	}
	if head == nil {
		return c.GoalVersion{}, c.ErrNotFound
	}
	view, err := head.view()
	if err != nil {
		return c.GoalVersion{}, err
	}
	if err := s.boundary.ReadStores(ctx, scope, view.Config.StoreIDs); err != nil {
		return c.GoalVersion{}, err
	}
	access, err = s.boundary.Current(ctx, scope)
	if err != nil {
		return c.GoalVersion{}, err
	}
	if !access.GoalsRead {
		return c.GoalVersion{}, c.ErrForbidden
	}
	return view, nil
}

func (s *Store) HeadMetadata(ctx context.Context, scope c.Scope) (c.HeadMetadata, error) {
	if !scope.Valid() {
		return c.HeadMetadata{}, c.ErrForbidden
	}
	access, err := s.boundary.Current(ctx, scope)
	if err != nil {
		return c.HeadMetadata{}, err
	}
	head, err := findGoal(s.db.WithContext(ctx), scope.OrganizationID, false)
	if err != nil {
		return c.HeadMetadata{}, err
	}
	if head == nil {
		return c.HeadMetadata{}, c.ErrNotFound
	}
	if !head.head().CanMaintain(scope, access) {
		return c.HeadMetadata{}, c.ErrForbidden
	}
	view, err := head.view()
	if err != nil {
		return c.HeadMetadata{}, err
	}
	err = s.boundary.ReadStores(ctx, scope, view.Config.StoreIDs)
	valid := err == nil
	if err != nil && !errors.Is(err, c.ErrForbidden) && !errors.Is(err, c.ErrNotFound) {
		return c.HeadMetadata{}, err
	}
	access, err = s.boundary.Current(ctx, scope)
	if err != nil {
		return c.HeadMetadata{}, err
	}
	return c.MaintenanceMetadata(head.head(), scope, access, valid)
}
