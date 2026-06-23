package listing

import (
	"fmt"
	"strings"

	"task-processor/internal/infra/clients/management"

	"github.com/sirupsen/logrus"
)

func validateListingLocalRuntime(platform string, client *management.ClientManager, logger *logrus.Logger) error {
	if !strings.EqualFold(strings.TrimSpace(platform), "shein") {
		return nil
	}
	if client == nil {
		return fmt.Errorf("SHEIN listing local runtime is not ready: management client is not initialized")
	}

	report, err := client.ValidateLocalListingRuntime()
	if logger != nil {
		fields := logrus.Fields{}
		for key, value := range report.Fields() {
			fields[key] = value
		}
		entry := logger.WithFields(fields)
		if err != nil {
			entry.WithError(err).Error("SHEIN listing local runtime check failed")
		} else {
			entry.Info("SHEIN listing local runtime check passed")
		}
	}
	if err != nil {
		return fmt.Errorf("SHEIN listing local runtime is not ready: %w", err)
	}
	return nil
}
