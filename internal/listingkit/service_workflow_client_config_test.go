package listingkit

import "testing"

func TestConfigureWorkflowClientsSyncRootAndDependencyGroups(t *testing.T) {
	t.Parallel()

	t.Run("task submitter mirrors root and task deps", func(t *testing.T) {
		t.Parallel()

		svc := &service{}
		submitter := noopTaskSubmitter{}

		svc.SetTaskSubmitter(submitter)
		if svc.taskSubmitter != submitter {
			t.Fatalf("root task submitter = %v, want assigned submitter", svc.taskSubmitter)
		}
		if svc.taskDeps.taskSubmitter != submitter {
			t.Fatalf("task deps submitter = %v, want assigned submitter", svc.taskDeps.taskSubmitter)
		}

		svc.SetTaskSubmitter(nil)
		if svc.taskSubmitter != nil {
			t.Fatalf("root task submitter = %v, want nil", svc.taskSubmitter)
		}
		if svc.taskDeps.taskSubmitter != nil {
			t.Fatalf("task deps submitter = %v, want nil", svc.taskDeps.taskSubmitter)
		}
	})

	t.Run("shein publish workflow mirrors enabled state", func(t *testing.T) {
		t.Parallel()

		svc := &service{}
		client := &stubSheinPublishWorkflowClient{}

		svc.ConfigureSheinPublishWorkflowClient(client, true)
		if svc.sheinPublishWorkflowClient != client || !svc.sheinPublishWorkflowEnabled {
			t.Fatalf("root shein workflow = (%v, %v), want assigned+enabled", svc.sheinPublishWorkflowClient, svc.sheinPublishWorkflowEnabled)
		}
		if svc.submissionDeps.sheinPublishWorkflowClient != client || !svc.submissionDeps.sheinPublishWorkflowEnabled {
			t.Fatalf("submission deps shein workflow = (%v, %v), want assigned+enabled", svc.submissionDeps.sheinPublishWorkflowClient, svc.submissionDeps.sheinPublishWorkflowEnabled)
		}

		svc.ConfigureSheinPublishWorkflowClient(nil, true)
		if svc.sheinPublishWorkflowClient != nil || svc.sheinPublishWorkflowEnabled {
			t.Fatalf("root shein workflow = (%v, %v), want nil+disabled", svc.sheinPublishWorkflowClient, svc.sheinPublishWorkflowEnabled)
		}
		if svc.submissionDeps.sheinPublishWorkflowClient != nil || svc.submissionDeps.sheinPublishWorkflowEnabled {
			t.Fatalf("submission deps shein workflow = (%v, %v), want nil+disabled", svc.submissionDeps.sheinPublishWorkflowClient, svc.submissionDeps.sheinPublishWorkflowEnabled)
		}
	})

	t.Run("standard workflow mirrors enabled state", func(t *testing.T) {
		t.Parallel()

		svc := &service{}
		client := &stubStandardProductWorkflowClient{}

		svc.ConfigureStandardProductWorkflowClient(client, true)
		if svc.standardProductWorkflowClient != client || !svc.standardProductWorkflowEnabled {
			t.Fatalf("root standard workflow = (%v, %v), want assigned+enabled", svc.standardProductWorkflowClient, svc.standardProductWorkflowEnabled)
		}
		if svc.taskDeps.standardWorkflowClient != client || !svc.taskDeps.standardWorkflowEnabled {
			t.Fatalf("task deps standard workflow = (%v, %v), want assigned+enabled", svc.taskDeps.standardWorkflowClient, svc.taskDeps.standardWorkflowEnabled)
		}

		svc.ConfigureStandardProductWorkflowClient(nil, true)
		if svc.standardProductWorkflowClient != nil || svc.standardProductWorkflowEnabled {
			t.Fatalf("root standard workflow = (%v, %v), want nil+disabled", svc.standardProductWorkflowClient, svc.standardProductWorkflowEnabled)
		}
		if svc.taskDeps.standardWorkflowClient != nil || svc.taskDeps.standardWorkflowEnabled {
			t.Fatalf("task deps standard workflow = (%v, %v), want nil+disabled", svc.taskDeps.standardWorkflowClient, svc.taskDeps.standardWorkflowEnabled)
		}
	})

	t.Run("platform adapt workflow mirrors enabled state", func(t *testing.T) {
		t.Parallel()

		svc := &service{}
		client := &stubPlatformAdaptWorkflowClient{}

		svc.ConfigurePlatformAdaptWorkflowClient(client, true)
		if svc.platformAdaptWorkflowClient != client || !svc.platformAdaptWorkflowEnabled {
			t.Fatalf("root platform adapt workflow = (%v, %v), want assigned+enabled", svc.platformAdaptWorkflowClient, svc.platformAdaptWorkflowEnabled)
		}
		if svc.taskDeps.platformAdaptWorkflowClient != client || !svc.taskDeps.platformAdaptWorkflowEnabled {
			t.Fatalf("task deps platform adapt workflow = (%v, %v), want assigned+enabled", svc.taskDeps.platformAdaptWorkflowClient, svc.taskDeps.platformAdaptWorkflowEnabled)
		}

		svc.ConfigurePlatformAdaptWorkflowClient(nil, true)
		if svc.platformAdaptWorkflowClient != nil || svc.platformAdaptWorkflowEnabled {
			t.Fatalf("root platform adapt workflow = (%v, %v), want nil+disabled", svc.platformAdaptWorkflowClient, svc.platformAdaptWorkflowEnabled)
		}
		if svc.taskDeps.platformAdaptWorkflowClient != nil || svc.taskDeps.platformAdaptWorkflowEnabled {
			t.Fatalf("task deps platform adapt workflow = (%v, %v), want nil+disabled", svc.taskDeps.platformAdaptWorkflowClient, svc.taskDeps.platformAdaptWorkflowEnabled)
		}
	})
}
