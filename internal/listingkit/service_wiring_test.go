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
	if impl.taskStudioSession == nil {
		t.Fatal("expected taskStudioSession to be initialized")
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
	if svc.taskStudioSession == nil {
		t.Fatal("expected taskStudioSession to be initialized")
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
				"buildTaskLifecycleServiceConfig(s)",
				"buildSDSBaselineServiceConfig(s)",
			},
			inlineConfig: []string{
				"newTaskGenerationService(taskGenerationServiceConfig{",
				"newTaskRevisionService(taskRevisionServiceConfig{",
				"newTaskLifecycleService(taskLifecycleServiceConfig{",
			},
		},
		{
			name:         "task service",
			file:         "service_task_export.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name:         "generation service",
			file:         "service_generation.go",
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
				"buildTaskStudioBatchServiceConfig(s)",
				"buildTaskStudioBatchRunServiceConfig(s)",
			},
			inlineConfig: []string{
				"newTaskStudioSessionService(taskStudioSessionServiceConfig{",
				"newTaskStudioBatchDraftService(taskStudioBatchDraftServiceConfig{",
				"newTaskStudioMediaService(taskStudioMediaServiceConfig{",
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
			name: "studio wiring",
			file: "service_studio_wiring.go",
			builderCalls: []string{
				"buildStudioBatchGenerationServiceConfig(s)",
			},
			inlineConfig: []string{
				"newStudioBatchGenerationService(studioBatchGenerationServiceConfig{",
			},
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
			file:         "service_submit.go",
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
			file:         "service_submit_temporal_adapter.go",
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

func TestSubmitRuntimeContextFilesUseExplicitResolverSeam(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		file    string
		needles []string
	}{
		{
			name: "submit store context",
			file: "service_submit_store_context_facade.go",
			needles: []string{
				"buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)",
			},
		},
		{
			name: "shein store client",
			file: "service_submit_context_facade.go",
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

	workflowSrc, err := os.ReadFile("service_submit_workflow_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_workflow_facade.go) error = %v", err)
	}
	workflowContent := string(workflowSrc)

	if !strings.Contains(workflowContent, "func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {") {
		t.Fatalf("service_submit_workflow_facade.go should contain %q", "func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {")
	}

	legacySrc, err := os.ReadFile("service_submit_workflow.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_workflow.go) error = %v", err)
	}
	if strings.Contains(string(legacySrc), "func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {") {
		t.Fatalf("service_submit_workflow.go should not contain %q", "func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {")
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

	facadeSrc, err := os.ReadFile("service_studio_batch_run.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_batch_run.go) error = %v", err)
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
			t.Fatalf("service_studio_batch_run.go should contain %q", needle)
		}
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

	facadeSrc, err := os.ReadFile("service_studio_batch.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_batch.go) error = %v", err)
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
			t.Fatalf("service_studio_batch.go should contain %q", needle)
		}
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

func TestStudioSessionFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_studio_session.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_session.go) error = %v", err)
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
			t.Fatalf("service_studio_session.go should contain %q", needle)
		}
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

	facadeSrc, err := os.ReadFile("service_studio_media.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_media.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error) {",
		"return s.taskStudioMediaOrDefault().GenerateStudioDesigns(ctx, req)",
		"func (s *service) GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {",
		"return s.taskStudioMediaOrDefault().GenerateStudioProductImages(ctx, req)",
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
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_studio_media.go should contain %q", needle)
		}
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

func TestStoreProfileFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_store_profile.go")
	if err != nil {
		t.Fatalf("ReadFile(service_store_profile.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) ListSheinStoreProfiles(ctx context.Context) ([]ListingKitStoreProfile, error) {",
		"return s.settingsAdminOrDefault().ListSheinStoreProfiles(ctx)",
		"func (s *service) UpsertSheinStoreProfile(ctx context.Context, req *ListingKitStoreProfile) (*ListingKitStoreProfile, error) {",
		"return s.settingsAdminOrDefault().UpsertSheinStoreProfile(ctx, req)",
		"func (s *service) DeleteSheinStoreProfile(ctx context.Context, id int64) error {",
		"return s.settingsAdminOrDefault().DeleteSheinStoreProfile(ctx, id)",
		"func (s *service) GetSheinStoreRoutingSettings(ctx context.Context) (*ListingKitStoreRoutingSettings, error) {",
		"return s.settingsAdminOrDefault().GetSheinStoreRoutingSettings(ctx)",
		"func (s *service) UpdateSheinStoreRoutingSettings(ctx context.Context, req *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error) {",
		"return s.settingsAdminOrDefault().UpdateSheinStoreRoutingSettings(ctx, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_store_profile.go should contain %q", needle)
		}
	}

	legacySrc, err := os.ReadFile("store_profile_service.go")
	if err == nil {
		legacyContent := string(legacySrc)
		for _, needle := range []string{
			"func (s *service) ListSheinStoreProfiles(ctx context.Context) ([]ListingKitStoreProfile, error) {",
			"func (s *service) UpsertSheinStoreProfile(ctx context.Context, req *ListingKitStoreProfile) (*ListingKitStoreProfile, error) {",
			"func (s *service) DeleteSheinStoreProfile(ctx context.Context, id int64) error {",
			"func (s *service) GetSheinStoreRoutingSettings(ctx context.Context) (*ListingKitStoreRoutingSettings, error) {",
			"func (s *service) UpdateSheinStoreRoutingSettings(ctx context.Context, req *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error) {",
		} {
			if strings.Contains(legacyContent, needle) {
				t.Fatalf("store_profile_service.go should not contain %q", needle)
			}
		}
	}
}

func TestAIClientSettingsFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_ai_client_settings.go")
	if err != nil {
		t.Fatalf("ReadFile(service_ai_client_settings.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error) {",
		"return s.settingsAdminOrDefault().GetAIClientSettings(ctx, scope, clientName)",
		"func (s *service) UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error) {",
		"return s.settingsAdminOrDefault().UpdateAIClientSettings(ctx, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_ai_client_settings.go should contain %q", needle)
		}
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

func TestSheinSettingsFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_settings.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_settings.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GetSheinSettings(ctx context.Context) (*SheinSettings, error) {",
		"return s.settingsAdminOrDefault().GetSheinSettings(ctx)",
		"func (s *service) UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error) {",
		"return s.settingsAdminOrDefault().UpdateSheinSettings(ctx, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_settings.go should contain %q", needle)
		}
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

func TestSubmitTemporalFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_temporal.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_temporal.go) error = %v", err)
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
			t.Fatalf("service_submit_temporal.go should contain %q", needle)
		}
	}

	adapterSrc, err := os.ReadFile("service_submit_temporal_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_temporal_adapter.go) error = %v", err)
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
			t.Fatalf("service_submit_temporal_adapter.go should not contain %q", needle)
		}
	}

	if !strings.Contains(adapterContent, "func (s *service) loadSheinPublishTaskForTemporal(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {") {
		t.Fatalf("service_submit_temporal_adapter.go should keep %q", "func (s *service) loadSheinPublishTaskForTemporal(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {")
	}
}

func TestTaskPreviewFacadeFileOwnsRootDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_task_preview.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_preview.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {",
		"task, err := s.repo.GetTask(ctx, taskID)",
		"preview, err := s.buildTaskPreview(ctx, task, platform)",
		"s.decorateSheinStoreResolutionPreview(ctx, task, preview)",
		"return preview, nil",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_task_preview.go should contain %q", needle)
		}
	}

	previewSrc, err := os.ReadFile("service_preview.go")
	if err != nil {
		t.Fatalf("ReadFile(service_preview.go) error = %v", err)
	}
	previewContent := string(previewSrc)

	if strings.Contains(previewContent, "func (s *service) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {") {
		t.Fatalf("service_preview.go should not contain %q", "func (s *service) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {")
	}

	for _, needle := range []string{
		"func (s *service) buildTaskPreview(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {",
		"s.decorateSheinCookieAvailabilityPreview(ctx, task, preview)",
	} {
		if strings.Contains(previewContent, needle) {
			t.Fatalf("service_preview.go should not contain %q after preview builder split", needle)
		}
	}
}

func TestTaskPreviewBuilderFileOwnsPreviewBuilderHelper(t *testing.T) {
	t.Parallel()

	builderSrc, err := os.ReadFile("service_task_preview_builder.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_preview_builder.go) error = %v", err)
	}
	builderContent := string(builderSrc)

	for _, needle := range []string{
		"func (s *service) buildTaskPreview(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {",
		"preview, err := buildListingKitPreview(task, platform)",
		"s.decorateSheinCookieAvailabilityPreview(ctx, task, preview)",
	} {
		if !strings.Contains(builderContent, needle) {
			t.Fatalf("service_task_preview_builder.go should contain %q", needle)
		}
	}

	previewSrc, err := os.ReadFile("service_preview.go")
	if err != nil {
		t.Fatalf("ReadFile(service_preview.go) error = %v", err)
	}
	previewContent := string(previewSrc)

	for _, needle := range []string{
		"func (s *service) decorateSheinStoreResolutionPreview(ctx context.Context, task *Task, preview *ListingKitPreview) {",
	} {
		if !strings.Contains(previewContent, needle) {
			t.Fatalf("service_preview.go should keep %q", needle)
		}
	}
}

func TestSheinCategorySearchFacadeFileOwnsRootDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_category_search.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_category_search.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {",
		"return s.sheinAdminOrDefault().SearchSheinCategories(ctx, taskID, query)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_category_search.go should contain %q", needle)
		}
	}

	categorySrc, err := os.ReadFile("service_shein_categories.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_categories.go) error = %v", err)
	}
	categoryContent := string(categorySrc)

	if strings.Contains(categoryContent, "func (s *service) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {") {
		t.Fatalf("service_shein_categories.go should not contain %q", "func (s *service) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {")
	}

	for _, needle := range []string{
		"func (s *service) buildSheinAttributeAPI(ctx context.Context, task *Task) (sheinpub.AttributeAPI, error) {",
		"func (s *service) buildSheinCategoryAPI(ctx context.Context, task *Task) (sheincategory.CategoryAPI, error) {",
	} {
		if strings.Contains(categoryContent, needle) {
			t.Fatalf("service_shein_categories.go should not contain %q after facade split", needle)
		}
	}

	for _, needle := range []string{
		"type sheinCategorySearchMatch struct {",
		"func searchSheinCategoryCandidates(nodes []sheincategory.CategoryTreeNode, query string) []SheinCategorySearchCandidate {",
		"func sheinCategoryMatchScore(path []string, normalizedQuery string, tokens []string) (int, bool) {",
	} {
		if !strings.Contains(categoryContent, needle) {
			t.Fatalf("service_shein_categories.go should keep %q", needle)
		}
	}
}

func TestSheinCategoryClientFacadeFileOwnsRootHelpers(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_category_client.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_category_client.go) error = %v", err)
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
			t.Fatalf("service_shein_category_client.go should contain %q", needle)
		}
	}
}

func TestSheinStoreSelectionFacadeFileOwnsRootHelpers(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_store_selection.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_store_selection.go) error = %v", err)
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
			t.Fatalf("service_shein_store_selection.go should contain %q", needle)
		}
	}

	categorySrc, err := os.ReadFile("service_shein_categories.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_categories.go) error = %v", err)
	}
	categoryContent := string(categorySrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinStoreID(ctx context.Context, task *Task) (int64, error) {",
		"func (s *service) resolveSheinStoreProfile(ctx context.Context, task *Task) (*ListingKitStoreProfile, error) {",
		"func (s *service) resolveSheinStoreSelection(ctx context.Context, task *Task) (*sheinStoreSelection, error) {",
	} {
		if strings.Contains(categoryContent, needle) {
			t.Fatalf("service_shein_categories.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"type sheinStoreSelection struct {",
		"func selectionFromSnapshot(snapshot *SheinStoreResolutionSnapshot) *sheinStoreSelection {",
		"func matchStoreProfileForTask(",
	} {
		if !strings.Contains(categoryContent, needle) {
			t.Fatalf("service_shein_categories.go should keep %q", needle)
		}
	}
}

func TestSubmitDefaultActionFacadeFileOwnsRootDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_default_action_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_default_action_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) resolveDefaultSheinSubmitAction(ctx context.Context, taskID string) (string, error) {",
		"task, err := s.repo.GetTask(ctx, taskID)",
		"if action := sheinPreferredSubmitAction(task, buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)); action != \"\" {",
		"return \"publish\", nil",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_submit_default_action_facade.go should contain %q", needle)
		}
	}

	helperSrc, err := os.ReadFile("service_submit_default_action.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_default_action.go) error = %v", err)
	}
	helperContent := string(helperSrc)

	if strings.Contains(helperContent, "func (s *service) resolveDefaultSheinSubmitAction(ctx context.Context, taskID string) (string, error) {") {
		t.Fatalf("service_submit_default_action.go should not contain %q", "func (s *service) resolveDefaultSheinSubmitAction(ctx context.Context, taskID string) (string, error) {")
	}

	for _, needle := range []string{
		"func sheinPreferredSubmitAction(task *Task, settings SheinSettings) string {",
		"func normalizePreferredSheinSubmitAction(action string) string {",
	} {
		if !strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_default_action.go should keep %q", needle)
		}
	}
}

func TestSheinCookiePreviewFacadeFileOwnsRootHelper(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_cookie_preview_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_cookie_preview_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) decorateSheinCookieAvailabilityPreview(ctx context.Context, task *Task, preview *ListingKitPreview) {",
		"note := s.resolveSheinCookieAvailabilityNote(ctx, task)",
		"rebuilt := buildSheinPreviewPayload(",
		"preview.NeedsReview = preview.NeedsReview || rebuilt.NeedsReview",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_cookie_preview_facade.go should contain %q", needle)
		}
	}

	helperSrc, err := os.ReadFile("service_shein_cookie_preview.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_cookie_preview.go) error = %v", err)
	}
	helperContent := string(helperSrc)

	if strings.Contains(helperContent, "func (s *service) decorateSheinCookieAvailabilityPreview(ctx context.Context, task *Task, preview *ListingKitPreview) {") {
		t.Fatalf("service_shein_cookie_preview.go should not contain %q", "func (s *service) decorateSheinCookieAvailabilityPreview(ctx context.Context, task *Task, preview *ListingKitPreview) {")
	}
	if strings.Contains(helperContent, "func (s *service) resolveSheinCookieAvailabilityNote(ctx context.Context, task *Task) string {") {
		t.Fatalf("service_shein_cookie_preview.go should not contain %q", "func (s *service) resolveSheinCookieAvailabilityNote(ctx context.Context, task *Task) string {")
	}
}

func TestSheinCookieNoteFileOwnsCookieAvailabilityResolver(t *testing.T) {
	t.Parallel()

	noteSrc, err := os.ReadFile("service_shein_cookie_note.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_cookie_note.go) error = %v", err)
	}
	noteContent := string(noteSrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinCookieAvailabilityNote(ctx context.Context, task *Task) string {",
		"apiClient, _, err := s.newSheinAPIClient(ctx, task)",
		"return fmt.Sprintf(\"SHEIN 店铺 cookie 不可用，在线类目、属性和销售属性解析受阻：%v\", err)",
		"return \"SHEIN 店铺 cookie 不可用，在线类目、属性和销售属性解析受阻：刷新后仍未获取到有效 cookie\"",
	} {
		if !strings.Contains(noteContent, needle) {
			t.Fatalf("service_shein_cookie_note.go should contain %q", needle)
		}
	}

	previewSrc, err := os.ReadFile("service_shein_cookie_preview.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_cookie_preview.go) error = %v", err)
	}
	previewContent := string(previewSrc)

	if strings.Contains(previewContent, "func (s *service) resolveSheinCookieAvailabilityNote(ctx context.Context, task *Task) string {") {
		t.Fatalf("service_shein_cookie_preview.go should not contain %q", "func (s *service) resolveSheinCookieAvailabilityNote(ctx context.Context, task *Task) string {")
	}
}

func TestSubmitStoreContextFacadeFileOwnsRootHelpers(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_store_context_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_store_context_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinSubmitSettings(ctx context.Context, task *Task) SheinSettings {",
		"return buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)",
		"func (s *service) resolveSheinWarehouseCode(ctx context.Context, task *Task, site string) string {",
		"return buildSubmitRuntimeContextResolver(s).resolveWarehouseCode(ctx, task, site)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_submit_store_context_facade.go should contain %q", needle)
		}
	}

	helperSrc, err := os.ReadFile("service_submit_store_context.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_store_context.go) error = %v", err)
	}
	helperContent := string(helperSrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinSubmitSettings(ctx context.Context, task *Task) SheinSettings {",
		"func (s *service) resolveSheinWarehouseCode(ctx context.Context, task *Task, site string) string {",
	} {
		if strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_store_context.go should not contain %q", needle)
		}
	}

	if !strings.Contains(helperContent, "func pickSheinWarehouseCode(warehouses *sheinwarehouse.WarehouseResponse, site string) string {") {
		t.Fatalf("service_submit_store_context.go should keep %q", "func pickSheinWarehouseCode(warehouses *sheinwarehouse.WarehouseResponse, site string) string {")
	}
}

func TestSubmitContextFacadeFileOwnsRootHelpers(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_context_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_context_facade.go) error = %v", err)
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
			t.Fatalf("service_submit_context_facade.go should contain %q", needle)
		}
	}

	helperSrc, err := os.ReadFile("service_submit_context_resolver.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_context_resolver.go) error = %v", err)
	}
	helperContent := string(helperSrc)

	for _, needle := range []string{
		"func (s *service) resolveSheinStoreInfo(ctx context.Context, task *Task) (*SheinStoreInfo, error) {",
		"func (s *service) newSheinAPIClient(ctx context.Context, task *Task) (*sheinclient.APIClient, int64, error) {",
		"func (s *service) buildSheinSubmitOtherAPI(ctx context.Context, task *Task) (sheinother.OtherAPI, error) {",
	} {
		if strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_context_resolver.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildSubmitRuntimeContextResolver(s *service) *submitRuntimeContextResolver {",
		"func (r *submitRuntimeContextResolver) resolveSubmitSettings(ctx context.Context, task *Task) SheinSettings {",
		"func (r *submitRuntimeContextResolver) resolveStoreInfo(ctx context.Context, task *Task) (*SheinStoreInfo, error) {",
		"func (r *submitRuntimeContextResolver) newAPIClient(ctx context.Context, task *Task) (*SheinRuntimeAPIClient, int64, error) {",
	} {
		if !strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_context_resolver.go should keep %q", needle)
		}
	}
}

func TestProcessFacadeFileOwnsRootDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_process_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_process_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error) {",
		"if task == nil {",
		"return buildListingKitProcessFlow(s).run(ctx, task, log)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_process_facade.go should contain %q", needle)
		}
	}

	processSrc, err := os.ReadFile("service_process.go")
	if err != nil {
		t.Fatalf("ReadFile(service_process.go) error = %v", err)
	}
	processContent := string(processSrc)

	if strings.Contains(processContent, "func (s *service) ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error) {") {
		t.Fatalf("service_process.go should not contain %q", "func (s *service) ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error) {")
	}

	if !strings.Contains(processContent, "func taskNeedsReviewReason(result *ListingKitResult) string {") {
		t.Fatalf("service_process.go should keep %q", "func taskNeedsReviewReason(result *ListingKitResult) string {")
	}
}

func TestTaskLayersFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_task_layers_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_layers_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) ProcessStandardProductLayer(ctx context.Context, taskID string) (*StandardProductSnapshot, error) {",
		"ctx, task, err := s.loadTaskExecutionContext(ctx, taskID)",
		"func (s *service) ProcessPlatformAdaptationLayer(ctx context.Context, taskID string, platform string) (*ListingKitResult, error) {",
		"if err := s.persistProcessedTaskResult(ctx, task.ID, result); err != nil {",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_task_layers_facade.go should contain %q", needle)
		}
	}

	layersSrc, err := os.ReadFile("service_layers.go")
	if err != nil {
		t.Fatalf("ReadFile(service_layers.go) error = %v", err)
	}
	layersContent := string(layersSrc)

	for _, needle := range []string{
		"func (s *service) ProcessStandardProductLayer(ctx context.Context, taskID string) (*StandardProductSnapshot, error) {",
		"func (s *service) ProcessPlatformAdaptationLayer(ctx context.Context, taskID string, platform string) (*ListingKitResult, error) {",
	} {
		if strings.Contains(layersContent, needle) {
			t.Fatalf("service_layers.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *service) loadTaskExecutionContext(ctx context.Context, taskID string) (context.Context, *Task, error) {",
		"func (s *service) persistProcessedTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {",
	} {
		if !strings.Contains(layersContent, needle) {
			t.Fatalf("service_layers.go should keep %q", needle)
		}
	}
}

func TestUploadedImageFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_upload_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_upload_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) UploadImages(ctx context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error) {",
		"func (s *service) GetUploadedImage(ctx context.Context, key string) (*UploadedImageFile, error) {",
		"func (s *service) DeleteUploadedImage(ctx context.Context, key string) (*DeletedUploadedImage, error) {",
		"return &DeletedUploadedImage{Key: stored.Key, Size: stored.Size}, nil",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_upload_facade.go should contain %q", needle)
		}
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

func TestSheinPricingFacadeFileOwnsRootDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_pricing_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_pricing_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {",
		"return s.sheinAdminOrDefault().PreviewSheinPrice(ctx, taskID, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_pricing_facade.go should contain %q", needle)
		}
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

func TestSheinImageRegenerationFacadeFileOwnsRootDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_image_regeneration_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_image_regeneration_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) RegenerateSheinDataImage(ctx context.Context, taskID string, req *RegenerateSheinDataImageRequest) (*RegenerateSheinDataImageResponse, error) {",
		"productReq, role := buildSheinDataImageRegenerationRequest(task, req)",
		"replaced := replaceSheinDataImageURL(task, oldURL, newURL)",
		"return &RegenerateSheinDataImageResponse{",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_image_regeneration_facade.go should contain %q", needle)
		}
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

func TestSheinSubmissionEventsFacadeFileOwnsRootDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_submission_events_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_submission_events_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {",
		"return s.sheinAdminOrDefault().GetSubmissionEvents(ctx, taskID)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_submission_events_facade.go should contain %q", needle)
		}
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

func TestSheinFinalDraftFacadeFileOwnsRootDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_shein_final_draft_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_final_draft_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error) {",
		"return s.sheinAdminOrDefault().UpdateSheinFinalDraft(ctx, taskID, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_shein_final_draft_facade.go should contain %q", needle)
		}
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

func TestChildTaskRetryFacadeFileOwnsRootDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_child_task_retry_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_child_task_retry_facade.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) RetryTaskChildTask(ctx context.Context, taskID string, req *RetryChildTaskRequest) (*TaskResult, error) {",
		"taskID = strings.TrimSpace(taskID)",
		"switch kind {",
		"return s.persistRetriedChildTaskResult(ctx, task, result, kind, nil)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_child_task_retry_facade.go should contain %q", needle)
		}
	}

	retrySrc, err := os.ReadFile("service_child_task_retry.go")
	if err != nil {
		t.Fatalf("ReadFile(service_child_task_retry.go) error = %v", err)
	}
	retryContent := string(retrySrc)

	if strings.Contains(retryContent, "func (s *service) RetryTaskChildTask(ctx context.Context, taskID string, req *RetryChildTaskRequest) (*TaskResult, error) {") {
		t.Fatalf("service_child_task_retry.go should not contain %q", "func (s *service) RetryTaskChildTask(ctx context.Context, taskID string, req *RetryChildTaskRequest) (*TaskResult, error) {")
	}

	for _, needle := range []string{
		"func (s *service) retrySDSCatalogProduct(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) error {",
		"func (s *service) retrySDSDesignSync(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) error {",
		"func (s *service) persistRetriedChildTaskResult(ctx context.Context, task *Task, result *ListingKitResult, kind string, retryErr error) (*TaskResult, error) {",
	} {
		if !strings.Contains(retryContent, needle) {
			t.Fatalf("service_child_task_retry.go should keep %q", needle)
		}
	}
}

func TestSubmitWorkflowFacadeFileOwnsRootHelpers(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_submit_workflow_facade.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_workflow_facade.go) error = %v", err)
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
			t.Fatalf("service_submit_workflow_facade.go should contain %q", needle)
		}
	}

	workflowSrc, err := os.ReadFile("service_submit_workflow.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_workflow.go) error = %v", err)
	}
	workflowContent := string(workflowSrc)

	for _, needle := range []string{
		"func (s *service) submitSheinTaskWithWorkflow(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinWorkflowSubmitOptions) (*ListingKitPreview, error) {",
		"func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {",
	} {
		if strings.Contains(workflowContent, needle) {
			t.Fatalf("service_submit_workflow.go should not contain %q", needle)
		}
	}
}

func TestTaskGenerationFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_task_generation.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_generation.go) error = %v", err)
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
			t.Fatalf("service_task_generation.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("service_generation.go")
	if err != nil {
		t.Fatalf("ReadFile(service_generation.go) error = %v", err)
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
			t.Fatalf("service_generation.go should not contain %q", needle)
		}
	}
}

func TestTaskRevisionFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_task_revision.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_revision.go) error = %v", err)
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
			t.Fatalf("service_task_revision.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("service_task_export.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_export.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func (s *service) GetTaskRevisionHistory(ctx context.Context, taskID string, query *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error) {",
		"func (s *service) GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *RevisionHistoryDetailQuery) (*ListingKitRevisionHistoryDetail, error) {",
		"func (s *service) ApplyTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*ListingKitPreview, error) {",
		"func (s *service) ValidateTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*RevisionValidationResult, error) {",
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("service_task_export.go should not contain %q", needle)
		}
	}
}

func TestTaskLifecycleFacadeFileOwnsRootDelegates(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_task_lifecycle.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_lifecycle.go) error = %v", err)
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
			t.Fatalf("service_task_lifecycle.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("service_task_export.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_export.go) error = %v", err)
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
			t.Fatalf("service_task_export.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *service) GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error) {",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("service_task_export.go should keep %q", needle)
		}
	}
}

func TestTaskSDSBaselineFacadeFileOwnsWarmDelegate(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_task_sds_baseline.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_sds_baseline.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"func (s *service) WarmSDSBaseline(ctx context.Context, req *WarmSDSBaselineRequest) (*SDSBaselineReadiness, error) {",
		"return s.warmSDSBaseline(ctx, req)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_task_sds_baseline.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("service_task_export.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_export.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	if strings.Contains(serviceContent, "func (s *service) WarmSDSBaseline(ctx context.Context, req *WarmSDSBaselineRequest) (*SDSBaselineReadiness, error) {") {
		t.Fatalf("service_task_export.go should not contain %q", "func (s *service) WarmSDSBaseline(ctx context.Context, req *WarmSDSBaselineRequest) (*SDSBaselineReadiness, error) {")
	}

	for _, needle := range []string{
		"func (s *service) GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error) {",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("service_task_export.go should keep %q", needle)
		}
	}
}
