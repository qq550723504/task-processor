package referral

import (
	"context"
	"github.com/google/uuid"
	e "task-processor/internal/referraleconomics"
)

func (r *Repository) ListNoticeAdjustments(ctx context.Context, subject, after string, limit int) ([]e.EarningsLedgerEntry, string, error) {
	if limit < 1 || limit > 100 || after != "" && uuid.Validate(after) != nil {
		return nil, "", e.ErrInvalid
	}
	q := r.db.WithContext(ctx).Where("referrer=? AND currency=? AND entry_type IN ?", subject, e.CurrencyCNY, []string{"REFUND_ADJUSTMENT", "CHARGEBACK_ADJUSTMENT"})
	if after != "" {
		var cursor earningLedgerEntry
		if err := r.db.WithContext(ctx).Where("referrer=? AND entry_id=?", subject, after).Take(&cursor).Error; err != nil {
			return nil, "", e.ErrUnavailable
		}
		q = q.Where("(occurred_at,entry_id)<(?,?)", cursor.OccurredAt, cursor.EntryID)
	}
	var rows []earningLedgerEntry
	if err := q.Order("occurred_at DESC,entry_id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", e.ErrUnavailable
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].EntryID
	}
	result := make([]e.EarningsLedgerEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, e.EarningsLedgerEntry{EntryID: row.EntryID, Referrer: row.Referrer, Currency: row.Currency, EntryType: row.EntryType, AmountMinor: row.AmountMinor, OccurredAt: row.OccurredAt})
	}
	return result, next, nil
}
func (r *Repository) ListNoticeWithdrawals(ctx context.Context, subject, after string, limit int) ([]e.Withdrawal, string, error) {
	if limit < 1 || limit > 100 || after != "" && uuid.Validate(after) != nil {
		return nil, "", e.ErrInvalid
	}
	q := r.db.WithContext(ctx).Where("referrer=?", subject)
	if after != "" {
		var cursor withdrawalRow
		if err := r.db.WithContext(ctx).Where("referrer=? AND id=?", subject, after).Take(&cursor).Error; err != nil {
			return nil, "", e.ErrUnavailable
		}
		q = q.Where("(created_at,id)<(?,?)", cursor.CreatedAt, cursor.ID)
	}
	var rows []withdrawalRow
	if err := q.Order("created_at DESC,id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", e.ErrUnavailable
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].ID
	}
	result := make([]e.Withdrawal, 0, len(rows))
	for _, row := range rows {
		result = append(result, withdrawalFromRow(row))
	}
	return result, next, nil
}
