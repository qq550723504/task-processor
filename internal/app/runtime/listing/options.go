package listing

import (
	"os"
	"strconv"
	"strings"
)

const defaultConfigPath = "config/config-prod.yaml"

type Options struct {
	Platform  string
	Config    string
	AppConfig string
	LogLevel  string
	Version   string
	BuildTime string
}

func (o Options) ConfigPath() string {
	return ResolveConfigPath(o.Config, o.AppConfig)
}

func ResolveConfigPath(configPath, appConfigPath string) string {
	if configPath != "" {
		return configPath
	}
	if appConfigPath != "" {
		return appConfigPath
	}
	return defaultConfigPath
}

func ResolveDebugTaskID() int64 {
	raw := strings.TrimSpace(os.Getenv("TASK_PROCESSOR_DEBUG_TASK_ID"))
	if raw == "" {
		return 0
	}

	taskID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || taskID <= 0 {
		return 0
	}

	return taskID
}
