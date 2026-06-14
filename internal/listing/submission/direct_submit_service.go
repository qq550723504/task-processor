package submission

import (
	"context"
	"errors"
	"time"
)

type DirectSubmitFlowOptions struct {
	Action    string
	RequestID string
	StartedAt time.Time
}

type DirectSubmitFlowInput[TTask, TPackage any] struct {
	TaskID   string
	Task     TTask
	Package  TPackage
	Options  DirectSubmitFlowOptions
	Platform string
}

type DirectSubmitFlowPhases struct {
	PrepareProduct string
	UploadImages   string
	PreValidate    string
	SubmitRemote   string
}

type DirectSubmitFlowService[TTask, TPackage, TProductAPI, TProduct, TPreview any] struct {
	phases           DirectSubmitFlowPhases
	buildProductAPI  func(context.Context, string, TTask, TPackage, DirectSubmitFlowOptions) (TProductAPI, error)
	persistPhase     func(context.Context, string, TTask, TPackage, DirectSubmitFlowOptions, string) error
	prepareProduct   func(context.Context, string, TTask, TPackage, DirectSubmitFlowOptions) (TProduct, error)
	needsImageUpload func(TProduct) bool
	uploadImages     func(context.Context, string, TTask, TPackage, TProduct, DirectSubmitFlowOptions) error
	preValidate      func(context.Context, string, TTask, TPackage, TProduct, DirectSubmitFlowOptions) error
	submitRemote     func(context.Context, string, TTask, TPackage, TProductAPI, TProduct, DirectSubmitFlowOptions) error
	buildTaskPreview func(context.Context, TTask, string) (TPreview, error)
}

type DirectSubmitFlowServiceConfig[TTask, TPackage, TProductAPI, TProduct, TPreview any] struct {
	Phases           DirectSubmitFlowPhases
	BuildProductAPI  func(context.Context, string, TTask, TPackage, DirectSubmitFlowOptions) (TProductAPI, error)
	PersistPhase     func(context.Context, string, TTask, TPackage, DirectSubmitFlowOptions, string) error
	PrepareProduct   func(context.Context, string, TTask, TPackage, DirectSubmitFlowOptions) (TProduct, error)
	NeedsImageUpload func(TProduct) bool
	UploadImages     func(context.Context, string, TTask, TPackage, TProduct, DirectSubmitFlowOptions) error
	PreValidate      func(context.Context, string, TTask, TPackage, TProduct, DirectSubmitFlowOptions) error
	SubmitRemote     func(context.Context, string, TTask, TPackage, TProductAPI, TProduct, DirectSubmitFlowOptions) error
	BuildTaskPreview func(context.Context, TTask, string) (TPreview, error)
}

func NewDirectSubmitFlowService[TTask, TPackage, TProductAPI, TProduct, TPreview any](config DirectSubmitFlowServiceConfig[TTask, TPackage, TProductAPI, TProduct, TPreview]) *DirectSubmitFlowService[TTask, TPackage, TProductAPI, TProduct, TPreview] {
	return &DirectSubmitFlowService[TTask, TPackage, TProductAPI, TProduct, TPreview]{
		phases:           config.Phases,
		buildProductAPI:  config.BuildProductAPI,
		persistPhase:     config.PersistPhase,
		prepareProduct:   config.PrepareProduct,
		needsImageUpload: config.NeedsImageUpload,
		uploadImages:     config.UploadImages,
		preValidate:      config.PreValidate,
		submitRemote:     config.SubmitRemote,
		buildTaskPreview: config.BuildTaskPreview,
	}
}

func (s *DirectSubmitFlowService[TTask, TPackage, TProductAPI, TProduct, TPreview]) Submit(
	ctx context.Context,
	in DirectSubmitFlowInput[TTask, TPackage],
) (TPreview, error) {
	var zero TPreview
	if s == nil {
		return zero, errors.New("direct submit flow service is not configured")
	}
	if s.buildProductAPI == nil || s.persistPhase == nil || s.prepareProduct == nil || s.preValidate == nil || s.submitRemote == nil || s.buildTaskPreview == nil {
		return zero, errors.New("direct submit flow service is incomplete")
	}

	productAPI, err := s.buildProductAPI(ctx, in.TaskID, in.Task, in.Package, in.Options)
	if err != nil {
		return zero, err
	}
	if err := s.persistPhase(ctx, in.TaskID, in.Task, in.Package, in.Options, s.phases.PrepareProduct); err != nil {
		return zero, err
	}
	submitProduct, err := s.prepareProduct(ctx, in.TaskID, in.Task, in.Package, in.Options)
	if err != nil {
		return zero, err
	}
	if s.needsImageUpload != nil && s.needsImageUpload(submitProduct) {
		if err := s.persistPhase(ctx, in.TaskID, in.Task, in.Package, in.Options, s.phases.UploadImages); err != nil {
			return zero, err
		}
		if s.uploadImages != nil {
			if err := s.uploadImages(ctx, in.TaskID, in.Task, in.Package, submitProduct, in.Options); err != nil {
				return zero, err
			}
		}
	}
	if err := s.persistPhase(ctx, in.TaskID, in.Task, in.Package, in.Options, s.phases.PreValidate); err != nil {
		return zero, err
	}
	if err := s.preValidate(ctx, in.TaskID, in.Task, in.Package, submitProduct, in.Options); err != nil {
		return zero, err
	}
	if err := s.persistPhase(ctx, in.TaskID, in.Task, in.Package, in.Options, s.phases.SubmitRemote); err != nil {
		return zero, err
	}
	if err := s.submitRemote(ctx, in.TaskID, in.Task, in.Package, productAPI, submitProduct, in.Options); err != nil {
		return zero, err
	}
	return s.buildTaskPreview(ctx, in.Task, in.Platform)
}
