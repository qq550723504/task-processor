package main

import (
	"flag"

	"task-processor/internal/app/httpapi"
	"task-processor/internal/pkg/appenv"
)

var (
	configPath = flag.String("config", "config/config-dev.yaml", "config file path")
	logLevel   = flag.String("log-level", "info", "log level")
	port       = flag.Int("port", 8085, "API service port")
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	logger := appenv.SetupLoggerWithLevel(*logLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("starting product listing API service")
	logger.Infof("config path: %s", *configPath)
	logger.Infof("API port: %d", *port)

	if err := httpapi.Run(logger, httpapi.Options{
		ConfigPath: *configPath,
		Port:       *port,
	}); err != nil {
		logger.Fatalf("service start failed: %v", err)
	}
}
