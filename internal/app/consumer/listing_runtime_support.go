package consumer

import (
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

func EnableDynamicStoreAssignment(cfg *config.Config, logger *logrus.Logger, serviceManager *ServiceManager) error {
	if cfg == nil || serviceManager == nil {
		return nil
	}
	if !cfg.RabbitMQ.Node.UseStoreQueues || cfg.Redis == nil {
		return nil
	}

	provider, err := NewRedisStoreAssignmentProvider(cfg.Redis, logger)
	if err != nil {
		return fmt.Errorf("create dynamic store assignment provider failed: %w", err)
	}
	serviceManager.SetStoreAssignmentProvider(provider)
	logger.Infof("dynamic store assignment provider enabled: nodeID=%s", cfg.RabbitMQ.Node.NodeID)
	return nil
}

func ConfigureStaticStoreGuard(
	cfg *config.Config,
	logger *logrus.Logger,
	serviceManager *ServiceManager,
	storeAPI api.StoreAPI,
) {
	if cfg == nil || logger == nil || serviceManager == nil {
		return
	}
	if storeAPI == nil {
		logger.Warn("management client unavailable; store dispatch guard is disabled")
		return
	}

	serviceManager.SetStoreComponents(storeAPI, cfg.RabbitMQ.Node.OwnedStores, nil)
	logger.Info("store dispatch guard initialized")
}
