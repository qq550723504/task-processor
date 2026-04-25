package listing

import "github.com/sirupsen/logrus"

func logHealthEndpoints(logger *logrus.Logger) {
	logger.Info("health: http://localhost:8081/health")
	logger.Info("ready: http://localhost:8081/ready")
	logger.Info("metrics: http://localhost:8082/metrics")
}
