package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type RuntimeBuildInput struct {
	Logger         *logrus.Logger
	Config         *config.Config
	ProductService productenrich.ProductService
	ImageService   productimage.Service
}

func BuildRuntimeModule(input RuntimeBuildInput) (*Module, error) {
	return BuildModule(BuildModuleInput{
		Config:         input.Config,
		Logger:         input.Logger,
		ProductService: input.ProductService,
		ImageService:   input.ImageService,
	})
}
