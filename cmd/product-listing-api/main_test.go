package main

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"task-processor/internal/app/httpapi"
)

func TestStartFailsClosedWithoutPersistentConsumerStores(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	err := start(logger, defaultTestOptions())
	if err == nil || !strings.Contains(err.Error(), "listingkit database config is required") {
		t.Fatalf("start() error = %v, want persistent ListingKit repository failure", err)
	}
}

func defaultTestOptions() httpapi.Options {
	return httpapi.Options{
		ConfigPath: "../../config/config-test.yaml",
		Port:       0,
	}
}
