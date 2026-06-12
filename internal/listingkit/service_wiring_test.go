package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestNewServiceInitializesCollaborators(t *testing.T) {
	t.Parallel()

	svc, err := NewService(newTestServiceConfig(&stubSubmitRepo{}))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	impl, ok := svc.(*service)
	if !ok {
		t.Fatalf("service type = %T, want *service", svc)
	}
	if impl.taskLifecycle == nil {
		t.Fatal("expected taskLifecycle to be initialized")
	}
	if impl.taskGeneration == nil {
		t.Fatal("expected taskGeneration to be initialized")
	}
	if impl.taskRevision == nil {
		t.Fatal("expected taskRevision to be initialized")
	}
	if impl.taskPreview == nil {
		t.Fatal("expected taskPreview to be initialized")
	}
	if impl.sdsBaseline == nil {
		t.Fatal("expected sdsBaseline to be initialized")
	}
	if impl.taskStudioSession == nil {
		t.Fatal("expected taskStudioSession to be initialized")
	}
	if impl.studioBatchGeneration == nil {
		t.Fatal("expected studioBatchGeneration to be initialized")
	}
	if impl.taskStudioMedia == nil {
		t.Fatal("expected taskStudioMedia to be initialized")
	}
	if impl.settingsAdmin == nil {
		t.Fatal("expected settingsAdmin to be initialized")
	}
	if impl.sheinAdmin == nil {
		t.Fatal("expected sheinAdmin to be initialized")
	}
	if impl.submission.taskSubmission == nil {
		t.Fatal("expected taskSubmission to be initialized")
	}
	if impl.submission.taskSubmissionRefresh == nil {
		t.Fatal("expected taskSubmissionRefresh to be initialized")
	}
	if impl.submission.taskRecovery == nil {
		t.Fatal("expected taskRecovery to be initialized")
	}
	if impl.submission.taskRequeue == nil {
		t.Fatal("expected taskRequeue to be initialized")
	}
	if impl.submission.taskSubmissionRecovery == nil {
		t.Fatal("expected taskSubmissionRecovery to be initialized")
	}
	if impl.submission.taskSubmissionExecution == nil {
		t.Fatal("expected taskSubmissionExecution to be initialized")
	}
	if impl.submission.taskSubmissionState == nil {
		t.Fatal("expected taskSubmissionState to be initialized")
	}
	if impl.submission.taskDirectSubmission == nil {
		t.Fatal("expected taskDirectSubmission to be initialized")
	}
	if impl.submission.taskTemporalSubmissionAdapter == nil {
		t.Fatal("expected taskTemporalSubmissionAdapter to be initialized")
	}
}

func TestServiceInitializeCollaboratorGroups(t *testing.T) {
	t.Parallel()

	svc := &service{repo: &stubSubmitRepo{}}

	svc.initializeTaskCollaborators()
	if svc.taskLifecycle == nil {
		t.Fatal("expected taskLifecycle to be initialized")
	}
	if svc.taskGeneration == nil {
		t.Fatal("expected taskGeneration to be initialized")
	}
	if svc.taskRevision == nil {
		t.Fatal("expected taskRevision to be initialized")
	}
	if svc.taskPreview == nil {
		t.Fatal("expected taskPreview to be initialized")
	}
	if svc.sdsBaseline == nil {
		t.Fatal("expected sdsBaseline to be initialized")
	}
	if svc.taskStudioSession == nil {
		t.Fatal("expected taskStudioSession to be initialized")
	}
	if svc.studioBatchGeneration == nil {
		t.Fatal("expected studioBatchGeneration to be initialized")
	}
	if svc.taskStudioMedia == nil {
		t.Fatal("expected taskStudioMedia to be initialized")
	}

	svc.initializeAdminCollaborators()
	if svc.settingsAdmin == nil {
		t.Fatal("expected settingsAdmin to be initialized")
	}
	if svc.sheinAdmin == nil {
		t.Fatal("expected sheinAdmin to be initialized")
	}

	svc.initializeSubmitCollaborators()
	if svc.submission.taskRecovery == nil {
		t.Fatal("expected taskRecovery to be initialized")
	}
	if svc.submission.taskRequeue == nil {
		t.Fatal("expected taskRequeue to be initialized")
	}
	if svc.submission.taskSubmission == nil {
		t.Fatal("expected taskSubmission to be initialized")
	}
	if svc.submission.taskSubmissionRefresh == nil {
		t.Fatal("expected taskSubmissionRefresh to be initialized")
	}
	if svc.submission.taskSubmissionRecovery == nil {
		t.Fatal("expected taskSubmissionRecovery to be initialized")
	}
	if svc.submission.taskSubmissionExecution == nil {
		t.Fatal("expected taskSubmissionExecution to be initialized")
	}
	if svc.submission.taskSubmissionState == nil {
		t.Fatal("expected taskSubmissionState to be initialized")
	}
	if svc.submission.taskDirectSubmission == nil {
		t.Fatal("expected taskDirectSubmission to be initialized")
	}

	svc.initializeTemporalCollaborators()
	if svc.submission.taskTemporalSubmissionAdapter == nil {
		t.Fatal("expected taskTemporalSubmissionAdapter to be initialized")
	}
}

func TestServiceRootFileDoesNotOwnCollaboratorGroupInitializationBodies(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("ReadFile(service.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func (s *service) initializeCollaborators() {",
		"func (s *service) initializeTaskCollaborators() {",
		"func (s *service) initializeAdminCollaborators() {",
		"func (s *service) initializeSubmitCollaborators() {",
		"func (s *service) initializeTemporalCollaborators() {",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("service.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *service) SetTaskSubmitter(submitter TaskSubmitter) {",
		"func (s *service) ConfigureSheinPublishWorkflowClient(client SheinPublishWorkflowClient, enabled bool) {",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service.go should keep %q", needle)
		}
	}

	configSrc, err := os.ReadFile("service_config.go")
	if err != nil {
		t.Fatalf("ReadFile(service_config.go) error = %v", err)
	}
	configContent := string(configSrc)

	for _, needle := range []string{
		"func NewService(config *ServiceConfig) (Service, error) {",
		"func newServiceWithConfig(config *ServiceConfig) *service {",
	} {
		if !strings.Contains(configContent, needle) {
			t.Fatalf("service_config.go should keep %q", needle)
		}
	}
}

func TestAdminCollaboratorFilesUseExplicitWiringBuilders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		file         string
		builderCalls []string
		inlineConfig []string
	}{
		{
			name: "admin collaborators",
			file: "service_admin_collaborators.go",
			builderCalls: []string{
				"buildSettingsAdminServiceConfig(s)",
				"buildSheinAdminServiceConfig(s)",
			},
			inlineConfig: []string{
				"newSettingsAdminService(settingsAdminServiceConfig{",
				"newSheinAdminService(sheinAdminServiceConfig{",
			},
		},
		{
			name:         "settings admin service",
			file:         "settings_admin_service.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name:         "shein admin service",
			file:         "shein_admin_service.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
			}
			content := string(src)

			for _, builderCall := range tc.builderCalls {
				if !strings.Contains(content, builderCall) {
					t.Fatalf("%s should contain %q", tc.file, builderCall)
				}
			}
			for _, inlineConfig := range tc.inlineConfig {
				if strings.Contains(content, inlineConfig) {
					t.Fatalf("%s should not contain %q", tc.file, inlineConfig)
				}
			}
		})
	}
}

func TestTaskCollaboratorFilesUseExplicitWiringBuilders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		file         string
		builderCalls []string
		inlineConfig []string
	}{
		{
			name: "task collaborators",
			file: "service_task_collaborators.go",
			builderCalls: []string{
				"buildTaskGenerationServiceConfig(s)",
				"buildTaskRevisionServiceConfig(s)",
				"buildTaskPreviewServiceConfig(s)",
				"buildTaskLifecycleServiceConfig(s)",
				"buildSDSBaselineServiceConfig(s)",
			},
			inlineConfig: []string{
				"newTaskGenerationService(taskGenerationServiceConfig{",
				"newTaskRevisionService(taskRevisionServiceConfig{",
				"newTaskPreviewService(taskPreviewServiceConfig{",
				"newTaskLifecycleService(taskLifecycleServiceConfig{",
			},
		},
		{
			name:         "task service",
			file:         "service_task_export_logic.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name:         "generation service",
			file:         "service_task_generation_support_helpers.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name: "sds baseline service",
			file: "sds_baseline_service.go",
			builderCalls: []string{
				"newSDSBaselineService(",
			},
			inlineConfig: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
			}
			content := string(src)

			for _, builderCall := range tc.builderCalls {
				if !strings.Contains(content, builderCall) {
					t.Fatalf("%s should contain %q", tc.file, builderCall)
				}
			}
			for _, inlineConfig := range tc.inlineConfig {
				if strings.Contains(content, inlineConfig) {
					t.Fatalf("%s should not contain %q", tc.file, inlineConfig)
				}
			}
		})
	}
}

func TestStudioCollaboratorFilesUseExplicitWiringBuilders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		file         string
		builderCalls []string
		inlineConfig []string
	}{
		{
			name: "studio collaborators",
			file: "service_studio_collaborators.go",
			builderCalls: []string{
				"buildTaskStudioSessionServiceConfig(s)",
				"buildTaskStudioBatchDraftServiceConfig(s)",
				"buildTaskStudioMediaServiceConfig(s)",
				"buildStudioBatchGenerationServiceConfig(s)",
				"buildTaskStudioBatchServiceConfig(s)",
				"buildTaskStudioBatchRunServiceConfig(s)",
			},
			inlineConfig: []string{
				"newTaskStudioSessionService(taskStudioSessionServiceConfig{",
				"newTaskStudioBatchDraftService(taskStudioBatchDraftServiceConfig{",
				"newTaskStudioMediaService(taskStudioMediaServiceConfig{",
				"newStudioBatchGenerationService(studioBatchGenerationServiceConfig{",
				"newTaskStudioBatchService(taskStudioBatchServiceConfig{",
				"newTaskStudioBatchRunService(taskStudioBatchRunServiceConfig{",
			},
		},
		{
			name:         "studio session service",
			file:         "studio_session_service.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name:         "studio media",
			file:         "studio_designs.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name:         "studio batch",
			file:         "studio_batch_service.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name:         "studio wiring",
			file:         "service_studio_wiring.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name:         "studio batch run",
			file:         "studio_batch_run_service.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name: "studio batch run coordinator",
			file: "studio_batch_run_coordinator.go",
			builderCalls: []string{
				"buildStudioBatchRunCoordinatorConfig(s)",
				"buildTaskStudioBatchRunExecutorConfig(s)",
			},
			inlineConfig: []string{
				"newStudioBatchRunCoordinator(studioBatchRunCoordinatorConfig{",
				"newTaskStudioBatchRunExecutor(taskStudioBatchRunExecutorConfig{",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
			}
			content := string(src)

			for _, builderCall := range tc.builderCalls {
				if !strings.Contains(content, builderCall) {
					t.Fatalf("%s should contain %q", tc.file, builderCall)
				}
			}
			for _, inlineConfig := range tc.inlineConfig {
				if strings.Contains(content, inlineConfig) {
					t.Fatalf("%s should not contain %q", tc.file, inlineConfig)
				}
			}
		})
	}
}

func TestSubmitCollaboratorFilesUseExplicitWiringBuilders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		file         string
		builderCalls []string
		inlineConfig []string
	}{
		{
			name:         "task recovery service",
			file:         "task_recovery_service.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name:         "task requeue service",
			file:         "task_requeue_service.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name:         "submit facade",
			file:         "service_submit_routing.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name: "submit collaborators",
			file: "service_submit_collaborators.go",
			builderCalls: []string{
				"buildTaskRecoveryServiceConfig(s)",
				"buildTaskRequeueServiceConfig(s)",
				"buildTaskSubmissionServiceConfig(s)",
				"buildTaskSubmissionRefreshServiceConfig(s)",
				"buildTaskSubmissionExecutionServiceConfig(s)",
				"buildTaskSubmissionStateServiceConfig(s)",
				"buildTaskSubmissionRecoveryServiceConfig(s)",
				"buildTaskDirectSubmissionServiceConfig(s)",
				"buildTaskTemporalSubmissionAdapterConfig(s)",
			},
			inlineConfig: []string{
				"newTaskSubmissionService(taskSubmissionServiceConfig{",
				"newTaskSubmissionRefreshService(taskSubmissionRefreshServiceConfig{",
				"newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{",
				"newTaskSubmissionStateService(taskSubmissionStateServiceConfig{",
				"newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{",
				"newTaskDirectSubmissionService(taskDirectSubmissionServiceConfig{",
				"newTaskTemporalSubmissionAdapter(taskTemporalSubmissionAdapterConfig{",
			},
		},
		{
			name:         "temporal submission facade",
			file:         "task_temporal_submission_adapter.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
			}
			content := string(src)

			for _, builderCall := range tc.builderCalls {
				if !strings.Contains(content, builderCall) {
					t.Fatalf("%s should contain %q", tc.file, builderCall)
				}
			}
			for _, inlineConfig := range tc.inlineConfig {
				if strings.Contains(content, inlineConfig) {
					t.Fatalf("%s should not contain %q", tc.file, inlineConfig)
				}
			}
		})
	}
}

func TestSubmitRoutingFileOwnsRootSubmitEntryDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_routing.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_routing.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error) {",
		"return s.taskSubmissionOrDefault().SubmitTask(ctx, taskID, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_submit_routing.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_submit_entrypoint.go"); err == nil {
		t.Fatal("service_submit_entrypoint.go should be removed after submit routing merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_entrypoint.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_submit.go"); err == nil {
		t.Fatal("service_submit.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit.go) unexpected error = %v", err)
	}
}

func TestSubmitLeaseHelperFileOwnsSharedTTLAndSentinelErrors(t *testing.T) {
	t.Parallel()

	helperSrc, err := os.ReadFile("service_submit_lease_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_lease_helper.go) error = %v", err)
	}
	helperContent := string(helperSrc)

	for _, needle := range []string{
		"const sheinSubmitInFlightTTL = submission.InFlightTTL",
		"errSheinSubmitReplayExisting = errors.New(\"shein submit replay existing\")",
		"errSheinSubmitRecoverRemote  = errors.New(\"shein submit recover remote\")",
		"errSheinSubmitMissingPackage = errors.New(\"shein submit missing package\")",
	} {
		if !strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_lease_helper.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_submit_primitives.go"); err == nil {
		t.Fatal("service_submit_primitives.go should be removed after submit lease helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_primitives.go) unexpected error = %v", err)
	}
}

func TestSubmitRuntimeContextFilesUseExplicitResolverSeam(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		file    string
		needles []string
	}{
		{
			name: "submit store context",
			file: "service_submit_settings_resolution_helpers.go",
			needles: []string{
				"buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)",
			},
		},
		{
			name: "shein store client",
			file: "service_submit_remote_context_helpers.go",
			needles: []string{
				"buildSubmitRuntimeContextResolver(s).resolveStoreInfo(ctx, task)",
			},
		},
		{
			name: "submit wiring",
			file: "service_submit_wiring.go",
			needles: []string{
				"resolver := buildSubmitRuntimeContextResolver(s)",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
			}
			content := string(src)
			for _, needle := range tc.needles {
				if !strings.Contains(content, needle) {
					t.Fatalf("%s should contain %q", tc.file, needle)
				}
			}
		})
	}
}

func TestSubmitRoutingFileOwnsRootSubmitDelegates(t *testing.T) {
	t.Parallel()

	routingSrc, err := os.ReadFile("service_submit_routing.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_routing.go) error = %v", err)
	}
	routingContent := string(routingSrc)

	for _, needle := range []string{
		"func (s *service) SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error) {",
		"return s.taskSubmissionOrDefault().SubmitTask(ctx, taskID, req)",
		"func (s *service) RecoverTaskNow(ctx context.Context, taskID string) (*Task, error) {",
		"return s.taskRecoveryOrDefault().RecoverTaskNow(ctx, taskID)",
		"func (s *service) RunRecoverySweep(ctx context.Context, now time.Time, limit int) (int64, error) {",
		"return s.taskRecoveryOrDefault().RunRecoverySweep(ctx, now, limit)",
		"func (s *service) BulkRecoverTasks(ctx context.Context, query *RecoverBlockedTasksQuery) (int64, error) {",
		"return s.taskRecoveryOrDefault().BulkRecoverTasks(ctx, query)",
		"func (s *service) RequeuePendingTasks(ctx context.Context, req *RequeuePendingTasksRequest) (*RequeuePendingTasksResult, error) {",
		"return s.taskRequeueOrDefault().RequeuePendingTasks(ctx, req)",
	} {
		if !strings.Contains(routingContent, needle) {
			t.Fatalf("service_submit_routing.go should contain %q", needle)
		}
	}

	for _, tc := range []struct {
		file    string
		needles []string
	}{
		{
			file: "task_recovery_service.go",
			needles: []string{
				"func (s *service) RecoverTaskNow(ctx context.Context, taskID string) (*Task, error) {",
				"func (s *service) RunRecoverySweep(ctx context.Context, now time.Time, limit int) (int64, error) {",
				"func (s *service) BulkRecoverTasks(ctx context.Context, query *RecoverBlockedTasksQuery) (int64, error) {",
			},
		},
		{
			file: "task_requeue_service.go",
			needles: []string{
				"func (s *service) RequeuePendingTasks(ctx context.Context, req *RequeuePendingTasksRequest) (*RequeuePendingTasksResult, error) {",
			},
		},
	} {
		src, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
		}
		content := string(src)
		for _, needle := range tc.needles {
			if strings.Contains(content, needle) {
				t.Fatalf("%s should not contain %q", tc.file, needle)
			}
		}
	}
}

func TestSubmitWorkflowFileOwnsWorkflowGatingHelpers(t *testing.T) {
	t.Parallel()

	workflowSrc, err := os.ReadFile("service_submit_workflow_entry_helpers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_workflow_entry_helpers.go) error = %v", err)
	}
	workflowContent := string(workflowSrc)

	if !strings.Contains(workflowContent, "func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {") {
		t.Fatalf("service_submit_workflow_entry_helpers.go should contain %q", "func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {")
	}

	if _, err := os.ReadFile("service_submit_workflow_helpers.go"); err == nil {
		t.Fatal("service_submit_workflow_helpers.go should be removed after workflow entry helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_workflow_helpers.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_submit_workflow.go"); err == nil {
		t.Fatal("service_submit_workflow.go should be removed after workflow helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_workflow.go) unexpected error = %v", err)
	}

	routingSrc, err := os.ReadFile("service_submit_routing.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_routing.go) error = %v", err)
	}
	routingContent := string(routingSrc)

	if strings.Contains(routingContent, "func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {") {
		t.Fatalf("service_submit_routing.go should not contain %q", "func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {")
	}
}

func TestStudioBatchRunCoordinatorFileOwnsRunStarter(t *testing.T) {
	t.Parallel()

	coordinatorSrc, err := os.ReadFile("studio_batch_run_coordinator.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_run_coordinator.go) error = %v", err)
	}
	coordinatorContent := string(coordinatorSrc)

	if !strings.Contains(coordinatorContent, "func (s *service) startStudioBatchRun(ctx context.Context, runID string) error {") {
		t.Fatalf("studio_batch_run_coordinator.go should contain %q", "func (s *service) startStudioBatchRun(ctx context.Context, runID string) error {")
	}

	serviceSrc, err := os.ReadFile("studio_batch_run_service.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_run_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	if strings.Contains(serviceContent, "func (s *service) startStudioBatchRun(ctx context.Context, runID string) error {") {
		t.Fatalf("studio_batch_run_service.go should not contain %q", "func (s *service) startStudioBatchRun(ctx context.Context, runID string) error {")
	}
}

func TestStudioBatchRunFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_studio_batch_run_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_batch_run_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) CreateStudioBatchRun(ctx context.Context, req *CreateStudioBatchRunRequest) (*StudioBatchRunRecord, []StudioBatchRunItemRecord, error) {",
		"return s.taskStudioBatchRunOrDefault().CreateStudioBatchRun(ctx, req)",
		"func (s *service) GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error) {",
		"return s.taskStudioBatchRunOrDefault().GetStudioBatchRun(ctx, runID)",
		"func (s *service) ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error) {",
		"return s.taskStudioBatchRunOrDefault().ListStudioBatchRunItems(ctx, runID)",
		"func (s *service) CancelStudioBatchRun(ctx context.Context, runID string) error {",
		"return s.taskStudioBatchRunOrDefault().CancelStudioBatchRun(ctx, runID)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_studio_batch_run_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_studio_batch_run.go"); err == nil {
		t.Fatal("service_studio_batch_run.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_studio_batch_run.go) unexpected error = %v", err)
	}

	serviceSrc, err := os.ReadFile("studio_batch_run_service.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_run_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func (s *service) CreateStudioBatchRun(ctx context.Context, req *CreateStudioBatchRunRequest) (*StudioBatchRunRecord, []StudioBatchRunItemRecord, error) {",
		"func (s *service) GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error) {",
		"func (s *service) ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error) {",
		"func (s *service) CancelStudioBatchRun(ctx context.Context, runID string) error {",
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("studio_batch_run_service.go should not contain %q", needle)
		}
	}
}

func TestStudioBatchFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_studio_batch_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_batch_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"return s.taskStudioBatchOrDefault().GetStudioBatchDetail(ctx, batchID)",
		"func (s *service) StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"return s.taskStudioBatchOrDefault().StartStudioBatchGeneration(ctx, batchID)",
		"func (s *service) PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"return s.taskStudioBatchOrDefault().PrepareStudioBatchGeneration(ctx, batchID)",
		"func (s *service) ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"return s.taskStudioBatchOrDefault().ResumeStudioBatchGeneration(ctx, batchID)",
		"func (s *service) PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {",
		"return s.taskStudioBatchOrDefault().PrepareRetryStudioBatchItems(ctx, batchID, req)",
		"func (s *service) RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {",
		"return s.taskStudioBatchOrDefault().RetryStudioBatchItems(ctx, batchID, req)",
		"func (s *service) ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error) {",
		"return s.taskStudioBatchOrDefault().ApproveStudioBatchDesigns(ctx, batchID, req)",
		"func (s *service) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {",
		"return s.taskStudioBatchOrDefault().CreateStudioBatchTasks(ctx, batchID, req)",
		"func (s *service) PrepareCreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {",
		"return s.taskStudioBatchOrDefault().PrepareCreateStudioBatchTasks(ctx, batchID, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_studio_batch_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_studio_batch.go"); err == nil {
		t.Fatal("service_studio_batch.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_studio_batch.go) unexpected error = %v", err)
	}

	serviceSrc, err := os.ReadFile("studio_batch_service.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func (s *service) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"func (s *service) StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"func (s *service) PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"func (s *service) ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"func (s *service) PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {",
		"func (s *service) RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {",
		"func (s *service) ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error) {",
		"func (s *service) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {",
		"func (s *service) PrepareCreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {",
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("studio_batch_service.go should not contain %q", needle)
		}
	}
}

func TestStudioSessionEntrypointsFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_studio_batch_draft_session_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_batch_draft_session_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error) {",
		"return s.taskStudioBatchDraftOrDefault().ListStudioSessionGallery(ctx, limit)",
		"func (s *service) ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error) {",
		"return s.taskStudioBatchDraftOrDefault().ListStudioBatches(ctx, limit)",
		"func (s *service) GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchDraftDetail, error) {",
		"return s.taskStudioBatchDraftOrDefault().GetStudioBatch(ctx, batchID)",
		"func (s *service) UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*StudioBatchDraftDetail, error) {",
		"return s.taskStudioBatchDraftOrDefault().UpsertStudioBatch(ctx, req)",
		"func (s *service) DeleteStudioBatch(ctx context.Context, batchID string) error {",
		"return s.taskStudioBatchDraftOrDefault().DeleteStudioBatch(ctx, batchID)",
		"func (s *service) SyncStudioDesignAsyncJob(ctx context.Context, sessionID string, jobStatus StudioAsyncJobStatus, jobID string, errMessage string) error {",
		"return s.taskStudioSessionOrDefault().SyncStudioDesignAsyncJob(ctx, sessionID, jobStatus, jobID, errMessage)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_studio_batch_draft_session_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_studio_session.go"); err == nil {
		t.Fatal("service_studio_session.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_studio_session.go) unexpected error = %v", err)
	}

	serviceSrc, err := os.ReadFile("studio_session_service.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_session_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func (s *service) ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error) {",
		"func (s *service) ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error) {",
		"func (s *service) GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchDraftDetail, error) {",
		"func (s *service) UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*StudioBatchDraftDetail, error) {",
		"func (s *service) DeleteStudioBatch(ctx context.Context, batchID string) error {",
		"func (s *service) SyncStudioDesignAsyncJob(ctx context.Context, sessionID string, jobStatus StudioAsyncJobStatus, jobID string, errMessage string) error {",
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("studio_session_service.go should not contain %q", needle)
		}
	}
}

func TestStudioMediaFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_studio_media_generation_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_media_generation_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error) {",
		"return s.taskStudioMediaOrDefault().GenerateStudioDesigns(ctx, req)",
		"func (s *service) GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {",
		"return s.taskStudioMediaOrDefault().GenerateStudioProductImages(ctx, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_studio_media_generation_entrypoints.go should contain %q", needle)
		}
	}

	mediaSrc, err := os.ReadFile("service_studio_media_generation_helpers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_media_generation_helpers.go) error = %v", err)
	}
	mediaContent := string(mediaSrc)

	for _, needle := range []string{
		"func (s *service) GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error) {",
		"func (s *service) GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {",
	} {
		if strings.Contains(mediaContent, needle) {
			t.Fatalf("service_studio_media_generation_helpers.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *service) sanitizeStudioImageInputURLs(ctx context.Context, inputURLs []string) ([]string, error) {",
		"return s.taskStudioMediaOrDefault().sanitizeStudioImageInputURLs(ctx, inputURLs)",
		"func (s *service) generateStudioDesignSiblingThemes(ctx context.Context, req *StudioDesignRequest, count int) ([]string, error) {",
		"return s.taskStudioMediaOrDefault().generateStudioDesignSiblingThemes(ctx, req, count)",
		"func (s *service) generateStudioDesignImage(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {",
		"return s.taskStudioMediaOrDefault().generateStudioDesignImage(ctx, model, promptText, size, referenceURLs)",
		"func (s *service) editStudioDesignImageWithReferences(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {",
		"return s.taskStudioMediaOrDefault().editStudioDesignImageWithReferences(ctx, model, promptText, size, referenceURLs)",
		"func (s *service) generateStudioDesignImageWithoutReferences(ctx context.Context, model string, promptText string, size string) (*openaiclient.ImageResponse, error) {",
		"return s.taskStudioMediaOrDefault().generateStudioDesignImageWithoutReferences(ctx, model, promptText, size)",
		"func (s *service) persistGeneratedStudioImage(ctx context.Context, response *openaiclient.ImageResponse, filename string) (string, string, error) {",
		"return s.taskStudioMediaOrDefault().persistGeneratedStudioImage(ctx, response, filename)",
		"func (s *service) generateOneStudioProductImage(ctx context.Context, req *StudioProductImageRequest, sourceURL string, basePrompt string) (string, error) {",
		"return s.taskStudioMediaOrDefault().generateOneStudioProductImage(ctx, req, sourceURL, basePrompt)",
		"func (s *service) tryGenerateStudioProductImage(ctx context.Context, inputImages []string, promptText string) (*openaiclient.ImageResponse, error) {",
		"return s.taskStudioMediaOrDefault().tryGenerateStudioProductImage(ctx, inputImages, promptText)",
	} {
		if !strings.Contains(mediaContent, needle) {
			t.Fatalf("service_studio_media_generation_helpers.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_studio_media_helpers.go"); err == nil {
		t.Fatal("service_studio_media_helpers.go should be removed after studio media generation helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_studio_media_helpers.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_studio_media.go"); err == nil {
		t.Fatal("service_studio_media.go should be removed after helper file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_studio_media.go) unexpected error = %v", err)
	}

	for _, tc := range []struct {
		file    string
		needles []string
	}{
		{
			file: "studio_designs.go",
			needles: []string{
				"func (s *service) GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error) {",
				"func (s *service) generateStudioDesignSiblingThemes(ctx context.Context, req *StudioDesignRequest, count int) ([]string, error) {",
				"func (s *service) generateStudioDesignImage(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {",
				"func (s *service) editStudioDesignImageWithReferences(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*openaiclient.ImageResponse, error) {",
				"func (s *service) generateStudioDesignImageWithoutReferences(ctx context.Context, model string, promptText string, size string) (*openaiclient.ImageResponse, error) {",
				"func (s *service) persistGeneratedStudioImage(ctx context.Context, response *openaiclient.ImageResponse, filename string) (string, string, error) {",
			},
		},
		{
			file: "studio_product_images.go",
			needles: []string{
				"func (s *service) GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {",
				"func (s *service) generateOneStudioProductImage(ctx context.Context, req *StudioProductImageRequest, sourceURL string, basePrompt string) (string, error) {",
				"func (s *service) tryGenerateStudioProductImage(ctx context.Context, inputImages []string, promptText string) (*openaiclient.ImageResponse, error) {",
			},
		},
		{
			file: "studio_image_input_compat.go",
			needles: []string{
				"func (s *service) sanitizeStudioImageInputURLs(ctx context.Context, inputURLs []string) ([]string, error) {",
			},
		},
	} {
		src, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
		}
		content := string(src)
		for _, needle := range tc.needles {
			if strings.Contains(content, needle) {
				t.Fatalf("%s should not contain %q", tc.file, needle)
			}
		}
	}
}

func TestStoreProfileEntrypointsFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_store_settings_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_store_settings_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) ListSheinStoreProfiles(ctx context.Context) ([]ListingKitStoreProfile, error) {",
		"return s.settingsAdminOrDefault().ListSheinStoreProfiles(ctx)",
		"func (s *service) UpsertSheinStoreProfile(ctx context.Context, req *ListingKitStoreProfile) (*ListingKitStoreProfile, error) {",
		"return s.settingsAdminOrDefault().UpsertSheinStoreProfile(ctx, req)",
		"func (s *service) DeleteSheinStoreProfile(ctx context.Context, id int64) error {",
		"return s.settingsAdminOrDefault().DeleteSheinStoreProfile(ctx, id)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_store_settings_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_store_profile.go"); err == nil {
		t.Fatal("service_store_profile.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_store_profile.go) unexpected error = %v", err)
	}

	legacySrc, err := os.ReadFile("store_profile_service.go")
	if err == nil {
		legacyContent := string(legacySrc)
		for _, needle := range []string{
			"func (s *service) ListSheinStoreProfiles(ctx context.Context) ([]ListingKitStoreProfile, error) {",
			"func (s *service) UpsertSheinStoreProfile(ctx context.Context, req *ListingKitStoreProfile) (*ListingKitStoreProfile, error) {",
			"func (s *service) DeleteSheinStoreProfile(ctx context.Context, id int64) error {",
		} {
			if strings.Contains(legacyContent, needle) {
				t.Fatalf("store_profile_service.go should not contain %q", needle)
			}
		}
	}
}

func TestSheinSettingsEntrypointsFileOwnsAIClientSettingsDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_ai_client_settings_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_ai_client_settings_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error) {",
		"return s.settingsAdminOrDefault().GetAIClientSettings(ctx, scope, clientName)",
		"func (s *service) UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error) {",
		"return s.settingsAdminOrDefault().UpdateAIClientSettings(ctx, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_ai_client_settings_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_ai_client_settings.go"); err == nil {
		t.Fatal("service_ai_client_settings.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_ai_client_settings.go) unexpected error = %v", err)
	}

	helperSrc, err := os.ReadFile("ai_client_settings.go")
	if err != nil {
		t.Fatalf("ReadFile(ai_client_settings.go) error = %v", err)
	}
	helperContent := string(helperSrc)

	for _, needle := range []string{
		"func (s *service) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error) {",
		"func (s *service) UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error) {",
	} {
		if strings.Contains(helperContent, needle) {
			t.Fatalf("ai_client_settings.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func aiSettingsUserID(identity openaiclient.Identity, scope string) string {",
		"func normalizeAISettingsScope(scope string, userID string) string {",
		"func normalizeAIClientName(name string) string {",
	} {
		if !strings.Contains(helperContent, needle) {
			t.Fatalf("ai_client_settings.go should keep %q", needle)
		}
	}
}

func TestSheinSettingsEntrypointsFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_settings_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_settings_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GetSheinSettings(ctx context.Context) (*SheinSettings, error) {",
		"return s.settingsAdminOrDefault().GetSheinSettings(ctx)",
		"func (s *service) UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error) {",
		"return s.settingsAdminOrDefault().UpdateSheinSettings(ctx, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_settings_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_shein_category_search_entrypoint.go"); err == nil {
		t.Fatal("service_shein_category_search_entrypoint.go should be removed after shein admin entrypoint merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_category_search_entrypoint.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_shein_pricing_preview_entrypoint.go"); err == nil {
		t.Fatal("service_shein_pricing_preview_entrypoint.go should be removed after shein admin entrypoint merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_pricing_preview_entrypoint.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_shein_resolution_cache_clear_entrypoint.go"); err == nil {
		t.Fatal("service_shein_resolution_cache_clear_entrypoint.go should be removed after shein admin entrypoint merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_resolution_cache_clear_entrypoint.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_shein_settings.go"); err == nil {
		t.Fatal("service_shein_settings.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_settings.go) unexpected error = %v", err)
	}

	helperSrc, err := os.ReadFile("shein_settings.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_settings.go) error = %v", err)
	}
	helperContent := string(helperSrc)

	for _, needle := range []string{
		"func (s *service) GetSheinSettings(ctx context.Context) (*SheinSettings, error) {",
		"func (s *service) UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error) {",
	} {
		if strings.Contains(helperContent, needle) {
			t.Fatalf("shein_settings.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *service) listSheinStoreOptions(ctx context.Context) []SheinStoreOption {",
		"func tenantIDInt64FromContext(ctx context.Context) (int64, bool) {",
		"func tenantIDInt64FromTask(task *Task) int64 {",
	} {
		if !strings.Contains(helperContent, needle) {
			t.Fatalf("shein_settings.go should keep %q", needle)
		}
	}
}

func TestSubmitTemporalEntrypointsFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_publish_temporal_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_publish_temporal_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) BeginSheinPublishAttempt(ctx context.Context, in SheinPublishAttemptInput) error {",
		"return s.taskTemporalSubmissionAdapterOrDefault().BeginSheinPublishAttempt(ctx, in)",
		"func (s *service) ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error {",
		"return s.taskTemporalSubmissionAdapterOrDefault().ValidateSheinPublishReadiness(ctx, in)",
		"func (s *service) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {",
		"return s.taskTemporalSubmissionAdapterOrDefault().PrepareSheinPublishPayload(ctx, in)",
		"func (s *service) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {",
		"return s.taskTemporalSubmissionAdapterOrDefault().UploadSheinPublishImages(ctx, in)",
		"func (s *service) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {",
		"return s.taskTemporalSubmissionAdapterOrDefault().PreValidateSheinPublish(ctx, in)",
		"func (s *service) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {",
		"return s.taskTemporalSubmissionAdapterOrDefault().SubmitSheinPublishRemote(ctx, in)",
		"func (s *service) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {",
		"return s.taskTemporalSubmissionAdapterOrDefault().PersistSheinPublishSuccess(ctx, in)",
		"func (s *service) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {",
		"return s.taskTemporalSubmissionAdapterOrDefault().PersistSheinPublishFailure(ctx, in)",
		"func (s *service) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {",
		"return s.taskTemporalSubmissionAdapterOrDefault().RefreshSheinPublishRemoteStatus(ctx, in)",
		"func (s *service) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {",
		"return s.taskTemporalSubmissionAdapterOrDefault().BuildSheinTaskPreview(ctx, taskID)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_publish_temporal_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_submit_temporal.go"); err == nil {
		t.Fatal("service_submit_temporal.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_temporal.go) unexpected error = %v", err)
	}

	adapterSrc, err := os.ReadFile("task_temporal_submission_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_temporal_submission_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"func (s *service) BeginSheinPublishAttempt(ctx context.Context, in SheinPublishAttemptInput) error {",
		"func (s *service) ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error {",
		"func (s *service) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {",
		"func (s *service) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {",
		"func (s *service) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {",
		"func (s *service) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {",
		"func (s *service) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {",
		"func (s *service) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {",
		"func (s *service) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {",
		"func (s *service) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {",
	} {
		if strings.Contains(adapterContent, needle) {
			t.Fatalf("task_temporal_submission_adapter.go should not contain %q", needle)
		}
	}

	if !strings.Contains(adapterContent, "func (s *service) loadSheinPublishTaskForTemporal(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {") {
		t.Fatalf("task_temporal_submission_adapter.go should keep %q", "func (s *service) loadSheinPublishTaskForTemporal(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {")
	}

	if _, err := os.ReadFile("service_submit_temporal_task_loader_helper.go"); err == nil {
		t.Fatal("service_submit_temporal_task_loader_helper.go should be removed after temporal task loader merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_temporal_task_loader_helper.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_submit_temporal_loader_helper.go"); err == nil {
		t.Fatal("service_submit_temporal_loader_helper.go should be removed after temporal task loader helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_temporal_loader_helper.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_submit_temporal_adapter.go"); err == nil {
		t.Fatal("service_submit_temporal_adapter.go should be removed after temporal loader helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_temporal_adapter.go) unexpected error = %v", err)
	}
}

func TestTaskPreviewFileOwnsRootDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_task_preview_logic.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_preview_logic.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {",
		"return s.taskPreviewOrDefault().GetTaskPreview(ctx, taskID, platform)",
		"func (s *service) buildTaskPreview(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {",
		"return s.taskPreviewOrDefault().buildTaskPreview(ctx, task, platform)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_task_preview_logic.go should contain %q", needle)
		}
	}

	previewServiceSrc, err := os.ReadFile("task_preview_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_preview_service.go) error = %v", err)
	}
	previewServiceContent := string(previewServiceSrc)
	for _, needle := range []string{
		"type taskPreviewServiceConfig struct {",
		"func newTaskPreviewService(config taskPreviewServiceConfig) *taskPreviewService {",
		"func (s *taskPreviewService) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {",
		"task, err := s.repo.GetTask(ctx, taskID)",
		"func (s *taskPreviewService) buildTaskPreview(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {",
		"preview, err := buildListingKitPreview(task, platform)",
	} {
		if !strings.Contains(previewServiceContent, needle) {
			t.Fatalf("task_preview_service.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_task_preview_facade.go"); err == nil {
		t.Fatal("service_task_preview_facade.go should be removed after task preview logic rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_task_preview_facade.go) unexpected error = %v", err)
	}

	previewSrc, err := os.ReadFile("service_shein_store_resolution_preview_support_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_store_resolution_preview_support_helper.go) error = %v", err)
	}
	previewContent := string(previewSrc)

	if strings.Contains(previewContent, "func (s *service) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {") {
		t.Fatalf("service_shein_store_resolution_preview_support_helper.go should not contain %q", "func (s *service) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {")
	}

	for _, needle := range []string{
		"func (s *service) buildTaskPreview(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {",
		"s.decorateSheinCookieAvailabilityPreview(ctx, task, preview)",
	} {
		if strings.Contains(previewContent, needle) {
			t.Fatalf("service_shein_store_resolution_preview_support_helper.go should not contain %q after preview builder split", needle)
		}
	}

	if _, err := os.ReadFile("service_preview.go"); err == nil {
		t.Fatal("service_preview.go should be removed after shein store resolution preview helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_preview.go) unexpected error = %v", err)
	}
}

func TestTaskPreviewLogicFileOwnsPreviewBuilderHelper(t *testing.T) {
	t.Parallel()

	if _, err := os.ReadFile("service_task_preview_payload_helper.go"); err == nil {
		t.Fatal("service_task_preview_payload_helper.go should be removed after preview builder merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_task_preview_payload_helper.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_task_preview_helper.go"); err == nil {
		t.Fatal("service_task_preview_helper.go should be removed after preview builder helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_task_preview_helper.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_task_preview_builder.go"); err == nil {
		t.Fatal("service_task_preview_builder.go should be removed after preview helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_task_preview_builder.go) unexpected error = %v", err)
	}

	previewSrc, err := os.ReadFile("service_shein_store_resolution_preview_support_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_store_resolution_preview_support_helper.go) error = %v", err)
	}
	previewContent := string(previewSrc)

	for _, needle := range []string{
		"func (s *service) decorateSheinStoreResolutionPreview(ctx context.Context, task *Task, preview *ListingKitPreview) {",
	} {
		if !strings.Contains(previewContent, needle) {
			t.Fatalf("service_shein_store_resolution_preview_support_helper.go should keep %q", needle)
		}
	}
}

func TestPreviewPlatformBuilderRegistryLivesOutsidePreviewBuilderRoot(t *testing.T) {
	t.Parallel()

	builderSrc, err := os.ReadFile("preview_builder.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder.go) error = %v", err)
	}
	builderContent := string(builderSrc)
	if strings.Contains(builderContent, "func buildPreviewPlatformSections(task *Task, preview *ListingKitPreview, selectedPlatform string) error {") {
		t.Fatalf("preview_builder.go should not contain %q", "func buildPreviewPlatformSections(task *Task, preview *ListingKitPreview, selectedPlatform string) error {")
	}

	platformsSrc, err := os.ReadFile("preview_builder_platforms.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_platforms.go) error = %v", err)
	}
	platformsContent := string(platformsSrc)
	for _, needle := range []string{
		"type previewPlatformBuilder interface {",
		"func previewPlatformBuilders() []previewPlatformBuilder {",
		"func buildPreviewPlatformSections(task *Task, preview *ListingKitPreview, selectedPlatform string) error {",
		"for _, builder := range previewPlatformBuilders() {",
	} {
		if !strings.Contains(platformsContent, needle) {
			t.Fatalf("preview_builder_platforms.go should contain %q", needle)
		}
	}

	sectionsSrc, err := os.ReadFile("preview_platform_sections.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_platform_sections.go) error = %v", err)
	}
	sectionsContent := string(sectionsSrc)
	for _, needle := range []string{
		"type previewPlatformBuilder interface {",
		"func previewPlatformBuilders() []previewPlatformBuilder {",
	} {
		if strings.Contains(sectionsContent, needle) {
			t.Fatalf("preview_platform_sections.go should not contain %q", needle)
		}
	}
}

func TestPreviewPlatformSelectionLivesOutsidePreviewBuilderRoot(t *testing.T) {
	t.Parallel()

	builderSrc, err := os.ReadFile("preview_builder.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder.go) error = %v", err)
	}
	builderContent := string(builderSrc)
	for _, needle := range []string{
		"func validateSelectedPreviewPlatform(selectedPlatform string) (string, error) {",
		"func normalizePreviewPlatform(platform string) string {",
		"func shouldBuildPreviewPlatform(selectedPlatform, platform string) bool {",
		"func isSelectedPreviewPlatform(selectedPlatform, platform string) bool {",
	} {
		if strings.Contains(builderContent, needle) {
			t.Fatalf("preview_builder.go should not contain %q", needle)
		}
	}

	selectionSrc, err := os.ReadFile("preview_platform_selection.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_platform_selection.go) error = %v", err)
	}
	selectionContent := string(selectionSrc)
	for _, needle := range []string{
		"func validateSelectedPreviewPlatform(selectedPlatform string) (string, error) {",
		"func normalizePreviewPlatform(platform string) string {",
		"func shouldBuildPreviewPlatform(selectedPlatform, platform string) bool {",
		"func isSelectedPreviewPlatform(selectedPlatform, platform string) bool {",
	} {
		if !strings.Contains(selectionContent, needle) {
			t.Fatalf("preview_platform_selection.go should contain %q", needle)
		}
	}
}

func TestSheinResolutionCachePreviewHelpersLiveOutsideMainSheinPreviewBuilder(t *testing.T) {
	t.Parallel()

	mainSrc, err := os.ReadFile("preview_builder_shein.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein.go) error = %v", err)
	}
	mainContent := string(mainSrc)
	for _, needle := range []string{
		"func buildSheinResolutionCacheSummary(pkg *sheinpub.Package) *SheinResolutionCacheSummary {",
		"func enrichCategoryResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, resolution *sheinpub.CategoryResolution) {",
	} {
		if strings.Contains(mainContent, needle) {
			t.Fatalf("preview_builder_shein.go should not contain %q", needle)
		}
	}

	cacheSrc, err := os.ReadFile("preview_builder_shein_resolution_cache.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein_resolution_cache.go) error = %v", err)
	}
	cacheContent := string(cacheSrc)
	for _, needle := range []string{
		"func buildSheinResolutionCacheSummary(pkg *sheinpub.Package) *SheinResolutionCacheSummary {",
		"func enrichCategoryResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, resolution *sheinpub.CategoryResolution) {",
		"func enrichPricingResolutionCacheInfo(info *sheinpub.ResolutionCacheInfo, review *sheinpub.PricingReview) {",
	} {
		if !strings.Contains(cacheContent, needle) {
			t.Fatalf("preview_builder_shein_resolution_cache.go should contain %q", needle)
		}
	}
}

func TestSheinFinalReviewImageHelpersLiveOutsideMainFinalReviewBuilder(t *testing.T) {
	t.Parallel()

	imageHelpersSrc, err := os.ReadFile("preview_builder_shein_final_review_images.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein_final_review_images.go) error = %v", err)
	}
	imageHelpersContent := string(imageHelpersSrc)

	for _, needle := range []string{
		"func buildSheinFinalReviewImages(draft *sheinpub.RequestDraft, finalDraft *sheinpub.FinalDraft, product *sheinproduct.Product) []SheinFinalReviewImage {",
		"func resolveSheinFinalReviewImageRole(url, role string, main bool, finalDraft *sheinpub.FinalDraft, sizeMapURLs map[string]struct{}) (string, bool) {",
		"func isSheinFinalReviewSwatchRole(role string) bool {",
		"func mergeSheinFinalReviewImage(existing *SheinFinalReviewImage, role string, main bool) {",
	} {
		if !strings.Contains(imageHelpersContent, needle) {
			t.Fatalf("preview_builder_shein_final_review_images.go should contain %q", needle)
		}
	}

	finalReviewSrc, err := os.ReadFile("preview_builder_shein_final_review.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein_final_review.go) error = %v", err)
	}
	finalReviewContent := string(finalReviewSrc)

	for _, needle := range []string{
		"func buildSheinFinalReviewImages(draft *sheinpub.RequestDraft, finalDraft *sheinpub.FinalDraft, product *sheinproduct.Product) []SheinFinalReviewImage {",
		"func resolveSheinFinalReviewImageRole(url, role string, main bool, finalDraft *sheinpub.FinalDraft, sizeMapURLs map[string]struct{}) (string, bool) {",
		"func isSheinFinalReviewSwatchRole(role string) bool {",
		"func mergeSheinFinalReviewImage(existing *SheinFinalReviewImage, role string, main bool) {",
	} {
		if strings.Contains(finalReviewContent, needle) {
			t.Fatalf("preview_builder_shein_final_review.go should not contain %q", needle)
		}
	}
}

func TestSheinImageUploadPreviewHelpersLiveOutsideSubmitImageRuntime(t *testing.T) {
	t.Parallel()

	previewHelpersSrc, err := os.ReadFile("preview_builder_shein_image_upload.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein_image_upload.go) error = %v", err)
	}
	previewHelpersContent := string(previewHelpersSrc)

	for _, needle := range []string{
		"func buildSheinImageUploadPreflight(pkg *SheinPackage) *SheinImageUploadPreflight {",
		"func collectSheinProductImageURLs(product *sheinproduct.Product) []string {",
		"func appendSheinImageInfoURLs(urls []string, info *sheinproduct.ImageInfo) []string {",
		"func buildSheinImageUploadPreflightSummary(report *SheinImageUploadPreflight) []string {",
	} {
		if !strings.Contains(previewHelpersContent, needle) {
			t.Fatalf("preview_builder_shein_image_upload.go should contain %q", needle)
		}
	}

	submitImagesSrc, err := os.ReadFile("shein_submit_images.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_images.go) error = %v", err)
	}
	submitImagesContent := string(submitImagesSrc)

	for _, needle := range []string{
		"func buildSheinImageUploadPreflight(pkg *SheinPackage) *SheinImageUploadPreflight {",
		"func collectSheinProductImageURLs(product *sheinproduct.Product) []string {",
		"func appendSheinImageInfoURLs(urls []string, info *sheinproduct.ImageInfo) []string {",
		"func buildSheinImageUploadPreflightSummary(report *SheinImageUploadPreflight) []string {",
	} {
		if strings.Contains(submitImagesContent, needle) {
			t.Fatalf("shein_submit_images.go should not contain %q", needle)
		}
	}
}

func TestSheinSettingsEntrypointsFileOwnsCategorySearchDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_settings_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_settings_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {",
		"return s.sheinAdminOrDefault().SearchSheinCategories(ctx, taskID, query)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_settings_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_shein_category_search_entrypoint.go"); err == nil {
		t.Fatal("service_shein_category_search_entrypoint.go should be removed after shein admin entrypoint merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_category_search_entrypoint.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_shein_category_search.go"); err == nil {
		t.Fatal("service_shein_category_search.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_category_search.go) unexpected error = %v", err)
	}

	categorySrc, err := os.ReadFile("service_shein_category_search_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_category_search_support.go) error = %v", err)
	}
	categoryContent := string(categorySrc)

	if strings.Contains(categoryContent, "func (s *service) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {") {
		t.Fatalf("service_shein_category_search_support.go should not contain %q", "func (s *service) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {")
	}

	for _, needle := range []string{
		"func (s *service) buildSheinAttributeAPI(ctx context.Context, task *Task) (sheinpub.AttributeAPI, error) {",
		"func (s *service) buildSheinCategoryAPI(ctx context.Context, task *Task) (sheincategory.CategoryAPI, error) {",
	} {
		if strings.Contains(categoryContent, needle) {
			t.Fatalf("service_shein_category_search_support.go should not contain %q after facade split", needle)
		}
	}

	for _, needle := range []string{
		"type sheinCategorySearchMatch struct {",
		"func searchSheinCategoryCandidates(nodes []sheincategory.CategoryTreeNode, query string) []SheinCategorySearchCandidate {",
		"func sheinCategoryMatchScore(path []string, normalizedQuery string, tokens []string) (int, bool) {",
	} {
		if !strings.Contains(categoryContent, needle) {
			t.Fatalf("service_shein_category_search_support.go should keep %q", needle)
		}
	}
}

func TestSheinCategoryClientHelpersFileOwnsRootHelpers(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_category_api_helpers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_category_api_helpers.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) buildSheinAttributeAPI(ctx context.Context, task *Task) (sheinpub.AttributeAPI, error) {",
		"baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)",
		"return sheinattribute.NewClient(baseAPI), nil",
		"func (s *service) buildSheinCategoryAPI(ctx context.Context, task *Task) (sheincategory.CategoryAPI, error) {",
		"return sheincategory.NewClient(baseAPI), nil",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_category_api_helpers.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_shein_category_client_helpers.go"); err == nil {
		t.Fatal("service_shein_category_client_helpers.go should be removed after category api helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_category_client_helpers.go) unexpected error = %v", err)
	}
}

func TestSubmitSettingsHelpersFileOwnsStoreSelectionResolvers(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_settings_resolution_helpers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_settings_resolution_helpers.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinStoreID(ctx context.Context, task *Task) (int64, error) {",
		"return buildSubmitRuntimeContextResolver(s).resolveStoreID(ctx, task)",
		"func (s *service) resolveSheinStoreProfile(ctx context.Context, task *Task) (*ListingKitStoreProfile, error) {",
		"return buildSubmitRuntimeContextResolver(s).resolveStoreProfile(ctx, task)",
		"func (s *service) resolveSheinStoreSelection(ctx context.Context, task *Task) (*sheinStoreSelection, error) {",
		"return buildSubmitRuntimeContextResolver(s).resolveStoreSelection(ctx, task)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_submit_settings_resolution_helpers.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_shein_store_selection_resolvers.go"); err == nil {
		t.Fatal("service_shein_store_selection_resolvers.go should be removed after submit settings helper merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_store_selection_resolvers.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_shein_store_selection_helpers.go"); err == nil {
		t.Fatal("service_shein_store_selection_helpers.go should be removed after store selection resolver rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_store_selection_helpers.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_shein_category_store_selection_support.go"); err == nil {
		t.Fatal("service_shein_category_store_selection_support.go should be removed after category/store support split")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_category_store_selection_support.go) unexpected error = %v", err)
	}

	categorySrc, err := os.ReadFile("service_shein_store_resolution_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_store_resolution_support.go) error = %v", err)
	}
	categoryContent := string(categorySrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinStoreID(ctx context.Context, task *Task) (int64, error) {",
		"func (s *service) resolveSheinStoreProfile(ctx context.Context, task *Task) (*ListingKitStoreProfile, error) {",
		"func (s *service) resolveSheinStoreSelection(ctx context.Context, task *Task) (*sheinStoreSelection, error) {",
	} {
		if strings.Contains(categoryContent, needle) {
			t.Fatalf("service_shein_store_resolution_support.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"type sheinStoreSelection struct {",
		"func selectionFromSnapshot(snapshot *SheinStoreResolutionSnapshot) *sheinStoreSelection {",
	} {
		if !strings.Contains(categoryContent, needle) {
			t.Fatalf("service_shein_store_resolution_support.go should keep %q", needle)
		}
	}
}

func TestSubmitSettingsHelpersFileOwnsDefaultActionResolver(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_settings_resolution_helpers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_settings_resolution_helpers.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) resolveDefaultSheinSubmitAction(ctx context.Context, taskID string) (string, error) {",
		"task, err := s.repo.GetTask(ctx, taskID)",
		"if action := sheinPreferredSubmitAction(task, buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)); action != \"\" {",
		"return \"publish\", nil",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_submit_settings_resolution_helpers.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_submit_default_action_resolver_helper.go"); err == nil {
		t.Fatal("service_submit_default_action_resolver_helper.go should be removed after submit default action resolver merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_default_action_resolver_helper.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_submit_default_action_helpers.go"); err == nil {
		t.Fatal("service_submit_default_action_helpers.go should be removed after submit default action helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_default_action_helpers.go) unexpected error = %v", err)
	}

	helperSrc, err := os.ReadFile("service_submit_action_normalization_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_action_normalization_helper.go) error = %v", err)
	}
	helperContent := string(helperSrc)

	if strings.Contains(helperContent, "func (s *service) resolveDefaultSheinSubmitAction(ctx context.Context, taskID string) (string, error) {") {
		t.Fatalf("service_submit_action_normalization_helper.go should not contain %q", "func (s *service) resolveDefaultSheinSubmitAction(ctx context.Context, taskID string) (string, error) {")
	}

	for _, needle := range []string{
		"func sheinPreferredSubmitAction(task *Task, settings SheinSettings) string {",
		"func normalizePreferredSheinSubmitAction(action string) string {",
	} {
		if !strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_action_normalization_helper.go should keep %q", needle)
		}
	}

	if _, err := os.ReadFile("service_submit_default_action.go"); err == nil {
		t.Fatal("service_submit_default_action.go should be removed after submit action normalization helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_default_action.go) unexpected error = %v", err)
	}
}

func TestSheinCookiePreviewHelpersFileOwnsRootHelper(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_cookie_preview_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_cookie_preview_helper.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) decorateSheinCookieAvailabilityPreview(ctx context.Context, task *Task, preview *ListingKitPreview) {",
		"note := s.resolveSheinCookieAvailabilityNote(ctx, task)",
		"rebuilt := buildSheinPreviewPayload(",
		"preview.NeedsReview = preview.NeedsReview || rebuilt.NeedsReview",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_cookie_preview_helper.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_shein_cookie_preview_helpers.go"); err == nil {
		t.Fatal("service_shein_cookie_preview_helpers.go should be removed after cookie preview helper singular rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_cookie_preview_helpers.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_shein_cookie_preview.go"); err == nil {
		t.Fatal("service_shein_cookie_preview.go should be removed after cookie preview helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_cookie_preview.go) unexpected error = %v", err)
	}
}

func TestSheinCookieNoteHelperFileOwnsCookieAvailabilityResolver(t *testing.T) {
	t.Parallel()

	noteSrc, err := os.ReadFile("service_shein_cookie_availability_note_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_cookie_availability_note_helper.go) error = %v", err)
	}
	noteContent := string(noteSrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinCookieAvailabilityNote(ctx context.Context, task *Task) string {",
		"apiClient, _, err := s.newSheinAPIClient(ctx, task)",
		"return fmt.Sprintf(\"SHEIN 店铺 cookie 不可用，在线类目、属性和销售属性解析受阻：%v\", err)",
		"return \"SHEIN 店铺 cookie 不可用，在线类目、属性和销售属性解析受阻：刷新后仍未获取到有效 cookie\"",
	} {
		if !strings.Contains(noteContent, needle) {
			t.Fatalf("service_shein_cookie_availability_note_helper.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_shein_cookie_note_helper.go"); err == nil {
		t.Fatal("service_shein_cookie_note_helper.go should be removed after cookie availability note helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_cookie_note_helper.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_shein_cookie_note.go"); err == nil {
		t.Fatal("service_shein_cookie_note.go should be removed after cookie note helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_cookie_note.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_shein_cookie_preview.go"); err == nil {
		t.Fatal("service_shein_cookie_preview.go should be removed after cookie preview helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_cookie_preview.go) unexpected error = %v", err)
	}
}

func TestSubmitSettingsContextHelpersFileOwnsRootHelpers(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_settings_resolution_helpers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_settings_resolution_helpers.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinSubmitSettings(ctx context.Context, task *Task) SheinSettings {",
		"return buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)",
		"func (s *service) resolveSheinWarehouseCode(ctx context.Context, task *Task, site string) string {",
		"return buildSubmitRuntimeContextResolver(s).resolveWarehouseCode(ctx, task, site)",
		"func (s *service) resolveSheinStoreID(ctx context.Context, task *Task) (int64, error) {",
		"return buildSubmitRuntimeContextResolver(s).resolveStoreID(ctx, task)",
		"func (s *service) resolveSheinStoreProfile(ctx context.Context, task *Task) (*ListingKitStoreProfile, error) {",
		"return buildSubmitRuntimeContextResolver(s).resolveStoreProfile(ctx, task)",
		"func (s *service) resolveSheinStoreSelection(ctx context.Context, task *Task) (*sheinStoreSelection, error) {",
		"return buildSubmitRuntimeContextResolver(s).resolveStoreSelection(ctx, task)",
		"func (s *service) resolveDefaultSheinSubmitAction(ctx context.Context, taskID string) (string, error) {",
		"if action := sheinPreferredSubmitAction(task, buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)); action != \"\" {",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_submit_settings_resolution_helpers.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_submit_settings_context_helpers.go"); err == nil {
		t.Fatal("service_submit_settings_context_helpers.go should be removed after submit settings resolution helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_settings_context_helpers.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_submit_store_context_helpers.go"); err == nil {
		t.Fatal("service_submit_store_context_helpers.go should be removed after submit settings context helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_store_context_helpers.go) unexpected error = %v", err)
	}

	helperSrc, err := os.ReadFile("service_submit_warehouse_selection_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_warehouse_selection_helper.go) error = %v", err)
	}
	helperContent := string(helperSrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinSubmitSettings(ctx context.Context, task *Task) SheinSettings {",
		"func (s *service) resolveSheinWarehouseCode(ctx context.Context, task *Task, site string) string {",
	} {
		if strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_warehouse_selection_helper.go should not contain %q", needle)
		}
	}

	if !strings.Contains(helperContent, "func pickSheinWarehouseCode(warehouses *sheinwarehouse.WarehouseResponse, site string) string {") {
		t.Fatalf("service_submit_warehouse_selection_helper.go should keep %q", "func pickSheinWarehouseCode(warehouses *sheinwarehouse.WarehouseResponse, site string) string {")
	}

	if _, err := os.ReadFile("service_submit_warehouse_helper.go"); err == nil {
		t.Fatal("service_submit_warehouse_helper.go should be removed after warehouse code helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_warehouse_helper.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_submit_store_context.go"); err == nil {
		t.Fatal("service_submit_store_context.go should be removed after warehouse helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_store_context.go) unexpected error = %v", err)
	}
}

func TestSubmitContextHelpersFileOwnsRootHelpers(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_remote_context_helpers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_remote_context_helpers.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinStoreInfo(ctx context.Context, task *Task) (*SheinStoreInfo, error) {",
		"return buildSubmitRuntimeContextResolver(s).resolveStoreInfo(ctx, task)",
		"func (s *service) newSheinAPIClient(ctx context.Context, task *Task) (*sheinclient.APIClient, int64, error) {",
		"return buildSubmitRuntimeContextResolver(s).newAPIClient(ctx, task)",
		"func (s *service) buildSheinSubmitOtherAPI(ctx context.Context, task *Task) (sheinother.OtherAPI, error) {",
		"resolver := buildSubmitRuntimeContextResolver(s)",
		"baseAPI := NewSheinRuntimeBaseAPIClient(apiClient, storeID)",
		"return sheinother.NewClient(baseAPI), nil",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_submit_remote_context_helpers.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_submit_context_helpers.go"); err == nil {
		t.Fatal("service_submit_context_helpers.go should be removed after submit runtime context helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_context_helpers.go) unexpected error = %v", err)
	}

	helperSrc, err := os.ReadFile("service_submit_runtime_context_resolver.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_runtime_context_resolver.go) error = %v", err)
	}
	helperContent := string(helperSrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinStoreInfo(ctx context.Context, task *Task) (*SheinStoreInfo, error) {",
		"func (s *service) newSheinAPIClient(ctx context.Context, task *Task) (*sheinclient.APIClient, int64, error) {",
		"func (s *service) buildSheinSubmitOtherAPI(ctx context.Context, task *Task) (sheinother.OtherAPI, error) {",
	} {
		if strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_runtime_context_resolver.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildSubmitRuntimeContextResolver(s *service) *submitRuntimeContextResolver {",
		"func (r *submitRuntimeContextResolver) resolveSubmitSettings(ctx context.Context, task *Task) SheinSettings {",
		"func (r *submitRuntimeContextResolver) resolveStoreInfo(ctx context.Context, task *Task) (*SheinStoreInfo, error) {",
		"func (r *submitRuntimeContextResolver) newAPIClient(ctx context.Context, task *Task) (*SheinRuntimeAPIClient, int64, error) {",
	} {
		if !strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_runtime_context_resolver.go should keep %q", needle)
		}
	}
}

func TestSubmitIdentityHelperFileOwnsTaskIdentityContextHelper(t *testing.T) {
	t.Parallel()

	helperSrc, err := os.ReadFile("service_submit_task_identity_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_task_identity_helper.go) error = %v", err)
	}
	helperContent := string(helperSrc)

	for _, needle := range []string{
		"func withSheinSubmitTaskIdentity(ctx context.Context, task *Task) (context.Context, error) {",
		"identity := openaiclient.IdentityFromContext(ctx)",
		"ctx = WithTenantID(ctx, tenantID)",
		"return openaiclient.WithIdentity(ctx, identity), nil",
	} {
		if !strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_task_identity_helper.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_submit_identity_helper.go"); err == nil {
		t.Fatal("service_submit_identity_helper.go should be removed after submit task identity helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_identity_helper.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_submit_runtime_context.go"); err == nil {
		t.Fatal("service_submit_runtime_context.go should be removed after submit identity helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_runtime_context.go) unexpected error = %v", err)
	}
}

func TestProcessEntryFileOwnsRootEntry(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_process_entry.go")
	if err != nil {
		t.Fatalf("ReadFile(service_process_entry.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error) {",
		"if task == nil {",
		"return buildListingKitProcessFlow(s).run(ctx, task, log)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_process_entry.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_process_facade.go"); err == nil {
		t.Fatal("service_process_facade.go should be removed after process entry rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_process_facade.go) unexpected error = %v", err)
	}

	_, err = os.ReadFile("service_process_review_helper.go")
	if err == nil {
		t.Fatal("service_process_review_helper.go should be removed after process persistence merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_process_review_helper.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_process_review.go"); err == nil {
		t.Fatal("service_process_review.go should be removed after process review helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_process_review.go) unexpected error = %v", err)
	}

	persistSrc, err := os.ReadFile("service_process_persistence_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_process_persistence_helper.go) error = %v", err)
	}
	persistContent := string(persistSrc)

	for _, needle := range []string{
		"func deriveProcessTerminalStatus(result *ListingKitResult) TaskStatus {",
		"func applyProcessTerminalResult(result *ListingKitResult, status TaskStatus) *ListingKitResult {",
		"func (s *service) persistProcessFailure(ctx context.Context, taskID string, result *ListingKitResult, err error) error {",
		"func (s *service) persistProcessSuccess(ctx context.Context, taskID string, result *ListingKitResult) error {",
		"func taskNeedsReviewReason(result *ListingKitResult) string {",
	} {
		if !strings.Contains(persistContent, needle) {
			t.Fatalf("service_process_persistence_helper.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_process_outcome.go"); err == nil {
		t.Fatal("service_process_outcome.go should be removed after process persistence helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_process_outcome.go) unexpected error = %v", err)
	}

	runnerSrc, err := os.ReadFile("service_process_runner_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_process_runner_helper.go) error = %v", err)
	}
	runnerContent := string(runnerSrc)

	for _, needle := range []string{
		"func buildListingKitProcessFlow(s *service) *listingKitProcessFlow {",
		"func (f *listingKitProcessFlow) run(ctx context.Context, task *Task, log *logrus.Entry) (*ListingKitResult, error) {",
		"func (f *listingKitProcessFlow) claimTask(ctx context.Context, task *Task) error {",
		"func processWarningCount(result *ListingKitResult) int {",
	} {
		if !strings.Contains(runnerContent, needle) {
			t.Fatalf("service_process_runner_helper.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_process_flow.go"); err == nil {
		t.Fatal("service_process_flow.go should be removed after process runner helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_process_flow.go) unexpected error = %v", err)
	}
}

func TestTaskLayersFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_task_layers_logic.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_layers_logic.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) ProcessStandardProductLayer(ctx context.Context, taskID string) (*StandardProductSnapshot, error) {",
		"ctx, task, err := s.loadTaskExecutionContext(ctx, taskID)",
		"func (s *service) ProcessPlatformAdaptationLayer(ctx context.Context, taskID string, platform string) (*ListingKitResult, error) {",
		"if err := s.persistProcessedTaskResult(ctx, task.ID, result); err != nil {",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_task_layers_logic.go should contain %q", needle)
		}
	}

	layersSrc, err := os.ReadFile("service_task_layer_processing_helpers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_layer_processing_helpers.go) error = %v", err)
	}
	layersContent := string(layersSrc)

	for _, needle := range []string{
		"func (s *service) ProcessStandardProductLayer(ctx context.Context, taskID string) (*StandardProductSnapshot, error) {",
		"func (s *service) ProcessPlatformAdaptationLayer(ctx context.Context, taskID string, platform string) (*ListingKitResult, error) {",
	} {
		if strings.Contains(layersContent, needle) {
			t.Fatalf("service_task_layer_processing_helpers.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *service) loadTaskExecutionContext(ctx context.Context, taskID string) (context.Context, *Task, error) {",
		"func (s *service) persistProcessedTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {",
	} {
		if !strings.Contains(layersContent, needle) {
			t.Fatalf("service_task_layer_processing_helpers.go should keep %q", needle)
		}
	}
}

func TestUploadedImageFileOwnsRootLogic(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_upload_logic.go")
	if err != nil {
		t.Fatalf("ReadFile(service_upload_logic.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) UploadImages(ctx context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error) {",
		"func (s *service) GetUploadedImage(ctx context.Context, key string) (*UploadedImageFile, error) {",
		"func (s *service) DeleteUploadedImage(ctx context.Context, key string) (*DeletedUploadedImage, error) {",
		"return &DeletedUploadedImage{Key: stored.Key, Size: stored.Size}, nil",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_upload_logic.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_upload_facade.go"); err == nil {
		t.Fatal("service_upload_facade.go should be removed after upload logic rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_upload_facade.go) unexpected error = %v", err)
	}

	uploadSrc, err := os.ReadFile("upload_service.go")
	if err != nil {
		t.Fatalf("ReadFile(upload_service.go) error = %v", err)
	}
	uploadContent := string(uploadSrc)

	for _, needle := range []string{
		"func (s *service) UploadImages(ctx context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error) {",
		"func (s *service) GetUploadedImage(ctx context.Context, key string) (*UploadedImageFile, error) {",
		"func (s *service) DeleteUploadedImage(ctx context.Context, key string) (*DeletedUploadedImage, error) {",
	} {
		if strings.Contains(uploadContent, needle) {
			t.Fatalf("upload_service.go should not contain %q", needle)
		}
	}

	if !strings.Contains(uploadContent, "func buildUploadedImagePath(key string) string {") {
		t.Fatalf("upload_service.go should keep %q", "func buildUploadedImagePath(key string) string {")
	}
}

func TestSheinSettingsEntrypointsFileOwnsPricingDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_settings_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_settings_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {",
		"return s.sheinAdminOrDefault().PreviewSheinPrice(ctx, taskID, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_settings_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_shein_pricing_preview_entrypoint.go"); err == nil {
		t.Fatal("service_shein_pricing_preview_entrypoint.go should be removed after shein admin entrypoint merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_pricing_preview_entrypoint.go) unexpected error = %v", err)
	}

	pricingSrc, err := os.ReadFile("shein_pricing.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_pricing.go) error = %v", err)
	}
	pricingContent := string(pricingSrc)

	if strings.Contains(pricingContent, "func (s *service) PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {") {
		t.Fatalf("shein_pricing.go should not contain %q", "func (s *service) PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {")
	}

	if !strings.Contains(pricingContent, "func (s *service) currentSheinPricingRule() sheinpub.PricingRule {") {
		t.Fatalf("shein_pricing.go should keep %q", "func (s *service) currentSheinPricingRule() sheinpub.PricingRule {")
	}
}

func TestSheinImageRegenerationFileOwnsRootLogic(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_data_image_regeneration_logic.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_data_image_regeneration_logic.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) RegenerateSheinDataImage(ctx context.Context, taskID string, req *RegenerateSheinDataImageRequest) (*RegenerateSheinDataImageResponse, error) {",
		"productReq, role := buildSheinDataImageRegenerationRequest(task, req)",
		"replaced := replaceSheinDataImageURL(task, oldURL, newURL)",
		"return &RegenerateSheinDataImageResponse{",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_data_image_regeneration_logic.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_shein_image_regeneration_facade.go"); err == nil {
		t.Fatal("service_shein_image_regeneration_facade.go should be removed after SHEIN image regeneration logic rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_image_regeneration_facade.go) unexpected error = %v", err)
	}

	regenSrc, err := os.ReadFile("shein_image_regeneration.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_image_regeneration.go) error = %v", err)
	}
	regenContent := string(regenSrc)

	if strings.Contains(regenContent, "func (s *service) RegenerateSheinDataImage(ctx context.Context, taskID string, req *RegenerateSheinDataImageRequest) (*RegenerateSheinDataImageResponse, error) {") {
		t.Fatalf("shein_image_regeneration.go should not contain %q", "func (s *service) RegenerateSheinDataImage(ctx context.Context, taskID string, req *RegenerateSheinDataImageRequest) (*RegenerateSheinDataImageResponse, error) {")
	}

	for _, needle := range []string{
		"func buildSheinDataImageRegenerationRequest(task *Task, req *RegenerateSheinDataImageRequest) (*StudioProductImageRequest, studioProductImageRole) {",
		"func replaceSheinDataImageURL(task *Task, oldURL string, newURL string) int {",
	} {
		if !strings.Contains(regenContent, needle) {
			t.Fatalf("shein_image_regeneration.go should keep %q", needle)
		}
	}
}

func TestSheinSettingsEntrypointsFileOwnsSubmissionEventsDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_settings_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_settings_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {",
		"return s.sheinAdminOrDefault().GetSubmissionEvents(ctx, taskID)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_settings_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_shein_submission_event_listing_entrypoint.go"); err == nil {
		t.Fatal("service_shein_submission_event_listing_entrypoint.go should be removed after shein admin entrypoint merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_submission_event_listing_entrypoint.go) unexpected error = %v", err)
	}

	eventsSrc, err := os.ReadFile("shein_submission_events.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submission_events.go) error = %v", err)
	}
	eventsContent := string(eventsSrc)

	if strings.Contains(eventsContent, "func (s *service) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {") {
		t.Fatalf("shein_submission_events.go should not contain %q", "func (s *service) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {")
	}

	for _, needle := range []string{
		"func sheinSubmissionEventsWithStoreResolution(events []sheinpub.SubmissionEvent, task *Task) []sheinpub.SubmissionEvent {",
		"func sheinSubmissionStoreResolutionFromSnapshot(snapshot *SheinStoreResolutionSnapshot) *sheinpub.SubmissionStoreResolution {",
	} {
		if !strings.Contains(eventsContent, needle) {
			t.Fatalf("shein_submission_events.go should keep %q", needle)
		}
	}
}

func TestSheinSettingsEntrypointsFileOwnsFinalDraftDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_settings_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_settings_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error) {",
		"return s.sheinAdminOrDefault().UpdateSheinFinalDraft(ctx, taskID, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_settings_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_shein_final_draft_update_entrypoint.go"); err == nil {
		t.Fatal("service_shein_final_draft_update_entrypoint.go should be removed after shein admin entrypoint merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_final_draft_update_entrypoint.go) unexpected error = %v", err)
	}

	draftSrc, err := os.ReadFile("shein_final_draft.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_final_draft.go) error = %v", err)
	}
	draftContent := string(draftSrc)

	if strings.Contains(draftContent, "func (s *service) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error) {") {
		t.Fatalf("shein_final_draft.go should not contain %q", "func (s *service) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error) {")
	}

	if !strings.Contains(draftContent, "func applySheinFinalImageDraft(pkg *sheinpub.Package) {") {
		t.Fatalf("shein_final_draft.go should keep %q", "func applySheinFinalImageDraft(pkg *sheinpub.Package) {")
	}
}

func TestSheinSettingsEntrypointsFileOwnsResolutionCacheDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_settings_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_settings_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*SheinResolutionCacheClearResult, error) {",
		"return s.sheinAdminOrDefault().ClearSheinResolutionCache(ctx, taskID, kind)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_settings_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_shein_resolution_cache_clear_entrypoint.go"); err == nil {
		t.Fatal("service_shein_resolution_cache_clear_entrypoint.go should be removed after shein admin entrypoint merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_resolution_cache_clear_entrypoint.go) unexpected error = %v", err)
	}

	cacheSrc, err := os.ReadFile("shein_resolution_cache.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_resolution_cache.go) error = %v", err)
	}
	cacheContent := string(cacheSrc)

	if strings.Contains(cacheContent, "func (s *service) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*SheinResolutionCacheClearResult, error) {") {
		t.Fatalf("shein_resolution_cache.go should not contain %q", "func (s *service) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*SheinResolutionCacheClearResult, error) {")
	}

	for _, needle := range []string{
		"func (s *service) rememberSheinSubmittedResolution(task *Task, action string) {",
		"func (s *service) rememberSheinCategoryResolution(task *Task) {",
		"func (s *service) rememberSheinAttributeResolution(task *Task) {",
		"func (s *service) rememberSheinSaleAttributeResolution(task *Task) {",
	} {
		if !strings.Contains(cacheContent, needle) {
			t.Fatalf("shein_resolution_cache.go should keep %q", needle)
		}
	}
}

func TestChildTaskRetryLogicFileOwnsRootEntry(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_child_task_retry_logic.go")
	if err != nil {
		t.Fatalf("ReadFile(service_child_task_retry_logic.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) RetryTaskChildTask(ctx context.Context, taskID string, req *RetryChildTaskRequest) (*TaskResult, error) {",
		"taskID = strings.TrimSpace(taskID)",
		"switch kind {",
		"return s.persistRetriedChildTaskResult(ctx, task, result, kind, nil)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_child_task_retry_logic.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_child_task_retry_facade.go"); err == nil {
		t.Fatal("service_child_task_retry_facade.go should be removed after child-task retry logic rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_child_task_retry_facade.go) unexpected error = %v", err)
	}

	retrySrc, err := os.ReadFile("service_child_task_retry_helpers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_child_task_retry_helpers.go) error = %v", err)
	}
	retryContent := string(retrySrc)

	if strings.Contains(retryContent, "func (s *service) RetryTaskChildTask(ctx context.Context, taskID string, req *RetryChildTaskRequest) (*TaskResult, error) {") {
		t.Fatalf("service_child_task_retry_helpers.go should not contain %q", "func (s *service) RetryTaskChildTask(ctx context.Context, taskID string, req *RetryChildTaskRequest) (*TaskResult, error) {")
	}

	for _, needle := range []string{
		"func (s *service) retrySDSCatalogProduct(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) error {",
		"func (s *service) retrySDSDesignSync(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) error {",
		"func (s *service) persistRetriedChildTaskResult(ctx context.Context, task *Task, result *ListingKitResult, kind string, retryErr error) (*TaskResult, error) {",
	} {
		if !strings.Contains(retryContent, needle) {
			t.Fatalf("service_child_task_retry_helpers.go should keep %q", needle)
		}
	}

	if _, err := os.ReadFile("service_child_task_retry.go"); err == nil {
		t.Fatal("service_child_task_retry.go should be removed after child-task retry helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_child_task_retry.go) unexpected error = %v", err)
	}
}

func TestSubmitWorkflowHelpersFileOwnsRootHelpers(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_workflow_entry_helpers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_workflow_entry_helpers.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) submitSheinTaskWithWorkflow(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinWorkflowSubmitOptions) (*ListingKitPreview, error) {",
		"return s.taskTemporalSubmissionAdapterOrDefault().startSheinPublishWorkflowAttempt(ctx, taskID, task, req, opts)",
		"func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {",
		"s.sheinPublishWorkflowEnabled &&",
		"action == \"publish\"",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_submit_workflow_entry_helpers.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_submit_workflow.go"); err == nil {
		t.Fatal("service_submit_workflow.go should be removed after workflow helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_workflow.go) unexpected error = %v", err)
	}
}

func TestTaskGenerationFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_task_generation_logic.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_generation_logic.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GetTaskGenerationTasks(ctx context.Context, taskID string, query *GenerationTaskQuery) (*GenerationTaskPage, error) {",
		"return s.taskGenerationOrDefault().GetTaskGenerationTasks(ctx, taskID, query)",
		"func (s *service) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (*GenerationActionExecutionResult, error) {",
		"return s.taskGenerationOrDefault().ExecuteTaskGenerationAction(ctx, taskID, req)",
		"func (s *service) GetTaskGenerationQueue(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationQueuePage, error) {",
		"return s.taskGenerationOrDefault().GetTaskGenerationQueue(ctx, taskID, query)",
		"func (s *service) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewPreviewResponse, error) {",
		"return s.taskGenerationOrDefault().GetTaskGenerationReviewPreview(ctx, taskID, query)",
		"func (s *service) GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewSessionResponse, error) {",
		"return s.taskGenerationOrDefault().GetTaskGenerationReviewSession(ctx, taskID, query)",
		"func (s *service) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error) {",
		"return s.taskGenerationOrDefault().DispatchTaskGenerationNavigation(ctx, taskID, req)",
		"func (s *service) executeGenerationNavigationDispatchPlan(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationNavigationDispatchExecution, error) {",
		"return s.taskGenerationOrDefault().executeGenerationNavigationDispatchPlan(ctx, taskID, target, responseMode)",
		"func (s *service) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *RetryGenerationTasksRequest) (*GenerationTaskPage, error) {",
		"return s.taskGenerationOrDefault().RetryTaskGenerationTasks(ctx, taskID, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_task_generation_logic.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_task_generation.go"); err == nil {
		t.Fatal("service_task_generation.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_task_generation.go) unexpected error = %v", err)
	}

	serviceSrc, err := os.ReadFile("service_task_generation_support_helpers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_generation_support_helpers.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func (s *service) GetTaskGenerationTasks(ctx context.Context, taskID string, query *GenerationTaskQuery) (*GenerationTaskPage, error) {",
		"func (s *service) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (*GenerationActionExecutionResult, error) {",
		"func (s *service) GetTaskGenerationQueue(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationQueuePage, error) {",
		"func (s *service) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewPreviewResponse, error) {",
		"func (s *service) GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewSessionResponse, error) {",
		"func (s *service) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error) {",
		"func (s *service) executeGenerationNavigationDispatchPlan(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationNavigationDispatchExecution, error) {",
		"func (s *service) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *RetryGenerationTasksRequest) (*GenerationTaskPage, error) {",
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("service_task_generation_support_helpers.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func resolveLayerTemporalPlatform(req *ExecuteGenerationActionRequest) string {",
		"func selectGenerationTasksForRetry(existing []assetgeneration.Task, result *ListingKitResult, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {",
		"func (s *service) buildRetryGenerationTaskSelection(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {",
		"func retrySelectionFilter(req *RetryGenerationTasksRequest) listinggeneration.RetrySelectionFilter {",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("service_task_generation_support_helpers.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_generation.go"); err == nil {
		t.Fatal("service_generation.go should be removed after generation helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_generation.go) unexpected error = %v", err)
	}
}

func TestTaskRevisionFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_task_revision_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_revision_entrypoints.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GetTaskRevisionHistory(ctx context.Context, taskID string, query *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error) {",
		"return s.taskRevisionOrDefault().GetTaskRevisionHistory(ctx, taskID, query)",
		"func (s *service) GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *RevisionHistoryDetailQuery) (*ListingKitRevisionHistoryDetail, error) {",
		"return s.taskRevisionOrDefault().GetTaskRevisionHistoryDetail(ctx, taskID, revisionID, query)",
		"func (s *service) ApplyTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*ListingKitPreview, error) {",
		"return s.taskRevisionOrDefault().ApplyTaskRevision(ctx, taskID, req)",
		"func (s *service) ValidateTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*RevisionValidationResult, error) {",
		"return s.taskRevisionOrDefault().ValidateTaskRevision(ctx, taskID, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_task_revision_entrypoints.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_task_revision.go"); err == nil {
		t.Fatal("service_task_revision.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_task_revision.go) unexpected error = %v", err)
	}

	serviceSrc, err := os.ReadFile("service_task_export_logic.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_export_logic.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func (s *service) GetTaskRevisionHistory(ctx context.Context, taskID string, query *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error) {",
		"func (s *service) GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *RevisionHistoryDetailQuery) (*ListingKitRevisionHistoryDetail, error) {",
		"func (s *service) ApplyTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*ListingKitPreview, error) {",
		"func (s *service) ValidateTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*RevisionValidationResult, error) {",
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("service_task_export_logic.go should not contain %q", needle)
		}
	}
}

func TestTaskLifecycleFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_task_lifecycle_logic.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_lifecycle_logic.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error) {",
		"return s.taskLifecycleOrDefault().CreateGenerateTask(ctx, req)",
		"func (s *service) enqueueOrRunStudioTask(ctx context.Context, task *Task) (*Task, error) {",
		"return s.taskLifecycleOrDefault().enqueueOrRunStudioTask(ctx, task)",
		"func (s *service) runTaskInline(ctx context.Context, task *Task) (*Task, error) {",
		"return s.taskLifecycleOrDefault().runTaskInline(ctx, task)",
		"func (s *service) enqueueTask(ctx context.Context, task *Task) error {",
		"return s.taskLifecycleOrDefault().enqueueTask(ctx, task)",
		"func (s *service) GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error) {",
		"return s.taskLifecycleOrDefault().GetTaskResult(ctx, taskID)",
		"func (s *service) ListTasks(ctx context.Context, query *TaskListQuery) (*TaskListPage, error) {",
		"return s.taskLifecycleOrDefault().ListTasks(ctx, query)",
		"func (s *service) GetSDSBaselineReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {",
		"return s.taskLifecycleOrDefault().GetSDSBaselineReadiness(ctx, query)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_task_lifecycle_logic.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_task_lifecycle.go"); err == nil {
		t.Fatal("service_task_lifecycle.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_task_lifecycle.go) unexpected error = %v", err)
	}

	serviceSrc, err := os.ReadFile("service_task_export_logic.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_export_logic.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func (s *service) CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error) {",
		"func (s *service) enqueueOrRunStudioTask(ctx context.Context, task *Task) (*Task, error) {",
		"func (s *service) runTaskInline(ctx context.Context, task *Task) (*Task, error) {",
		"func (s *service) enqueueTask(ctx context.Context, task *Task) error {",
		"func (s *service) GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error) {",
		"func (s *service) ListTasks(ctx context.Context, query *TaskListQuery) (*TaskListPage, error) {",
		"func (s *service) GetSDSBaselineReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {",
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("service_task_export_logic.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *service) GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error) {",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("service_task_export_logic.go should keep %q", needle)
		}
	}
}

func TestTaskSDSBaselineFacadeFileOwnsWarmDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_sds_baseline_warmup_entrypoint.go")
	if err != nil {
		t.Fatalf("ReadFile(service_sds_baseline_warmup_entrypoint.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) WarmSDSBaseline(ctx context.Context, req *WarmSDSBaselineRequest) (*SDSBaselineReadiness, error) {",
		"return s.warmSDSBaseline(ctx, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_sds_baseline_warmup_entrypoint.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_task_sds_baseline.go"); err == nil {
		t.Fatal("service_task_sds_baseline.go should be removed after facade file rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_task_sds_baseline.go) unexpected error = %v", err)
	}

	serviceSrc, err := os.ReadFile("service_task_export_logic.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_export_logic.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	if strings.Contains(serviceContent, "func (s *service) WarmSDSBaseline(ctx context.Context, req *WarmSDSBaselineRequest) (*SDSBaselineReadiness, error) {") {
		t.Fatalf("service_task_export_logic.go should not contain %q", "func (s *service) WarmSDSBaseline(ctx context.Context, req *WarmSDSBaselineRequest) (*SDSBaselineReadiness, error) {")
	}

	for _, needle := range []string{
		"func (s *service) GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error) {",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("service_task_export_logic.go should keep %q", needle)
		}
	}
}
