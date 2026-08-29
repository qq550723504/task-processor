package main

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/imageagent"
	imageagenttemporal "task-processor/internal/imageagent/temporal"
)

func TestRunFailsClosedWhenProductDependenciesAreUnavailable(t *testing.T) {
	want := errors.New("image agent provider runtime unavailable")
	started := false
	err := run(context.Background(), func(imageagenttemporal.WorkerWireMode) (resolvedDependencies, error) {
		return resolvedDependencies{}, want
	}, func(context.Context, appruntime.ImageAgentTemporalDependencies, appruntime.ImageAgentTemporalWorkerOptions, *logrus.Logger) error {
		started = true
		return nil
	}, appruntime.ImageAgentTemporalWorkerOptions{WireMode: "v3", TaskQueue: "image-agent-manual-v3"}, logrus.New())
	require.ErrorIs(t, err, want)
	require.False(t, started)
}

func TestRunPassesResolvedDependenciesToTemporalRuntime(t *testing.T) {
	want := appruntime.ImageAgentTemporalDependencies{Repository: commandRepository{}}
	called := false
	closed := false
	err := run(context.Background(), func(mode imageagenttemporal.WorkerWireMode) (resolvedDependencies, error) {
		require.Equal(t, imageagenttemporal.WorkerWireModeV2, mode)
		return resolvedDependencies{dependencies: want, close: func() error { closed = true; return nil }}, nil
	}, func(_ context.Context, got appruntime.ImageAgentTemporalDependencies, options appruntime.ImageAgentTemporalWorkerOptions, _ *logrus.Logger) error {
		called = true
		require.Equal(t, want.Repository, got.Repository)
		require.Equal(t, appruntime.ImageAgentTemporalWorkerOptions{WireMode: "v2", TaskQueue: "image-agent-manual"}, options)
		return errors.New("worker stopped")
	}, appruntime.ImageAgentTemporalWorkerOptions{WireMode: "v2", TaskQueue: "image-agent-manual"}, logrus.New())
	require.EqualError(t, err, "worker stopped")
	require.True(t, called)
	require.True(t, closed)
}

func TestRunForwardsV2AndV3ProcessConfigurationWithoutAmbientDefaults(t *testing.T) {
	for _, test := range []struct {
		name    string
		options appruntime.ImageAgentTemporalWorkerOptions
	}{
		{name: "v2", options: appruntime.ImageAgentTemporalWorkerOptions{WireMode: "v2", TaskQueue: "image-agent-manual"}},
		{name: "v3", options: appruntime.ImageAgentTemporalWorkerOptions{WireMode: "v3", TaskQueue: "image-agent-manual-v3"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			err := run(context.Background(), func(mode imageagenttemporal.WorkerWireMode) (resolvedDependencies, error) {
				require.Equal(t, test.options.WireMode, mode)
				return resolvedDependencies{dependencies: appruntime.ImageAgentTemporalDependencies{Repository: commandRepository{}}}, nil
			}, func(_ context.Context, _ appruntime.ImageAgentTemporalDependencies, got appruntime.ImageAgentTemporalWorkerOptions, _ *logrus.Logger) error {
				called = true
				require.Equal(t, test.options, got)
				return nil
			}, test.options, logrus.New())
			require.NoError(t, err)
			require.True(t, called)
		})
	}
}

func TestRunRejectsMissingExplicitProcessWorkerConfigurationBeforeResolvingDependencies(t *testing.T) {
	for _, test := range []struct {
		name    string
		options appruntime.ImageAgentTemporalWorkerOptions
		want    string
	}{
		{name: "missing mode", options: appruntime.ImageAgentTemporalWorkerOptions{TaskQueue: "image-agent-manual-v3"}, want: "wire mode is required"},
		{name: "missing queue", options: appruntime.ImageAgentTemporalWorkerOptions{WireMode: "v3"}, want: "task queue is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolved := false
			err := run(context.Background(), func(imageagenttemporal.WorkerWireMode) (resolvedDependencies, error) {
				resolved = true
				return resolvedDependencies{}, nil
			}, func(context.Context, appruntime.ImageAgentTemporalDependencies, appruntime.ImageAgentTemporalWorkerOptions, *logrus.Logger) error {
				return nil
			}, test.options, logrus.New())
			require.ErrorContains(t, err, test.want)
			require.False(t, resolved)
		})
	}
}

func TestRunCanaryDoesNotResolveProductDependenciesAndForwardsV3Queue(t *testing.T) {
	called := false
	err := runCanary(context.Background(), func(_ context.Context, _ *logrus.Logger, queue string) error {
		called = true
		require.Equal(t, "image-agent-manual-v3-canary", queue)
		return nil
	}, logrus.New(), "image-agent-manual-v3-canary")
	require.NoError(t, err)
	require.True(t, called)
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
