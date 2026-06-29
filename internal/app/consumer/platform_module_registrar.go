package consumer

import (
	"context"
	"strings"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

type platformModuleRegistrar struct {
	config   *config.Config
	logger   *logrus.Logger
	services PlatformRegistrationServices
}

func newPlatformModuleRegistrar(cfg *config.Config, logger *logrus.Logger, services PlatformRegistrationServices) platformModuleRegistrar {
	return platformModuleRegistrar{
		config:   cfg,
		logger:   logger,
		services: services,
	}
}

func (r platformModuleRegistrar) register(ctx context.Context, module PlatformModule, resources PlatformRuntimeResources) error {
	r.logger.Infof("registering %s processor", strings.ToUpper(module.Name()))
	runtimeContext := BuildPlatformRuntimeContext(PlatformRuntimeContextInput{
		Config:    r.config,
		Logger:    r.logger,
		Resources: resources,
		Services:  r.services,
	})
	if err := module.RegisterConsumer(ctx, runtimeContext, r.services); err != nil {
		return err
	}
	r.logger.Infof("%s processor registered", strings.ToUpper(module.Name()))
	return nil
}
