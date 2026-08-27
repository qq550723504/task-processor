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
	want := errors.New("image agent provider runtime unavailable")
	started := false
	err := run(context.Background(), func() (resolvedDependencies, error) {
		return resolvedDependencies{}, want
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
	closed := false
	err := run(context.Background(), func() (resolvedDependencies, error) {
		return resolvedDependencies{dependencies: want, close: func() error { closed = true; return nil }}, nil
	}, func(_ context.Context, got appruntime.ImageAgentTemporalDependencies, _ *logrus.Logger) error {
		called = true
		require.Equal(t, want.Repository, got.Repository)
		return errors.New("worker stopped")
	}, logrus.New())
	require.EqualError(t, err, "worker stopped")
	require.True(t, called)
	require.True(t, closed)
}

type commandRepository struct{}

func (commandRepository) InitializeRun(context.Context, imageagent.ProjectionInitialization) (imageagent.RunProjection, error) {
	return imageagent.RunProjection{}, nil
}
func (commandRepository) GetProjection(context.Context, imageagent.RunScope) (imageagent.RunProjection, error) {
	return imageagent.RunProjection{}, imageagent.ErrRunNotFound
}
func (commandRepository) CommitProjection(context.Context, imageagent.ProjectionCommit) (imageagent.RunProjection, error) {
	return imageagent.RunProjection{}, nil
}
func (commandRepository) GetAssetCatalog(context.Context, imageagent.RunScope) (imageagent.AssetCatalog, error) {
	return imageagent.AssetCatalog{}, nil
}
func (commandRepository) ListEvents(context.Context, imageagent.RunScope, int64, int) ([]imageagent.RunEvent, error) {
	return nil, nil
}
