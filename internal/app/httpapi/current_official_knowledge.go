package httpapi

import (
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	officialhttp "task-processor/internal/knowledge/httpapi/official"
	"task-processor/internal/knowledge/official"
)

func buildOfficialKnowledgeModule() (kernelmodule.Module, error) {
	catalog, err := official.NewEmbeddedCatalog()
	if err != nil {
		return nil, err
	}
	handler, err := officialhttp.NewHandler(catalog)
	if err != nil {
		return nil, err
	}
	return officialKnowledgeModule{handler}, nil
}

// Only assembly depends on the existing kernel's configuration contract.
type officialKnowledgeModule struct{ handler *officialhttp.Handler }

func (officialKnowledgeModule) Name() string { return "official-knowledge" }
func (m officialKnowledgeModule) Enabled(cfg *config.Config) bool {
	return m.handler != nil && cfg != nil && cfg.Workbench.Enabled
}
func (m officialKnowledgeModule) Register(registry *kernelmodule.Registry) error {
	for _, descriptor := range officialhttp.Routes(m.handler) {
		registry.AddRoutes(descriptor)
	}
	return nil
}
