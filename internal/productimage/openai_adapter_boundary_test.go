package productimage

import (
	"os"
	"strings"
	"testing"
)

func TestOpenAICompatibleRenderersUseLocalImageEditPort(t *testing.T) {
	for _, fileName := range []string{
		"openai_image_editor.go",
		"openai_scene_generator.go",
	} {
		source := readProductImageBoundaryFile(t, fileName)
		for _, marker := range []string{
			`"task-processor/internal/infra/clients/openai"`,
			"openaiclient.ImageEditRequest",
			"openaiclient.ImageResponse",
		} {
			if strings.Contains(source, marker) {
				t.Fatalf("%s should use the local image edit port instead of concrete OpenAI client types %s", fileName, marker)
			}
		}
	}

	adapterSource := readProductImageBoundaryFile(t, "openai_image_edit_adapter.go")
	for _, marker := range []string{
		`"task-processor/internal/infra/clients/openai"`,
		"type openAIImageEditClientAdapter struct",
		"func newOpenAIImageEditClientAdapter",
	} {
		if !strings.Contains(adapterSource, marker) {
			t.Fatalf("openai_image_edit_adapter.go missing %s", marker)
		}
	}
}

func readProductImageBoundaryFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
