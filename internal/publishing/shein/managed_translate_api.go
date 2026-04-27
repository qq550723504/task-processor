package shein

import (
	"task-processor/internal/infra/clients/management"
	sheintranslate "task-processor/internal/shein/api/translate"
)

type TranslateAPIBuilder interface {
	BuildTranslateAPI(storeID int64) (sheintranslate.TranslateAPI, string)
}

type managedTranslateAPIBuilder struct {
	factory *managedAPIFactory
}

func NewManagedTranslateAPIBuilder(client *management.ClientManager) TranslateAPIBuilder {
	return &managedTranslateAPIBuilder{
		factory: newManagedAPIFactory(client),
	}
}

func (b *managedTranslateAPIBuilder) BuildTranslateAPI(storeID int64) (sheintranslate.TranslateAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "management client 不可用，SHEIN 翻译未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheintranslate.NewClient(baseClient), ""
}
