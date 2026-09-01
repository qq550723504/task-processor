package httpapi

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting"
	amazonlistingapi "task-processor/internal/amazonlisting/api"
	amazonlistingstore "task-processor/internal/amazonlisting/store"
	"task-processor/internal/core/config"
	"task-processor/internal/httpbootstrap"
	platformdatabase "task-processor/internal/platform/database"
	worker "task-processor/internal/platform/workerpool"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type Module struct {
	Handler amazonlisting.Handler
	Pool    worker.WorkerPool
	Closers []func() error
}

type BuildModuleInput struct {
	Config         *config.Config
	Logger         *logrus.Logger
	ProductService productenrich.ProductService
	ImageService   productimage.Service
}

func BuildModule(input BuildModuleInput) (*Module, error) {
	repo, closers, err := buildTaskRepository(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}

	svc, err := amazonlisting.NewService(&amazonlisting.ServiceConfig{
		Repository:       repo,
		ProductService:   input.ProductService,
		ImageService:     input.ImageService,
		Assembler:        amazonlisting.NewAssembler(),
		ListingSubmitter: amazonlisting.NewSPAPISubmitter(input.Config),
		Validator:        amazonlisting.NewValidator(),
	})
	if err != nil {
		return nil, fmt.Errorf("create amazon listing service: %w", err)
	}

	processor, err := amazonlisting.NewProcessor(svc, repo, input.Logger, 2)
	if err != nil {
		return nil, fmt.Errorf("create amazon listing processor: %w", err)
	}
	pool := httpbootstrap.NewWorkerPool(processor, input.Config)
	submitter := &httpbootstrap.PoolSubmitter{Pool: pool}
	svc.SetTaskSubmitter(submitter)
	processor.SetTaskSubmitter(submitter)

	handler, err := amazonlistingapi.NewHandler(svc)
	if err != nil {
		return nil, fmt.Errorf("create amazon listing handler: %w", err)
	}

	return &Module{
		Handler: handler,
		Pool:    pool,
		Closers: closers,
	}, nil
}

func buildTaskRepository(cfg *config.Config, logger *logrus.Logger) (amazonlisting.Repository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create amazon listing task repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory amazonlisting repository")
	return amazonlistingstore.NewMemTaskRepository(), nil, nil
}

func newDBTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (amazonlisting.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}

	start := time.Now()
	logger.Infof("[amazonlisting] starting database connection...")

	databaseConfig := platformDatabaseConfig(cfg)
	db, err := platformdatabase.OpenShared(databaseConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s (took %v)", cfg.Host, cfg.Port, cfg.Database, time.Since(start))

	repo := amazonlistingstore.NewTaskRepository(db)
	closer := func() error { return platformdatabase.CloseShared(databaseConfig, db) }
	return repo, closer, nil
}

func platformDatabaseConfig(cfg *config.DatabaseConfig) *platformdatabase.Config {
	if cfg == nil {
		return nil
	}
	return &platformdatabase.Config{
		Host:                  cfg.Host,
		Port:                  cfg.Port,
		User:                  cfg.User,
		Password:              cfg.Password,
		Database:              cfg.Database,
		MaxConnections:        cfg.MaxConnections,
		MaxIdleConnections:    cfg.MaxIdleConnections,
		ConnectionMaxLifetime: cfg.ConnectionMaxLifetime,
	}
}
