package httpapi

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting/store"
	"task-processor/internal/core/config"
)

func TestBuildModuleFailsClosedWithoutTaskRepository(t *testing.T) {
	t.Parallel()

	module, err := BuildModule(BuildModuleInput{
		Config: &config.Config{},
		Logger: logrus.New(),
	})

	if err == nil || !strings.Contains(err.Error(), "task repository is required") {
		t.Fatalf("BuildModule() module/error = %v/%v, want nil explicit task repository error", module, err)
	}
	if module != nil {
		t.Fatalf("BuildModule() module = %#v, want nil before route or pool registration", module)
	}
}

func TestBuildModuleAcceptsExplicitMemoryTaskRepositoryFixture(t *testing.T) {
	t.Parallel()

	module, err := BuildModule(BuildModuleInput{
		Config: &config.Config{},
		Logger: logrus.New(),
		Repositories: RepositoryDependencies{
			Task: store.NewMemTaskRepository(),
		},
	})

	if err != nil {
		t.Fatalf("BuildModule() error = %v", err)
	}
	if module == nil || module.Handler == nil || module.Pool == nil {
		t.Fatalf("BuildModule() module = %#v, want explicit test fixture module", module)
	}
	if len(module.Closers) != 0 {
		t.Fatalf("BuildModule() closers = %d, want fixture ownership retained by test", len(module.Closers))
	}
}
