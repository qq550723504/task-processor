package httpapi

import (
	"context"
	"errors"
	"strconv"
	"task-processor/internal/aiworkbench"
	"task-processor/internal/authz"
	n "task-processor/internal/notificationcenter"
	"task-processor/internal/product/review"
)

func sourceError(e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, review.ErrForbidden) {
		return n.ErrForbidden
	}
	return n.ErrUnavailable
}
func notificationItem(source, id, kind, title, summary, org, revision string, attention bool, target n.Target) n.Item {
	return n.Item{Ref: n.Ref{Source: source, EntityID: id, Type: kind, Revision: revision, OrganizationID: org}, Category: n.Business, Type: kind, Title: title, Summary: summary, Paragraphs: []string{summary}, OrganizationID: org, Attention: attention, Target: target}
}
func (a *aiWorkbenchApplication) notificationSources() []n.Source {
	tasks := n.NewReaderSource("workbench-task", false, func(ctx context.Context, scope n.Scope, after string, limit int) (n.SourcePage, error) {
		i, e := a.freshWorkbenchIdentity(ctx, authz.PermissionWorkbenchTaskRead)
		if e != nil {
			return n.SourcePage{}, sourceError(e)
		}
		if i.TenantID != scope.OrganizationID || i.UserID != scope.Subject {
			return n.SourcePage{}, n.ErrForbidden
		}
		rows, next, e := a.store.ListTasks(ctx, aiworkbench.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.Subject}, after, min(limit, 50))
		if e != nil {
			return n.SourcePage{}, n.ErrUnavailable
		}
		page := n.SourcePage{Items: []n.Item{}, Next: next}
		for _, row := range rows {
			projection, e := a.taskProjectionReader().Read(ctx, row.Scope, row)
			if e != nil {
				return n.SourcePage{}, n.ErrUnavailable
			}
			if projection.State == aiworkbench.TaskRunning {
				continue
			}
			state := string(projection.State)
			label := map[aiworkbench.TaskState]string{aiworkbench.TaskWaitingConfirmation: "任务需要确认", aiworkbench.TaskCompleted: "任务已完成", aiworkbench.TaskError: "任务需要查看", aiworkbench.TaskPaused: "任务已暂停"}[projection.State]
			runID := ""
			if projection.Run != nil {
				runID = projection.Run.State.RunID
			}
			revision := n.Digest([]any{row.ExecutionRequestKey, runID, state, projection.Reason, projection.Review.State, projection.Review.Revision})
			item := notificationItem("workbench-task", row.ID, state, label, row.Title, scope.OrganizationID, revision, projection.State != aiworkbench.TaskCompleted, n.Target{Kind: "task", ID: row.ID})
			item.OccurredAt = projection.Review.OccurredAt
			if runID != "" && projection.Review.ID != "" {
				item.Association = scope.OrganizationID + ":" + runID
			}
			page.Items = append(page.Items, item)
		}
		return page, nil
	})
	plans := n.NewReaderSource("workbench-plan", false, func(ctx context.Context, scope n.Scope, after string, limit int) (n.SourcePage, error) {
		i, e := a.freshWorkbenchIdentity(ctx, authz.PermissionWorkbenchChatRead)
		if e != nil {
			return n.SourcePage{}, sourceError(e)
		}
		if i.TenantID != scope.OrganizationID || i.UserID != scope.Subject {
			return n.SourcePage{}, n.ErrForbidden
		}
		rows, next, e := a.store.ListPlanningNotices(ctx, aiworkbench.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.Subject}, after, limit)
		if e != nil {
			return n.SourcePage{}, n.ErrUnavailable
		}
		page := n.SourcePage{Items: []n.Item{}, Next: next}
		for _, row := range rows {
			c := row.Command
			if row.Archived || row.Confirmed {
				continue
			}
			label := ""
			switch c.State {
			case aiworkbench.PlanningUnknown:
				label = "规划结果待核实"
			case aiworkbench.PlanningInvalidOutput:
				label = "规划输出需要查看"
			case aiworkbench.PlanningFailedBeforeDispatch:
				label = "规划未能开始"
			case aiworkbench.PlanningComplete:
				if c.Mode == aiworkbench.PlanReady && c.ProposalID != "" {
					label = "执行方案待确认"
				}
			}
			if label == "" {
				continue
			}
			revision := n.Digest([]any{c.State, c.TerminalDigest, c.ProposalID, row.Confirmed, row.Archived})
			item := notificationItem("workbench-plan", c.PlannerInvocationID, string(c.State), label, "打开原对话查看结果，由本人确认后继续。", scope.OrganizationID, revision, true, n.Target{Kind: "chat", ID: c.ConversationID})
			item.OccurredAt = c.CommittedAt
			page.Items = append(page.Items, item)
		}
		return page, nil
	})
	return []n.Source{tasks, plans}
}
func reviewNotificationSource(service *review.Service) n.Source {
	return n.NewReaderSource("product-review", false, func(ctx context.Context, scope n.Scope, after string, limit int) (n.SourcePage, error) {
		rows, next, e := service.NotificationFacts(ctx, after, limit)
		if e != nil {
			return n.SourcePage{}, sourceError(e)
		}
		page := n.SourcePage{Items: []n.Item{}, Next: next}
		for _, row := range rows {
			if row.Org != scope.OrganizationID {
				return n.SourcePage{}, n.ErrUnavailable
			}
			label := map[string]string{"pending": "商品建议待审核", "accepted": "商品建议已接受，待应用", "rejected": "商品建议已拒绝", "applied": "商品建议已应用"}[row.State]
			item := notificationItem("product-review", row.ID, row.State, label, "打开商品审核查看当前结果。", scope.OrganizationID, strconv.FormatUint(row.Revision, 10), row.State == "pending" || row.State == "accepted", n.Target{Kind: "review", ID: row.ID})
			item.OccurredAt = row.TaskStateMetadata().OccurredAt
			if row.ContextProvenance != nil {
				item.Association = scope.OrganizationID + ":" + row.ContextProvenance.OriginAgentRunID
			}
			// The persisted review operation provides exact Agent run association.
			// No association is guessed from product, title or nearby timestamps.
			page.Items = append(page.Items, item)
		}
		return page, nil
	})
}
