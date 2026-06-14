package submission

import "context"

type PreparedPayload[TProduct, TSnapshot any] struct {
	Product          TProduct
	NeedsImageUpload bool
	Snapshot         TSnapshot
}

type PayloadStagePhases struct {
	PrepareProduct string
	UploadImages   string
	PreValidate    string
}

type PayloadStageContext[TTask, TPackage any] struct {
	TaskID    string
	Task      TTask
	Package   TPackage
	Action    string
	RequestID string
}

type PayloadStageService[TTask, TPackage, TProduct, TSnapshot any] struct {
	phases                 PayloadStagePhases
	persistPhase           func(context.Context, PayloadStageContext[TTask, TPackage], string) error
	preparePayload         func(context.Context, PayloadStageContext[TTask, TPackage]) (*PreparedPayload[TProduct, TSnapshot], error)
	persistSnapshot        func(context.Context, PayloadStageContext[TTask, TPackage], TSnapshot) error
	requirePreparedPayload func(*PreparedPayload[TProduct, TSnapshot]) error
	uploadImages           func(context.Context, PayloadStageContext[TTask, TPackage], TProduct) error
	finalizeUploaded       func(context.Context, PayloadStageContext[TTask, TPackage], *PreparedPayload[TProduct, TSnapshot]) (*PreparedPayload[TProduct, TSnapshot], error)
	preValidate            func(context.Context, PayloadStageContext[TTask, TPackage], TProduct) error
}

type PayloadStageServiceConfig[TTask, TPackage, TProduct, TSnapshot any] struct {
	Phases                 PayloadStagePhases
	PersistPhase           func(context.Context, PayloadStageContext[TTask, TPackage], string) error
	PreparePayload         func(context.Context, PayloadStageContext[TTask, TPackage]) (*PreparedPayload[TProduct, TSnapshot], error)
	PersistSnapshot        func(context.Context, PayloadStageContext[TTask, TPackage], TSnapshot) error
	RequirePreparedPayload func(*PreparedPayload[TProduct, TSnapshot]) error
	UploadImages           func(context.Context, PayloadStageContext[TTask, TPackage], TProduct) error
	FinalizeUploaded       func(context.Context, PayloadStageContext[TTask, TPackage], *PreparedPayload[TProduct, TSnapshot]) (*PreparedPayload[TProduct, TSnapshot], error)
	PreValidate            func(context.Context, PayloadStageContext[TTask, TPackage], TProduct) error
}

func NewPayloadStageService[TTask, TPackage, TProduct, TSnapshot any](config PayloadStageServiceConfig[TTask, TPackage, TProduct, TSnapshot]) *PayloadStageService[TTask, TPackage, TProduct, TSnapshot] {
	return &PayloadStageService[TTask, TPackage, TProduct, TSnapshot]{
		phases:                 config.Phases,
		persistPhase:           config.PersistPhase,
		preparePayload:         config.PreparePayload,
		persistSnapshot:        config.PersistSnapshot,
		requirePreparedPayload: config.RequirePreparedPayload,
		uploadImages:           config.UploadImages,
		finalizeUploaded:       config.FinalizeUploaded,
		preValidate:            config.PreValidate,
	}
}

func (s *PayloadStageService[TTask, TPackage, TProduct, TSnapshot]) Prepare(ctx context.Context, in PayloadStageContext[TTask, TPackage]) (*PreparedPayload[TProduct, TSnapshot], error) {
	if err := s.persistPhase(ctx, in, s.phases.PrepareProduct); err != nil {
		return nil, err
	}
	payload, err := s.preparePayload(ctx, in)
	if err != nil {
		return nil, err
	}
	if s.persistSnapshot != nil {
		if err := s.persistSnapshot(ctx, in, payload.Snapshot); err != nil {
			return nil, err
		}
	}
	return payload, nil
}

func (s *PayloadStageService[TTask, TPackage, TProduct, TSnapshot]) UploadImages(ctx context.Context, in PayloadStageContext[TTask, TPackage], payload *PreparedPayload[TProduct, TSnapshot]) (*PreparedPayload[TProduct, TSnapshot], error) {
	if s.requirePreparedPayload != nil {
		if err := s.requirePreparedPayload(payload); err != nil {
			return nil, err
		}
	}
	if payload == nil || !payload.NeedsImageUpload {
		return payload, nil
	}
	if err := s.persistPhase(ctx, in, s.phases.UploadImages); err != nil {
		return nil, err
	}
	if err := s.uploadImages(ctx, in, payload.Product); err != nil {
		return nil, err
	}
	out, err := s.finalizeUploaded(ctx, in, payload)
	if err != nil {
		return nil, err
	}
	if s.persistSnapshot != nil {
		if err := s.persistSnapshot(ctx, in, out.Snapshot); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *PayloadStageService[TTask, TPackage, TProduct, TSnapshot]) PreValidate(ctx context.Context, in PayloadStageContext[TTask, TPackage], payload *PreparedPayload[TProduct, TSnapshot]) error {
	if s.requirePreparedPayload != nil {
		if err := s.requirePreparedPayload(payload); err != nil {
			return err
		}
	}
	if err := s.persistPhase(ctx, in, s.phases.PreValidate); err != nil {
		return err
	}
	return s.preValidate(ctx, in, payload.Product)
}
