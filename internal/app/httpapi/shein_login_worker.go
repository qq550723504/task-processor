package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
	localruntime "task-processor/internal/listingruntime/local"
	sheinloginbootstrap "task-processor/internal/sheinlogin/bootstrap"
)

var errSheinLoginWorkerUnavailable = errors.New("SHEIN login worker is unavailable")

type sheinLoginWorkerRuntimeDependencies struct {
	LoadConfig            func(string) (*config.Config, error)
	BuildDatabaseStoreAPI func(*config.Config) (listingadmin.StoreAPI, func() error, error)
	BuildLoginService     func(*runtimeDeps) (*sheinloginbootstrap.BuildResult, func() error, error)
}

type sheinLoginWorkerRuntime struct {
	result  *sheinloginbootstrap.BuildResult
	closers []func() error
}

func (r *sheinLoginWorkerRuntime) Close() error {
	if r == nil {
		return nil
	}
	var closeErrors []error
	for _, closeFn := range r.closers {
		if closeFn == nil {
			continue
		}
		if err := closeFn(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

// RunSheinLoginWorker starts only the dependencies required by the dedicated
// browser worker. It deliberately does not construct an HTTP server.
func RunSheinLoginWorker(logger *logrus.Logger, options Options) error {
	workerRuntime, err := buildSheinLoginWorkerRuntime(options.ConfigPath)
	if err != nil {
		return fmt.Errorf("build SHEIN login worker runtime: %w", err)
	}
	defer func() {
		if err := workerRuntime.Close(); err != nil {
			logger.WithError(err).Warn("close SHEIN login worker runtime")
		}
	}()
	result := workerRuntime.result
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

func buildSheinLoginWorkerRuntime(configPath string) (*sheinLoginWorkerRuntime, error) {
	return buildSheinLoginWorkerRuntimeWithDependencies(configPath, sheinLoginWorkerRuntimeDependencies{
		LoadConfig:            loadSheinLoginWorkerConfig,
		BuildDatabaseStoreAPI: buildSheinLoginWorkerDatabaseStoreAPI,
		BuildLoginService:     buildSheinLoginModuleResult,
	})
}

func buildSheinLoginWorkerRuntimeWithDependencies(configPath string, dependencies sheinLoginWorkerRuntimeDependencies) (*sheinLoginWorkerRuntime, error) {
	if dependencies.LoadConfig == nil || dependencies.BuildDatabaseStoreAPI == nil || dependencies.BuildLoginService == nil {
		return nil, fmt.Errorf("SHEIN login worker runtime dependencies are incomplete")
	}

	cfg, err := dependencies.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	storeAPI, storeCloser, err := dependencies.BuildDatabaseStoreAPI(cfg)
	if err != nil {
		return nil, fmt.Errorf("build SHEIN login worker database StoreAPI: %w", err)
	}

	deps := &runtimeDeps{
		shared: &sharedRuntimeDeps{
			cfg:      cfg,
			storeAPI: storeAPI,
		},
		features: &featureRuntimeState{},
	}
	result, loginCloser, err := dependencies.BuildLoginService(deps)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("build SHEIN login worker service: %w", err), closeWorkerRuntimeClosers(loginCloser, storeCloser))
	}
	if result == nil || result.Service == nil {
		return nil, errors.Join(errSheinLoginWorkerUnavailable, closeWorkerRuntimeClosers(loginCloser, storeCloser))
	}

	return &sheinLoginWorkerRuntime{
		result:  result,
		closers: []func() error{loginCloser, storeCloser},
	}, nil
}

func buildSheinLoginWorkerDatabaseStoreAPI(cfg *config.Config) (listingadmin.StoreAPI, func() error, error) {
	provider, err := localruntime.NewLocalDataProvider(cfg.Database, nil)
	if err != nil {
		return nil, nil, err
	}
	if provider == nil {
		return nil, nil, nil
	}
	storeAPI := localruntime.NewLocalRuntime(provider, localruntime.LocalRuntimeOptions{}).GetStoreAPI()
	return storeAPI, provider.Close, nil
}

func closeWorkerRuntimeClosers(closers ...func() error) error {
	runtime := &sheinLoginWorkerRuntime{closers: closers}
	return runtime.Close()
}
