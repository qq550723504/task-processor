package agentconfigpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/aicapability"
	"time"
)

func snapshotQuery(db *gorm.DB, s agent.Scope, r agent.Request) *gorm.DB {
	return db.Where("organization_id=? AND actor_id=? AND context_kind=? AND context_id=? AND request_key=?", s.OrganizationID, s.ActorID, r.Binding.ContextKind, r.Binding.ContextID, r.Key)
}
func snapshotView(row snapshotRow) (agentconfig.Snapshot, error) {
	var v agentconfig.Snapshot
	if len(row.Payload) > 8192 || digest(row.Payload) != row.Digest || json.Unmarshal(row.Payload, &v) != nil {
		return v, agentconfig.ErrUnavailable
	}
	if v.ID != row.ID || v.Scope.OrganizationID != row.OrganizationID || v.Scope.ActorID != row.ActorID || v.AgentID != row.AgentID || v.AgentVersion != row.AgentVersion || v.Epoch != decimal(row.ActivationEpoch) || v.AgentRevision != decimal(row.AgentRevision) || v.Request.Binding.ContextKind != row.ContextKind || v.Request.Binding.ContextID != row.ContextID || v.Request.Key != row.RequestKey || !v.Request.Limits.Valid() || v.ExecutionModelProfile.Validate() != nil {
		return v, agentconfig.ErrUnavailable
	}
	v.Digest = row.Digest
	return v, nil
}
func (s *Store) Prepare(ctx context.Context, c agentconfig.StartCommand) (agentconfig.Snapshot, error) {
	if !scopeOK(c.Scope) || !agentconfig.UUID(c.Request.Key) || !c.Request.Binding.Valid() || !agentconfig.Platform(c.Request.Binding.TargetPlatform) || !agent.ValidID(c.AgentID) || !agent.ValidID(c.AgentVersion) || !c.Request.Limits.Valid() || !agent.ValidID(c.Request.PolicyVersion) || !agent.ValidID(c.Request.PromptVersion) || !c.Request.ContextSnapshotRef.Absent() || !c.Request.ConfigurationSnapshotRef.Absent() || c.ExecutionModelProfile.Validate() != nil || (c.KnowledgeBaseID != "" && !agentconfig.UUID(c.KnowledgeBaseID)) {
		return agentconfig.Snapshot{}, agentconfig.ErrInvalid
	}
	if c.Template != nil {
		if !agentconfig.UUID(c.Template.TemplateID) {
			return agentconfig.Snapshot{}, agentconfig.ErrInvalid
		}
		if _, e := version(c.Template.Revision); e != nil {
			return agentconfig.Snapshot{}, e
		}
	}
	// Deployment limits/prompt versions are frozen output, not public retry input.
	fp, e := hash(struct {
		AgentID               string
		Binding               agent.Binding
		Template              *agentconfig.TemplateRef
		KnowledgeBaseID       string
		ExecutionModelProfile aicapability.ModelProfile
	}{c.AgentID, c.Request.Binding, c.Template, c.KnowledgeBaseID, c.ExecutionModelProfile})
	if e != nil {
		return agentconfig.Snapshot{}, e
	}
	var result agentconfig.Snapshot
	e = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		a, ae := findAgent(tx, c.Scope.OrganizationID, c.AgentID, true)
		var old snapshotRow
		e := snapshotQuery(tx, c.Scope, c.Request).Take(&old).Error
		if e == nil {
			if old.Fingerprint != fp {
				return agentconfig.ErrConflict
			}
			result, e = snapshotView(old)
			return e
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if errors.Is(ae, agentconfig.ErrNotFound) {
			return agentconfig.ErrNotEnabled
		}
		if ae != nil {
			return ae
		}
		if a.Activation != "ENABLED" {
			return agentconfig.ErrNotEnabled
		}
		if c.Template != nil {
			t, e := findTemplate(tx, c.Scope.OrganizationID, c.AgentID, c.Template.TemplateID, true)
			if e != nil {
				return e
			}
			if t.Lifecycle != "ACTIVE" {
				return agentconfig.ErrArchived
			}
			v, _ := version(c.Template.Revision)
			if _, e := templateView(tx, t, v); e != nil {
				return e
			}
		}
		result = agentconfig.Snapshot{ID: uuid.NewString(), Scope: c.Scope, AgentID: c.AgentID, AgentVersion: c.AgentVersion, KnowledgeBaseID: c.KnowledgeBaseID, Epoch: decimal(a.ActivationEpoch), AgentRevision: decimal(a.Revision), Template: c.Template, Request: c.Request, ExecutionModelProfile: c.ExecutionModelProfile}
		raw, e := json.Marshal(result)
		if e != nil || len(raw) > 8192 {
			return agentconfig.ErrInvalid
		}
		row := snapshotRow{ID: result.ID, OrganizationID: c.Scope.OrganizationID, ActorID: c.Scope.ActorID, ContextKind: c.Request.Binding.ContextKind, ContextID: c.Request.Binding.ContextID, RequestKey: c.Request.Key, AgentID: c.AgentID, AgentVersion: c.AgentVersion, Fingerprint: fp, Payload: raw, Digest: digest(raw), ActivationEpoch: a.ActivationEpoch, AgentRevision: a.Revision, CreatedAt: time.Now().UTC()}
		if c.Template != nil {
			v, _ := version(c.Template.Revision)
			row.TemplateID = &c.Template.TemplateID
			row.TemplateRevision = &v
		}
		if e := tx.Create(&row).Error; e != nil {
			return e
		}
		result.Digest = row.Digest
		return nil
	})
	return result, e
}
func (s *Store) LoadSnapshot(ctx context.Context, scope agent.Scope, ref agent.ConfigurationSnapshotRef) (agentconfig.Snapshot, error) {
	if !scopeOK(scope) || ref.Absent() || !ref.ValidOrAbsent() {
		return agentconfig.Snapshot{}, agentconfig.ErrInvalid
	}
	var row snapshotRow
	e := s.db.WithContext(ctx).Where("id=? AND organization_id=? AND actor_id=?", ref.ID, scope.OrganizationID, scope.ActorID).Take(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return agentconfig.Snapshot{}, agentconfig.ErrUnavailable
	}
	if e != nil {
		return agentconfig.Snapshot{}, e
	}
	if row.Digest != ref.Digest {
		return agentconfig.Snapshot{}, agentconfig.ErrUnavailable
	}
	return snapshotView(row)
}

// Match checks the normalized public selection against a retained receipt.
func (s *Store) Match(ctx context.Context, c agentconfig.StartCommand, ref agent.ConfigurationSnapshotRef) (agentconfig.Snapshot, error) {
	snap, e := s.LoadSnapshot(ctx, c.Scope, ref)
	if e != nil {
		return snap, e
	}
	a, _ := hash(struct {
		Binding  agent.Binding
		Template *agentconfig.TemplateRef
		Base     string
	}{snap.Request.Binding, snap.Template, snap.KnowledgeBaseID})
	b, _ := hash(struct {
		Binding  agent.Binding
		Template *agentconfig.TemplateRef
		Base     string
	}{c.Request.Binding, c.Template, c.KnowledgeBaseID})
	if a != b || snap.AgentID != c.AgentID || snap.Request.Key != c.Request.Key {
		return snap, agentconfig.ErrConflict
	}
	return snap, nil
}
