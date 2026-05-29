package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
)

func Run(logger *logrus.Logger, options Options) error {
	bootstrap, err := buildBootstrap(logger, options)
	if err != nil {
		return fmt.Errorf("build bootstrap: %w", err)
	}
	defer closeResources(logger, bootstrap.closers)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, pool := range bootstrap.pools {
		pool.Start(ctx)
	}
	logger.Infof("worker pools started: %d", len(bootstrap.pools))

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- serveHTTP(logger, bootstrap.server, bootstrap.routes, options.Port)
	}()

	sigChan := options.ShutdownSignal
	if sigChan == nil {
		sigChan = make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	}

	select {
	case err := <-serverErr:
		if err != nil {
			return err
		}
	case sig := <-sigChan:
		logger.Infof("received signal %v, shutting down", sig)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := bootstrap.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	cancel()
	for _, pool := range bootstrap.pools {
		pool.Stop(shutdownCtx)
	}
	logger.Info("service shut down cleanly")
	return nil
}

func serveHTTP(logger *logrus.Logger, server *http.Server, routes []routeDescriptor, port int) error {
	logger.Infof("API service listening on port %d", port)
	logger.Info("available endpoints:")
	for _, route := range routes {
		logger.Infof("  - %-6s %s", route.Method, route.Path)
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server exited unexpectedly: %w", err)
	}

	return nil
}

func buildHTTPServerBundleFromHandlers(port int, cfg *config.Config, handlers httpModuleHandlers) (*http.Server, []routeDescriptor, error) {
	return buildHTTPServerBundleFromModules(port, cfg, buildHTTPModules(handlers))
}

func buildHTTPServerBundleFromModules(port int, cfg *config.Config, modules []kernelmodule.Module) (*http.Server, []routeDescriptor, error) {
	routes, err := buildRegisteredRoutesForModules(cfg, modules)
	if err != nil {
		return nil, nil, err
	}
	return buildHTTPServerFromRoutes(port, routes), routes, nil
}

func closeResources(logger *logrus.Logger, closers []func() error) {
	for _, closeFn := range closers {
		if closeFn == nil {
			continue
		}
		if err := closeFn(); err != nil {
			logger.Warnf("close resource failed: %v", err)
		}
	}
}
