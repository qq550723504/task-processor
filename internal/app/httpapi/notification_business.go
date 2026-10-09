package httpapi

import (
	"context"
	"errors"
	"task-processor/internal/authz"
	billing "task-processor/internal/commercial/billing"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	"task-processor/internal/knowledge"
	n "task-processor/internal/notificationcenter"
	"task-processor/internal/storecenter"
	verification "task-processor/internal/subjectverification"
	"time"
)

func storedNotificationSource(repo *storecenter.MemberScopedStoreRepository, auth *authz.ListingKitAuthorizer) n.Source {
	return n.NewReaderSource("store", false, func(ctx context.Context, scope n.Scope, after string, limit int) (n.SourcePage, error) {
		if _, e := notificationIdentity(ctx, scope, authz.PermissionWorkbenchStoreRead, auth); e != nil {
			return n.SourcePage{}, e
		}
		rows, next, e := repo.StoredNoticeFacts(ctx, scope.OrganizationID, after, min(limit, 50))
		if e != nil {
			return n.SourcePage{}, n.ErrUnavailable
		}
		page := n.SourcePage{Items: []n.Item{}, Next: next}
		for _, row := range rows {
			s, c := row.Store, row.Connection
			status := s.ServiceStatus
			if status == storecenter.ServiceStatusActive && s.ServiceExpiresAt != nil && !s.ServiceExpiresAt.After(time.Now()) {
				status = storecenter.ServiceStatusExpired
			}
			if status == storecenter.ServiceStatusExpired || status == storecenter.ServiceStatusSuspended {
				label := "店铺服务已到期"
				if status == storecenter.ServiceStatusSuspended {
					label = "店铺服务已暂停"
				}
				item := notificationItem("store", s.ID, "service", label, s.Name, scope.OrganizationID, n.Digest([]any{s.Version, status, s.ServiceExpiresAt}), true, n.Target{Kind: "store", ID: s.ID})
				if status == storecenter.ServiceStatusExpired {
					item.OccurredAt = s.ServiceExpiresAt
				} else {
					at := s.UpdatedAt
					item.OccurredAt = &at
				}
				page.Items = append(page.Items, item)
			}
			if c.AttemptID != "" && (c.Status == storecenter.ConnectionStatusExpired || c.Status == storecenter.ConnectionStatusDisconnected || c.State == "failed" || c.State == "unknown") {
				label := "店铺连接已断开"
				if c.Status == storecenter.ConnectionStatusExpired {
					label = "店铺连接已过期"
				}
				if c.State == "awaiting_consent" {
					label = "店铺连接等待授权"
				}
				if c.State == "exchange_dispatched" || c.State == "unknown" {
					label = "店铺连接结果待核实"
				}
				if c.State == "failed" {
					label = "店铺连接失败"
				}
				item := notificationItem("store", s.ID, "connection", label, s.Name, scope.OrganizationID, n.Digest([]any{c.AttemptID, c.State, c.Status, c.Version}), true, n.Target{Kind: "store", ID: s.ID})
				item.OccurredAt = c.ObservedAt
				page.Items = append(page.Items, item)
			}
		}
		return page, nil
	})
}
func billingNotificationSource(repo *commercialstore.Repository, auth *authz.ListingKitAuthorizer) n.Source {
	return n.NewReaderSource("billing", false, func(ctx context.Context, scope n.Scope, after string, limit int) (n.SourcePage, error) {
		if _, e := notificationIdentity(ctx, scope, authz.PermissionWorkbenchCommercialRead, auth); e != nil {
			return n.SourcePage{}, e
		}
		rows, e := repo.ListOrders(ctx, scope.OrganizationID, billing.OrderFilter{Cursor: after, Limit: limit})
		if e != nil {
			return n.SourcePage{}, n.ErrUnavailable
		}
		page := n.SourcePage{Items: []n.Item{}, Next: rows.NextCursor}
		for _, row := range rows.Items {
			state := string(row.Status)
			label := ""
			attention := false
			var occurred *time.Time
			attemptVersion := int64(0)
			if row.Kind == billing.OrderWalletTopUp {
				a, e := repo.ReadTopUpAttempt(ctx, scope.OrganizationID, row.OrderID)
				if e != nil && !errors.Is(e, billing.ErrNotFound) {
					return n.SourcePage{}, n.ErrUnavailable
				}
				if e == nil {
					state = string(a.Phase)
					attemptVersion = a.Version
				}
			}
			switch state {
			case "PENDING", "AWAITING_PAYMENT", "CREATED":
				label = "账单待支付"
				attention = true
			case "RECONCILIATION_REQUIRED":
				label = "付款结果待核对"
				attention = true
			case "PAID_PENDING_CREDIT":
				label = "付款已确认，等待入账"
				attention = true
			case "FULFILLED", "COMPLETED":
				label = "账单已完成"
			case "CANCELLED", "CLOSED_UNPAID":
				label = "账单已关闭"
			}
			if label == "" {
				continue
			}
			if attemptVersion == 0 {
				at := row.UpdatedAt
				occurred = &at
			}
			item := notificationItem("billing", row.OrderID, state, label, row.Description, scope.OrganizationID, n.Digest([]any{row.Version, attemptVersion, state}), attention, n.Target{Kind: "order", ID: row.OrderID})
			item.OccurredAt = occurred
			page.Items = append(page.Items, item)
		}
		return page, nil
	})
}
func knowledgeNotificationSource(service *knowledge.Service, auth *authz.ListingKitAuthorizer) n.Source {
	return n.NewReaderSource("knowledge", false, func(ctx context.Context, scope n.Scope, after string, limit int) (n.SourcePage, error) {
		if _, e := notificationIdentity(ctx, scope, authz.PermissionWorkbenchKnowledgeRead, auth); e != nil {
			return n.SourcePage{}, e
		}
		rows, next, e := service.NotificationFacts(ctx, knowledge.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.Subject}, after, limit)
		if e != nil {
			return n.SourcePage{}, n.ErrUnavailable
		}
		page := n.SourcePage{Items: []n.Item{}, Next: next}
		for _, row := range rows {
			r := row.LatestRevision
			if r == nil {
				continue
			}
			if r.State != knowledge.Available && r.State != knowledge.Partial && r.State != knowledge.Failed {
				continue
			}
			label := map[knowledge.ProcessingState]string{knowledge.Available: "知识文件处理完成", knowledge.Partial: "知识文件需要检查", knowledge.Failed: "知识文件处理失败"}[r.State]
			item := notificationItem("knowledge", row.ID, string(r.State), label, row.Name, scope.OrganizationID, n.Digest([]any{r.ID, r.State, r.Attempts}), r.State != knowledge.Available, n.Target{Kind: "knowledge", ID: row.BaseID})
			at := r.UpdatedAt
			item.OccurredAt = &at
			page.Items = append(page.Items, item)
		}
		return page, nil
	})
}
func verificationNotificationSource(name string, personal bool, read func(context.Context, n.Scope) (verification.NoticeFact, error), auth *authz.ListingKitAuthorizer) n.Source {
	return n.NewReaderSource(name, personal, func(ctx context.Context, scope n.Scope, after string, limit int) (n.SourcePage, error) {
		permission := ""
		if !personal {
			permission = authz.PermissionWorkbenchOrganizationMemberManage
		}
		if _, e := notificationIdentity(ctx, scope, permission, auth); e != nil {
			return n.SourcePage{}, e
		}
		if after != "" {
			return n.SourcePage{Items: []n.Item{}}, nil
		}
		r, e := read(ctx, scope)
		if e != nil {
			return n.SourcePage{}, n.ErrUnavailable
		}
		page := n.SourcePage{Items: []n.Item{}}
		if r.ID == "" {
			return page, nil
		}
		label := map[string]string{"PENDING": "认证待继续", "EXPIRED": "认证链接已到期", "OUTCOME_UNKNOWN": "认证结果待核实", "VERIFIED": "认证已完成", "REJECTED": "个人认证未通过"}[r.State]
		if label == "" {
			return n.SourcePage{}, n.ErrUnavailable
		}
		org, target := "", "personal-verification"
		if !personal {
			org, target = scope.OrganizationID, "organization-verification"
		}
		item := notificationItem(name, r.ID, r.State, label, "打开原认证页面查看当前结果。", org, n.Digest([]any{r.State, r.ExpiresAt, r.VerifiedAt}), r.State != "VERIFIED" && r.State != "REJECTED", n.Target{Kind: target})
		if r.State == "VERIFIED" {
			item.OccurredAt = &r.VerifiedAt
		} else if r.State == "EXPIRED" {
			item.OccurredAt = &r.ExpiresAt
		}
		page.Items = append(page.Items, item)
		return page, nil
	})
}
