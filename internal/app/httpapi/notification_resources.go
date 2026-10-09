package httpapi

import (
	"context"
	"errors"
	"strconv"
	"task-processor/internal/authz"
	"task-processor/internal/ledger/orgresource"
	n "task-processor/internal/notificationcenter"
	"task-processor/internal/product/sourcing"
	"time"
)

type acquisitionNoticeFacts interface {
	NotificationFacts(context.Context, string, int) ([]sourcing.AcquisitionNoticeFact, string, error)
}

func acquisitionNotificationSource(reader acquisitionNoticeFacts) n.Source {
	return n.NewReaderSource("acquisition", false, func(ctx context.Context, scope n.Scope, after string, limit int) (n.SourcePage, error) {
		rows, next, e := reader.NotificationFacts(ctx, after, limit)
		if errors.Is(e, sourcing.ErrPublicationForbidden) {
			return n.SourcePage{}, n.ErrForbidden
		}
		if e != nil {
			return n.SourcePage{}, n.ErrUnavailable
		}
		page := n.SourcePage{Items: []n.Item{}, Next: next}
		for _, row := range rows {
			op := row.Operation
			if op.Scope.OrganizationID != scope.OrganizationID || op.Scope.ActorID != scope.Subject {
				return n.SourcePage{}, n.ErrForbidden
			}
			label, kind, attention := "", "", true
			publication := ""
			switch op.State {
			case sourcing.AcquisitionPublished:
				if row.Publication == nil {
					return n.SourcePage{}, n.ErrUnavailable
				}
				label, kind, attention = "1688 商品已保存", "published", false
				publication = row.Publication.Receipt.PublicationID
			case sourcing.AcquisitionFailed:
				label, kind = "1688 获取受阻", "failed"
			case sourcing.AcquisitionPrepared, sourcing.AcquisitionPublishing, sourcing.AcquisitionAcquiring:
				if op.LeaseUntil.After(time.Now()) {
					continue
				}
				label, kind = "1688 获取结果待核实", "unknown"
			}
			if label == "" {
				continue
			}
			item := notificationItem("acquisition", op.ID, kind, label, "打开原获取结果页查看事实；待核实结果需按原流程处理。", scope.OrganizationID, n.Digest([]any{op.State, op.FailureCode, op.CommandHash, publication}), attention, n.Target{Kind: "acquisition", ID: op.ID})
			if row.Publication != nil {
				at := row.Publication.Receipt.PublishedAt
				item.OccurredAt = &at
			}
			page.Items = append(page.Items, item)
		}
		return page, nil
	})
}
func (m commercialResourcesModule) notificationSource(auth *authz.ListingKitAuthorizer) n.Source {
	return n.NewReaderSource("org-resource", false, func(ctx context.Context, scope n.Scope, after string, limit int) (n.SourcePage, error) {
		if _, e := notificationIdentity(ctx, scope, authz.PermissionWorkbenchCommercialRead, auth); e != nil {
			return n.SourcePage{}, e
		}
		page := n.SourcePage{Items: []n.Item{}}
		if after != "" {
			return page, nil
		}
		balance, e := m.reader.ReadBalances(ctx, scope.OrganizationID)
		if e != nil || balance.OrganizationID != scope.OrganizationID {
			return page, n.ErrUnavailable
		}
		for _, b := range balance.Resources {
			if b.State == "not_recorded" {
				continue
			}
			if b.State != "recorded" || b.Available == nil || b.Debt == nil {
				return page, n.ErrUnavailable
			}
			if *b.Available != "0" && *b.Debt == "0" {
				continue
			}
			events, e := m.events.ListEvents(ctx, orgresource.EventQuery{OrganizationID: scope.OrganizationID, ResourceType: b.ResourceType, Limit: 1})
			if e != nil {
				return page, n.ErrUnavailable
			}
			if len(events.Items) == 0 {
				if *b.Debt != "0" {
					return page, n.ErrUnavailable
				}
				continue
			}
			event := events.Items[0]
			if event.AvailableAfter != *b.Available {
				return page, n.ErrUnavailable
			}
			label := "企业资源已用尽"
			if *b.Debt != "0" {
				label = "企业资源存在待偿还用量"
			}
			item := notificationItem("org-resource", n.Digest([]string{scope.OrganizationID, string(b.ResourceType)}), string(b.ResourceType), label, "打开企业资源页查看可用资源和使用记录。", scope.OrganizationID, n.Digest([]any{event.EventID, *b.Available, *b.Debt}), true, n.Target{Kind: "resources"})
			at := event.OccurredAt
			item.OccurredAt = &at
			page.Items = append(page.Items, item)
		}
		return page, nil
	})
}
func (m memberResourcesModule) notificationSource(auth *authz.ListingKitAuthorizer) n.Source {
	return n.NewReaderSource("member-resource", false, func(ctx context.Context, scope n.Scope, after string, limit int) (n.SourcePage, error) {
		i, e := notificationIdentity(ctx, scope, authz.PermissionWorkbenchOrganizationMemberRead, auth)
		if e != nil {
			return n.SourcePage{}, e
		}
		page := n.SourcePage{Items: []n.Item{}}
		if after != "" {
			return page, nil
		}
		if i.EffectiveMemberID == "" {
			return page, n.ErrForbidden
		}
		for _, resource := range []orgresource.ResourceType{orgresource.ResourceDataRow, orgresource.ResourceStoreRenewalPeriod} {
			p, e := m.service.ReadPosition(ctx, orgresource.Principal{Kind: orgresource.PrincipalTenantHuman, ID: i.UserID}, scope.OrganizationID, i.EffectiveMemberID, resource)
			if e != nil {
				return page, n.ErrUnavailable
			}
			if p.Free > 0 || p.Version == 0 {
				continue
			}
			item := notificationItem("member-resource", n.Digest([]string{p.MemberID, string(resource)}), string(resource), "本人分配资源已用尽", "打开企业资源页查看自己的分配额度。", scope.OrganizationID, strconv.FormatInt(p.Version, 10), true, n.Target{Kind: "member-resources"})
			at := p.UpdatedAt
			if !at.IsZero() {
				item.OccurredAt = &at
			}
			page.Items = append(page.Items, item)
		}
		return page, nil
	})
}
func (m memberPointLimitModule) notificationSource(auth *authz.ListingKitAuthorizer) n.Source {
	return n.NewReaderSource("member-limit", false, func(ctx context.Context, scope n.Scope, after string, limit int) (n.SourcePage, error) {
		i, e := notificationIdentity(ctx, scope, authz.PermissionWorkbenchOrganizationMemberRead, auth)
		if e != nil {
			return n.SourcePage{}, e
		}
		page := n.SourcePage{Items: []n.Item{}}
		if after != "" {
			return page, nil
		}
		if i.EffectiveMemberID == "" {
			return page, n.ErrForbidden
		}
		p, e := m.service.ReadMonthlyLimit(ctx, orgresource.Principal{Kind: orgresource.PrincipalTenantHuman, ID: i.UserID}, scope.OrganizationID, i.EffectiveMemberID)
		if e != nil {
			return page, n.ErrUnavailable
		}
		if p.Consumed+p.Reserved < p.MonthlyLimit || p.Version == 0 && p.Consumed+p.Reserved == 0 {
			return page, nil
		}
		item := notificationItem("member-limit", p.MemberID, "monthly", "本人 AI 月度额度已用尽", "打开企业资源页查看当前月度额度。", scope.OrganizationID, n.Digest([]any{p.MonthStart, p.Version, p.Consumed, p.Reserved, p.MonthlyLimit}), true, n.Target{Kind: "member-resources"})
		page.Items = append(page.Items, item)
		return page, nil
	})
}
