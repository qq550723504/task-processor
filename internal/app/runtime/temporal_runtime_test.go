package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	sdkclient "go.temporal.io/sdk/client"

	"task-processor/internal/listingkit"
	listingtemporal "task-processor/internal/listingkit/temporal"
	platformtemporal "task-processor/internal/platform/temporal"
)

func TestShouldStartListingKitSheinPublishTemporalWorkerInProcessDefaultsTrue(t *testing.T) {
	t.Setenv(envListingKitTemporalWorker, "")
	if !ShouldStartListingKitSheinPublishTemporalWorkerInProcess() {
		t.Fatal("expected worker-in-process default to true")
	}
}

func TestShouldStartListingKitSheinPublishTemporalWorkerInProcessHonorsFalse(t *testing.T) {
	t.Setenv(envListingKitTemporalWorker, "false")
	if ShouldStartListingKitSheinPublishTemporalWorkerInProcess() {
		t.Fatal("expected worker-in-process to be disabled when env=false")
	}
}

func TestListingKitTemporalDialResolvesEnvironmentIntoPlatformConfig(t *testing.T) {
	t.Setenv(envListingKitTemporalAddress, " temporal.internal:7233 ")
	t.Setenv(envListingKitTemporalNamespace, " listingkit ")
	ctx := context.WithValue(context.Background(), listingKitTemporalContextKey{}, "request")
	var gotContext context.Context
	var gotConfig platformtemporal.Config
	client, closeFn, config, err := dialListingKitTemporalSDKClient(ctx, func(callContext context.Context, callConfig platformtemporal.Config) (sdkclient.Client, func() error, error) {
		gotContext = callContext
		gotConfig = callConfig
		return nil, func() error { return nil }, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if client != nil || closeFn == nil {
		t.Fatalf("client nil = %t, closeFn nil = %t; want true, false", client == nil, closeFn == nil)
	}
	if gotContext != ctx || gotContext.Value(listingKitTemporalContextKey{}) != "request" {
		t.Fatal("dial context was not forwarded")
	}
	want := platformtemporal.Config{Address: "temporal.internal:7233", Namespace: "listingkit"}
	if gotConfig != want || config != want {
		t.Fatalf("dial config, returned config = %#v, %#v; want %#v", gotConfig, config, want)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}

func TestListingKitTemporalDialRejectsMissingCloseOwner(t *testing.T) {
	client, closeFn, _, err := dialListingKitTemporalSDKClient(context.Background(), func(context.Context, platformtemporal.Config) (sdkclient.Client, func() error, error) {
		return nil, nil, nil
	})
	if err == nil || !strings.Contains(err.Error(), "close owner") {
		t.Fatalf("error = %v, want missing close owner", err)
	}
	if client != nil || closeFn != nil {
		t.Fatalf("client nil = %t, closeFn nil = %t; want true, true", client == nil, closeFn == nil)
	}
}

func TestListingKitTemporalWorkerRejectsMissingCloseOwnerBeforeHostConstruction(t *testing.T) {
	t.Setenv(envListingKitTemporalEnabled, "true")
	hostCalls := 0
	dependencies := successfulListingKitTemporalDependencies(new(int))
	dependencies.Dial = func(context.Context, platformtemporal.Config) (sdkclient.Client, func() error, error) {
		return nil, nil, nil
	}
	dependencies.NewActivityHost = func(any) (listingkit.SheinPublishActivityHost, error) {
		hostCalls++
		return nil, nil
	}
	closeFn, err := startListingKitSheinPublishTemporalWorkerWithDependencies(context.Background(), struct{}{}, nil, dependencies)
	if err == nil || !strings.Contains(err.Error(), "close owner") {
		t.Fatalf("error = %v, want missing close owner", err)
	}
	if closeFn != nil || hostCalls != 0 {
		t.Fatalf("closeFn nil = %t, host calls = %d; want true, 0", closeFn == nil, hostCalls)
	}
}

func TestListingKitTemporalWorkerUsesPlatformCloseOwnerOnEveryExit(t *testing.T) {
	t.Setenv(envListingKitTemporalEnabled, "true")
	want := errors.New("stage failed")
	for _, test := range []struct {
		name  string
		build func(*int) listingKitTemporalRuntimeDependencies
	}{
		{name: "activity host error", build: func(closed *int) listingKitTemporalRuntimeDependencies {
			deps := successfulListingKitTemporalDependencies(closed)
			deps.NewActivityHost = func(any) (listingkit.SheinPublishActivityHost, error) { return nil, want }
			return deps
		}},
		{name: "layer host error", build: func(closed *int) listingKitTemporalRuntimeDependencies {
			deps := successfulListingKitTemporalDependencies(closed)
			deps.NewLayerHost = func(any) (listingkit.LayerWorkflowActivityHost, error) { return nil, want }
			return deps
		}},
		{name: "worker construction error", build: func(closed *int) listingKitTemporalRuntimeDependencies {
			deps := successfulListingKitTemporalDependencies(closed)
			deps.NewWorker = func(listingtemporal.WorkerConfig) (listingKitTemporalWorker, error) { return nil, want }
			return deps
		}},
		{name: "worker start error", build: func(closed *int) listingKitTemporalRuntimeDependencies {
			deps := successfulListingKitTemporalDependencies(closed)
			deps.NewWorker = func(listingtemporal.WorkerConfig) (listingKitTemporalWorker, error) {
				return &recordingListingKitTemporalWorker{startErr: want}, nil
			}
			return deps
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			closed := 0
			closeFn, err := startListingKitSheinPublishTemporalWorkerWithDependencies(context.Background(), struct{}{}, nil, test.build(&closed))
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
			if closeFn != nil || closed != 1 {
				t.Fatalf("closeFn nil = %t, client closes = %d; want true, 1", closeFn == nil, closed)
			}
		})
	}

	closed := 0
	worker := &recordingListingKitTemporalWorker{}
	wantClose := errors.New("client close failed")
	deps := successfulListingKitTemporalDependencies(&closed)
	deps.Dial = func(context.Context, platformtemporal.Config) (sdkclient.Client, func() error, error) {
		return nil, func() error { closed++; return wantClose }, nil
	}
	deps.NewWorker = func(listingtemporal.WorkerConfig) (listingKitTemporalWorker, error) { return worker, nil }
	closeFn, err := startListingKitSheinPublishTemporalWorkerWithDependencies(context.Background(), struct{}{}, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := closeFn(); !errors.Is(err, wantClose) {
		t.Fatalf("close error = %v, want %v", err, wantClose)
	}
	if err := closeFn(); !errors.Is(err, wantClose) {
		t.Fatalf("second close error = %v, want %v", err, wantClose)
	}
	if closed != 1 || worker.stops != 1 {
		t.Fatalf("client closes, worker stops = %d, %d; want 1, 1", closed, worker.stops)
	}
}

type listingKitTemporalContextKey struct{}

func successfulListingKitTemporalDependencies(closed *int) listingKitTemporalRuntimeDependencies {
	return listingKitTemporalRuntimeDependencies{
		Dial: func(context.Context, platformtemporal.Config) (sdkclient.Client, func() error, error) {
			return nil, func() error { *closed++; return nil }, nil
		},
		NewActivityHost: func(any) (listingkit.SheinPublishActivityHost, error) { return nil, nil },
		NewLayerHost:    func(any) (listingkit.LayerWorkflowActivityHost, error) { return nil, nil },
		NewWorker: func(listingtemporal.WorkerConfig) (listingKitTemporalWorker, error) {
			return &recordingListingKitTemporalWorker{}, nil
		},
	}
}

type recordingListingKitTemporalWorker struct {
	startErr error
	stops    int
}

func (worker *recordingListingKitTemporalWorker) Start() error { return worker.startErr }
func (worker *recordingListingKitTemporalWorker) Stop()        { worker.stops++ }
