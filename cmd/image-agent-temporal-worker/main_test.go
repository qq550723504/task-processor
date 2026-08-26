package main

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/imageagent"
)

func TestRunFailsClosedWhenProductDependenciesAreUnavailable(t *testing.T) {
	want := errors.New("product image slot executor is not composed")
	started := false
	err := run(context.Background(), func() (appruntime.ImageAgentTemporalDependencies, error) {
		return appruntime.ImageAgentTemporalDependencies{}, want
	}, func(context.Context, appruntime.ImageAgentTemporalDependencies, *logrus.Logger) error {
		started = true
		return nil
	}, logrus.New())
	require.ErrorIs(t, err, want)
	require.False(t, started)
}

func TestRunPassesResolvedDependenciesToTemporalRuntime(t *testing.T) {
	want := appruntime.ImageAgentTemporalDependencies{Repository: commandRepository{}}
	called := false
	err := run(context.Background(), func() (appruntime.ImageAgentTemporalDependencies, error) {
		return want, nil
	}, func(_ context.Context, got appruntime.ImageAgentTemporalDependencies, _ *logrus.Logger) error {
		called = true
		require.Equal(t, want.Repository, got.Repository)
		return errors.New("worker stopped")
	}, logrus.New())
	require.EqualError(t, err, "worker stopped")
	require.True(t, called)
}

type commandRepository struct{}

func (commandRepository) CreateRun(context.Context, *imageagent.Run) error { return nil }
func (commandRepository) GetRun(context.Context, imageagent.RunScope) (*imageagent.Run, error) {
	return nil, imageagent.ErrRunNotFound
}
func (commandRepository) UpdateRun(context.Context, imageagent.RunScope, int64, imageagent.RunMutation) error {
	return nil
}
func (commandRepository) AppendPlan(context.Context, imageagent.RunScope, int64, imageagent.Plan) error {
	return nil
}
func (commandRepository) SaveSlotResult(context.Context, imageagent.RunScope, int64, imageagent.SlotResult) error {
	return nil
}
func (commandRepository) AppendAttempt(context.Context, imageagent.StepAttempt) error { return nil }
func (commandRepository) AppendEvent(context.Context, imageagent.RunEvent) error      { return nil }
func (commandRepository) AppendProjectionEvent(_ context.Context, event imageagent.RunEvent) (imageagent.RunEvent, error) {
	return event, nil
}
func (commandRepository) SaveAssetCatalog(context.Context, imageagent.RunScope, imageagent.AssetCatalog) error {
	return nil
}
func (commandRepository) GetAssetCatalog(context.Context, imageagent.RunScope) (imageagent.AssetCatalog, error) {
	return imageagent.AssetCatalog{}, nil
}
func (commandRepository) ListEvents(context.Context, imageagent.RunScope, int64, int) ([]imageagent.RunEvent, error) {
	return nil, nil
}
