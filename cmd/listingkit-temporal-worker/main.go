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
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func start(logger *logrus.Logger, options httpapi.Options) error {
	return httpapi.RunListingKitTemporalWorker(logger, options)
}

func main() {
	flag.Parse()

	logger := appenv.SetupLoggerWithLevel(*logLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("starting listingkit temporal worker")
	logger.Infof("config path: %s", *configPath)

	if err := start(logger, httpapi.Options{
		ConfigPath: *configPath,
	}); err != nil {
		logger.Fatalf("listingkit temporal worker start failed: %v", err)
	}
}
