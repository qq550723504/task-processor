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
	resources      *SharedResources
}

func newPlatformModuleRegistrar(cfg *config.Config, logger *logrus.Logger, serviceManager *ServiceManager, resources *SharedResources) platformModuleRegistrar {
	return platformModuleRegistrar{
		config:         cfg,
		logger:         logger,
		serviceManager: serviceManager,
		resources:      resources,
	}
}

func (r platformModuleRegistrar) register(ctx context.Context, module PlatformModule) error {
	r.logger.Infof("registering %s processor", strings.ToUpper(module.Name()))
	runtimeContext := BuildPlatformRuntimeContext(PlatformRuntimeContextInput{
		Config:         r.config,
		Logger:         r.logger,
		Resources:      r.resources,
		ServiceManager: r.serviceManager,
	})
	if err := module.RegisterConsumer(ctx, runtimeContext, r.serviceManager); err != nil {
		return err
	}
	r.logger.Infof("%s processor registered", strings.ToUpper(module.Name()))
	return nil
}
