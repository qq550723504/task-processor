package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
	"task-processor/internal/infra/worker"
)

type appBootstrap struct {
	productHandler productRouteHandler
	imageHandler   imageRouteHandler
	server         *http.Server
	routes         []httproute.Descriptor
	pools          []worker.WorkerPool
	closers        []func() error
}
