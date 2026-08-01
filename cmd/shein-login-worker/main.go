package main

import (
	"flag"

	"github.com/sirupsen/logrus"

	"task-processor/internal/app/httpapi"
	"task-processor/internal/pkg/appenv"
)

var (
	configPath = flag.String("config", "config/config-dev.yaml", "config file path")
	logLevel   = flag.String("log-level", "info", "log level")
	healthPort = flag.Int("health-port", 8086, "worker health server port")
)

func start(logger *logrus.Logger, options httpapi.Options) error {
	return httpapi.RunSheinLoginWorker(logger, options)
}

func main() {
	flag.Parse()
	logger := appenv.SetupLoggerWithLevel(*logLevel)
	if err := start(logger, httpapi.Options{ConfigPath: *configPath, Port: *healthPort}); err != nil {
		logger.Fatalf("SHEIN login worker exited: %v", err)
	}
}
