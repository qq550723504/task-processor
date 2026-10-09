package aiworkbenchpersistence

import (
	"context"
	"task-processor/internal/aiworkbench"
)

// PlanningNoticeReader is implemented on the original Workbench owner pool.
func (s *Store) ListPlanningNotices(ctx context.Context, scope aiworkbench.Scope, after string, limit int) ([]aiworkbench.PlanningNotice, string, error) {
	if !validScope(scope) || limit < 1 || limit > 100 || (after != "" && !validDigest(after)) {
		return nil, "", aiworkbench.ErrInvalid
	}
	q := s.db.WithContext(ctx).Where("organization_id=? AND actor_id=? AND operation=? AND planner_invocation_id IS NOT NULL", scope.OrganizationID, scope.ActorID, messageOperation)
	if after != "" {
		q = q.Where("planner_invocation_id>?", after)
	}
	var rows []commandRow
	if e := q.Order("planner_invocation_id ASC").Limit(limit + 1).Find(&rows).Error; e != nil {
		return nil, "", aiworkbench.ErrUnavailable
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = *rows[len(rows)-1].PlannerInvocationID
	}
	result := make([]aiworkbench.PlanningNotice, 0, len(rows))
	for _, row := range rows {
		c, e := s.Get(ctx, scope, row.ConversationID)
		if e != nil {
			return nil, "", aiworkbench.ErrUnavailable
		}
		confirmed := false
		if row.ProposalID != nil {
			var count int64
			if e = s.db.WithContext(ctx).Model(&taskRow{}).Where("organization_id=? AND owner_user_id=? AND proposal_id=?", scope.OrganizationID, scope.ActorID, *row.ProposalID).Count(&count).Error; e != nil {
				return nil, "", aiworkbench.ErrUnavailable
			}
			confirmed = count > 0
		}
		result = append(result, aiworkbench.PlanningNotice{Command: command(row), Confirmed: confirmed, Archived: c.Archived})
	}
	return result, next, nil
}
