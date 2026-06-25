package consumer

import (
	"context"
	"testing"

	api "task-processor/internal/listingadmin"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type autoShardStoreAPIStub struct {
	stores []*api.StoreRespDTO
}

func (s autoShardStoreAPIStub) GetStore(id int64) (*api.StoreRespDTO, error) { return nil, nil }
func (s autoShardStoreAPIStub) PageStores(req *api.StorePageReqDTO) (*api.PageResult[*api.StoreRespDTO], error) {
	return &api.PageResult[*api.StoreRespDTO]{
		List:     s.stores,
		Total:    int64(len(s.stores)),
		PageNo:   req.PageNo,
		PageSize: req.PageSize,
	}, nil
}
func (s autoShardStoreAPIStub) GetStoreCookie(id int64) (string, error) { return "", nil }
func (s autoShardStoreAPIStub) UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error) {
	return true, nil
}
func (s autoShardStoreAPIStub) UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error) {
	return true, nil
}
func (s autoShardStoreAPIStub) DeleteStoreCookie(id int64) (bool, error) { return true, nil }
func (s autoShardStoreAPIStub) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	return true, nil
}
func (s autoShardStoreAPIStub) GetStorePauseStatus(id int64) (bool, error) { return false, nil }
func (s autoShardStoreAPIStub) GetStorePauseStatusDetail(id int64) (*api.StorePauseStatusRespDTO, error) {
	return nil, nil
}

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

func TestBuildAssignmentsUsesActiveCandidateNodesByStoreCount(t *testing.T) {
	coordinator := &AutoShardCoordinator{}
	coordinator.cfg.CandidateNodes = []string{"node-a", "node-b", "node-c", "node-d"}
	coordinator.cfg.TargetStoresPerNode = 2

	assignments := coordinator.buildAssignments([]*api.StoreRespDTO{
		{TenantID: 1, ID: 1},
		{TenantID: 1, ID: 2},
		{TenantID: 1, ID: 3},
	}, nil, nil)

	assert.Len(t, assignments, 3)
	for _, owner := range assignments {
		assert.Contains(t, []string{"node-a", "node-b"}, owner)
	}
}

func TestActiveCandidateNodesDefaultsToAllCandidateNodes(t *testing.T) {
	coordinator := &AutoShardCoordinator{}
	coordinator.cfg.CandidateNodes = []string{"node-a", "node-b", "node-c", "node-d"}

	assert.Equal(t, coordinator.cfg.CandidateNodes, coordinator.activeCandidateNodes(3))
}

func TestListEligibleStoresSkipsDedicatedQueueStores(t *testing.T) {
	autoListingEnabled := true
	dedicatedQueueEnabled := true
	coordinator := &AutoShardCoordinator{
		storeAPI: autoShardStoreAPIStub{stores: []*api.StoreRespDTO{
			{TenantID: 322, ID: 976, Platform: "SHEIN", Status: autoShardStoreStatusEnabled, EnableAutoListing: &autoListingEnabled, DedicatedQueueEnabled: &dedicatedQueueEnabled},
			{TenantID: 322, ID: 977, Platform: "SHEIN", Status: autoShardStoreStatusEnabled, EnableAutoListing: &autoListingEnabled},
		}},
	}
	coordinator.cfg.Platform = "shein"
	coordinator.cfg.PageSize = 200

	stores, err := coordinator.listEligibleStores(context.Background())

	require.NoError(t, err)
	require.Len(t, stores, 1)
	assert.Equal(t, int64(977), stores[0].ID)
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
