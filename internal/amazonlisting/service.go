package amazonlisting

import (
	"fmt"
	"strings"
)

type service struct {
	repo             Repository
	assembler        Assembler
	exportBuilder    ExportBuilder
	listingSubmitter ListingSubmitter
	validator        Validator
	autoFixer        AutoFixer
	workflow         ListingWorkflow
	taskSubmitter    TaskSubmitter
}

type ServiceConfig struct {
	Repository                   Repository
	ProductSnapshotReader        ProductSnapshotReader
	ApprovedAssetInventoryReader ApprovedAssetInventoryReader
	Assembler                    Assembler
	ExportBuilder                ExportBuilder
	ListingSubmitter             ListingSubmitter
	Validator                    Validator
	AutoFixer                    AutoFixer
	Workflow                     ListingWorkflow
	TaskSubmitter                TaskSubmitter
}

func NewService(config *ServiceConfig) (Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Repository == nil {
		return nil, fmt.Errorf("repository cannot be nil")
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
	if config.Workflow == nil {
		config.Workflow = NewListingWorkflow(config.ProductSnapshotReader, config.ApprovedAssetInventoryReader, config.Assembler, config.AutoFixer, config.ExportBuilder)
	}
	return &service{
		repo:             config.Repository,
		assembler:        config.Assembler,
		exportBuilder:    config.ExportBuilder,
		listingSubmitter: config.ListingSubmitter,
		validator:        config.Validator,
		autoFixer:        config.AutoFixer,
		workflow:         config.Workflow,
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
	req.ProductKey = strings.TrimSpace(req.ProductKey)
	if req.Country == "" {
		req.Country = "US"
	}
	if req.Language == "" {
		req.Language = "en_US"
	}
}
