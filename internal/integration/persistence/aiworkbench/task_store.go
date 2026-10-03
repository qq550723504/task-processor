package aiworkbenchpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"task-processor/internal/agent"
	"task-processor/internal/aiworkbench"
)

type taskRow struct {
	ID                       string
	OrganizationID           string
	OwnerUserID              string
	ConversationID           string
	SourceMessageID          string
	ProposalID               string
	ProposalDigest           string
	ConfirmationFingerprint  string
	Kind                     string
	Title                    string
	GoalSummary              string
	OperationID              string
	ProductKey               string
	TargetPlatform           string
	AgentID                  string
	AgentVersion             string
	ExecutionRequestKey      string
	ConfigurationSnapshotRef json.RawMessage `gorm:"type:jsonb"`
	ContextSnapshotRef       json.RawMessage `gorm:"type:jsonb"`
	ExecutionRequestDigest   string
	ExecutionRequest         json.RawMessage `gorm:"type:jsonb"`
	CreatedAt                time.Time
}

func (taskRow) TableName() string { return "ai_workbench.business_tasks" }

func canonicalRequest(raw []byte) ([]byte, agent.Request, error) {
	var request agent.Request
	if json.Unmarshal(raw, &request) != nil {
		return nil, agent.Request{}, aiworkbench.ErrUnavailable
	}
	canonical, err := json.Marshal(request)
	if err != nil {
		return nil, agent.Request{}, aiworkbench.ErrUnavailable
	}
	return canonical, request, nil
}

func task(row taskRow) (aiworkbench.BusinessTask, error) {
	var config agent.ConfigurationSnapshotRef
	var contextRef agent.ContextSnapshotRef
	requestJSON, request, err := canonicalRequest(row.ExecutionRequest)
	if err != nil || json.Unmarshal(row.ConfigurationSnapshotRef, &config) != nil ||
		json.Unmarshal(row.ContextSnapshotRef, &contextRef) != nil ||
		!config.ValidOrAbsent() || config.Absent() || !contextRef.ValidOrAbsent() ||
		!validDigest(row.ExecutionRequestDigest) || digest(json.RawMessage(requestJSON)) != row.ExecutionRequestDigest ||
		request.GoalSummary != row.GoalSummary {
		return aiworkbench.BusinessTask{}, aiworkbench.ErrUnavailable
	}
	return aiworkbench.BusinessTask{ID: row.ID,
		Scope:          aiworkbench.Scope{OrganizationID: row.OrganizationID, ActorID: row.OwnerUserID},
		ConversationID: row.ConversationID, SourceMessageID: row.SourceMessageID,
		ProposalID: row.ProposalID, ProposalDigest: row.ProposalDigest,
		ConfirmationFingerprint: row.ConfirmationFingerprint, Kind: row.Kind,
		Title: row.Title, GoalSummary: row.GoalSummary, OperationID: row.OperationID,
		ProductKey: row.ProductKey, TargetPlatform: row.TargetPlatform,
		AgentID: row.AgentID, AgentVersion: row.AgentVersion, ExecutionRequestKey: row.ExecutionRequestKey,
		ConfigurationSnapshotRef: config, ContextSnapshotRef: contextRef,
		ExecutionRequestDigest: row.ExecutionRequestDigest,
		ExecutionRequest:       requestJSON, CreatedAt: row.CreatedAt.UTC()}, nil
}

func confirmFingerprint(conversationID, proposalID, key string) string {
	return digest(struct{ ConversationID, ProposalID, Key string }{conversationID, proposalID, key})
}

func (s *Store) lockTaskKey(tx *gorm.DB, scope aiworkbench.Scope, key string) error {
	token := "ai-workbench-task:" + scope.OrganizationID + "\x1f" + scope.ActorID + "\x1f" + key
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", token).Error
}

func (s *Store) lookupTaskKey(tx *gorm.DB, scope aiworkbench.Scope, key, fingerprint string) (aiworkbench.BusinessTask, bool, error) {
	var row taskRow
	err := tx.Where("organization_id = ? AND owner_user_id = ? AND execution_request_key = ?",
		scope.OrganizationID, scope.ActorID, key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return aiworkbench.BusinessTask{}, false, nil
	}
	if err != nil {
		return aiworkbench.BusinessTask{}, false, err
	}
	if row.ConfirmationFingerprint != fingerprint {
		return aiworkbench.BusinessTask{}, true, aiworkbench.ErrIdempotencyConflict
	}
	result, err := task(row)
	return result, true, err
}

func validatePreparedTask(p aiworkbench.PreparedTask, proposal proposalRow, key string) (json.RawMessage, agent.Request, error) {
	if !validDigest(p.ProposalDigest) || p.ProposalDigest != proposal.Digest ||
		!p.ConfigurationSnapshotRef.ValidOrAbsent() || p.ConfigurationSnapshotRef.Absent() ||
		!p.ContextSnapshotRef.ValidOrAbsent() || !json.Valid(p.ExecutionRequest) || len(p.ExecutionRequest) > 8192 {
		return nil, agent.Request{}, aiworkbench.ErrInvalid
	}
	canonical, request, err := canonicalRequest(p.ExecutionRequest)
	if err != nil ||
		request.Key != key || !request.Binding.Valid() || !request.Limits.Valid() ||
		request.GoalSummary != proposal.GoalSummary || !agent.ValidGoalSummary(request.GoalSummary) ||
		request.ConfigurationSnapshotRef != p.ConfigurationSnapshotRef ||
		request.ContextSnapshotRef != p.ContextSnapshotRef ||
		request.Binding.ContextID != proposal.OperationID ||
		request.Binding.ProductKey != proposal.ProductKey ||
		request.Binding.CatalogVersion != proposal.CatalogVersion ||
		request.Binding.PublicationID != proposal.PublicationID ||
		request.Binding.TargetPlatform != proposal.TargetPlatform ||
		!validName(request.PolicyVersion) || !validName(request.PromptVersion) {
		return nil, agent.Request{}, aiworkbench.ErrInvalid
	}
	if len(canonical) > 8192 {
		return nil, agent.Request{}, aiworkbench.ErrInvalid
	}
	return canonical, request, nil
}

// Confirm is the Workbench-local T1 transaction. Its scoped key lock and
// Conversation row lock order it with same-key confirms and later USER append
// or archive. No external owner call occurs under these locks.
func (s *Store) Confirm(ctx context.Context, scope aiworkbench.Scope, conversationID, proposalID, key string, prepared aiworkbench.PreparedTask) (aiworkbench.BusinessTask, bool, error) {
	if s == nil || s.db == nil || ctx == nil || !validScope(scope) || !validKey(conversationID) || !validKey(proposalID) || !validKey(key) {
		return aiworkbench.BusinessTask{}, false, aiworkbench.ErrInvalid
	}
	fingerprint := confirmFingerprint(conversationID, proposalID, key)
	var result aiworkbench.BusinessTask
	var replay bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockTaskKey(tx, scope, key); err != nil {
			return err
		}
		current, found, err := s.lookupTaskKey(tx, scope, key, fingerprint)
		if err != nil {
			return err
		}
		if found {
			result, replay = current, true
			return nil
		}
		conversation, err := s.lookupConversation(tx, scope, conversationID, true)
		if err != nil {
			return err
		}
		if conversation.Lifecycle != "ACTIVE" {
			return aiworkbench.ErrConversationArchived
		}
		var proposal proposalRow
		if err := tx.Where("id = ? AND organization_id = ? AND owner_user_id = ? AND conversation_id = ?",
			proposalID, scope.OrganizationID, scope.ActorID, conversationID).Take(&proposal).Error; err != nil {
			return missing(err)
		}
		if proposalDigest(proposal) != proposal.Digest || !validDigest(proposal.Digest) ||
			proposal.Digest != prepared.ProposalDigest {
			return aiworkbench.ErrRevisionMismatch
		}
		var latest messageRow
		if err := tx.Where("organization_id = ? AND owner_user_id = ? AND conversation_id = ? AND author_kind = ?",
			scope.OrganizationID, scope.ActorID, conversationID, string(aiworkbench.AuthorUser)).
			Order("sequence DESC").Take(&latest).Error; err != nil {
			return missing(err)
		}
		if latest.ID != proposal.SourceUserMessageID || latest.Sequence != proposal.SourceSequence {
			return aiworkbench.ErrRevisionMismatch
		}
		canonical, _, err := validatePreparedTask(prepared, proposal, key)
		if err != nil {
			return err
		}
		configJSON, _ := json.Marshal(prepared.ConfigurationSnapshotRef)
		contextJSON, _ := json.Marshal(prepared.ContextSnapshotRef)
		now := time.Now().UTC().Truncate(time.Microsecond)
		row := taskRow{ID: uuid.NewString(), OrganizationID: scope.OrganizationID, OwnerUserID: scope.ActorID,
			ConversationID: conversationID, SourceMessageID: proposal.SourceUserMessageID,
			ProposalID: proposalID, ProposalDigest: proposal.Digest, ConfirmationFingerprint: fingerprint,
			Kind: proposal.Kind, Title: initialTitle(proposal.GoalSummary), GoalSummary: proposal.GoalSummary,
			OperationID: proposal.OperationID, ProductKey: proposal.ProductKey, TargetPlatform: proposal.TargetPlatform,
			AgentID: proposal.AgentID, AgentVersion: proposal.AgentVersion, ExecutionRequestKey: key,
			ConfigurationSnapshotRef: configJSON, ContextSnapshotRef: contextJSON,
			ExecutionRequestDigest: digest(json.RawMessage(canonical)), ExecutionRequest: canonical, CreatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		result, err = task(row)
		return err
	})
	if err != nil {
		for _, known := range []error{aiworkbench.ErrInvalid, aiworkbench.ErrNotFound, aiworkbench.ErrIdempotencyConflict,
			aiworkbench.ErrConversationArchived, aiworkbench.ErrRevisionMismatch} {
			if errors.Is(err, known) {
				return aiworkbench.BusinessTask{}, false, err
			}
		}
		return aiworkbench.BusinessTask{}, false, unavailable(err)
	}
	return result, replay, nil
}

func (s *Store) GetTask(ctx context.Context, scope aiworkbench.Scope, id string) (aiworkbench.BusinessTask, error) {
	if s == nil || s.db == nil || ctx == nil || !validScope(scope) || !validKey(id) {
		return aiworkbench.BusinessTask{}, aiworkbench.ErrInvalid
	}
	var row taskRow
	if err := s.db.WithContext(ctx).Where("id = ? AND organization_id = ? AND owner_user_id = ?", id, scope.OrganizationID, scope.ActorID).
		Take(&row).Error; err != nil {
		return aiworkbench.BusinessTask{}, missing(err)
	}
	return task(row)
}

// LookupTask is the receipt-first read for a confirm replay. The key is
// scoped to the original organization and actor before any mutable Product,
// AgentConfig, Knowledge or provider preflight is consulted.
func (s *Store) LookupTask(ctx context.Context, scope aiworkbench.Scope, conversationID, proposalID, key string) (aiworkbench.BusinessTask, bool, error) {
	if s == nil || s.db == nil || ctx == nil || !validScope(scope) ||
		!validKey(conversationID) || !validKey(proposalID) || !validKey(key) {
		return aiworkbench.BusinessTask{}, false, aiworkbench.ErrInvalid
	}
	result, found, err := s.lookupTaskKey(s.db.WithContext(ctx), scope, key, confirmFingerprint(conversationID, proposalID, key))
	if err != nil {
		if errors.Is(err, aiworkbench.ErrIdempotencyConflict) {
			return aiworkbench.BusinessTask{}, false, err
		}
		return aiworkbench.BusinessTask{}, false, unavailable(err)
	}
	return result, found, nil
}

// ReplayOrFail serializes an external-owner preflight rejection with a
// competing T1. Only authoritative absence under the same scoped key lock
// permits the original preflight error to be returned.
func (s *Store) ReplayOrFail(ctx context.Context, scope aiworkbench.Scope, conversationID, proposalID, key string, preflightErr error) (aiworkbench.BusinessTask, bool, error) {
	if s == nil || s.db == nil || ctx == nil || preflightErr == nil || !validScope(scope) ||
		!validKey(conversationID) || !validKey(proposalID) || !validKey(key) {
		return aiworkbench.BusinessTask{}, false, aiworkbench.ErrInvalid
	}
	fingerprint := confirmFingerprint(conversationID, proposalID, key)
	var result aiworkbench.BusinessTask
	var found bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockTaskKey(tx, scope, key); err != nil {
			return err
		}
		var lookupErr error
		result, found, lookupErr = s.lookupTaskKey(tx, scope, key, fingerprint)
		return lookupErr
	})
	if err != nil {
		if errors.Is(err, aiworkbench.ErrIdempotencyConflict) {
			return aiworkbench.BusinessTask{}, false, err
		}
		return aiworkbench.BusinessTask{}, false, unavailable(err)
	}
	if found {
		return result, true, nil
	}
	return aiworkbench.BusinessTask{}, false, preflightErr
}
