package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ObservedUsagePosition struct {
	At  time.Time
	Key string
}
type ObservedUsage struct {
	InvocationID, MemberID, Key string
	Tokens                      int64
	At                          time.Time
}
type ObservedUsagePage struct {
	Items []ObservedUsage
	Next  *ObservedUsagePosition
}

// Namespace distinguishes the actual invocation owners when an application
// merges their independently read pages. It never changes a persisted fact.
func (r *GormInvocationRecorder) ListObservedUsage(ctx context.Context, org, namespace string, limit int, after *ObservedUsagePosition) (ObservedUsagePage, error) {
	if r == nil || r.db == nil || org == "" || strings.TrimSpace(org) != org || len(org) > 128 || (namespace != "image" && namespace != "product") || limit < 1 || limit > 50 {
		return ObservedUsagePage{}, fmt.Errorf("invalid invocation usage query")
	}
	query := r.db.WithContext(ctx).Model(&invocationRow{}).Where("tenant_id = ? AND usage_known = ? AND outcome IN ? AND total_tokens > 0 AND member_id <> '' AND finished_at IS NOT NULL", org, true, []string{"succeeded", "usage_observed_failed"})
	if after != nil {
		if after.At.IsZero() || len(after.Key) > 160 || (!strings.HasPrefix(after.Key, "image:") && !strings.HasPrefix(after.Key, "product:")) {
			return ObservedUsagePage{}, fmt.Errorf("invalid invocation usage position")
		}
		query = query.Where("finished_at < ? OR (finished_at = ? AND (? || invocation_id) < ?)", after.At, after.At, namespace+":", after.Key)
	}
	var rows []invocationRow
	if err := query.Order("finished_at DESC, invocation_id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return ObservedUsagePage{}, err
	}
	page := ObservedUsagePage{Items: make([]ObservedUsage, 0, len(rows))}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	for _, row := range rows {
		if row.PromptTokens < 0 || row.CompletionTokens < 0 || row.PromptTokens+row.CompletionTokens != row.TotalTokens || row.FinishedAt.IsZero() {
			return ObservedUsagePage{}, fmt.Errorf("invalid observed invocation usage")
		}
		page.Items = append(page.Items, ObservedUsage{InvocationID: row.InvocationID, MemberID: row.MemberID, Key: namespace + ":" + row.InvocationID, Tokens: int64(row.TotalTokens), At: row.FinishedAt.UTC()})
	}
	if more {
		last := page.Items[len(page.Items)-1]
		page.Next = &ObservedUsagePosition{At: last.At, Key: last.Key}
	}
	return page, nil
}
