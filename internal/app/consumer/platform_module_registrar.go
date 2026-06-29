package consumer

import (
	"context"
	"strings"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

type platformModuleRegistrar struct {
	config         *config.Config
	logger         *logrus.Logger
	serviceManager *ServiceManager
}

func newPlatformModuleRegistrar(cfg *config.Config, logger *logrus.Logger, serviceManager *ServiceManager) platformModuleRegistrar {
	return platformModuleRegistrar{
		config:         cfg,
		logger:         logger,
		serviceManager: serviceManager,
	}
}

func (r platformModuleRegistrar) register(ctx context.Context, module PlatformModule, resources SharedResources) error {
	r.logger.Infof("registering %s processor", strings.ToUpper(module.Name()))
	runtimeContext := BuildPlatformRuntimeContext(PlatformRuntimeContextInput{
		Config:          r.config,
		Logger:          r.logger,
		Resources:       resources,
		RuntimeServices: r.serviceManager,
	})
	if err := module.RegisterConsumer(ctx, runtimeContext, r.serviceManager); err != nil {
		return err
	}
	r.logger.Infof("%s processor registered", strings.ToUpper(module.Name()))
	return nil
}
