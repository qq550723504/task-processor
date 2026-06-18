package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/infra/worker"
)

func BuildHandlers(logger *logrus.Logger, options Options) (productRouteHandler, imageRouteHandler, []worker.WorkerPool, []func() error, error) {
	bootstrap, err := buildBootstrap(logger, options)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return bootstrap.productHandler, bootstrap.imageHandler, bootstrap.pools, bootstrap.closers, nil
}
