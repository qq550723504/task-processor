package agentconfigpersistence

import (
	"context"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	runstore "task-processor/internal/integration/persistence/agent"
	"time"
)

type RecentRun struct {
	RunID          string      `json:"runId"`
	OperationID    string      `json:"operationId"`
	RequestKey     string      `json:"requestKey"`
	ProductKey     string      `json:"productKey"`
	TargetPlatform string      `json:"targetPlatform"`
	Phase          agent.Phase `json:"phase"`
	Revision       string      `json:"revision"`
	StartedAt      time.Time   `json:"startedAt"`
}

func (s *Store) Recent(ctx context.Context, scope agent.Scope, id, after string, size int, runs *runstore.Store, authorize func(agent.Binding) error) ([]RecentRun, string, error) {
	if !scopeOK(scope) || size < 1 || size > 20 || (after != "" && !agentconfig.UUID(after)) || runs == nil || authorize == nil {
		return nil, "", agentconfig.ErrInvalid
	}
	q := s.db.WithContext(ctx).Table("agent_configuration.start_snapshots AS s").Select("s.*").Joins("JOIN product_agent_runs r ON r.org=s.organization_id AND r.actor=s.actor_id AND r.context_kind=s.context_kind AND r.context_id=s.context_id AND r.request_key=s.request_key").Where("s.organization_id=? AND s.actor_id=? AND s.agent_id=?", scope.OrganizationID, scope.ActorID, id)
	if after != "" {
		var anchor snapshotRow
		if e := s.db.WithContext(ctx).Where("id=? AND organization_id=? AND actor_id=? AND agent_id=?", after, scope.OrganizationID, scope.ActorID, id).Take(&anchor).Error; e != nil {
			return nil, "", agentconfig.ErrInvalid
		}
		q = q.Where("(s.created_at,s.id) < (?,?)", anchor.CreatedAt, anchor.ID)
	}
	var rows []snapshotRow
	if e := q.Order("s.created_at DESC,s.id DESC").Limit(size + 1).Scan(&rows).Error; e != nil {
		return nil, "", e
	}
	next := ""
	if len(rows) > size {
		rows = rows[:size]
		next = rows[size-1].ID
	}
	out := []RecentRun{}
	for _, row := range rows {
		snap, e := snapshotView(row)
		if e != nil {
			return nil, "", e
		}
		if authorize(snap.Request.Binding) != nil {
			continue
		}
		r, found, e := runs.Lookup(ctx, scope, snap.Request.Binding, snap.Request.Key)
		if e != nil {
			return nil, "", e
		}
		if !found {
			continue
		}
		if r.State.Request.ConfigurationSnapshotRef.ID != snap.ID || r.State.Request.ConfigurationSnapshotRef.Digest != snap.Digest {
			return nil, "", agentconfig.ErrUnavailable
		}
		out = append(out, RecentRun{RunID: r.State.RunID, OperationID: snap.Request.Binding.ContextID, RequestKey: snap.Request.Key, ProductKey: snap.Request.Binding.ProductKey, TargetPlatform: snap.Request.Binding.TargetPlatform, Phase: r.State.Phase, Revision: decimal(int64(r.State.Revision)), StartedAt: r.State.StartedAt})
	}
	return out, next, nil
}
