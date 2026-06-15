package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
)

func BuildModule(input BuildModuleInput) (_ *Module, err error) {
	bundle, err := BuildService(input.ServiceInput)
	if err != nil {
		return nil, err
	}
	return buildModuleRuntime(input, bundle)
}

func BuildService(input BuildServiceInput) (_ *ServiceBundle, err error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	closers := &closerStack{}
	defer func() {
		if err == nil {
			return
		}
		_ = closers.Close()
	}()
	repositories, err := buildRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	if input.Logger != nil {
		input.Logger.WithField("component", "listingkit/httpapi").Info("listingkit repositories ready")
	}
	return buildServiceRuntime(input, repositories, closers)
}

func buildWithClosers[T any](builder func(*config.Config, *logrus.Logger) (T, []func() error, error), cfg *config.Config, logger *logrus.Logger, closers *closerStack) (T, error) {
	value, items, err := builder(cfg, logger)
	if err != nil {
		var zero T
		return zero, err
	}
	closers.Add(items...)
	return value, nil
}
