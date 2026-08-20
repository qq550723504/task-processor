package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/sourceaccount"
)

type sourceAccountRepositoryBuilder func(*config.Config, *logrus.Logger) (sourceaccount.Repository, []func() error, error)

var buildSourceAccountRepository = listingkithttpapi.BuildSourceAccountRepository
