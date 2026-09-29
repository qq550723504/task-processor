package agentconfigpersistence

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	runstore "task-processor/internal/integration/persistence/agent"
)

type Guard struct {
	db       *gorm.DB
	config   *Store
	runs     *runstore.Store
	ceilings func(string, string) (agent.Limits, error)
}

func NewGuard(db *gorm.DB, s *Store, r *runstore.Store, c func(string, string) (agent.Limits, error)) (*Guard, error) {
	if db == nil || s == nil || s.db != db || r == nil || !r.UsesPool(db) || c == nil {
		return nil, agentconfig.ErrUnavailable
	}
	return &Guard{db, s, r, c}, nil
}
func (g *Guard) Claim(ctx context.Context, initial agent.Record, expected uint64) (agent.Record, bool, error) {
	snap, e := g.config.LoadSnapshot(ctx, initial.State.Scope, initial.State.Request.ConfigurationSnapshotRef)
	if e != nil {
		return agent.Record{}, false, e
	}
	command := initial.State.Request
	command.ContextSnapshotRef = agent.ContextSnapshotRef{}
	command.ConfigurationSnapshotRef = agent.ConfigurationSnapshotRef{}
	if command != snap.Request || (snap.KnowledgeBaseID == "") != initial.State.Request.ContextSnapshotRef.Absent() {
		return agent.Record{}, false, agentconfig.ErrConflict
	}
	var result agent.Record
	acquired := false
	e = g.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		a, ae := findAgent(tx, snap.Scope.OrganizationID, snap.AgentID, true)
		// Return an existing exact receipt before activation/template/ceiling checks.
		// This lookup shares the lock and transaction used for new acquisition.
		existing, found, re := g.runs.LookupInTransaction(ctx, tx, initial.State.Scope, initial.State.Request.Binding, initial.State.Request.Key)
		if re != nil {
			return re
		}
		if found && expected == 0 {
			if existing.State.Request != initial.State.Request || existing.State.Fingerprint != initial.State.Fingerprint {
				return agent.ErrConflict
			}
			result = existing
			return nil
		}
		if errors.Is(ae, agentconfig.ErrNotFound) || ae == nil && a.Activation != "ENABLED" {
			return agentconfig.ErrNotEnabled
		}
		if ae != nil {
			return ae
		}
		if expected == 0 {
			if decimal(a.ActivationEpoch) != snap.Epoch {
				return agentconfig.ErrChanged
			}
			if snap.Template != nil {
				t, e := findTemplate(tx, snap.Scope.OrganizationID, snap.AgentID, snap.Template.TemplateID, true)
				if e != nil {
					return e
				}
				if t.Lifecycle != "ACTIVE" {
					return agentconfig.ErrArchived
				}
			}
		}
		ceiling, e := g.ceilings(snap.AgentID, snap.AgentVersion)
		if e != nil {
			return e
		}
		if !agentconfig.LimitsAdmissible(snap.Request.Limits, ceiling) {
			return agentconfig.ErrChanged
		}
		result, acquired, e = g.runs.ClaimInTransaction(ctx, tx, initial, expected)
		return e
	})
	if e != nil {
		return agent.Record{}, false, e
	}
	return result, acquired, nil
}
func (g *Guard) Commit(ctx context.Context, r agent.Record, expected uint64) (agent.Record, error) {
	return g.runs.Commit(ctx, r, expected)
}

var _ agent.Store = (*Guard)(nil)
