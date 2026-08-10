package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
	localruntime "task-processor/internal/listingruntime/local"
)

// RunSheinLoginWorker starts only the dependencies required by the dedicated
// browser worker. It deliberately does not construct an HTTP server.
func RunSheinLoginWorker(logger *logrus.Logger, options Options) error {
	deps, err := buildSheinLoginWorkerRuntimeDeps(options.ConfigPath)
	if err != nil {
		return fmt.Errorf("build runtime deps: %w", err)
	}
	defer closeResources(logger, deps.shared.closers)

	result, closer, err := buildSheinLoginModuleResult(deps)
	if err != nil {
		return fmt.Errorf("build SHEIN login worker: %w", err)
	}
	if closer != nil {
		defer func() { _ = closer() }()
	}
	if result == nil || result.Service == nil {
		return fmt.Errorf("SHEIN login worker is unavailable")
	}
	var healthServer *http.Server
	if options.Port > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			health := result.Service.Health(r.Context())
			if !health.Initialized || !health.RedisReady || !health.ManagementReady {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"initialized":%t,"redis_ready":%t,"management_ready":%t}`+"\n", health.Initialized, health.RedisReady, health.ManagementReady)
		})
		healthServer = &http.Server{Addr: fmt.Sprintf(":%d", options.Port), Handler: mux}
		go func() {
			if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.WithError(err).Error("SHEIN login worker health server exited")
			}
		}()
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), options.shutdownTimeout())
			defer shutdownCancel()
			_ = healthServer.Shutdown(shutdownCtx)
		}()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigChan := options.ShutdownSignal
	if sigChan == nil {
		sigChan = make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigChan)
	}
	go func() {
		<-sigChan
		cancel()
	}()

	return result.Service.RunWorker(ctx, "")
}

func loadSheinLoginWorkerConfig(configPath string) (*config.Config, error) {
	cfg, err := config.LoadConfigFromFileWithoutValidation(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	login := &cfg.Platforms.Shein.LoginService
	login.BaseURL = ""
	login.SharedKey = ""
	login.TenantID = ""
	login.Identifier = ""
	login.MerchantName = ""
	login.Username = ""
	login.Password = ""
	return cfg, nil
}

func buildSheinLoginWorkerRuntimeDeps(configPath string) (*runtimeDeps, error) {
	cfg, err := loadSheinLoginWorkerConfig(configPath)
	if err != nil {
		return nil, err
	}

	provider, err := localruntime.NewLocalDataProvider(cfg.Database, nil)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN login worker StoreAPI: %w", err)
	}

	var storeAPI listingadmin.StoreAPI
	var closers []func() error
	if provider != nil {
		storeAPI = localruntime.NewLocalRuntime(provider, localruntime.LocalRuntimeOptions{}).GetStoreAPI()
		closers = append(closers, provider.Close)
	}

	return &runtimeDeps{
		shared: &sharedRuntimeDeps{
			cfg:      cfg,
			closers:  closers,
			storeAPI: storeAPI,
		},
		features: &featureRuntimeState{},
	}, nil
}
