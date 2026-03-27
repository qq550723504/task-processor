package main

import (
	"net/http"

	"github.com/sirupsen/logrus"

	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type appBootstrap struct {
	productHandler productenrich.ProductHandler
	imageHandler   productimage.Handler
	server         *http.Server
	pools          []worker.WorkerPool
	closers        []func() error
}

type productModule struct {
	handler productenrich.ProductHandler
	pool    worker.WorkerPool
}

type imageModule struct {
	handler productimage.Handler
	pool    worker.WorkerPool
}

func buildBootstrap(logger *logrus.Logger) (*appBootstrap, error) {
	deps, err := buildRuntimeDeps(logger)
	if err != nil {
		return nil, err
	}

	productModule, err := buildProductModule(logger, deps)
	if err != nil {
		return nil, err
	}

	imageModule, err := buildImageModule(logger, deps)
	if err != nil {
		return nil, err
	}

	server := buildHTTPServer(productModule.handler, imageModule.handler)
	return &appBootstrap{
		productHandler: productModule.handler,
		imageHandler:   imageModule.handler,
		server:         server,
		pools:          []worker.WorkerPool{productModule.pool, imageModule.pool},
		closers:        deps.closers,
	}, nil
}

func buildHandlers(logger *logrus.Logger) (productenrich.ProductHandler, productimage.Handler, []worker.WorkerPool, []func() error, error) {
	bootstrap, err := buildBootstrap(logger)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return bootstrap.productHandler, bootstrap.imageHandler, bootstrap.pools, bootstrap.closers, nil
}
