package listing

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

type listingLocalRuntimeValidator interface {
	ValidateLocalListingRuntimeFields() (map[string]bool, error)
}

func validateListingLocalRuntime(platform string, validator listingLocalRuntimeValidator, logger *logrus.Logger) error {
	if !strings.EqualFold(strings.TrimSpace(platform), "shein") {
		return nil
	}
	if validator == nil {
		return fmt.Errorf("SHEIN listing local runtime is not ready: management client is not initialized")
	}

	fields, err := validator.ValidateLocalListingRuntimeFields()
	if logger != nil {
		logFields := logrus.Fields{}
		for key, value := range fields {
			logFields[key] = value
		}
		entry := logger.WithFields(logFields)
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
