package submission

import (
	"context"
	"errors"
)

type LeaseAcquireService[TTask any, TPreview any] struct {
	beginLease         func(context.Context, string, string, string) (*TTask, error)
	isReplayExisting   func(error) bool
	isRecoverRemote    func(error) bool
	isMissingPackage   func(error) bool
	buildReplayPreview func(context.Context, *TTask) (*TPreview, error)
	recoverRemote      func(context.Context, *TTask, string) (*TPreview, error)
	buildMissingPkgErr func(error) error
}

type LeaseAcquireServiceConfig[TTask any, TPreview any] struct {
	BeginLease         func(context.Context, string, string, string) (*TTask, error)
	IsReplayExisting   func(error) bool
	IsRecoverRemote    func(error) bool
	IsMissingPackage   func(error) bool
	BuildReplayPreview func(context.Context, *TTask) (*TPreview, error)
	RecoverRemote      func(context.Context, *TTask, string) (*TPreview, error)
	BuildMissingPkgErr func(error) error
}

func NewLeaseAcquireService[TTask any, TPreview any](config LeaseAcquireServiceConfig[TTask, TPreview]) *LeaseAcquireService[TTask, TPreview] {
	return &LeaseAcquireService[TTask, TPreview]{
		beginLease:         config.BeginLease,
		isReplayExisting:   config.IsReplayExisting,
		isRecoverRemote:    config.IsRecoverRemote,
		isMissingPackage:   config.IsMissingPackage,
		buildReplayPreview: config.BuildReplayPreview,
		recoverRemote:      config.RecoverRemote,
		buildMissingPkgErr: config.BuildMissingPkgErr,
	}
}

func (s *LeaseAcquireService[TTask, TPreview]) Acquire(ctx context.Context, taskID, action, requestID string) (*TTask, *TPreview, error) {
	if s == nil || s.beginLease == nil {
		return nil, nil, errors.New("lease acquire service is not configured")
	}
	task, err := s.beginLease(ctx, taskID, action, requestID)
	if err == nil {
		return task, nil, nil
	}
	if s.isReplayExisting != nil && s.isReplayExisting(err) {
		if s.buildReplayPreview == nil {
			return nil, nil, err
		}
		preview, previewErr := s.buildReplayPreview(ctx, task)
		return nil, preview, previewErr
	}
	if s.isRecoverRemote != nil && s.isRecoverRemote(err) {
		if s.recoverRemote == nil {
			return nil, nil, err
		}
		preview, previewErr := s.recoverRemote(ctx, task, action)
		return nil, preview, previewErr
	}
	if s.isMissingPackage != nil && s.isMissingPackage(err) && s.buildMissingPkgErr != nil {
		return nil, nil, s.buildMissingPkgErr(err)
	}
	return nil, nil, err
}
