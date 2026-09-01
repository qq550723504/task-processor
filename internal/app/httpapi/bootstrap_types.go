package httpapi

import (
	"net/http"

	"task-processor/internal/httproute"
	worker "task-processor/internal/platform/workerpool"
	"task-processor/internal/productenrich"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

type appBootstrap struct {
	productHandler productenrich.ProductHandler
	imageHandler   productimagehttpapi.RouteHandler
	server         *http.Server
	routes         []httproute.Descriptor
	pools          []worker.WorkerPool
	closers        []func() error
}
