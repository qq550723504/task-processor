package amazonlisting

import (
	"fmt"
	"strings"
)

type service struct {
	repo             Repository
	productService   ProductService
	imageService     ImageService
	assembler        Assembler
	exportBuilder    ExportBuilder
	listingSubmitter ListingSubmitter
	validator        Validator
	autoFixer        AutoFixer
	taskSubmitter    TaskSubmitter
}

type ServiceConfig struct {
	Repository       Repository
	ProductService   ProductService
	ImageService     ImageService
	Assembler        Assembler
	ExportBuilder    ExportBuilder
	ListingSubmitter ListingSubmitter
	Validator        Validator
	AutoFixer        AutoFixer
	TaskSubmitter    TaskSubmitter
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
		config.Assembler = NewAssembler()
	}
	if config.ExportBuilder == nil {
		config.ExportBuilder = NewExportBuilder()
	}
	if config.Validator == nil {
		config.Validator = NewValidator()
	}
	if config.AutoFixer == nil {
		config.AutoFixer = NewAutoFixer()
	}
	return &service{
		repo:             config.Repository,
		productService:   config.ProductService,
		imageService:     config.ImageService,
		assembler:        config.Assembler,
		exportBuilder:    config.ExportBuilder,
		listingSubmitter: config.ListingSubmitter,
		validator:        config.Validator,
		autoFixer:        config.AutoFixer,
		taskSubmitter:    config.TaskSubmitter,
	}, nil
}

func (s *service) SetTaskSubmitter(submitter TaskSubmitter) {
	s.taskSubmitter = submitter
}

func normalizeGenerateRequest(req *GenerateRequest) {
	if req == nil {
		return
	}
	req.Marketplace = strings.ToLower(strings.TrimSpace(req.Marketplace))
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
}
