package bootstrap

import (
	"context"

	"task-processor/internal/core/config"
	sdsclient "task-processor/internal/sds/client"
	"task-processor/internal/sdslogin"
)

type StatusProvider interface {
	Status(ctx context.Context) (*sdslogin.Status, error)
}

type BuildResult struct {
	Handler        sdslogin.HTTPRouteHandler
	StatusProvider StatusProvider
}

func BuildHandler(cfg *config.Config) (*BuildResult, error) {
	result, err := sdslogin.BuildHandler(cfg)
	if err != nil {
		return nil, err
	}
	if result == nil || result.Service == nil {
		return nil, nil
	}

	sdsclient.ConfigureLocalLoginProvider(result.Service)

	return &BuildResult{
		Handler:        result.Handler,
		StatusProvider: result.Service,
	}, nil
}
