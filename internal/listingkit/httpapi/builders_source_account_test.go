package httpapi

import (
	"testing"

	"task-processor/internal/core/config"
)

func TestBuildSourceAccountRepositoryWithoutDatabaseKeepsPublicPathAvailable(t *testing.T) {
	repository, closers, err := BuildSourceAccountRepository(&config.Config{}, nil)
	if err != nil {
		t.Fatalf("BuildSourceAccountRepository() error = %v", err)
	}
	if repository != nil || len(closers) != 0 {
		t.Fatalf("repository/closers = %v/%d, want nil/0 without database", repository, len(closers))
	}
}
