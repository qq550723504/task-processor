package consumer

import (
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/clients/management"

	"github.com/sirupsen/logrus"
)

type SharedResources struct {
	ManagementClient *management.ClientManager
	AmazonProcessor  *amazon.AmazonProcessor
}

type SharedResourceProvider func(cfg *config.Config, logger *logrus.Logger, needsAmazon bool) (*SharedResources, error)
