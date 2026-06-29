package consumer

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformModuleRegistrarDoesNotStoreSharedResources(t *testing.T) {
	t.Parallel()

	files := []string{"dependencies.go", "platform_module_registrar.go"}
	for _, name := range files {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(content)

		for _, marker := range []string{
			"resources *SharedResources) platformModuleRegistrar",
			"resources      *SharedResources",
			"resources:      resources",
			"func (r platformModuleRegistrar) register(ctx context.Context, module PlatformModule) error",
		} {
			if strings.Contains(source, marker) {
				t.Fatalf("%s mentions %q; pass SharedResources into register instead of storing it on platformModuleRegistrar", name, marker)
			}
		}
	}
}
