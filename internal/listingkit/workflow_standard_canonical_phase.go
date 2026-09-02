package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/product/catalog"
)

const (
	productSnapshotStageKind         = "product_snapshot"
	productSnapshotNotReadyIssueCode = "product_snapshot_not_ready"
	productSnapshotNotReadyMessage   = "Product snapshot is not ready"
)

type standardWorkflowCanonicalPhase struct {
	snapshots ProductSnapshotReader
}

func buildStandardWorkflowCanonicalPhase(s *service) *standardWorkflowCanonicalPhase {
	return &standardWorkflowCanonicalPhase{snapshots: resolveWorkflowProductSnapshots(s)}
}

func (p standardWorkflowCanonicalPhase) run(ctx context.Context, query ProductSnapshotQuery) (catalog.ProductSnapshot, error) {
	query.TenantID = strings.TrimSpace(query.TenantID)
	query.ProductKey = strings.TrimSpace(query.ProductKey)
	if query.TenantID == "" || query.ProductKey == "" || p.snapshots == nil {
		return catalog.ProductSnapshot{}, ErrProductSnapshotNotReady
	}
	snapshot, err := p.snapshots.GetProductSnapshot(ctx, query)
	if err != nil {
		return catalog.ProductSnapshot{}, err
	}
	return cloneProductSnapshot(snapshot)
}

func cloneProductSnapshot(snapshot catalog.ProductSnapshot) (catalog.ProductSnapshot, error) {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return catalog.ProductSnapshot{}, fmt.Errorf("clone product snapshot: %w", err)
	}
	if string(raw) == "{}" {
		return catalog.ProductSnapshot{}, ErrProductSnapshotNotReady
	}
	var cloned catalog.ProductSnapshot
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return catalog.ProductSnapshot{}, fmt.Errorf("clone product snapshot: %w", err)
	}
	return cloned, nil
}

func productSnapshotQueryForTask(task *Task) ProductSnapshotQuery {
	if task == nil {
		return ProductSnapshotQuery{}
	}
	query := ProductSnapshotQuery{TenantID: task.TenantID}
	if task.Request != nil {
		query.ProductKey = task.Request.ProductKey
	}
	return query
}
