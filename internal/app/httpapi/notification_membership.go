package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	n "task-processor/internal/notificationcenter"
	"task-processor/internal/organization/membership"
	flow "task-processor/internal/organization/membership/inviteflow"
	"time"
)

type notificationBearerContextKey struct{}

func notificationRequest(ctx context.Context) *http.Request {
	r, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost/", nil)
	bearer, _ := ctx.Value(notificationBearerContextKey{}).(string)
	r.Header.Set("Authorization", bearer)
	return r
}
func (m currentMembershipModule) notificationSources() []n.Source {
	var sources []n.Source
	for _, recipient := range []bool{false, true} {
		name := "invitation-admin"
		if recipient {
			name = "invitation-recipient"
		}
		name, recipient := name, recipient
		sources = append(sources, n.NewReaderSource(name, recipient, func(ctx context.Context, s n.Scope, after string, limit int) (n.SourcePage, error) {
			service, e := m.invitations(notificationRequest(ctx))
			if e != nil {
				return n.SourcePage{}, n.ErrUnavailable
			}
			rows, next, e := service.NotificationFacts(ctx, recipient, after, limit)
			if errors.Is(e, flow.ErrPermission) || errors.Is(e, membership.ErrPermission) {
				return n.SourcePage{}, n.ErrForbidden
			}
			if e != nil {
				return n.SourcePage{}, n.ErrUnavailable
			}
			page := n.SourcePage{Items: []n.Item{}, Next: next}
			for _, row := range rows {
				org, target := "", n.Target{Kind: "invitation", ID: row.ID}
				if !recipient {
					org, target = row.OrganizationID, n.Target{Kind: "members"}
					if org != s.OrganizationID {
						return page, n.ErrForbidden
					}
				}
				label := map[string]string{flow.Pending: "邀请待确认", flow.Accepting: "邀请结果待核对", flow.Accepted: "邀请已接受", flow.Declined: "邀请已拒绝", flow.Expired: "邀请已过期", flow.Cancelled: "邀请已取消"}[row.State]
				attention := row.State == flow.Pending || row.State == flow.Accepting
				var occurred *time.Time
				if !recipient && row.State == flow.Pending {
					if row.DeliveryState == "failed" {
						label = "邀请投递失败"
						attention = true
						occurred = row.DeliveryUpdatedAt
					} else if row.DeliveryState == "unknown" {
						label = "邀请投递结果待核对"
						attention = true
						occurred = row.DeliveryUpdatedAt
					}
				}
				if row.State == flow.Expired {
					at := row.ExpiresAt
					occurred = &at
				}
				if label == "" {
					return page, n.ErrUnavailable
				}
				item := notificationItem(name, row.ID, row.State, label, "打开原邀请页面查看当前状态。", org, n.Digest([]any{row.Revision, row.State, row.DeliveryState, row.DeliveryAttempt, row.ExpiresAt}), attention, target)
				item.OccurredAt = occurred
				page.Items = append(page.Items, item)
			}
			return page, nil
		}))
	}
	sources = append(sources, n.NewReaderSource("membership", false, func(ctx context.Context, s n.Scope, after string, limit int) (n.SourcePage, error) {
		service, e := m.commands(notificationRequest(ctx))
		if e != nil {
			return n.SourcePage{}, n.ErrUnavailable
		}
		commands, ok := service.(*membership.Commands)
		if !ok {
			return n.SourcePage{}, n.ErrUnavailable
		}
		rows, next, e := commands.NotificationFacts(ctx, after, limit)
		if errors.Is(e, membership.ErrPermission) || errors.Is(e, membership.ErrAuthentication) {
			return n.SourcePage{}, n.ErrForbidden
		}
		if e != nil {
			return n.SourcePage{}, n.ErrUnavailable
		}
		page := n.SourcePage{Items: []n.Item{}, Next: next}
		for _, row := range rows {
			if row.Scope.OrganizationID != s.OrganizationID || row.Scope.ActorID != s.Subject {
				return page, n.ErrForbidden
			}
			label := "成员操作待核对"
			attention := true
			switch row.Phase {
			case membership.PhaseCompleted:
				label = "成员操作已完成"
				attention = false
			case membership.PhaseRejected:
				label = "成员操作已拒绝"
				attention = false
			}
			item := notificationItem("membership", row.Key, string(row.Phase), label, "打开成员管理查看原操作结果。", s.OrganizationID, strconv.FormatInt(row.Revision, 10), attention, n.Target{Kind: "members"})
			if row.Acknowledgment != nil {
				if at, e := time.Parse(time.RFC3339Nano, row.Acknowledgment.At); e == nil {
					item.OccurredAt = &at
				}
			}
			page.Items = append(page.Items, item)
		}
		return page, nil
	}))
	return sources
}
