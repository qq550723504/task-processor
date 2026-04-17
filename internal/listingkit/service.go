package listingkit

import (
	"fmt"
	"strings"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type service struct {
	repo          Repository
	productSvc    ProductService
	imageSvc      ImageService
	assembler     Assembler
	taskSubmitter TaskSubmitter
}

type ServiceConfig struct {
	Repository     Repository
	ProductService ProductService
	ImageService   ImageService
	Assembler      Assembler
	TaskSubmitter  TaskSubmitter
}

func NewService(config *ServiceConfig) (Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Repository == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}
	if config.ProductService == nil {
		return nil, fmt.Errorf("product service cannot be nil")
	}
	if config.Assembler == nil {
		config.Assembler = NewAssemblerWithConfig(AssemblerConfig{
			AmazonBuilder:              newAmazonDraftBuilder(),
			SheinCategoryResolver:      NewSheinCategoryResolver(nil),
			SheinAttributeResolver:     NewSheinAttributeResolver(nil),
			SheinSaleAttributeResolver: NewSheinSaleAttributeResolver(nil),
		})
	}
	return &service{
		repo:          config.Repository,
		productSvc:    config.ProductService,
		imageSvc:      config.ImageService,
		assembler:     config.Assembler,
		taskSubmitter: config.TaskSubmitter,
	}, nil
}

func (s *service) SetTaskSubmitter(submitter TaskSubmitter) {
	s.taskSubmitter = submitter
}

func normalizeGenerateRequest(req *GenerateRequest) {
	if req == nil {
		return
	}
	req.Country = strings.ToUpper(strings.TrimSpace(req.Country))
	req.Language = strings.TrimSpace(req.Language)
	if req.Country == "" {
		req.Country = "US"
	}
	if req.Language == "" {
		req.Language = "en_US"
	}
	if req.Options == nil {
		req.Options = &GenerateOptions{ProcessImages: true}
	}
	req.Platforms = normalizePlatforms(req.Platforms)
	if len(req.Platforms) == 0 {
		req.Platforms = []string{"amazon", "shein", "temu", "walmart"}
	}
}

func normalizePlatforms(platforms []string) []string {
	if len(platforms) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		normalized := strings.ToLower(strings.TrimSpace(platform))
		switch normalized {
		case "amazon", "shein", "temu", "walmart":
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			result = append(result, normalized)
		}
	}
	return result
}

type amazonDraftBuilder struct {
	assembler amazonlisting.Assembler
}

func newAmazonDraftBuilder() AmazonDraftBuilder {
	return &amazonDraftBuilder{assembler: amazonlisting.NewAssembler()}
}

func (b *amazonDraftBuilder) Build(req *GenerateRequest, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *amazonlisting.AmazonListingDraft {
	task := &amazonlisting.Task{
		ID: "listingkit-amazon-preview",
		Request: &amazonlisting.GenerateRequest{
			Marketplace:        "amazon",
			Country:            req.Country,
			Language:           req.Language,
			ImageURLs:          append([]string(nil), req.ImageURLs...),
			Text:               req.Text,
			ProductURL:         req.ProductURL,
			TargetCategoryHint: req.TargetCategoryHint,
			BrandHint:          req.BrandHint,
			Options: &amazonlisting.GenerateOptions{
				ProcessImages: req.Options != nil && req.Options.ProcessImages,
			},
		},
	}
	return b.assembler.Assemble(task, canonical, image)
}
