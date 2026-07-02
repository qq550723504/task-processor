package config

import "testing"

func TestProcessorSchedulerEnabledDefaultsToTrue(t *testing.T) {
	cfg := NewDefaultConfig()
	if !cfg.Processor.SchedulerEnabled {
		t.Fatal("expected processor scheduler to be enabled by default")
	}
}

func TestBuildConfigReadsProcessorSchedulerEnabled(t *testing.T) {
	v := newViper()
	v.Set("processor.schedulerEnabled", false)

	cfg := BuildConfig(v)

	if cfg.Processor.SchedulerEnabled {
		t.Fatal("expected processor scheduler switch to be loaded from config")
	}
}
