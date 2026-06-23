package consumer

import (
	"context"
	"fmt"
	"hash/fnv"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/redisclient"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

const (
	storeQueueModeKeyPattern    = "listing:queue:mode:%d:%d"
	storeQueueOwnerKeyPattern   = "listing:queue:owner:%d:%d"
	autoShardDefaultLockKey     = "listing:queue:auto-shard:lock"
	autoShardStoreStatusEnabled = int16(0)
	storeQueueModeDedicated     = "store-dedicated"
	autoShardScanBatchSize      = int64(200)
)

type autoShardManagedKey struct {
	tenantID  int64
	storeID   int64
	ownerKey  string
	modeKey   string
	ownerNode string
}

type autoShardStoreLoad struct {
	store        *api.StoreRespDTO
	backlog      int
	currentOwner string
	stableOwner  string
}

type autoShardNodeLoad struct {
	nodeID     string
	weight     int
	load       int64
	storeCount int
}

// AutoShardCoordinator assigns dedicated-queue stores to candidate nodes and writes Redis ownership.
type AutoShardCoordinator struct {
	cfg         config.AutoShardConfig
	storeAPI    api.StoreAPI
	redis       *redisclient.Client
	logger      *logrus.Logger
	nodeID      string
	rabbitMQURL string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu          sync.RWMutex
	started     bool
	lastRunAt   time.Time
	lastError   string
	lastSummary map[string]any
}

func NewAutoShardCoordinator(
	cfg config.AutoShardConfig,
	storeAPI api.StoreAPI,
	redisCfg *config.RedisConfig,
	rabbitMQURL string,
	nodeID string,
	logger *logrus.Logger,
) (*AutoShardCoordinator, error) {
	if storeAPI == nil {
		return nil, fmt.Errorf("store API is nil")
	}
	if redisCfg == nil {
		return nil, fmt.Errorf("redis config is nil")
	}
	redisClient, err := redisclient.New(redisCfg)
	if err != nil {
		return nil, err
	}

	normalizedCfg := cfg
	if strings.TrimSpace(normalizedCfg.Platform) == "" {
		normalizedCfg.Platform = "shein"
	}
	if normalizedCfg.PageSize <= 0 {
		normalizedCfg.PageSize = 200
	}
	if normalizedCfg.Interval <= 0 {
		normalizedCfg.Interval = 30 * time.Second
	}
	if normalizedCfg.LockTTL <= 0 {
		normalizedCfg.LockTTL = 25 * time.Second
	}
	if strings.TrimSpace(normalizedCfg.LockKey) == "" {
		normalizedCfg.LockKey = autoShardDefaultLockKey
	}
	normalizedCfg.CandidateNodes = normalizeCandidateNodes(normalizedCfg.CandidateNodes)

	return &AutoShardCoordinator{
		cfg:         normalizedCfg,
		storeAPI:    storeAPI,
		redis:       redisClient,
		logger:      logger,
		nodeID:      strings.TrimSpace(nodeID),
		rabbitMQURL: strings.TrimSpace(rabbitMQURL),
		lastSummary: map[string]any{},
	}, nil
}

func (c *AutoShardCoordinator) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return nil
	}

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.started = true
	c.wg.Add(1)
	go c.loop()
	return nil
}

func (c *AutoShardCoordinator) Stop(ctx context.Context) error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	cancel := c.cancel
	c.started = false
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.wg.Wait()
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	if c.redis != nil {
		return c.redis.Close()
	}
	return nil
}

func (c *AutoShardCoordinator) GetStatus() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]any{
		"started":         c.started,
		"platform":        c.cfg.Platform,
		"interval":        c.cfg.Interval.String(),
		"candidate_nodes": append([]string(nil), c.cfg.CandidateNodes...),
		"last_run_at":     c.lastRunAt,
		"last_error":      c.lastError,
		"last_summary":    c.lastSummary,
	}
}

func (c *AutoShardCoordinator) loop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.cfg.Interval)
	defer ticker.Stop()

	c.runOnce()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.runOnce()
		}
	}
}

func (c *AutoShardCoordinator) runOnce() {
	ctx := c.ctx
	if ctx == nil {
		return
	}

	acquired, err := c.redis.SetNX(ctx, c.cfg.LockKey, c.lockValue(), c.cfg.LockTTL)
	if err != nil {
		c.recordResult(time.Now(), err, nil)
		c.logger.WithError(err).Warn("auto shard: acquire lock failed")
		return
	}
	if !acquired {
		return
	}

	summary, runErr := c.reconcile(ctx)
	c.recordResult(time.Now(), runErr, summary)
	if runErr != nil {
		c.logger.WithError(runErr).Warn("auto shard reconcile failed")
		return
	}
	c.logger.WithFields(logrus.Fields(summary)).Info("auto shard reconcile completed")
}

func (c *AutoShardCoordinator) reconcile(ctx context.Context) (map[string]any, error) {
	stores, err := c.listEligibleStores(ctx)
	if err != nil {
		return nil, err
	}
	currentManaged, err := c.loadCurrentManagedKeys(ctx)
	if err != nil {
		return nil, err
	}
	queueBacklogs := c.loadStoreQueueBacklogs(ctx, stores)
	assignments := c.buildAssignments(stores, queueBacklogs, currentManaged)
	activeCandidateNodes := c.activeCandidateNodes(len(stores))

	nodeMembers := make(map[string][]string, len(c.cfg.CandidateNodes))
	nodeBacklogs := make(map[string]int, len(c.cfg.CandidateNodes))
	queueBacklogTotal := 0
	for _, store := range stores {
		owner := assignments[store.ID]
		modeKey := fmt.Sprintf(storeQueueModeKeyPattern, store.TenantID, store.ID)
		ownerKey := fmt.Sprintf(storeQueueOwnerKeyPattern, store.TenantID, store.ID)
		if err := c.redis.Set(ctx, modeKey, storeQueueModeDedicated, 0); err != nil {
			return nil, err
		}
		if err := c.redis.Set(ctx, ownerKey, owner, 0); err != nil {
			return nil, err
		}
		nodeMembers[owner] = append(nodeMembers[owner], strconv.FormatInt(store.ID, 10))
		backlog := queueBacklogs[store.ID]
		nodeBacklogs[owner] += backlog
		queueBacklogTotal += backlog
		delete(currentManaged, store.ID)
	}

	for _, members := range nodeMembers {
		slices.Sort(members)
	}
	for _, nodeID := range c.cfg.CandidateNodes {
		members := nodeMembers[nodeID]
		if err := c.redis.ReplaceSet(ctx, fmt.Sprintf(storeQueueOwnerNodeKey, nodeID), members...); err != nil {
			return nil, err
		}
	}

	cleaned := 0
	for _, managed := range currentManaged {
		if err := c.redis.Delete(ctx, managed.ownerKey); err != nil {
			return nil, err
		}
		if err := c.redis.Delete(ctx, managed.modeKey); err != nil {
			return nil, err
		}
		cleaned++
	}

	summary := map[string]any{
		"managed_store_count":    len(stores),
		"candidate_nodes":        len(c.cfg.CandidateNodes),
		"active_candidate_nodes": len(activeCandidateNodes),
		"target_stores_per_node": c.cfg.TargetStoresPerNode,
		"cleaned_store_count":    cleaned,
		"queue_backlog_total":    queueBacklogTotal,
	}
	for nodeID, members := range nodeMembers {
		summary["node_"+nodeID] = len(members)
		summary["node_"+nodeID+"_backlog"] = nodeBacklogs[nodeID]
	}
	return summary, nil
}

func (c *AutoShardCoordinator) listEligibleStores(ctx context.Context) ([]*api.StoreRespDTO, error) {
	platform := strings.ToLower(strings.TrimSpace(c.cfg.Platform))
	pageNo := 1
	result := make([]*api.StoreRespDTO, 0)

	for {
		page, err := c.storeAPI.PageStores(&api.StorePageReqDTO{
			PageNo:   pageNo,
			PageSize: c.cfg.PageSize,
		})
		if err != nil {
			return nil, err
		}
		if page == nil || len(page.List) == 0 {
			break
		}
		for _, store := range page.List {
			if store == nil {
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(store.Platform), platform) {
				continue
			}
			if store.Status != autoShardStoreStatusEnabled {
				continue
			}
			if store.EnableAutoListing == nil || !*store.EnableAutoListing {
				continue
			}
			if store.DedicatedQueueEnabled != nil && *store.DedicatedQueueEnabled {
				continue
			}
			result = append(result, store)
		}
		if int64(pageNo*page.PageSize) >= page.Total {
			break
		}
		pageNo++
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	slices.SortFunc(result, func(a, b *api.StoreRespDTO) int {
		switch {
		case a.TenantID < b.TenantID:
			return -1
		case a.TenantID > b.TenantID:
			return 1
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	return result, nil
}

func (c *AutoShardCoordinator) buildAssignments(
	stores []*api.StoreRespDTO,
	queueBacklogs map[int64]int,
	currentManaged map[int64]autoShardManagedKey,
) map[int64]string {
	assignments := make(map[int64]string, len(stores))
	if len(c.cfg.CandidateNodes) == 0 {
		for _, store := range stores {
			assignments[store.ID] = ""
		}
		return assignments
	}

	activeCandidateNodes := c.activeCandidateNodes(len(stores))
	nodeLoads := make([]*autoShardNodeLoad, 0, len(activeCandidateNodes))
	for _, nodeID := range activeCandidateNodes {
		nodeLoads = append(nodeLoads, &autoShardNodeLoad{
			nodeID: nodeID,
			weight: normalizeNodeWeight(c.cfg.NodeWeights[nodeID]),
		})
	}

	storeLoads := make([]autoShardStoreLoad, 0, len(stores))
	for _, store := range stores {
		currentOwner := ""
		if managed, ok := currentManaged[store.ID]; ok {
			currentOwner = managed.ownerNode
		}
		storeLoads = append(storeLoads, autoShardStoreLoad{
			store:        store,
			backlog:      queueBacklogs[store.ID],
			currentOwner: currentOwner,
			stableOwner:  pickCandidateNode(activeCandidateNodes, store.TenantID, store.ID),
		})
	}
	slices.SortFunc(storeLoads, func(a, b autoShardStoreLoad) int {
		switch {
		case a.backlog > b.backlog:
			return -1
		case a.backlog < b.backlog:
			return 1
		case a.store.TenantID < b.store.TenantID:
			return -1
		case a.store.TenantID > b.store.TenantID:
			return 1
		case a.store.ID < b.store.ID:
			return -1
		case a.store.ID > b.store.ID:
			return 1
		default:
			return 0
		}
	})

	for _, storeLoad := range storeLoads {
		nodeLoad := pickLeastLoadedNode(nodeLoads, storeLoad.currentOwner, storeLoad.stableOwner)
		assignments[storeLoad.store.ID] = nodeLoad.nodeID
		nodeLoad.load += storeAssignmentCost(storeLoad.backlog)
		nodeLoad.storeCount++
	}
	return assignments
}

func (c *AutoShardCoordinator) activeCandidateNodes(storeCount int) []string {
	if len(c.cfg.CandidateNodes) == 0 {
		return nil
	}
	targetStoresPerNode := c.cfg.TargetStoresPerNode
	if targetStoresPerNode <= 0 {
		return append([]string(nil), c.cfg.CandidateNodes...)
	}
	if storeCount <= 0 {
		return nil
	}
	activeCount := (storeCount + targetStoresPerNode - 1) / targetStoresPerNode
	if activeCount < 1 {
		activeCount = 1
	}
	if activeCount > len(c.cfg.CandidateNodes) {
		activeCount = len(c.cfg.CandidateNodes)
	}
	return append([]string(nil), c.cfg.CandidateNodes[:activeCount]...)
}

func (c *AutoShardCoordinator) loadStoreQueueBacklogs(ctx context.Context, stores []*api.StoreRespDTO) map[int64]int {
	backlogs := make(map[int64]int, len(stores))
	rabbitMQURL := strings.TrimSpace(c.rabbitMQURL)
	if rabbitMQURL == "" || len(stores) == 0 {
		return backlogs
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := amqp.DialConfig(rabbitMQURL, amqp.Config{Dial: dialer.Dial})
	if err != nil {
		c.logger.WithError(err).Warn("auto shard: inspect queue backlog skipped because RabbitMQ dial failed")
		return backlogs
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			c.logger.WithError(closeErr).Debug("auto shard: close RabbitMQ inspection connection failed")
		}
	}()

	platform := strings.ToLower(strings.TrimSpace(c.cfg.Platform))
	if platform == "" {
		platform = "shein"
	}
	for _, store := range stores {
		select {
		case <-ctx.Done():
			return backlogs
		default:
		}

		backlog, inspectErr := inspectStoreQueueBacklog(conn, platform, store.ID)
		if inspectErr != nil {
			c.logger.WithError(inspectErr).WithField("store_id", store.ID).Debug("auto shard: inspect store queue backlog failed")
			continue
		}
		backlogs[store.ID] = backlog
	}
	return backlogs
}

func inspectStoreQueueBacklog(conn *amqp.Connection, platform string, storeID int64) (int, error) {
	ch, err := conn.Channel()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = ch.Close()
	}()

	queue, err := ch.QueueInspect(rabbitmq.GetStoreQueueName(platform, storeID))
	if err != nil {
		return 0, err
	}
	return queue.Messages, nil
}

func normalizeNodeWeight(weight int) int {
	if weight <= 0 {
		return 1
	}
	return weight
}

func storeAssignmentCost(backlog int) int64 {
	if backlog <= 0 {
		return 1
	}
	return int64(backlog)
}

func pickLeastLoadedNode(nodeLoads []*autoShardNodeLoad, currentOwner string, stableOwner string) *autoShardNodeLoad {
	best := nodeLoads[0]
	for _, candidate := range nodeLoads[1:] {
		if isBetterNodeLoad(candidate, best, currentOwner, stableOwner) {
			best = candidate
		}
	}
	return best
}

func isBetterNodeLoad(candidate, best *autoShardNodeLoad, currentOwner string, stableOwner string) bool {
	candidateScore := candidate.load * int64(best.weight)
	bestScore := best.load * int64(candidate.weight)
	if candidateScore != bestScore {
		return candidateScore < bestScore
	}
	if currentOwner != "" {
		switch {
		case candidate.nodeID == currentOwner && best.nodeID != currentOwner:
			return true
		case best.nodeID == currentOwner && candidate.nodeID != currentOwner:
			return false
		}
	}
	if stableOwner != "" {
		switch {
		case candidate.nodeID == stableOwner && best.nodeID != stableOwner:
			return true
		case best.nodeID == stableOwner && candidate.nodeID != stableOwner:
			return false
		}
	}
	if candidate.storeCount != best.storeCount {
		return candidate.storeCount < best.storeCount
	}
	return candidate.nodeID < best.nodeID
}

func (c *AutoShardCoordinator) loadCurrentManagedKeys(ctx context.Context) (map[int64]autoShardManagedKey, error) {
	currentManaged := make(map[int64]autoShardManagedKey)
	candidateSet := make(map[string]struct{}, len(c.cfg.CandidateNodes))
	for _, nodeID := range c.cfg.CandidateNodes {
		candidateSet[nodeID] = struct{}{}
	}

	var cursor uint64
	for {
		nextCursor, keys, err := c.redis.Scan(ctx, cursor, "listing:queue:owner:*", autoShardScanBatchSize)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			ownerNode, getErr := c.redis.Get(ctx, key)
			if getErr != nil {
				continue
			}
			if _, ok := candidateSet[strings.TrimSpace(ownerNode)]; !ok {
				continue
			}
			tenantID, storeID, ok := parseStoreOwnerKey(key)
			if !ok {
				continue
			}
			currentManaged[storeID] = autoShardManagedKey{
				tenantID:  tenantID,
				storeID:   storeID,
				ownerKey:  key,
				modeKey:   fmt.Sprintf(storeQueueModeKeyPattern, tenantID, storeID),
				ownerNode: strings.TrimSpace(ownerNode),
			}
		}
		if nextCursor == 0 {
			break
		}
		cursor = nextCursor
	}
	return currentManaged, nil
}

func parseStoreOwnerKey(key string) (int64, int64, bool) {
	parts := strings.Split(key, ":")
	if len(parts) != 5 {
		return 0, 0, false
	}
	tenantID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	storeID, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return tenantID, storeID, true
}

func pickCandidateNode(candidateNodes []string, tenantID, storeID int64) string {
	if len(candidateNodes) == 0 {
		return ""
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(fmt.Sprintf("%d:%d", tenantID, storeID)))
	index := int(h.Sum32() % uint32(len(candidateNodes)))
	return candidateNodes[index]
}

func normalizeCandidateNodes(nodes []string) []string {
	seen := make(map[string]struct{}, len(nodes))
	normalized := make([]string, 0, len(nodes))
	for _, node := range nodes {
		node = strings.TrimSpace(node)
		if node == "" {
			continue
		}
		if _, ok := seen[node]; ok {
			continue
		}
		seen[node] = struct{}{}
		normalized = append(normalized, node)
	}
	slices.Sort(normalized)
	return normalized
}

func (c *AutoShardCoordinator) lockValue() string {
	if c.nodeID != "" {
		return c.nodeID
	}
	return "auto-shard"
}

func (c *AutoShardCoordinator) recordResult(runAt time.Time, err error, summary map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastRunAt = runAt
	if err != nil {
		c.lastError = err.Error()
	} else {
		c.lastError = ""
	}
	if summary == nil {
		summary = map[string]any{}
	}
	c.lastSummary = summary
}
