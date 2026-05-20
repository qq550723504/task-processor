package httpapicmd

import (
	"task-processor/internal/app/httpapi"

	"github.com/sirupsen/logrus"
)

func Run(logger *logrus.Logger, serviceName string, options httpapi.Options) error {
	logger.Infof("starting %s", serviceName)
	logger.Infof("config path: %s", options.ConfigPath)
	logger.Infof("API port: %d", options.Port)
	return httpapi.Run(logger, options)
}
