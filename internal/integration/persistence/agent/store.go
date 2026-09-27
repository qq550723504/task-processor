// Package agentpersistence stores an Agent control record and its opaque Eino
// checkpoint atomically. It neither dispatches nor recovers provider calls.
package agentpersistence

import (
	"context"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"task-processor/internal/agent"
)

//go:embed schema.sql
var schemaSQL string

type Store struct{ db *gorm.DB }

type runRow struct {
	RunID, Org, Actor, ContextKind, ContextID, RequestKey, Fingerprint string
	Revision                                                           uint64
	Phase                                                              agent.Phase
	Payload                                                            []byte
}

func (runRow) TableName() string { return "product_agent_runs" }

func New(db *gorm.DB) (*Store, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, agent.ErrUnavailable
	}
	return &Store{db: db}, nil
}

// InstallSchema is an explicit new-install composition operation. New/Claim do
// not initialize or migrate shared databases as a side effect of user traffic.
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return agent.ErrUnavailable
	}
	return db.Exec(schemaSQL).Error
}

func (s *Store) Claim(ctx context.Context, initial agent.Record, expected uint64) (agent.Record, bool, error) {
	if s == nil || s.db == nil {
		return agent.Record{}, false, agent.ErrUnavailable
	}
	if !validEnvelope(initial.State) || initial.State.Revision != 0 || initial.State.Phase != agent.Running || len(initial.Checkpoint) != 0 || expected >= math.MaxInt64 {
		return agent.Record{}, false, agent.ErrInvalid
	}
	var result agent.Record
	acquired := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if expected == 0 {
			initial.State.Revision = 1
			row, err := encode(initial)
			if err != nil {
				return err
			}
			created := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "org"}, {Name: "actor"}, {Name: "context_kind"}, {Name: "context_id"}, {Name: "request_key"}}, DoNothing: true}).Create(&row)
			if created.Error != nil {
				return created.Error
			}
			if created.RowsAffected == 1 {
				result, acquired = initial, true
				return nil
			}
		}
		var row runRow
		query := tx.Where("org = ? AND actor = ? AND context_kind = ? AND context_id = ? AND request_key = ?", initial.State.Scope.OrganizationID, initial.State.Scope.ActorID, initial.State.Request.Binding.ContextKind, initial.State.Request.Binding.ContextID, initial.State.Request.Key)
		if expected > 0 {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return agent.ErrConflict
			}
			return err
		}
		stored, err := decode(row)
		if err != nil {
			return err
		}
		if stored.State.Scope != initial.State.Scope || stored.State.Request != initial.State.Request || stored.State.Fingerprint != initial.State.Fingerprint {
			return agent.ErrConflict
		}
		if expected == 0 {
			result = stored
			return nil
		}
		if stored.State.Phase != agent.Interrupted || stored.State.Revision != expected || len(stored.Checkpoint) == 0 {
			return agent.ErrConflict
		}
		stored.State.Phase = agent.Running
		stored.State.Revision++
		updated, err := encode(stored)
		if err != nil {
			return err
		}
		write := tx.Model(&runRow{}).Where("run_id = ? AND revision = ? AND phase = ?", row.RunID, expected, agent.Interrupted).Updates(map[string]any{"revision": updated.Revision, "phase": updated.Phase, "payload": updated.Payload})
		if write.Error != nil {
			return write.Error
		}
		if write.RowsAffected != 1 {
			return agent.ErrConflict
		}
		result, acquired = stored, true
		return nil
	})
	if err != nil {
		return agent.Record{}, false, err
	}
	return result, acquired, nil
}

func (s *Store) Commit(ctx context.Context, record agent.Record, expected uint64) (agent.Record, error) {
	if s == nil || s.db == nil {
		return agent.Record{}, agent.ErrUnavailable
	}
	if expected == 0 || expected >= math.MaxInt64 || record.State.Revision != expected || !validEnvelope(record.State) {
		return agent.Record{}, agent.ErrInvalid
	}
	if record.State.Phase != agent.Interrupted && record.State.Phase != agent.HumanReviewRequired && record.State.Phase != agent.Stopped {
		return agent.Record{}, agent.ErrInvalid
	}
	if (record.State.Phase == agent.Interrupted) != (len(record.Checkpoint) > 0) {
		return agent.Record{}, agent.ErrInvalid
	}
	// Check before locking; the database also enforces the serialized row bound.
	final := record
	final.State.Revision++
	updated, err := encode(final)
	if err != nil {
		return agent.Record{}, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row runRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id = ? AND org = ? AND actor = ?", record.State.RunID, record.State.Scope.OrganizationID, record.State.Scope.ActorID).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return agent.ErrConflict
			}
			return err
		}
		stored, err := decode(row)
		if err != nil {
			return err
		}
		a, b := stored.State, record.State
		if a.Phase != agent.Running || a.Revision != expected || a.Fingerprint != b.Fingerprint || a.Request != b.Request || a.Scope != b.Scope || !a.StartedAt.Equal(b.StartedAt) || !a.Deadline.Equal(b.Deadline) || a.TraceID != b.TraceID || b.Usage.Steps < a.Usage.Steps || b.Usage.ModelCalls < a.Usage.ModelCalls || b.Usage.Tokens < a.Usage.Tokens || b.Usage.CostMicros < a.Usage.CostMicros {
			return agent.ErrConflict
		}
		write := tx.Model(&runRow{}).Where("run_id = ? AND revision = ? AND phase = ?", row.RunID, expected, agent.Running).Updates(map[string]any{"revision": updated.Revision, "phase": updated.Phase, "payload": updated.Payload})
		if write.Error != nil {
			return write.Error
		}
		if write.RowsAffected != 1 {
			return agent.ErrConflict
		}
		return nil
	})
	if err != nil {
		return agent.Record{}, err
	}
	return final, nil
}

func validEnvelope(s agent.State) bool {
	digest, err := hex.DecodeString(s.Fingerprint)
	return err == nil && len(digest) == 32 && hex.EncodeToString(digest) == s.Fingerprint && agent.ValidID(s.RunID) && agent.ValidID(s.Scope.OrganizationID) && agent.ValidID(s.Scope.ActorID) && agent.ValidID(s.Request.Key) && s.Request.Binding.Valid() && agent.ValidID(s.Request.PolicyVersion) && agent.ValidID(s.Request.PromptVersion) && s.Request.Limits.Valid() && !s.StartedAt.IsZero() && s.Deadline.After(s.StartedAt) && s.HumanReviewRequired && s.Usage.Steps >= 0 && s.Usage.ModelCalls >= 0 && s.Usage.Tokens >= 0 && s.Usage.CostMicros >= 0
}

func encode(record agent.Record) (runRow, error) {
	payload, err := json.Marshal(record)
	if err != nil || len(payload) > agent.MaxStateBytes {
		return runRow{}, agent.ErrInvalid
	}
	s := record.State
	return runRow{RunID: s.RunID, Org: s.Scope.OrganizationID, Actor: s.Scope.ActorID, ContextKind: s.Request.Binding.ContextKind, ContextID: s.Request.Binding.ContextID, RequestKey: s.Request.Key, Fingerprint: s.Fingerprint, Revision: s.Revision, Phase: s.Phase, Payload: payload}, nil
}

func decode(row runRow) (agent.Record, error) {
	var record agent.Record
	if len(row.Payload) > agent.MaxStateBytes || json.Unmarshal(row.Payload, &record) != nil || !validEnvelope(record.State) {
		return agent.Record{}, agent.ErrUnavailable
	}
	reencoded, err := encode(record)
	if err != nil || row.RunID != reencoded.RunID || row.Org != reencoded.Org || row.Actor != reencoded.Actor || row.ContextKind != reencoded.ContextKind || row.ContextID != reencoded.ContextID || row.RequestKey != reencoded.RequestKey || row.Fingerprint != reencoded.Fingerprint || row.Revision != reencoded.Revision || row.Phase != reencoded.Phase {
		return agent.Record{}, agent.ErrUnavailable
	}
	return record, nil
}
