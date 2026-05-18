package runtime

import "testing"

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
