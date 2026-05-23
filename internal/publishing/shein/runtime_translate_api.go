package shein

import (
	"context"

	sheintranslate "task-processor/internal/shein/api/translate"
)

type runtimeTranslateAPIBuilder struct {
	factory *runtimeAPIFactory
}

func NewRuntimeTranslateAPIBuilder(factory RuntimeAPIClientFactory) TranslateAPIBuilder {
	return &runtimeTranslateAPIBuilder{
		factory: newRuntimeAPIFactory(factory),
	}
}

func (b *runtimeTranslateAPIBuilder) BuildTranslateAPI(ctx context.Context, storeID int64) (sheintranslate.TranslateAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "shein runtime client factory 不可用，SHEIN 翻译未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(ctx, storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheintranslate.NewClient(baseClient), ""
}
