package ai_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"task-processor/internal/ai"
	"task-processor/internal/infra/clients/geminiimage"
	"task-processor/internal/infra/clients/grsai"
	"task-processor/internal/infra/clients/openai"
)

var (
	_ ai.ImageGenerator = (*openai.Client)(nil)
	_ ai.ImageGenerator = (*geminiimage.Client)(nil)
	_ ai.ImageGenerator = (*grsai.Client)(nil)
)

func TestProviderAdaptersDoNotImportOpenAIForSharedContracts(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(currentFile)
	for _, relative := range []string{
		filepath.Join("..", "infra", "clients", "geminiimage", "client.go"),
		filepath.Join("..", "infra", "clients", "grsai", "client.go"),
	} {
		path := filepath.Join(root, relative)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(content), "task-processor/internal/infra/clients/openai") {
			t.Fatalf("provider adapter %s imports the OpenAI implementation package", path)
		}
	}
}
