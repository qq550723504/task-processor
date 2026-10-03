package aiworkbenchpersistence

import (
	"context"
	"strconv"

	"task-processor/internal/aiworkbench"
)

// Cursors name a scoped row rather than accepting caller-supplied timestamps.
// Filters are applied before the page boundary. A returned cursor belongs to
// the same active/saved view and orders by last activity, then ID.
func (s *Store) ListConversations(ctx context.Context, scope aiworkbench.Scope, after string, limit int, savedOnly bool) ([]aiworkbench.Conversation, string, error) {
	if s == nil || s.db == nil || ctx == nil || !validScope(scope) || limit < 1 || limit > 50 || (after != "" && !validKey(after)) {
		return nil, "", aiworkbench.ErrInvalid
	}
	query := s.db.WithContext(ctx).Where("organization_id = ? AND owner_user_id = ? AND lifecycle = ?", scope.OrganizationID, scope.ActorID, "ACTIVE")
	if savedOnly {
		query = query.Where("favorite = ?", true)
	}
	if after != "" {
		cursor, err := s.lookupConversation(s.db.WithContext(ctx), scope, after, false)
		if err != nil {
			return nil, "", err
		}
		if cursor.Lifecycle != "ACTIVE" || savedOnly && !cursor.Favorite {
			return nil, "", aiworkbench.ErrNotFound
		}
		query = query.Where("(updated_at < ? OR (updated_at = ? AND id < ?))", cursor.UpdatedAt, cursor.UpdatedAt, cursor.ID)
	}
	var rows []conversationRow
	if err := query.Order("updated_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", unavailable(err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].ID
	}
	result := make([]aiworkbench.Conversation, 0, len(rows))
	for _, row := range rows {
		result = append(result, conversation(row))
	}
	return result, next, nil
}

func (s *Store) ListMessagesPage(ctx context.Context, scope aiworkbench.Scope, conversationID string, before uint64, limit int) ([]aiworkbench.Message, string, error) {
	if s == nil || s.db == nil || ctx == nil || !validScope(scope) || !validKey(conversationID) || limit < 1 || limit > 50 {
		return nil, "", aiworkbench.ErrInvalid
	}
	if _, err := s.Get(ctx, scope, conversationID); err != nil {
		return nil, "", err
	}
	query := s.db.WithContext(ctx).Where("organization_id = ? AND owner_user_id = ? AND conversation_id = ?", scope.OrganizationID, scope.ActorID, conversationID)
	if before != 0 {
		query = query.Where("sequence < ?", before)
	}
	var rows []messageRow
	if err := query.Order("sequence DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", unavailable(err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = strconv.FormatUint(rows[len(rows)-1].Sequence, 10)
	}
	result := make([]aiworkbench.Message, len(rows))
	for i, row := range rows {
		result[len(rows)-i-1] = aiworkbench.Message{ID: row.ID, ConversationID: row.ConversationID, Sequence: row.Sequence,
			Author: aiworkbench.MessageAuthor(row.AuthorKind), Content: row.Content, CreatedAt: row.CreatedAt.UTC()}
	}
	return result, next, nil
}

func (s *Store) ListTasks(ctx context.Context, scope aiworkbench.Scope, after string, limit int) ([]aiworkbench.BusinessTask, string, error) {
	if s == nil || s.db == nil || ctx == nil || !validScope(scope) || limit < 1 || limit > 50 || (after != "" && !validKey(after)) {
		return nil, "", aiworkbench.ErrInvalid
	}
	query := s.db.WithContext(ctx).Where("organization_id = ? AND owner_user_id = ?", scope.OrganizationID, scope.ActorID)
	if after != "" {
		var cursor taskRow
		if err := s.db.WithContext(ctx).Where("id = ? AND organization_id = ? AND owner_user_id = ?", after, scope.OrganizationID, scope.ActorID).Take(&cursor).Error; err != nil {
			return nil, "", missing(err)
		}
		query = query.Where("(created_at < ? OR (created_at = ? AND id < ?))", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	var rows []taskRow
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", unavailable(err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].ID
	}
	result := make([]aiworkbench.BusinessTask, 0, len(rows))
	for _, row := range rows {
		item, err := task(row)
		if err != nil {
			return nil, "", err
		}
		result = append(result, item)
	}
	return result, next, nil
}

func (s *Store) ListProposals(ctx context.Context, scope aiworkbench.Scope, conversationID string, limit int) ([]aiworkbench.ExecutionProposal, error) {
	if s == nil || s.db == nil || ctx == nil || !validScope(scope) || !validKey(conversationID) || limit < 1 || limit > 50 {
		return nil, aiworkbench.ErrInvalid
	}
	if _, err := s.Get(ctx, scope, conversationID); err != nil {
		return nil, err
	}
	var rows []proposalRow
	if err := s.db.WithContext(ctx).Where("organization_id = ? AND owner_user_id = ? AND conversation_id = ?",
		scope.OrganizationID, scope.ActorID, conversationID).Order("source_sequence DESC, id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, unavailable(err)
	}
	result := make([]aiworkbench.ExecutionProposal, 0, len(rows))
	for _, row := range rows {
		if row.Digest != proposalDigest(row) {
			return nil, aiworkbench.ErrUnavailable
		}
		result = append(result, proposal(row))
	}
	return result, nil
}
