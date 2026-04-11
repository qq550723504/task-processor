package consumer

import (
	"testing"

	"task-processor/internal/infra/clients/management/api"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeCandidateNodes(t *testing.T) {
	assert.Equal(t,
		[]string{"node-a", "node-b"},
		normalizeCandidateNodes([]string{" node-b ", "node-a", "node-b", ""}),
	)
}

func TestPickCandidateNodeStable(t *testing.T) {
	nodes := []string{"node-a", "node-b", "node-c"}
	assert.Equal(t, pickCandidateNode(nodes, 227, 181), pickCandidateNode(nodes, 227, 181))
	assert.NotEmpty(t, pickCandidateNode(nodes, 227, 181))
}

func TestBuildAssignmentsUsesCandidateNodes(t *testing.T) {
	coordinator := &AutoShardCoordinator{}
	coordinator.cfg.CandidateNodes = []string{"node-a", "node-b"}

	assignments := coordinator.buildAssignments([]*api.StoreRespDTO{
		{TenantID: 227, ID: 181},
		{TenantID: 246, ID: 867},
	}, nil, nil)

	assert.Len(t, assignments, 2)
	assert.Contains(t, coordinator.cfg.CandidateNodes, assignments[181])
	assert.Contains(t, coordinator.cfg.CandidateNodes, assignments[867])
}

func TestBuildAssignmentsBalancesBacklogByNodeWeight(t *testing.T) {
	coordinator := &AutoShardCoordinator{}
	coordinator.cfg.CandidateNodes = []string{"heavy", "lite"}
	coordinator.cfg.NodeWeights = map[string]int{"heavy": 3, "lite": 1}

	assignments := coordinator.buildAssignments([]*api.StoreRespDTO{
		{TenantID: 1, ID: 1},
		{TenantID: 1, ID: 2},
		{TenantID: 1, ID: 3},
		{TenantID: 1, ID: 4},
	}, map[int64]int{
		1: 90,
		2: 10,
		3: 10,
		4: 10,
	}, map[int64]autoShardManagedKey{
		1: {ownerNode: "heavy"},
		2: {ownerNode: "lite"},
		3: {ownerNode: "lite"},
		4: {ownerNode: "lite"},
	})

	assert.Equal(t, "heavy", assignments[1])
	assert.Equal(t, "lite", assignments[2])
	assert.Equal(t, "lite", assignments[3])
	assert.Equal(t, "lite", assignments[4])
}

func TestParseStoreOwnerKey(t *testing.T) {
	tenantID, storeID, ok := parseStoreOwnerKey("listing:queue:owner:227:181")
	assert.True(t, ok)
	assert.Equal(t, int64(227), tenantID)
	assert.Equal(t, int64(181), storeID)
}
