package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting"
	amazonlistingapi "task-processor/internal/amazonlisting/api"
	"task-processor/internal/core/config"
	"task-processor/internal/httpbootstrap"
	worker "task-processor/internal/platform/workerpool"
)

type Module struct {
	Handler amazonlisting.Handler
	Pool    worker.WorkerPool
	Closers []func() error
}

type BuildModuleInput struct {
	Config                       *config.Config
	Logger                       *logrus.Logger
	ProductSnapshotReader        amazonlisting.ProductSnapshotReader
	ApprovedAssetInventoryReader amazonlisting.ApprovedAssetInventoryReader
	Repositories                 RepositoryDependencies
}

type RepositoryDependencies struct {
	Task amazonlisting.Repository
}

func BuildModule(input BuildModuleInput) (*Module, error) {
	repo, err := resolveTaskRepository(input)
	if err != nil {
		return nil, err
	}

	svc, err := amazonlisting.NewService(&amazonlisting.ServiceConfig{
		Repository:                   repo,
		ProductSnapshotReader:        input.ProductSnapshotReader,
		ApprovedAssetInventoryReader: input.ApprovedAssetInventoryReader,
		Assembler:                    amazonlisting.NewAssembler(),
		ListingSubmitter:             amazonlisting.NewSPAPISubmitter(input.Config),
		Validator:                    amazonlisting.NewValidator(),
	})
	if err != nil {
		return nil, fmt.Errorf("create amazon listing service: %w", err)
	}

	processor, err := amazonlisting.NewProcessor(svc, repo, input.Logger)
	if err != nil {
		return nil, fmt.Errorf("create amazon listing processor: %w", err)
	}
	pool := httpbootstrap.NewWorkerPool(processor, input.Config)
	submitter := &httpbootstrap.PoolSubmitter{Pool: pool}
	svc.SetTaskSubmitter(submitter)

	handler, err := amazonlistingapi.NewHandler(svc)
	if err != nil {
		return nil, fmt.Errorf("create amazon listing handler: %w", err)
	}

	return &Module{
		Handler: handler,
		Pool:    pool,
	}, nil
}

func resolveTaskRepository(input BuildModuleInput) (amazonlisting.Repository, error) {
	if input.Repositories.Task == nil {
		return nil, fmt.Errorf("amazon listing task repository is required")
	}
	return input.Repositories.Task, nil
}
