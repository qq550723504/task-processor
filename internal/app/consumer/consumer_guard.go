package consumer

import (
	"context"
	"time"
)

type consumerGuardState struct {
	started          bool
	connected        bool
	consumerActive   bool
	consumersHealthy bool
	ctx              context.Context
}

type consumerGuardCoordinator struct {
	service *RabbitMQService
}

func newConsumerGuardCoordinator(service *RabbitMQService) *consumerGuardCoordinator {
	return &consumerGuardCoordinator{service: service}
}

func (c *consumerGuardCoordinator) snapshotState() consumerGuardState {
	c.service.mutex.RLock()
	defer c.service.mutex.RUnlock()

	return consumerGuardState{
		started:          c.service.started,
		connected:        c.service.connManager.IsConnected(),
		consumerActive:   c.service.consumerActive,
		consumersHealthy: c.service.consumer.HasHealthyRequiredConsumers(),
		ctx:              c.service.ctx,
	}
}

func (c *consumerGuardCoordinator) start() {
	state := c.snapshotState()
	if state.ctx == nil {
		return
	}

	c.service.wg.Add(1)
	go func() {
		defer c.service.wg.Done()

		ticker := time.NewTicker(consumerGuardInterval)
		defer ticker.Stop()

		c.reconcile()
		for {
			select {
			case <-state.ctx.Done():
				return
			case <-ticker.C:
				c.reconcile()
			}
		}
	}()
}

func (c *consumerGuardCoordinator) reconcile() {
	state := c.snapshotState()
	action := decideConsumerAction(state.started, state.connected, state.consumerActive, state.consumersHealthy)
	if action == consumerActionNone || state.ctx == nil {
		return
	}

	switch action {
	case consumerActionPause:
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.service.pauseConsumers(stopCtx, "rabbitmq disconnected"); err != nil {
			c.service.logger.WithError(err).Warn("暂停消费者失败")
		}
	case consumerActionResume:
		if err := c.service.resumeConsumers("rabbitmq reconnected or consumer previously paused"); err != nil {
			c.service.logger.WithError(err).Warn("恢复消费者失败")
		}
	case consumerActionRestart:
		if err := c.service.restartConsumers("required consumers unhealthy"); err != nil {
			c.service.logger.WithError(err).Warn("重启消费者失败")
		}
	}
}
