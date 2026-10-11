package httpapi

import (
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/knowledge/official"
	officialhttp "task-processor/internal/knowledge/official/httpapi"
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
	return officialhttp.NewModule(handler), nil
}
