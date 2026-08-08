package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	crawler1688 "task-processor/internal/crawler/alibaba1688"
	"task-processor/internal/httproute"
	"task-processor/internal/infra/httpx"
	"task-processor/internal/infra/worker"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingadmin"
)

const crawler1688HTTPModuleName = "crawler-1688"

type crawler1688HTTPModule struct {
	service *crawler1688.Service
	handler http.Handler
}

func newCrawler1688HTTPModule(cfg *config.Config, logger *logrus.Logger, repository listingadmin.StoreRepository) *crawler1688HTTPModule {
	var resolver crawler1688.AccountProfileResolver
	if repository != nil {
		resolver = crawler1688.NewAccountProfileResolver(repository, cfg.Platforms.Alibaba1688.ProfileRootDir)
	}
	service := crawler1688.NewService(cfg, logger, resolver)
	handler := httpx.NewCrawler1688Handler(service, logger, crawler1688.VerifiedCrawlerTenantResolver)
	return &crawler1688HTTPModule{service: service, handler: handler.RegisterRoutes()}
}

func (m *crawler1688HTTPModule) Name() string { return crawler1688HTTPModuleName }

func (m *crawler1688HTTPModule) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Platforms.Alibaba1688.Enabled
}

func (m *crawler1688HTTPModule) Register(reg *kernelmodule.Registry) error {
	if m == nil || m.service == nil || m.handler == nil {
		return nil
	}
	handler := gin.WrapH(m.handler)
	for _, route := range crawler1688Routes(handler) {
		reg.AddRoutes(route)
	}
	return reg.AddWorkerPool(crawler1688HTTPModuleName, m.service.WorkerPool())
}

func (m *crawler1688HTTPModule) Close() error {
	if m == nil || m.service == nil {
		return nil
	}
	return m.service.Stop(context.Background())
}

func (m *crawler1688HTTPModule) WorkerPool() worker.WorkerPool {
	if m == nil || m.service == nil {
		return nil
	}
	return m.service.WorkerPool()
}

func crawler1688Routes(handler gin.HandlerFunc) []httproute.Descriptor {
	routes := make([]httproute.Descriptor, 0, 16)
	add := func(method, path string) {
		routes = append(routes, httproute.Descriptor{Method: method, Path: path, Module: crawler1688HTTPModuleName, Handler: handler})
	}
	add(http.MethodPost, "/api/v1/crawl")
	add(http.MethodGet, "/api/v1/tasks")
	add(http.MethodGet, "/api/v1/tasks/*task_id")
	add(http.MethodDelete, "/api/v1/tasks/*task_id")
	add(http.MethodGet, "/api/v1/stats")
	add(http.MethodGet, "/metrics")
	add(http.MethodGet, "/health")
	add(http.MethodGet, "/ready")
	for _, route := range append([]httproute.Descriptor(nil), routes...) {
		route.Method = http.MethodOptions
		routes = append(routes, route)
	}
	return routes
}
