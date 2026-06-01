package runtime

import (
	"errors"
	"testing"

	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestBuildTemporalRuntimeBundleFromModulesCollectsNamedWorkers(t *testing.T) {
	t.Parallel()

	bundle, err := BuildTemporalRuntimeBundleFromModules(&config.Config{}, []kernelmodule.Module{
		stubTemporalModule{name: "listingkit_publish", enabled: true},
		nil,
		stubTemporalModule{name: "disabled", enabled: false},
	})
	if err != nil {
		t.Fatalf("BuildTemporalRuntimeBundleFromModules() error = %v", err)
	}
	if len(bundle.Workers) != 1 {
		t.Fatalf("workers = %d, want 1", len(bundle.Workers))
	}
	if bundle.Workers[0].Name != "listingkit_publish" {
		t.Fatalf("worker name = %s, want listingkit_publish", bundle.Workers[0].Name)
	}
}

func TestTemporalRuntimeBundleStartStartsWorkersAndReturnsClosers(t *testing.T) {
	t.Parallel()

	started := make([]string, 0, 2)
	stopped := make([]string, 0, 2)
	bundle := TemporalRuntimeBundle{
		Workers: []kernelmodule.NamedTemporalWorker{
			{
				Name: "first",
				Start: func() (func() error, error) {
					started = append(started, "first")
					return func() error {
						stopped = append(stopped, "first")
						return nil
					}, nil
				},
			},
			{
				Name: "second",
				Start: func() (func() error, error) {
					started = append(started, "second")
					return func() error {
						stopped = append(stopped, "second")
						return nil
					}, nil
				},
			},
		},
	}

	closers, err := bundle.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if len(started) != 2 {
		t.Fatalf("started = %d, want 2", len(started))
	}
	if len(closers) != 2 {
		t.Fatalf("closers = %d, want 2", len(closers))
	}
	for i := len(closers) - 1; i >= 0; i-- {
		if err := closers[i](); err != nil {
			t.Fatalf("closer %d error = %v", i, err)
		}
	}
	if len(stopped) != 2 {
		t.Fatalf("stopped = %d, want 2", len(stopped))
	}
}

func TestTemporalRuntimeBundleStartClosesStartedWorkersOnFailure(t *testing.T) {
	t.Parallel()

	stopped := make([]string, 0, 1)
	bundle := TemporalRuntimeBundle{
		Workers: []kernelmodule.NamedTemporalWorker{
			{
				Name: "first",
				Start: func() (func() error, error) {
					return func() error {
						stopped = append(stopped, "first")
						return nil
					}, nil
				},
			},
			{
				Name: "broken",
				Start: func() (func() error, error) {
					return nil, errors.New("boom")
				},
			},
		},
	}

	if _, err := bundle.Start(); err == nil {
		t.Fatal("expected Start() error")
	}
	if len(stopped) != 1 {
		t.Fatalf("stopped = %d, want 1", len(stopped))
	}
}

type stubTemporalModule struct {
	name    string
	enabled bool
}

func (m stubTemporalModule) Name() string { return m.name }

func (m stubTemporalModule) Enabled(*config.Config) bool { return m.enabled }

func (m stubTemporalModule) Register(reg *kernelmodule.Registry) error {
	if reg == nil {
		return nil
	}
	return reg.AddTemporalWorker(m.name, func() (func() error, error) {
		return func() error { return nil }, nil
	})
}
