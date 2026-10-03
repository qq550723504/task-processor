package aiworkbenchpersistence

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/aiworkbench"
)

//go:embed schema.sql
var schemaFiles embed.FS

const (
	createOperation  = "conversation_create"
	messageOperation = "chat_message_plan"
)

type Store struct{ db *gorm.DB }

type conversationRow struct {
	ID               string
	OrganizationID   string
	OwnerUserID      string
	Title            string
	Favorite         bool
	Lifecycle        string
	MetadataRevision uint64
	NextSequence     uint64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (conversationRow) TableName() string { return "ai_workbench.conversations" }

type messageRow struct {
	ID             string
	OrganizationID string
	OwnerUserID    string
	ConversationID string
	Sequence       uint64
	AuthorKind     string
	Content        string
	CreatedAt      time.Time
}

func (messageRow) TableName() string { return "ai_workbench.messages" }

type commandRow struct {
	OrganizationID      string
	ActorID             string
	IdempotencyKey      string
	Operation           string
	RequestFingerprint  string
	ConversationID      string
	State               string
	UserMessageID       *string
	SourceSequence      *uint64
	PlannerInvocationID *string
	PlannerInputHash    *string
	PlannerModelProfile json.RawMessage `gorm:"type:jsonb"`
	PlannerStartedAt    *time.Time
	PlannerDeadline     *time.Time
	AssistantMessageID  *string
	TerminalDigest      *string
	Mode                *string
	CreatedAt           time.Time
	CommittedAt         *time.Time
}

func (commandRow) TableName() string { return "ai_workbench.commands" }

func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return aiworkbench.ErrUnavailable
	}
	sql, err := schemaFiles.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("%w: %v", aiworkbench.ErrUnavailable, err)
	}
	return db.Exec(string(sql)).Error
}

func New(db *gorm.DB) (*Store, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, aiworkbench.ErrUnavailable
	}
	return &Store{db: db}, nil
}

func validScope(scope aiworkbench.Scope) bool {
	return validName(scope.OrganizationID) && validName(scope.ActorID)
}
func validName(value string) bool {
	if value == "" || len(value) > 128 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func validKey(key string) bool {
	parsed, err := uuid.Parse(key)
	return err == nil && parsed.String() == key
}
func digest(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func unavailable(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", aiworkbench.ErrUnavailable, err)
}
func missing(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return aiworkbench.ErrNotFound
	}
	return unavailable(err)
}
func conversation(row conversationRow) aiworkbench.Conversation {
	return aiworkbench.Conversation{
		ID:    row.ID,
		Scope: aiworkbench.Scope{OrganizationID: row.OrganizationID, ActorID: row.OwnerUserID},
		Title: row.Title, Favorite: row.Favorite, Archived: row.Lifecycle == "ARCHIVED",
		MetadataRevision: row.MetadataRevision, NextSequence: row.NextSequence,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
func command(row commandRow) aiworkbench.PlanningCommand {
	result := aiworkbench.PlanningCommand{
		Scope:          aiworkbench.Scope{OrganizationID: row.OrganizationID, ActorID: row.ActorID},
		IdempotencyKey: row.IdempotencyKey, Operation: row.Operation,
		ConversationID: row.ConversationID, RequestFingerprint: row.RequestFingerprint,
		State: aiworkbench.PlanningState(row.State), ModelProfile: append([]byte(nil), row.PlannerModelProfile...),
		StartedAt: time.Time{}, CommittedAt: row.CommittedAt,
	}
	if row.UserMessageID != nil {
		result.UserMessageID = *row.UserMessageID
	}
	if row.SourceSequence != nil {
		result.SourceSequence = *row.SourceSequence
	}
	if row.PlannerInvocationID != nil {
		result.PlannerInvocationID = *row.PlannerInvocationID
	}
	if row.PlannerInputHash != nil {
		result.InputHash = *row.PlannerInputHash
	}
	if row.PlannerStartedAt != nil {
		result.StartedAt = *row.PlannerStartedAt
	}
	if row.PlannerDeadline != nil {
		result.Deadline = *row.PlannerDeadline
	}
	if row.AssistantMessageID != nil {
		result.AssistantMessageID = *row.AssistantMessageID
	}
	if row.TerminalDigest != nil {
		result.TerminalDigest = *row.TerminalDigest
	}
	if row.Mode != nil {
		result.Mode = aiworkbench.PlanMode(*row.Mode)
	}
	return result
}
func (s *Store) lockKey(tx *gorm.DB, scope aiworkbench.Scope, key string) error {
	token := "ai-workbench-command:" + scope.OrganizationID + "\x1f" + scope.ActorID + "\x1f" + key
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", token).Error
}
func (s *Store) lookupCommand(tx *gorm.DB, scope aiworkbench.Scope, key string) (commandRow, bool, error) {
	var row commandRow
	err := tx.Where("organization_id = ? AND actor_id = ? AND idempotency_key = ?", scope.OrganizationID, scope.ActorID, key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return commandRow{}, false, nil
	}
	if err != nil {
		return commandRow{}, false, err
	}
	return row, true, nil
}
func (s *Store) lookupConversation(tx *gorm.DB, scope aiworkbench.Scope, id string, lock bool) (conversationRow, error) {
	var row conversationRow
	query := tx.Where("id = ? AND organization_id = ? AND owner_user_id = ?", id, scope.OrganizationID, scope.ActorID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.Take(&row).Error
	if err != nil {
		return conversationRow{}, missing(err)
	}
	return row, nil
}

func (s *Store) Create(ctx context.Context, scope aiworkbench.Scope, key string, input aiworkbench.CreateInput) (aiworkbench.Conversation, bool, error) {
	if s == nil || s.db == nil {
		return aiworkbench.Conversation{}, false, aiworkbench.ErrUnavailable
	}
	if !validScope(scope) || !validKey(key) {
		return aiworkbench.Conversation{}, false, aiworkbench.ErrInvalid
	}
	fingerprint := digest(struct {
		Operation string
		Favorite  bool
	}{createOperation, input.Favorite})
	var result aiworkbench.Conversation
	var replay bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockKey(tx, scope, key); err != nil {
			return err
		}
		existing, found, err := s.lookupCommand(tx, scope, key)
		if err != nil {
			return err
		}
		if found {
			if existing.Operation != createOperation || existing.RequestFingerprint != fingerprint {
				return aiworkbench.ErrIdempotencyConflict
			}
			current, err := s.lookupConversation(tx, scope, existing.ConversationID, false)
			if err != nil {
				return err
			}
			result, replay = conversation(current), true
			return nil
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		row := conversationRow{ID: uuid.NewString(), OrganizationID: scope.OrganizationID, OwnerUserID: scope.ActorID,
			Favorite: input.Favorite, Lifecycle: "ACTIVE", MetadataRevision: 1, NextSequence: 1, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		receipt := commandRow{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, IdempotencyKey: key,
			Operation: createOperation, RequestFingerprint: fingerprint, ConversationID: row.ID,
			State: string(aiworkbench.PlanningComplete), CreatedAt: now, CommittedAt: &now}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		result = conversation(row)
		return nil
	})
	if err != nil {
		if errors.Is(err, aiworkbench.ErrIdempotencyConflict) || errors.Is(err, aiworkbench.ErrNotFound) {
			return aiworkbench.Conversation{}, false, err
		}
		return aiworkbench.Conversation{}, false, unavailable(err)
	}
	return result, replay, nil
}

func (s *Store) Get(ctx context.Context, scope aiworkbench.Scope, id string) (aiworkbench.Conversation, error) {
	if s == nil || s.db == nil {
		return aiworkbench.Conversation{}, aiworkbench.ErrUnavailable
	}
	if !validScope(scope) || !validKey(id) {
		return aiworkbench.Conversation{}, aiworkbench.ErrInvalid
	}
	row, err := s.lookupConversation(s.db.WithContext(ctx), scope, id, false)
	if err != nil {
		return aiworkbench.Conversation{}, err
	}
	return conversation(row), nil
}

func (s *Store) SetMetadata(ctx context.Context, scope aiworkbench.Scope, id string, expected uint64, change aiworkbench.MetadataChange) (aiworkbench.Conversation, error) {
	if s == nil || s.db == nil {
		return aiworkbench.Conversation{}, aiworkbench.ErrUnavailable
	}
	if !validScope(scope) || !validKey(id) || expected == 0 || (change.Title == nil && change.Favorite == nil && change.Archived == nil) {
		return aiworkbench.Conversation{}, aiworkbench.ErrInvalid
	}
	if change.Title != nil && (len(*change.Title) > 256 || !utf8.ValidString(*change.Title)) {
		return aiworkbench.Conversation{}, aiworkbench.ErrInvalid
	}
	var result aiworkbench.Conversation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := s.lookupConversation(tx, scope, id, true)
		if err != nil {
			return err
		}
		if row.MetadataRevision != expected {
			return aiworkbench.ErrRevisionMismatch
		}
		if change.Title != nil {
			row.Title = *change.Title
		}
		if change.Favorite != nil {
			row.Favorite = *change.Favorite
		}
		if change.Archived != nil {
			if *change.Archived {
				row.Lifecycle = "ARCHIVED"
			} else {
				row.Lifecycle = "ACTIVE"
			}
		}
		row.MetadataRevision++
		row.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
		if err := tx.Model(&conversationRow{}).Where("id = ? AND organization_id = ? AND owner_user_id = ?", id, scope.OrganizationID, scope.ActorID).
			Updates(map[string]any{"title": row.Title, "favorite": row.Favorite, "lifecycle": row.Lifecycle,
				"metadata_revision": row.MetadataRevision, "updated_at": row.UpdatedAt}).Error; err != nil {
			return err
		}
		result = conversation(row)
		return nil
	})
	if err != nil {
		if errors.Is(err, aiworkbench.ErrNotFound) || errors.Is(err, aiworkbench.ErrRevisionMismatch) {
			return aiworkbench.Conversation{}, err
		}
		return aiworkbench.Conversation{}, unavailable(err)
	}
	return result, nil
}

func validMessage(input aiworkbench.MessageInput) bool {
	if len(input.Content) == 0 || len(input.Content) > 8192 || !utf8.ValidString(input.Content) {
		return false
	}
	if !validName(input.OperationID) || len(input.TargetPlatform) == 0 || len(input.TargetPlatform) > 32 {
		return false
	}
	if input.TargetPlatform != "shein" && input.TargetPlatform != "temu" && input.TargetPlatform != "amazon" {
		return false
	}
	if len(input.TemplateID) > 128 || len(input.TemplateRevision) > 128 || len(input.KnowledgeBaseID) > 128 {
		return false
	}
	return true
}
func initialTitle(content string) string {
	value := strings.TrimSpace(content)
	var title strings.Builder
	for _, r := range value {
		if title.Len()+utf8.RuneLen(r) > 120 {
			break
		}
		if r == '\n' || r == '\r' {
			break
		}
		title.WriteRune(r)
	}
	return title.String()
}

func (s *Store) AppendUser(ctx context.Context, scope aiworkbench.Scope, conversationID, key string, input aiworkbench.MessageInput, prepared aiworkbench.PreparedPlan) (aiworkbench.PlanningCommand, bool, error) {
	if s == nil || s.db == nil {
		return aiworkbench.PlanningCommand{}, false, aiworkbench.ErrUnavailable
	}
	if !validScope(scope) || !validKey(conversationID) || !validKey(key) || !validMessage(input) {
		return aiworkbench.PlanningCommand{}, false, aiworkbench.ErrInvalid
	}
	// The fingerprint is wire identity. A stored receipt is adopted before a
	// new mutable route/profile is consulted.
	fingerprint := digest(struct {
		Operation      string
		ConversationID string
		Input          aiworkbench.MessageInput
	}{messageOperation, conversationID, input})
	var result aiworkbench.PlanningCommand
	var replay bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockKey(tx, scope, key); err != nil {
			return err
		}
		existing, found, err := s.lookupCommand(tx, scope, key)
		if err != nil {
			return err
		}
		if found {
			if existing.Operation != messageOperation || existing.ConversationID != conversationID || existing.RequestFingerprint != fingerprint {
				return aiworkbench.ErrIdempotencyConflict
			}
			result, replay = command(existing), true
			return nil
		}
		if !validDigest(prepared.InputHash) || !json.Valid(prepared.ModelProfile) || prepared.Deadline.IsZero() {
			return aiworkbench.ErrInvalid
		}
		row, err := s.lookupConversation(tx, scope, conversationID, true)
		if err != nil {
			return err
		}
		if row.Lifecycle != "ACTIVE" {
			return aiworkbench.ErrConversationArchived
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		messageID := uuid.NewString()
		message := messageRow{ID: messageID, OrganizationID: scope.OrganizationID, OwnerUserID: scope.ActorID,
			ConversationID: conversationID, Sequence: row.NextSequence, AuthorKind: string(aiworkbench.AuthorUser),
			Content: input.Content, CreatedAt: now}
		if err := tx.Create(&message).Error; err != nil {
			return err
		}
		if row.NextSequence == 1 && row.Title == "" {
			row.Title = initialTitle(input.Content)
		}
		sourceSequence := row.NextSequence
		row.NextSequence++
		row.UpdatedAt = now
		if err := tx.Model(&conversationRow{}).Where("id = ? AND organization_id = ? AND owner_user_id = ?", conversationID, scope.OrganizationID, scope.ActorID).
			Updates(map[string]any{"title": row.Title, "next_sequence": row.NextSequence, "updated_at": row.UpdatedAt}).Error; err != nil {
			return err
		}
		invocationID := digest(struct {
			OrganizationID string
			ActorID        string
			ConversationID string
			Key            string
			Fingerprint    string
		}{scope.OrganizationID, scope.ActorID, conversationID, key, fingerprint})
		profile := json.RawMessage(append([]byte(nil), prepared.ModelProfile...))
		receipt := commandRow{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, IdempotencyKey: key,
			Operation: messageOperation, RequestFingerprint: fingerprint, ConversationID: conversationID,
			State: string(aiworkbench.PlanningReadyToDispatch), UserMessageID: &messageID, SourceSequence: &sourceSequence,
			PlannerInvocationID: &invocationID, PlannerInputHash: &prepared.InputHash, PlannerModelProfile: profile,
			PlannerStartedAt: &now, PlannerDeadline: &prepared.Deadline, CreatedAt: now}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		result = command(receipt)
		return nil
	})
	if err != nil {
		if errors.Is(err, aiworkbench.ErrInvalid) || errors.Is(err, aiworkbench.ErrNotFound) ||
			errors.Is(err, aiworkbench.ErrConversationArchived) || errors.Is(err, aiworkbench.ErrIdempotencyConflict) {
			return aiworkbench.PlanningCommand{}, false, err
		}
		return aiworkbench.PlanningCommand{}, false, unavailable(err)
	}
	return result, replay, nil
}

func (s *Store) CompletePlan(ctx context.Context, scope aiworkbench.Scope, key string, terminal aiworkbench.PlanTerminal) (aiworkbench.PlanningCommand, bool, error) {
	if s == nil || s.db == nil {
		return aiworkbench.PlanningCommand{}, false, aiworkbench.ErrUnavailable
	}
	if ctx == nil || !validScope(scope) || !validKey(key) || len(terminal.AssistantText) == 0 || len(terminal.AssistantText) > 16384 ||
		!utf8.ValidString(terminal.AssistantText) || (terminal.Mode != aiworkbench.PlanClarify && terminal.Mode != aiworkbench.PlanReady) ||
		len(terminal.GoalSummary) > 1024 {
		return aiworkbench.PlanningCommand{}, false, aiworkbench.ErrInvalid
	}
	terminalDigest := digest(terminal)
	var result aiworkbench.PlanningCommand
	var replay bool
	// The provider may have completed after the HTTP request was canceled or
	// the conversation was archived. Persist that already-dispatched outcome
	// under a short cleanup deadline without granting any new dispatch.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	err := s.db.WithContext(writeCtx).Transaction(func(tx *gorm.DB) error {
		var receipt commandRow
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("organization_id = ? AND actor_id = ? AND idempotency_key = ?", scope.OrganizationID, scope.ActorID, key).
			Take(&receipt).Error
		if err != nil {
			return missing(err)
		}
		if receipt.Operation != messageOperation {
			return aiworkbench.ErrIdempotencyConflict
		}
		if receipt.State == string(aiworkbench.PlanningComplete) {
			if receipt.TerminalDigest == nil || *receipt.TerminalDigest != terminalDigest {
				return aiworkbench.ErrIdempotencyConflict
			}
			result, replay = command(receipt), true
			return nil
		}
		if receipt.State != string(aiworkbench.PlanningReadyToDispatch) {
			return aiworkbench.ErrIdempotencyConflict
		}
		row, err := s.lookupConversation(tx, scope, receipt.ConversationID, true)
		if err != nil {
			return err
		}
		// An in-flight command may terminalize after ARCHIVED. It gains no
		// permission to append another USER message or confirm a proposal.
		now := time.Now().UTC().Truncate(time.Microsecond)
		assistantID := uuid.NewString()
		assistant := messageRow{ID: assistantID, OrganizationID: scope.OrganizationID, OwnerUserID: scope.ActorID,
			ConversationID: receipt.ConversationID, Sequence: row.NextSequence,
			AuthorKind: string(aiworkbench.AuthorAssistant), Content: terminal.AssistantText, CreatedAt: now}
		if err := tx.Create(&assistant).Error; err != nil {
			return err
		}
		row.NextSequence++
		if err := tx.Model(&conversationRow{}).Where("id = ? AND organization_id = ? AND owner_user_id = ?", row.ID, scope.OrganizationID, scope.ActorID).
			Update("next_sequence", row.NextSequence).Error; err != nil {
			return err
		}
		mode := string(terminal.Mode)
		if err := tx.Model(&commandRow{}).Where("organization_id = ? AND actor_id = ? AND idempotency_key = ?", scope.OrganizationID, scope.ActorID, key).
			Updates(map[string]any{"state": string(aiworkbench.PlanningComplete), "assistant_message_id": assistantID,
				"terminal_digest": terminalDigest, "mode": mode, "committed_at": now}).Error; err != nil {
			return err
		}
		receipt.State = string(aiworkbench.PlanningComplete)
		receipt.AssistantMessageID = &assistantID
		receipt.TerminalDigest = &terminalDigest
		receipt.Mode = &mode
		receipt.CommittedAt = &now
		result = command(receipt)
		return nil
	})
	if err != nil {
		if errors.Is(err, aiworkbench.ErrNotFound) || errors.Is(err, aiworkbench.ErrIdempotencyConflict) {
			return aiworkbench.PlanningCommand{}, false, err
		}
		return aiworkbench.PlanningCommand{}, false, unavailable(err)
	}
	return result, replay, nil
}

func (s *Store) GetCommand(ctx context.Context, scope aiworkbench.Scope, key string) (aiworkbench.PlanningCommand, error) {
	if s == nil || s.db == nil {
		return aiworkbench.PlanningCommand{}, aiworkbench.ErrUnavailable
	}
	if !validScope(scope) || !validKey(key) {
		return aiworkbench.PlanningCommand{}, aiworkbench.ErrInvalid
	}
	row, found, err := s.lookupCommand(s.db.WithContext(ctx), scope, key)
	if err != nil {
		return aiworkbench.PlanningCommand{}, unavailable(err)
	}
	if !found || row.Operation != messageOperation {
		return aiworkbench.PlanningCommand{}, aiworkbench.ErrNotFound
	}
	return command(row), nil
}

func (s *Store) ListMessages(ctx context.Context, scope aiworkbench.Scope, conversationID string, limit int) ([]aiworkbench.Message, error) {
	if s == nil || s.db == nil {
		return nil, aiworkbench.ErrUnavailable
	}
	if !validScope(scope) || !validKey(conversationID) || limit < 1 || limit > 50 {
		return nil, aiworkbench.ErrInvalid
	}
	if _, err := s.Get(ctx, scope, conversationID); err != nil {
		return nil, err
	}
	var rows []messageRow
	err := s.db.WithContext(ctx).Where("organization_id = ? AND owner_user_id = ? AND conversation_id = ?", scope.OrganizationID, scope.ActorID, conversationID).
		Order("sequence ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, unavailable(err)
	}
	result := make([]aiworkbench.Message, 0, len(rows))
	for _, row := range rows {
		result = append(result, aiworkbench.Message{ID: row.ID, ConversationID: row.ConversationID,
			Sequence: row.Sequence, Author: aiworkbench.MessageAuthor(row.AuthorKind), Content: row.Content, CreatedAt: row.CreatedAt})
	}
	return result, nil
}
