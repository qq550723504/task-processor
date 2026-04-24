package shein

import (
	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

func sheinLogger(component string) *logrus.Entry {
	return logger.GetGlobalLogger(component)
}
