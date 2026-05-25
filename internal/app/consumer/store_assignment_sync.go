package consumer

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const storeAssignmentSyncInterval = 15 * time.Second

type storeAssignmentSyncState struct {
	provider       StoreAssignmentProvider
	useStoreQueues bool
	ctx            context.Context
	nodeID         string
	currentStores  []int64
	started        bool
}

type storeAssignmentSyncCoordinator struct {
	service *RabbitMQService
}

func newStoreAssignmentSyncCoordinator(service *RabbitMQService) *storeAssignmentSyncCoordinator {
	return &storeAssignmentSyncCoordinator{service: service}
}

func (c *storeAssignmentSyncCoordinator) shouldRun(state storeAssignmentSyncState) bool {
	return state.useStoreQueues && state.provider != nil && strings.TrimSpace(state.nodeID) != "" && state.ctx != nil
}

func (c *storeAssignmentSyncCoordinator) snapshotState(includeCurrentStores bool) storeAssignmentSyncState {
	c.service.mutex.RLock()
	defer c.service.mutex.RUnlock()

	state := storeAssignmentSyncState{
		provider:       c.service.storeAssignmentProvider,
		useStoreQueues: c.service.useStoreQueues,
		ctx:            c.service.ctx,
		nodeID:         c.service.config.Node.NodeID,
		started:        c.service.started,
	}
	if includeCurrentStores {
		state.currentStores = append([]int64(nil), c.service.ownedStores...)
	}
	return state
}

func (c *storeAssignmentSyncCoordinator) start() {
	state := c.snapshotState(false)
	if !c.shouldRun(state) {
		return
	}

	c.service.wg.Add(1)
	go func() {
		defer c.service.wg.Done()

		ticker := time.NewTicker(storeAssignmentSyncInterval)
		defer ticker.Stop()

		c.sync(state.ctx)
		for {
			select {
			case <-state.ctx.Done():
				return
			case <-ticker.C:
				c.sync(state.ctx)
			}
		}
	}()
}

func (c *storeAssignmentSyncCoordinator) syncInitial(ctx context.Context) {
	state := c.snapshotState(false)
	state.ctx = ctx
	if !c.shouldRun(state) {
		return
	}
	c.sync(ctx)
}

func (c *storeAssignmentSyncCoordinator) sync(ctx context.Context) {
	state := c.snapshotState(true)
	state.ctx = ctx
	if !c.shouldRun(state) {
		return
	}

	ownedStores, err := state.provider.GetOwnedStores(ctx, state.nodeID)
	if err != nil {
		c.service.logger.WithError(err).WithField("node_id", state.nodeID).Warn("refresh dynamic store assignments failed")
		return
	}
	if slices.Equal(state.currentStores, ownedStores) {
		return
	}

	c.service.logger.WithFields(logrus.Fields{
		"node_id":    state.nodeID,
		"old_stores": state.currentStores,
		"new_stores": ownedStores,
	}).Info("dynamic store assignments changed, reloading consumers")

	if err := c.reloadOwnedStores(ctx, ownedStores, state.started); err != nil {
		c.service.logger.WithError(err).WithField("node_id", state.nodeID).Error("reload consumers for dynamic store assignments failed")
	}
}

func (c *storeAssignmentSyncCoordinator) reloadOwnedStores(ctx context.Context, ownedStores []int64, started bool) error {
	c.service.mutex.Lock()
	c.service.ownedStores = append([]int64(nil), ownedStores...)
	c.service.processorRegistry.UpdateComponents(
		c.service.resultReporter,
		c.service.storeAPI,
		append([]int64(nil), c.service.ownedStores...),
		&c.service.useStoreQueues,
		c.service.deduplicator,
	)
	platforms := c.service.getRegisteredPlatforms()
	c.service.mutex.Unlock()

	if c.service.usesDedicatedStoreQueues() && len(ownedStores) > 0 {
		for _, platform := range platforms {
			if err := c.service.initializer.InitializeStoreQueues(platform, ownedStores); err != nil {
				return fmt.Errorf("初始化动态店铺队列失败: %w", err)
			}
		}
	}
	c.service.preloadOwnedStoreConfigs(ownedStores)

	c.service.registerMessageHandlers()
	if !started {
		return nil
	}
	return c.service.consumer.Restart()
}
