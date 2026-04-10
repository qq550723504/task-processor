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
	})

	assert.Len(t, assignments, 2)
	assert.Contains(t, coordinator.cfg.CandidateNodes, assignments[181])
	assert.Contains(t, coordinator.cfg.CandidateNodes, assignments[867])
}

func TestParseStoreOwnerKey(t *testing.T) {
	tenantID, storeID, ok := parseStoreOwnerKey("listing:queue:owner:227:181")
	assert.True(t, ok)
	assert.Equal(t, int64(227), tenantID)
	assert.Equal(t, int64(181), storeID)
}
