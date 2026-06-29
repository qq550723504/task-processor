package consumer

import (
	"fmt"

	"task-processor/internal/core/config"
	api "task-processor/internal/listingadmin"

	"github.com/sirupsen/logrus"
)

func EnableDynamicStoreAssignment(cfg *config.Config, logger *logrus.Logger, runtime StoreAssignmentRuntime) error {
	if cfg == nil || runtime == nil {
		return nil
	}
	if !cfg.RabbitMQ.Node.UseStoreQueues || cfg.Redis == nil {
		return nil
	}

	provider, err := NewRedisStoreAssignmentProvider(cfg.Redis, logger)
	if err != nil {
		return fmt.Errorf("create dynamic store assignment provider failed: %w", err)
	}
	runtime.SetStoreAssignmentProvider(provider)
	logger.Infof("dynamic store assignment provider enabled: nodeID=%s", cfg.RabbitMQ.Node.NodeID)
	return nil
}

func ConfigureStaticStoreGuard(
	cfg *config.Config,
	logger *logrus.Logger,
	runtime StaticStoreGuardRuntime,
	storeAPI api.StoreAPI,
) {
	if cfg == nil || logger == nil || runtime == nil {
		return
	}
	if storeAPI == nil {
		logger.Warn("store API unavailable; store dispatch guard is disabled")
		return
	}

	runtime.SetStoreComponents(storeAPI, cfg.RabbitMQ.Node.OwnedStores, nil)
	logger.Info("store dispatch guard initialized")
}
