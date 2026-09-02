package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/core/config"
)

type RuntimeBuildInput struct {
	Logger                       *logrus.Logger
	Config                       *config.Config
	ProductSnapshotReader        amazonlisting.ProductSnapshotReader
	ApprovedAssetInventoryReader amazonlisting.ApprovedAssetInventoryReader
	Repositories                 RepositoryDependencies
}

func BuildRuntimeModule(input RuntimeBuildInput) (*Module, error) {
	return BuildModule(BuildModuleInput{
		Config:                       input.Config,
		Logger:                       input.Logger,
		ProductSnapshotReader:        input.ProductSnapshotReader,
		ApprovedAssetInventoryReader: input.ApprovedAssetInventoryReader,
		Repositories:                 input.Repositories,
	})
}
