package main

import (
	"flag"
	"fmt"

	"task-processor/internal/app/httpapi"
	"task-processor/internal/pkg/appenv"
	"task-processor/internal/pkg/httpapicmd"

	"github.com/sirupsen/logrus"
)

var (
	configPath = flag.String("config", "config/config-dev.yaml", "config file path")
	logLevel   = flag.String("log-level", "info", "log level")
	port       = flag.Int("port", 8085, "API service port")
	bindAddr   = flag.String("bind-address", "", "HTTP bind address (empty binds all interfaces)")
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func start(logger *logrus.Logger, options httpapi.Options) error {
	return startWithApplication(logger, options, httpapicmd.Run)
}

type applicationRunner func(*logrus.Logger, string, httpapi.Options) error

func startWithApplication(logger *logrus.Logger, options httpapi.Options, application applicationRunner) error {
	if application == nil {
		return fmt.Errorf("product listing application runner is required")
	}
	return application(logger, "product listing API service", options)
}

func main() {
	flag.Parse()

	logger := appenv.SetupLoggerWithLevel(*logLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	if err := start(logger, httpapi.Options{
		ConfigPath:  *configPath,
		Port:        *port,
		BindAddress: *bindAddr,
	}); err != nil {
		logger.Fatalf("service start failed: %v", err)
	}
}
