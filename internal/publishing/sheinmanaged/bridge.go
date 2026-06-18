package sheinmanaged

import (
	"context"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/infra/clients/management"
	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslate "task-processor/internal/shein/api/translate"
)

type categoryResolver struct {
	fallback sheinpub.CategoryResolver
	factory  *apiFactory
	aiConfig sheinpub.CategoryAIConfig
}

func NewCategoryResolver(client *management.ClientManager, llmClient ...openaiclient.ChatCompleter) sheinpub.CategoryResolver {
	var aiConfig sheinpub.CategoryAIConfig
	if len(llmClient) > 0 && llmClient[0] != nil {
		aiConfig.Selector = newCategorySelectorAdapter(llmClient[0])
		aiConfig.SemanticVerifier = llmClient[0]
	}
	return &categoryResolver{
		fallback: sheinpub.NewCategoryResolver(nil),
		factory:  newAPIFactory(client),
		aiConfig: aiConfig,
	}
}

func (r *categoryResolver) Resolve(req *sheinpub.BuildRequest, canonicalProduct *canonical.Product, pkg *sheinpub.Package) *sheinpub.CategoryResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonicalProduct, pkg)
	}

	api, note := r.buildAPI(req.Context, req.SheinStoreID)
	resolver := sheinpub.NewCategoryResolverWithAI(api, r.aiConfig)
	resolution := resolver.Resolve(req, canonicalProduct, pkg)
	if note != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		resolution.Status = "partial"
	}
	return resolution
}

func (r *categoryResolver) buildAPI(_ context.Context, storeID int64) (sheinpub.CategoryAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheincategory.NewClient(baseAPIClient), ""
}

type attributeResolver struct {
	fallback sheinpub.AttributeResolver
	factory  *apiFactory
	llm      openaiclient.ChatCompleter
}

func NewAttributeResolver(client *management.ClientManager, llm openaiclient.ChatCompleter) sheinpub.AttributeResolver {
	return &attributeResolver{
		fallback: sheinpub.NewAttributeResolver(nil, llm),
		factory:  newAPIFactory(client),
		llm:      llm,
	}
}

func (r *attributeResolver) Resolve(req *sheinpub.BuildRequest, canonicalProduct *canonical.Product, pkg *sheinpub.Package) *sheinpub.AttributeResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonicalProduct, pkg)
	}

	api, note := r.buildAPI(req.SheinStoreID)
	resolver := sheinpub.NewAttributeResolver(api, r.llm)
	resolution := resolver.Resolve(req, canonicalProduct, pkg)
	if note != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		if resolution.Status == "" || resolution.Status == "unresolved" {
			resolution.Status = "partial"
		}
	}
	return resolution
}

func (r *attributeResolver) buildAPI(storeID int64) (sheinpub.AttributeAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheinattribute.NewClient(baseAPIClient), ""
}

type saleAttributeResolver struct {
	fallback sheinpub.SaleAttributeResolver
	factory  *apiFactory
	llm      openaiclient.ChatCompleter
}

func NewSaleAttributeResolver(client *management.ClientManager, llm openaiclient.ChatCompleter) sheinpub.SaleAttributeResolver {
	return &saleAttributeResolver{
		fallback: sheinpub.NewSaleAttributeResolver(nil, llm),
		factory:  newAPIFactory(client),
		llm:      llm,
	}
}

func (r *saleAttributeResolver) Resolve(req *sheinpub.BuildRequest, canonicalProduct *canonical.Product, pkg *sheinpub.Package) *sheinpub.SaleAttributeResolution {
	if req == nil {
		return r.fallback.Resolve(req, canonicalProduct, pkg)
	}

	api, note := r.buildAPI(req.SheinStoreID)
	resolver := sheinpub.NewSaleAttributeResolver(api, r.llm)
	resolution := resolver.Resolve(req, canonicalProduct, pkg)
	if note != "" {
		resolution.ReviewNotes = append(resolution.ReviewNotes, note)
		if resolution.Status == "" || resolution.Status == "unresolved" {
			resolution.Status = "partial"
		}
	}
	return resolution
}

func (r *saleAttributeResolver) buildAPI(storeID int64) (sheinpub.AttributeAPI, string) {
	baseAPIClient, note := r.factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheinattribute.NewClient(baseAPIClient), ""
}

type productAPIBuilder struct {
	factory *apiFactory
}

func NewProductAPIBuilder(client *management.ClientManager) sheinpub.ProductAPIBuilder {
	return &productAPIBuilder{factory: newAPIFactory(client)}
}

func (b *productAPIBuilder) BuildProductAPI(_ context.Context, storeID int64) (sheinproduct.ProductAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "management client 不可用，SHEIN 提交未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheinproduct.NewClient(baseClient), ""
}

type imageAPIBuilder struct {
	factory *apiFactory
}

func NewImageAPIBuilder(client *management.ClientManager) sheinpub.ImageAPIBuilder {
	return &imageAPIBuilder{factory: newAPIFactory(client)}
}

func (b *imageAPIBuilder) BuildImageAPI(_ context.Context, storeID int64) (sheinimage.ImageAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "management client 不可用，SHEIN 图片上传未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheinimage.NewClientWithImageDownloader(baseClient, b.factory.client.GetImageDownloader()), ""
}

type translateAPIBuilder struct {
	factory *apiFactory
}

func NewTranslateAPIBuilder(client *management.ClientManager) sheinpub.TranslateAPIBuilder {
	return &translateAPIBuilder{factory: newAPIFactory(client)}
}

func (b *translateAPIBuilder) BuildTranslateAPI(_ context.Context, storeID int64) (sheintranslate.TranslateAPI, string) {
	if b == nil || b.factory == nil {
		return nil, "management client 不可用，SHEIN 翻译未启用"
	}
	baseClient, fallback := b.factory.BuildBaseClient(storeID)
	if baseClient == nil {
		return nil, fallback
	}
	return sheintranslate.NewClient(baseClient), ""
}
