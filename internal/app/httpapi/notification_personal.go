package httpapi

import (
	"context"
	"strconv"
	"task-processor/internal/authz"
	n "task-processor/internal/notificationcenter"
	economics "task-processor/internal/referraleconomics"
)

func referralNotificationSources(reader economics.NoticeReader, auth *authz.ListingKitAuthorizer) []n.Source {
	earnings := n.NewReaderSource("referral-earnings", true, func(ctx context.Context, s n.Scope, after string, limit int) (n.SourcePage, error) {
		if _, e := notificationIdentity(ctx, s, "", auth); e != nil {
			return n.SourcePage{}, e
		}
		rows, next, e := reader.ListNoticeAdjustments(ctx, s.Subject, after, limit)
		if e != nil {
			return n.SourcePage{}, n.ErrUnavailable
		}
		page := n.SourcePage{Items: []n.Item{}, Next: next}
		for _, row := range rows {
			if row.Referrer != s.Subject {
				return page, n.ErrForbidden
			}
			item := notificationItem("referral-earnings", row.EntryID, "adjustment", "推广收益发生调整", "打开推广收益页查看已记录的调整。", "", "1", false, n.Target{Kind: "earnings"})
			at := row.OccurredAt
			item.OccurredAt = &at
			page.Items = append(page.Items, item)
		}
		return page, nil
	})
	withdrawals := n.NewReaderSource("referral-withdrawal", true, func(ctx context.Context, s n.Scope, after string, limit int) (n.SourcePage, error) {
		if _, e := notificationIdentity(ctx, s, "", auth); e != nil {
			return n.SourcePage{}, e
		}
		rows, next, e := reader.ListNoticeWithdrawals(ctx, s.Subject, after, limit)
		if e != nil {
			return n.SourcePage{}, n.ErrUnavailable
		}
		page := n.SourcePage{Items: []n.Item{}, Next: next}
		for _, row := range rows {
			if row.Referrer != s.Subject {
				return page, n.ErrForbidden
			}
			label := map[economics.WithdrawalStatus]string{economics.WithdrawalRequested: "提现申请待审核", economics.WithdrawalApproved: "提现申请已批准", economics.WithdrawalPaid: "提现已支付", economics.WithdrawalRejected: "提现申请被拒绝", economics.WithdrawalCanceled: "提现申请已取消"}[row.Status]
			if label == "" {
				return page, n.ErrUnavailable
			}
			item := notificationItem("referral-withdrawal", row.ID, string(row.Status), label, "打开提现记录查看原申请结果。", "", strconv.FormatInt(row.Version, 10), false, n.Target{Kind: "withdrawals"})
			at := row.UpdatedAt
			item.OccurredAt = &at
			page.Items = append(page.Items, item)
		}
		return page, nil
	})
	return []n.Source{earnings, withdrawals}
}
