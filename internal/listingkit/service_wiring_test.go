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
	if impl.task.lifecycle == nil {
		t.Fatal("expected task group lifecycle to be initialized")
	}
	if impl.task.generation == nil {
		t.Fatal("expected task group generation to be initialized")
	}
	if impl.task.revision == nil {
		t.Fatal("expected task group revision to be initialized")
	}
	if impl.task.preview == nil {
		t.Fatal("expected task group preview to be initialized")
	}
	if impl.task.sdsBaseline == nil {
		t.Fatal("expected task group sdsBaseline to be initialized")
	}
	if impl.studio.sessionGroup.session == nil {
		t.Fatal("expected studio group session to be initialized")
	}
	if impl.studio.batchGroup.batchGeneration == nil {
		t.Fatal("expected studio group batchGeneration to be initialized")
	}
	if impl.studio.sessionGroup.media == nil {
		t.Fatal("expected studio group media to be initialized")
	}
	if impl.admin.settings == nil {
		t.Fatal("expected admin group settings to be initialized")
	}
	if impl.admin.shein == nil {
		t.Fatal("expected admin group shein to be initialized")
	}
	if impl.submission.managedGroup.submission == nil {
		t.Fatal("expected taskSubmission to be initialized")
	}
	if impl.submission.managedGroup.refresh == nil {
		t.Fatal("expected taskSubmissionRefresh to be initialized")
	}
	if impl.submission.recoveryGroup.taskRecovery == nil {
		t.Fatal("expected taskRecovery to be initialized")
	}
	if impl.submission.recoveryGroup.taskRequeue == nil {
		t.Fatal("expected taskRequeue to be initialized")
	}
	if impl.submission.managedGroup.recovery == nil {
		t.Fatal("expected taskSubmissionRecovery to be initialized")
	}
	if impl.submission.coreGroup.execution == nil {
		t.Fatal("expected taskSubmissionExecution to be initialized")
	}
	if impl.submission.coreGroup.state == nil {
		t.Fatal("expected taskSubmissionState to be initialized")
	}
	if impl.submission.managedGroup.direct == nil {
		t.Fatal("expected taskDirectSubmission to be initialized")
	}
	if impl.submission.temporalGroup.lifecycle == nil {
		t.Fatal("expected taskTemporalSubmissionLifecycle to be initialized")
	}
	if impl.submission.temporalGroup.flow == nil {
		t.Fatal("expected taskTemporalSubmissionFlow to be initialized")
	}
	if impl.submission.temporalGroup.persistence == nil {
		t.Fatal("expected taskTemporalSubmissionPersistence to be initialized")
	}
	if impl.submission.temporalGroup.refresh == nil {
		t.Fatal("expected taskTemporalSubmissionRefresh to be initialized")
	}
}

func TestServiceInitializeCollaboratorGroups(t *testing.T) {
	t.Parallel()

	svc := &service{repo: &stubSubmitRepo{}}

	svc.initializeTaskCollaborators()
	if svc.task.lifecycle == nil {
		t.Fatal("expected task group lifecycle to be initialized")
	}
	if svc.task.generation == nil {
		t.Fatal("expected task group generation to be initialized")
	}
	if svc.task.revision == nil {
		t.Fatal("expected task group revision to be initialized")
	}
	if svc.task.preview == nil {
		t.Fatal("expected task group preview to be initialized")
	}
	if svc.task.sdsBaseline == nil {
		t.Fatal("expected task group sdsBaseline to be initialized")
	}
	if svc.studio.sessionGroup.session == nil {
		t.Fatal("expected studio group session to be initialized")
	}
	if svc.studio.batchGroup.batchGeneration == nil {
		t.Fatal("expected studio group batchGeneration to be initialized")
	}
	if svc.studio.sessionGroup.media == nil {
		t.Fatal("expected studio group media to be initialized")
	}

	svc.initializeAdminCollaborators()
	if svc.admin.settings == nil {
		t.Fatal("expected admin group settings to be initialized")
	}
	if svc.admin.shein == nil {
		t.Fatal("expected admin group shein to be initialized")
	}

	svc.initializeSubmitCollaborators()
	if svc.submission.recoveryGroup.taskRecovery == nil {
		t.Fatal("expected taskRecovery to be initialized")
	}
	if svc.submission.recoveryGroup.taskRequeue == nil {
		t.Fatal("expected taskRequeue to be initialized")
	}
	if svc.submission.managedGroup.submission == nil {
		t.Fatal("expected taskSubmission to be initialized")
	}
	if svc.submission.managedGroup.refresh == nil {
		t.Fatal("expected taskSubmissionRefresh to be initialized")
	}
	if svc.submission.managedGroup.recovery == nil {
		t.Fatal("expected taskSubmissionRecovery to be initialized")
	}
	if svc.submission.coreGroup.execution == nil {
		t.Fatal("expected taskSubmissionExecution to be initialized")
	}
	if svc.submission.coreGroup.state == nil {
		t.Fatal("expected taskSubmissionState to be initialized")
	}
	if svc.submission.managedGroup.direct == nil {
		t.Fatal("expected taskDirectSubmission to be initialized")
	}

	svc.initializeSubmitWorkflowCollaborators()
	if svc.submission.temporalGroup.lifecycle == nil {
		t.Fatal("expected taskTemporalSubmissionLifecycle to be initialized")
	}
	if svc.submission.temporalGroup.flow == nil {
		t.Fatal("expected taskTemporalSubmissionFlow to be initialized")
	}
	if svc.submission.temporalGroup.persistence == nil {
		t.Fatal("expected taskTemporalSubmissionPersistence to be initialized")
	}
	if svc.submission.temporalGroup.refresh == nil {
		t.Fatal("expected taskTemporalSubmissionRefresh to be initialized")
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

func TestServiceRootPlatformNormalizationUsesListingPlatformRegistry(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("ReadFile(service.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		`listingplatform "task-processor/internal/listing/platform"`,
		"req.Platforms = listingplatform.SupportedPlatforms()",
		"req.Platforms = listingplatform.NormalizeSupportedPlatforms(req.Platforms)",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		`[]string{"amazon", "shein", "temu", "walmart"}`,
		`case "amazon", "shein", "temu", "walmart":`,
		"func normalizePlatforms(",
		"listingplatform.Normalize(platform)",
		"listingplatform.IsSupported(normalized)",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("service.go should not contain %q after platform registry extraction", needle)
		}
	}
}

func TestAdminCollaboratorFilesUseExplicitWiringBuilders(t *testing.T) {
	t.Parallel()

	if _, err := os.ReadFile("service_admin_wiring.go"); err == nil {
		t.Fatal("service_admin_wiring.go should be removed after admin collaborator wiring consolidation")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_admin_wiring.go) unexpected error = %v", err)
	}

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
				"buildSettingsAdminServiceConfigWithWiring(buildSettingsAdminWiring(s))",
				"buildSheinAdminServiceConfigWithWiring(buildSheinAdminWiring(s))",
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

	adminSrc, err := os.ReadFile("shein_admin_service.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_admin_service.go) error = %v", err)
	}
	adminContent := string(adminSrc)

	for _, needle := range []string{
		"func (s *sheinAdminService) loadAdminTask(",
		"func (s *sheinAdminService) loadAdminTaskPackage(",
		"func (s *sheinAdminService) newSheinCategoryAPI(",
		"func applySheinAdminPricingReview(",
		"func (s *sheinAdminService) applySheinFinalDraftUpdate(",
		"func applySheinFinalDraftRequest(",
		"func (s *sheinAdminService) clearSheinAdminResolutionKinds(",
	} {
		if !strings.Contains(adminContent, needle) {
			t.Fatalf("shein_admin_service.go should contain %q after admin support cleanup", needle)
		}
	}

	for _, needle := range []string{
		"sheinpub.ApplyFinalDraftUpdate(pkg, sheinpub.FinalDraftUpdate{",
		"sheinpub.ApplyFinalImageDraft(pkg)",
		"sheinworkspace.SearchCategoryCandidates(tree.Data, trimmedQuery)",
	} {
		if !strings.Contains(adminContent, needle) {
			t.Fatalf("shein_admin_service.go should delegate via %q", needle)
		}
	}

	if _, err := os.ReadFile("shein_admin_service_support.go"); err == nil {
		t.Fatal("shein_admin_service_support.go should be removed after admin support cleanup")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(shein_admin_service_support.go) unexpected error = %v", err)
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

func TestTaskPreviewServiceConfigUsesPreviewSpecificWiring(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service_task_wiring.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_wiring.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func buildTaskPreviewServiceConfig(s *service) taskPreviewServiceConfig {",
		"repository := buildTaskRepositoryWiring(s)",
		"decorators := buildTaskPreviewDecorationWiring(s)",
		"repo:       repository.repo",
		"decorators: decorators",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service_task_wiring.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"readWiring := buildTaskPreviewExportReadWiring(s)",
		"listAssetGenerationTasks:               readWiring.listAssetGenerationTasks",
		"repo:                                   readWiring.repo",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("service_task_wiring.go should not contain %q in buildTaskPreviewServiceConfig", needle)
		}
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
			name:         "studio collaborators",
			file:         "service_studio_collaborators.go",
			builderCalls: nil,
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
			name:         "studio batch run",
			file:         "studio_batch_run_service.go",
			builderCalls: nil,
			inlineConfig: nil,
		},
		{
			name:         "studio batch run coordinator",
			file:         "studio_batch_run_coordinator.go",
			builderCalls: nil,
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

func TestTaskStudioServiceConfigsInjectListingStudioRunners(t *testing.T) {
	t.Parallel()

	if _, err := os.ReadFile("service_studio_wiring.go"); err == nil {
		t.Fatal("service_studio_wiring.go should be removed after studio collaborator wiring consolidation")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_studio_wiring.go) unexpected error = %v", err)
	}

	sessionSrc, err := os.ReadFile("service_studio_session_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_session_wiring_support.go) error = %v", err)
	}
	sessionContent := string(sessionSrc)

	for _, needle := range []string{
		"type taskStudioSessionConfigWiring struct {",
		"func buildTaskStudioSessionConfigWiring(s *service) taskStudioSessionConfigWiring {",
		"func buildTaskStudioSessionServiceConfigWithWiring(config taskStudioSessionConfigWiring) taskStudioSessionServiceConfig {",
		"runner:                   config.runner,",
		"asyncJobRunner:           config.asyncJobRunner,",
		"generationMetadataRunner: config.generationMetadataRunner,",
		"reviewTaskMetadataRunner: config.reviewTaskMetadataRunner,",
		"generalMetadataRunner:    config.generalMetadataRunner,",
		"func buildTaskStudioBatchDraftServiceConfigWithWiring(config taskStudioSessionConfigWiring) taskStudioBatchDraftServiceConfig {",
		"loadDetail: config.loadBatchDetail,",
		"runner:     config.batchDraftRunner,",
		"func buildTaskStudioMediaServiceConfigWithWiring(wiring taskStudioMediaWiring) taskStudioMediaServiceConfig {",
	} {
		if !strings.Contains(sessionContent, needle) {
			t.Fatalf("service_studio_session_wiring_support.go should contain %q", needle)
		}
	}

	batchSrc, err := os.ReadFile("service_studio_batch_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_batch_wiring_support.go) error = %v", err)
	}
	batchContent := string(batchSrc)

	for _, needle := range []string{
		"type taskStudioBatchServiceConfigWiring struct {",
		"type taskStudioBatchCollaboratorWiring struct {",
		"type taskStudioBatchCollaborators struct {",
		"resetRetryItems    func(context.Context, []StudioBatchItemRecord) error",
		"taskPrepare  *listingStudioBatchTaskPrepareRunner",
		"taskResume   *listingStudioBatchTaskResumeRunner",
		"func buildStudioBatchGenerationServiceConfigWithWiring(wiring studioBatchGenerationWiring) studioBatchGenerationServiceConfig {",
		"func buildTaskStudioBatchServiceConfigWithCollaborators(",
		"detailRunner:       config.detailRunner,",
		"reviewRunner:       config.reviewRunner,",
		"retryRunner:        config.retryRunner,",
		"taskPrepareRunner:  config.taskPrepare,",
		"taskResumeRunner:   config.taskResume,",
		"func buildTaskStudioBatchServiceConfigWiring(s *service) taskStudioBatchServiceConfigWiring {",
		"func buildTaskStudioBatchServiceConfigWiringWithGenerator(s *service, generator *studioBatchGenerationService) taskStudioBatchServiceConfigWiring {",
		"func buildTaskStudioBatchServiceWiringWithGenerator(s *service, generator *studioBatchGenerationService) taskStudioBatchServiceWiring {",
		"func buildTaskStudioBatchCollaboratorWiring(s *service) taskStudioBatchCollaboratorWiring {",
		"func (w taskStudioBatchCollaboratorWiring) newBatchGeneration() *studioBatchGenerationService {",
		"func (w taskStudioBatchCollaboratorWiring) newBatch(batchGeneration *studioBatchGenerationService) *taskStudioBatchService {",
		"func (w taskStudioBatchCollaboratorWiring) resolve(existing taskStudioBatchCollaborators) taskStudioBatchCollaborators {",
		"taskResume: newListingStudioBatchTaskResumeService(",
	} {
		if !strings.Contains(batchContent, needle) {
			t.Fatalf("service_studio_batch_wiring_support.go should contain %q", needle)
		}
	}

	batchRunSrc, err := os.ReadFile("service_studio_batch_run_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_batch_run_wiring_support.go) error = %v", err)
	}
	batchRunContent := string(batchRunSrc)

	for _, needle := range []string{
		"type taskStudioBatchRunConfigWiring struct {",
		"func buildTaskStudioBatchRunConfigWiring(s *service) taskStudioBatchRunConfigWiring {",
		"func buildTaskStudioBatchRunServiceConfigWithCollaborators(",
		"runner:            wiring.newServiceRunner(startRun),",
		"func buildStudioBatchRunCoordinatorConfigWithCollaborators(",
		"func buildTaskStudioBatchRunExecutorConfigWithWiring(",
		"executeGenerateOne: s.executeStudioBatchRunItem,",
		"executeCreateTasks: s.executeStudioBatchRunTaskCreation,",
		"completionRunner:   wiring.newCompletionRunner(nil),",
	} {
		if !strings.Contains(batchRunContent, needle) {
			t.Fatalf("service_studio_batch_run_wiring_support.go should contain %q", needle)
		}
	}
}

func TestTaskStudioSessionCollaboratorsShareOneEnsureSeam(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service_studio_collaborators.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_collaborators.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func (s *service) taskStudioSessionOrDefault() *taskStudioSessionService {",
		"return s.resolveTaskStudioSessionCollaborators().session",
		"func (s *service) taskStudioBatchDraftOrDefault() *taskStudioBatchDraftService {",
		"return s.resolveTaskStudioSessionCollaborators().batchDraft",
		"func (s *service) taskStudioMediaOrDefault() *taskStudioMediaService {",
		"return s.resolveTaskStudioSessionCollaborators().media",
		"func (s *service) resolveTaskStudioSessionCollaborators() taskStudioSessionCollaborators {",
		"wiring := buildTaskStudioSessionCollaboratorWiring(s)",
		"s.studio.sessionGroup = wiring.resolve(s.studio.sessionGroup)",
		"return s.studio.sessionGroup",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service_studio_collaborators.go should contain %q", needle)
		}
	}

	stageSrc, err := os.ReadFile("service_task_collaborator_stages.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_collaborator_stages.go) error = %v", err)
	}
	stageContent := string(stageSrc)

	for _, needle := range []string{
		"s.taskStudioSessionOrDefault()",
		"s.taskStudioBatchDraftOrDefault()",
		"s.taskStudioMediaOrDefault()",
	} {
		if !strings.Contains(stageContent, needle) {
			t.Fatalf("service_task_collaborator_stages.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("service_studio_session_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_session_wiring_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"type taskStudioSessionCollaboratorWiring struct {",
		"type taskStudioSessionCollaborators struct {",
		"func buildTaskStudioSessionCollaboratorWiring(s *service) taskStudioSessionCollaboratorWiring {",
		"func (w taskStudioSessionCollaboratorWiring) newSession() *taskStudioSessionService {",
		"func (w taskStudioSessionCollaboratorWiring) newBatchDraft() *taskStudioBatchDraftService {",
		"func (w taskStudioSessionCollaboratorWiring) newMedia() *taskStudioMediaService {",
		"func (w taskStudioSessionCollaboratorWiring) resolve(existing taskStudioSessionCollaborators) taskStudioSessionCollaborators {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("service_studio_session_wiring_support.go should contain %q", needle)
		}
	}
}

func TestTaskStudioBatchCollaboratorsShareOneEnsureSeam(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service_studio_collaborators.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_collaborators.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func (s *service) studioBatchGenerationOrDefault() *studioBatchGenerationService {",
		"return s.resolveTaskStudioBatchCollaborators().batchGeneration",
		"func (s *service) taskStudioBatchOrDefault() *taskStudioBatchService {",
		"return s.resolveTaskStudioBatchCollaborators().batch",
		"func (s *service) resolveTaskStudioBatchCollaborators() taskStudioBatchCollaborators {",
		"wiring := buildTaskStudioBatchCollaboratorWiring(s)",
		"s.studio.batchGroup = wiring.resolve(s.studio.batchGroup)",
		"return s.studio.batchGroup",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service_studio_collaborators.go should contain %q", needle)
		}
	}

	stageSrc, err := os.ReadFile("service_task_collaborator_stages.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_collaborator_stages.go) error = %v", err)
	}
	stageContent := string(stageSrc)

	for _, needle := range []string{
		"s.studioBatchGenerationOrDefault()",
		"s.taskStudioBatchOrDefault()",
	} {
		if !strings.Contains(stageContent, needle) {
			t.Fatalf("service_task_collaborator_stages.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("service_studio_batch_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_batch_wiring_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"type taskStudioBatchCollaboratorWiring struct {",
		"type taskStudioBatchCollaborators struct {",
		"func buildTaskStudioBatchServiceConfigWithCollaborators(",
		"func buildTaskStudioBatchServiceWiringWithGenerator(s *service, generator *studioBatchGenerationService) taskStudioBatchServiceWiring {",
		"func buildTaskStudioBatchServiceConfigWiringWithGenerator(s *service, generator *studioBatchGenerationService) taskStudioBatchServiceConfigWiring {",
		"func buildTaskStudioBatchCollaboratorWiring(s *service) taskStudioBatchCollaboratorWiring {",
		"func (w taskStudioBatchCollaboratorWiring) newBatchGeneration() *studioBatchGenerationService {",
		"func (w taskStudioBatchCollaboratorWiring) newBatch(batchGeneration *studioBatchGenerationService) *taskStudioBatchService {",
		"func (w taskStudioBatchCollaboratorWiring) resolve(existing taskStudioBatchCollaborators) taskStudioBatchCollaborators {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("service_studio_batch_wiring_support.go should contain %q", needle)
		}
	}
}

func TestTaskStudioBatchRunCollaboratorsShareOneEnsureSeam(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service_studio_collaborators.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_collaborators.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func (s *service) taskStudioBatchRunOrDefault() *taskStudioBatchRunService {",
		"return s.resolveTaskStudioBatchRunCollaborators().batchRun",
		"func (s *service) studioBatchRunExecutorOrDefault() *taskStudioBatchRunExecutor {",
		"return s.resolveTaskStudioBatchRunCollaborators().runExecutor",
		"func (s *service) studioBatchRunCoordinatorOrDefault() *studioBatchRunCoordinator {",
		"return s.resolveTaskStudioBatchRunCollaborators().runCoordinator",
		"func (s *service) resolveTaskStudioBatchRunCollaborators() taskStudioBatchRunCollaborators {",
		"wiring := buildTaskStudioBatchRunCollaboratorWiring(s)",
		"s.studio.runGroup = wiring.resolve(s.studio.runGroup)",
		"return s.studio.runGroup",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service_studio_collaborators.go should contain %q", needle)
		}
	}

	stageSrc, err := os.ReadFile("service_task_collaborator_stages.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_collaborator_stages.go) error = %v", err)
	}
	stageContent := string(stageSrc)

	for _, needle := range []string{
		"s.taskStudioBatchRunOrDefault()",
		"s.studioBatchRunExecutorOrDefault()",
		"s.studioBatchRunCoordinatorOrDefault()",
	} {
		if !strings.Contains(stageContent, needle) {
			t.Fatalf("service_task_collaborator_stages.go should contain %q", needle)
		}
	}

	coordinatorSrc, err := os.ReadFile("studio_batch_run_coordinator.go")
	if err != nil {
		t.Fatalf("ReadFile(studio_batch_run_coordinator.go) error = %v", err)
	}
	coordinatorContent := string(coordinatorSrc)

	for _, needle := range []string{
		"func (s *service) initializeStudioBatchRunSupportCollaborators() {",
		"s.resolveTaskStudioBatchRunCollaborators()",
		"func (s *service) buildStudioBatchRunCoordinator() *studioBatchRunCoordinator {",
		"return s.resolveTaskStudioBatchRunCollaborators().runCoordinator",
		"func (s *service) buildStudioBatchRunExecutor() *taskStudioBatchRunExecutor {",
		"return s.resolveTaskStudioBatchRunCollaborators().runExecutor",
	} {
		if !strings.Contains(coordinatorContent, needle) {
			t.Fatalf("studio_batch_run_coordinator.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("service_studio_batch_run_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_studio_batch_run_wiring_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"type taskStudioBatchRunCollaboratorWiring struct {",
		"type taskStudioBatchRunCollaborators struct {",
		"func buildTaskStudioBatchRunServiceConfigWithCollaborators(",
		"startRun = coordinator.StartRun",
		"func buildStudioBatchRunCoordinatorConfigWithCollaborators(",
		"func buildTaskStudioBatchRunExecutorConfigWithWiring(",
		"func buildTaskStudioBatchRunCollaboratorWiring(s *service) taskStudioBatchRunCollaboratorWiring {",
		"func (w taskStudioBatchRunCollaboratorWiring) newRunExecutor() *taskStudioBatchRunExecutor {",
		"func (w taskStudioBatchRunCollaboratorWiring) newRunCoordinator(executor *taskStudioBatchRunExecutor) *studioBatchRunCoordinator {",
		"func (w taskStudioBatchRunCollaboratorWiring) newBatchRun(coordinator *studioBatchRunCoordinator) *taskStudioBatchRunService {",
		"func (w taskStudioBatchRunCollaboratorWiring) resolve(existing taskStudioBatchRunCollaborators) taskStudioBatchRunCollaborators {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("service_studio_batch_run_wiring_support.go should contain %q", needle)
		}
	}
}

func TestStudioBatchRunExecutorDelegatesCompletionRulesToStudioDomain(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_studio_batch_run_executor.go", "cancelUnfinishedItems")
	resolveSource := readNamedFunctionSource(t, "task_studio_batch_run_executor.go", "resolveFinalRunStatus")
	counterSource := readNamedFunctionSource(t, "task_studio_batch_run_executor.go", "refreshRunCounters")
	ensureSource := readNamedFunctionSource(t, "task_studio_batch_run_executor.go", "ensureCompletionRunner")

	assertSourceContainsAll(t, source, []string{
		"return e.completionRunner.CancelUnfinishedItems(ctx, items)",
	})
	assertSourceContainsAll(t, resolveSource, []string{
		"return e.completionRunner.ResolveFinalStatus(run != nil && run.CancelRequested, items)",
	})
	assertSourceContainsAll(t, counterSource, []string{
		"counters := e.completionRunner.CountItems(items)",
		"run.TotalBatches = counters.Total",
		"run.CompletedBatches = counters.Completed",
		"run.SucceededBatches = counters.Succeeded",
		"run.FailedBatches = counters.Failed",
	})
	assertSourceContainsAll(t, ensureSource, []string{
		"e.completionRunner = newListingStudioBatchRunCompletionService(e.repo, e.now)",
	})
	assertSourceExcludesAll(t, source, []string{
		"case StudioBatchRunItemStatusSucceeded, StudioBatchRunItemStatusFailed, StudioBatchRunItemStatusCancelled:",
	})
	assertSourceExcludesAll(t, resolveSource, []string{
		"failed > 0 && succeeded > 0",
	})
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
				"resolveTaskSubmitTaskRecoveryCollaborators()",
				"resolveTaskSubmissionCoreCollaborators()",
				"resolveTaskManagedSubmissionCollaborators()",
				"resolveTaskTemporalSubmissionCollaborators()",
			},
			inlineConfig: []string{},
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

func TestTaskSubmissionRefreshServiceUsesSubmissionDomainRunner(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("task_submission_refresh_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_refresh_service.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		`submissiondomain.NewStatusRefreshService(`,
		`LockKeySuffix:       "refresh_submission_status",`,
		`LoadState:           svc.loadSheinSubmissionRefreshState,`,
		`ResolveConfirmation: svc.resolveSubmissionRefreshConfirmation,`,
		`Finish:              svc.finishSubmissionRefresh,`,
		`return s.refreshRunner.RefreshStatus(ctx, taskID)`,
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("task_submission_refresh_service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		`unlockSubmit := s.lockSubmit(taskID + ":refresh_submission_status")`,
		`confirmation, remoteErr := s.resolveSubmissionRefreshConfirmation(taskID, refreshState)`,
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("task_submission_refresh_service.go should not contain %q after submission-domain refresh runner extraction", needle)
		}
	}
}

func TestTaskRequeueServiceUsesSubmissionDomainRunner(t *testing.T) {
	t.Parallel()

	serviceSrc, err := os.ReadFile("task_requeue_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_requeue_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		`type taskRequeueRunnerWiring struct {`,
		`func buildTaskRequeueRunnerWiring(svc *taskRequeueService) taskRequeueRunnerWiring {`,
		`wiring := buildTaskRequeueRunnerWiring(svc)`,
		`submissiondomain.NewRequeueService(submissiondomain.RequeueServiceConfig{`,
		`LoadTask:          wiring.loadTask,`,
		`CurrentSubmitter:  wiring.currentSubmitter,`,
		`SubmitTask:        wiring.submitTask,`,
		`result, err := s.runner.RequeueTasks(ctx, &submissiondomain.RequeueRequest{`,
		`return adaptSubmissionDomainRequeueResult(result), nil`,
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_requeue_service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		`for _, taskID := range taskIDs {`,
		`result.Skipped = append(result.Skipped, TaskRequeueSkip{`,
		`result.Failed = append(result.Failed, TaskRequeueFailure{`,
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("task_requeue_service.go should not contain %q after submission-domain requeue runner extraction", needle)
		}
	}

	adapterSrc, err := os.ReadFile("task_requeue_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_requeue_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)
	for _, needle := range []string{
		`func adaptSubmissionDomainRequeueResult(result *submissiondomain.RequeueResult) *RequeuePendingTasksResult {`,
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_requeue_adapter.go should contain %q", needle)
		}
	}
	if strings.Contains(adapterContent, `type taskRequeueSubmitterFunc func(taskID string) error`) {
		t.Fatalf("task_requeue_adapter.go should not keep unused submitter adapter %q; use submissiondomain.RequeueSubmitFunc directly", `type taskRequeueSubmitterFunc func(taskID string) error`)
	}
}

func TestTaskRecoveryServiceUsesSubmissionDomainRunner(t *testing.T) {
	t.Parallel()

	serviceSrc, err := os.ReadFile("task_recovery_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_recovery_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		`type taskRecoveryRunnerWiring struct {`,
		`func buildTaskRecoveryRunnerWiring(svc *taskRecoveryService) taskRecoveryRunnerWiring {`,
		`wiring := buildTaskRecoveryRunnerWiring(svc)`,
		`submissiondomain.NewRecoveryNowService(submissiondomain.RecoveryNowServiceConfig[Task]{`,
		`submissiondomain.NewRecoveryBatchService(submissiondomain.RecoveryBatchServiceConfig[Task]{`,
		`CurrentSubmitter: wiring.currentSubmitter,`,
		`LoadTask:         wiring.loadTask,`,
		`ListCandidates:       wiring.listCandidates,`,
		`SubmitRecovered:  wiring.submitRecoveredNow,`,
		`SubmitRecovered:      wiring.submitRecoveredBatch,`,
		`return s.recoveryNow.RecoverNow(ctx, taskID)`,
		`return s.recoveryBatch.RecoverBatch(ctx, request)`,
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_recovery_service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		`taskID = strings.TrimSpace(taskID)`,
		`current, err := s.repo.GetTask(ctx, taskID)`,
		`return s.repo.GetTask(ctx, taskID)`,
		`tasks, err := s.repo.ListRecoverableTasks(ctx, listQuery)`,
		`for i := range tasks {`,
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("task_recovery_service.go should not contain %q after submission-domain recovery extraction", needle)
		}
	}

	if _, err := os.ReadFile("task_recovery_adapter.go"); err == nil {
		t.Fatal("task_recovery_adapter.go should be removed after recovery submit callbacks use submissiondomain.RecoverySubmitFunc directly")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(task_recovery_adapter.go) unexpected error = %v", err)
	}
}

func TestTaskRecoverySubmitRecoveredDelegatesRetryablePersistenceSkeleton(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_recovery_service.go", "submitRecoveredTask")
	callNames := readNamedFunctionCallNames(t, "task_recovery_service.go", "submitRecoveredTask")
	durabilitySource := readTaskGenerationSourceFile(t, "task_recovery_durability.go")

	assertSourceContainsAll(t, source, []string{
		"return submissiondomain.SubmitRecoveredWithRetryablePersistence(",
		"PreviousBlock:        adaptRetryableBlockState(previousBlock)",
		"MarkBlockedRetryable: func(block *submissiondomain.RetryableBlockState, errorMsg string) error {",
		"PersistFailure: func(errorMsg string, submitErr error) error {",
		"RestoreDurability: func(errorMsg string, submitErr error, persistErr error) error {",
	})
	assertSourceExcludesAll(t, source, []string{
		"taskRecoverySubmitterFunc(",
		"if err := submitter.Submit(taskID); err != nil {",
		"if block, ok := classifyRetryableTaskFailure(err); ok {",
		"updated := s.buildReblockedTask(previousBlock, block, recoveredAt)",
	})
	assertSourceExcludesAll(t, durabilitySource, []string{
		"func (s *taskRecoveryService) buildReblockedTask(",
		"func cloneTimePointer(",
	})
	assertFunctionCallsContainAll(t, callNames, []string{
		"SubmitRecoveredWithRetryablePersistence",
	})
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
			name: "submit shared wiring",
			file: "service_submit_wiring_resolution_support.go",
			needles: []string{
				"func buildTaskSubmissionExecutionServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionExecutionServiceConfig {",
				"func buildTaskSubmissionStateServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionStateServiceConfig {",
			},
		},
		{
			name: "submit managed wiring",
			file: "service_submit_managed_wiring_support.go",
			needles: []string{
				"func buildTaskSubmissionServiceConfigWithSupportAndCollaborators(",
				"func buildTaskSubmissionRefreshServiceConfigWithWiring(wiring taskManagedSubmissionWiring) taskSubmissionRefreshServiceConfig {",
			},
		},
		{
			name: "submit temporal wiring",
			file: "service_submit_temporal_wiring_support.go",
			needles: []string{
				"func buildTaskTemporalSubmissionLifecycleServiceConfigWithWiring(wiring taskTemporalSubmissionWiring) taskTemporalSubmissionLifecycleServiceConfig {",
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

func TestTaskStudioBatchDraftAdapterUsesListingStudioRunner(t *testing.T) {
	t.Parallel()

	adapterSrc, err := os.ReadFile("task_studio_batch_draft_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_draft_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"func newListingStudioBatchDraftService(repo studioBatchDraftRepository) *listingStudioBatchDraftRunner {",
		"return studiodomain.NewBatchDraftService(studiodomain.BatchDraftServiceConfig[",
		"Repo: studioBatchDraftRepositoryAdapter{repo: repo},",
		"IsSavedBatch: func(session *SheinStudioSession) bool {",
		"SessionID: func(session *SheinStudioSession) string {",
		"MapBatchListItem: mapStudioBatchListItem,",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_batch_draft_adapter.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("task_studio_batch_draft_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_draft_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"service.ensureRunner()",
		"func (s *taskStudioBatchDraftService) ensureRunner() {",
		"s.runner = newListingStudioBatchDraftService(s.repo)",
		"result, err := s.runner.ListSessionGallery(ctx, limit)",
		"result, err := s.runner.ListBatches(ctx, limit)",
		"result, err := s.runner.GetBatch(ctx, batchID)",
		"return adaptStudioBatchDraftError(s.runner.DeleteBatch(ctx, batchID))",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_draft_service.go should contain %q", needle)
		}
	}
}

func TestTaskStudioSessionAdapterUsesListingStudioRunner(t *testing.T) {
	t.Parallel()

	adapterSrc, err := os.ReadFile("task_studio_session_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_session_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"func newListingStudioSessionService(repo studioSessionDraftRepository) *listingStudioSessionRunner {",
		"return studiodomain.NewSessionService(studiodomain.SessionServiceConfig[",
		"Repo:              studioSessionRepositoryAdapter{repo: repo},",
		"ValidateSelection: validateStudioSessionSelection,",
		"BuildSelectionKey: buildStudioSelectionKey,",
		"NewSession:        newListingStudioSessionRecord,",
		"RequestUserID: RequestUserIDFromContext,",
		"NewSessionID:  uuid.NewString,",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_session_adapter.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"type studioSessionRepositoryAdapter struct {",
		"func applyListingStudioSessionGeneralMetadataPatch(session *SheinStudioSession, req *UpdateStudioSessionRequest) {",
		"func adaptStudioSessionError(err error) error {",
	} {
		if strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_session_adapter.go should not contain %q after adapter support split", needle)
		}
	}

	adapterSupportSrc, err := os.ReadFile("task_studio_session_adapter_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_session_adapter_support.go) error = %v", err)
	}
	adapterSupportContent := string(adapterSupportSrc)

	for _, needle := range []string{
		"type studioSessionRepositoryAdapter struct {",
		"type studioSessionMutationRepositoryAdapter struct {",
		"func validateStudioSessionSelection(selection *SheinStudioSelection) error {",
		"func newListingStudioSessionRecord(id string, userID string, selectionKey string, selection *SheinStudioSelection) *SheinStudioSession {",
		"func studioSessionStatusForAsyncJob(jobStatus string) SheinStudioSessionStatus {",
		"func applyListingStudioSessionGeneralMetadataPatch(session *SheinStudioSession, req *UpdateStudioSessionRequest) {",
		"func adaptStudioSessionError(err error) error {",
	} {
		if !strings.Contains(adapterSupportContent, needle) {
			t.Fatalf("task_studio_session_adapter_support.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func newListingStudioSessionAsyncJobService(repo studioSessionDraftRepository) *listingStudioSessionAsyncJobRunner {",
		"return studiodomain.NewSessionAsyncJobSyncService(studiodomain.SessionAsyncJobSyncServiceConfig[",
		"Repo:               studioSessionMutationRepositoryAdapter{repo: repo},",
		"StatusForJob:       studioSessionStatusForAsyncJob,",
		"SetStatus:          setListingStudioSessionStatus,",
		"SetGenerationJob:   setListingStudioSessionGenerationJobID,",
		"SetGenerationError: setListingStudioSessionGenerationError,",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_session_adapter.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func newListingStudioSessionGenerationMetadataService(repo studioSessionDraftRepository) *listingStudioSessionGenerationMetadataRunner {",
		"return studiodomain.NewSessionGenerationMetadataService(studiodomain.SessionGenerationMetadataServiceConfig[",
		"SetStatus:          setListingStudioSessionStatus,",
		"SetGenerationJobID: setListingStudioSessionGenerationJobID,",
		"SetGenerationJobs:  setListingStudioSessionGenerationJobs,",
		"SetGenerationError: setListingStudioSessionGenerationError,",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_session_adapter.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func newListingStudioSessionReviewTaskMetadataService(repo studioSessionDraftRepository) *listingStudioSessionReviewTaskMetadataRunner {",
		"return studiodomain.NewSessionReviewTaskMetadataService(studiodomain.SessionReviewTaskMetadataServiceConfig[",
		"SetApprovedDesignIDs: setListingStudioSessionApprovedDesignIDs,",
		"SetCreatedTasks:      setListingStudioSessionCreatedTasks,",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_session_adapter.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("task_studio_session_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_session_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"service.ensureRunner()",
		"service.ensureAsyncJobRunner()",
		"service.ensureGenerationMetadataRunner()",
		"service.ensureReviewTaskMetadataRunner()",
		"if isStudioSessionGenerationMetadataOnlyUpdate(req) {",
		"session, err = s.generationMetadataRunner.Patch(ctx, studiodomain.SessionGenerationMetadataPatchRequest[",
		"if isStudioSessionReviewTaskMetadataOnlyUpdate(req) {",
		"session, err = s.reviewTaskMetadataRunner.Patch(ctx, studiodomain.SessionReviewTaskMetadataPatchRequest[SheinStudioCreatedTask]{",
		"result, err := s.runner.EnsureSession(ctx, &studiodomain.EnsureSessionRequest[SheinStudioSelection]{",
		"result, err := s.runner.GetSession(ctx, sessionID)",
		"return &SheinStudioSessionDetail{Session: result.Session, Designs: result.Designs}, nil",
		"return adaptStudioSessionError(s.asyncJobRunner.SyncAsyncJob(ctx, studiodomain.SessionAsyncJobSyncRequest{",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_session_service.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("task_studio_session_service_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_session_service_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func (s *taskStudioSessionService) ensureRunner() {",
		"func (s *taskStudioSessionService) ensureAsyncJobRunner() {",
		"func (s *taskStudioSessionService) ensureGenerationMetadataRunner() {",
		"func (s *taskStudioSessionService) ensureReviewTaskMetadataRunner() {",
		"func (s *taskStudioSessionService) ensureGeneralMetadataRunner() {",
		"s.runner = newListingStudioSessionService(s.repo)",
		"s.asyncJobRunner = newListingStudioSessionAsyncJobService(s.repo)",
		"s.generationMetadataRunner = newListingStudioSessionGenerationMetadataService(s.repo)",
		"s.reviewTaskMetadataRunner = newListingStudioSessionReviewTaskMetadataService(s.repo)",
		"s.generalMetadataRunner = newListingStudioSessionGeneralMetadataService(s.repo)",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_studio_session_service_support.go should contain %q", needle)
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
		"func (s *service) generateStudioDesignImage(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*AIImageResponse, error) {",
		"return s.taskStudioMediaOrDefault().generateStudioDesignImage(ctx, model, promptText, size, referenceURLs)",
		"func (s *service) editStudioDesignImageWithReferences(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*AIImageResponse, error) {",
		"return s.taskStudioMediaOrDefault().editStudioDesignImageWithReferences(ctx, model, promptText, size, referenceURLs)",
		"func (s *service) generateStudioDesignImageWithoutReferences(ctx context.Context, model string, promptText string, size string) (*AIImageResponse, error) {",
		"return s.taskStudioMediaOrDefault().generateStudioDesignImageWithoutReferences(ctx, model, promptText, size)",
		"func (s *service) persistGeneratedStudioImage(ctx context.Context, response *AIImageResponse, filename string) (string, string, error) {",
		"return s.taskStudioMediaOrDefault().persistGeneratedStudioImage(ctx, response, filename)",
		"func (s *service) generateOneStudioProductImage(ctx context.Context, req *StudioProductImageRequest, sourceURL string, basePrompt string) (string, error) {",
		"return s.taskStudioMediaOrDefault().generateOneStudioProductImage(ctx, req, sourceURL, basePrompt)",
		"func (s *service) tryGenerateStudioProductImage(ctx context.Context, inputImages []string, promptText string) (*AIImageResponse, error) {",
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
				"func (s *service) generateStudioDesignImage(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*AIImageResponse, error) {",
				"func (s *service) editStudioDesignImageWithReferences(ctx context.Context, model string, promptText string, size string, referenceURLs []string) (*AIImageResponse, error) {",
				"func (s *service) generateStudioDesignImageWithoutReferences(ctx context.Context, model string, promptText string, size string) (*AIImageResponse, error) {",
				"func (s *service) persistGeneratedStudioImage(ctx context.Context, response *AIImageResponse, filename string) (string, string, error) {",
			},
		},
		{
			file: "studio_product_images.go",
			needles: []string{
				"func (s *service) GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error) {",
				"func (s *service) generateOneStudioProductImage(ctx context.Context, req *StudioProductImageRequest, sourceURL string, basePrompt string) (string, error) {",
				"func (s *service) tryGenerateStudioProductImage(ctx context.Context, inputImages []string, promptText string) (*AIImageResponse, error) {",
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
		"func aiSettingsUserID(identity RequestIdentity, scope string) string {",
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
		"return lifecycle.BeginSheinPublishAttempt(ctx, in)",
		"func (s *service) ValidateSheinPublishReadiness(ctx context.Context, in SheinPublishAttemptInput) error {",
		"return lifecycle.ValidateSheinPublishReadiness(ctx, in)",
		"func (s *service) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {",
		"return flow.PrepareSheinPublishPayload(ctx, in)",
		"func (s *service) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {",
		"return flow.UploadSheinPublishImages(ctx, in)",
		"func (s *service) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {",
		"return flow.PreValidateSheinPublish(ctx, in)",
		"func (s *service) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {",
		"return flow.SubmitSheinPublishRemote(ctx, in)",
		"func (s *service) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {",
		"return persistence.PersistSheinPublishSuccess(ctx, in)",
		"func (s *service) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {",
		"return persistence.PersistSheinPublishFailure(ctx, in)",
		"func (s *service) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {",
		"return refresh.RefreshSheinPublishRemoteStatus(ctx, in)",
		"func (s *service) BuildSheinTaskPreview(ctx context.Context, taskID string) (*ListingKitPreview, error) {",
		"return lifecycle.BuildSheinTaskPreview(ctx, taskID)",
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

	adapterSrc, err := os.ReadFile("task_temporal_submission_lifecycle_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_temporal_submission_lifecycle_service.go) error = %v", err)
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
			t.Fatalf("task_temporal_submission_lifecycle_service.go should not contain %q", needle)
		}
	}

	if !strings.Contains(adapterContent, "func (s *service) loadSheinPublishTaskForTemporal(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {") {
		t.Fatalf("task_temporal_submission_lifecycle_service.go should keep %q", "func (s *service) loadSheinPublishTaskForTemporal(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {")
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
		"return buildTaskPreview(ctx, task, platform, buildTaskPreviewDecorationWiring(s))",
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
		"type taskPreviewReader interface {",
		"type taskPreviewServiceConfig struct {",
		"func newTaskPreviewService(config taskPreviewServiceConfig) *taskPreviewService {",
		"func (s *taskPreviewService) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {",
		"return s.reader.GetTaskPreview(ctx, taskID, platform)",
	} {
		if !strings.Contains(previewServiceContent, needle) {
			t.Fatalf("task_preview_service.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"task, err := s.repo.GetTask(ctx, taskID)",
		"type taskPreviewService struct {\n\trepo",
		"type taskPreviewService struct {\n\treader                                 *previewdomain.TaskPreviewService[Task, ListingKitPreview]\n\tlistAssetGenerationTasks",
		"type taskPreviewService struct {\n\treader                                 *previewdomain.TaskPreviewService[Task, ListingKitPreview]\n\tdecorateSheinCookieAvailabilityPreview",
		"type taskPreviewService struct {\n\treader                                 *previewdomain.TaskPreviewService[Task, ListingKitPreview]\n\tdecorateSheinStoreResolutionPreview",
		"buildTaskPreview(context.Context, *Task, string) (*ListingKitPreview, error)",
		"func (s *taskPreviewService) buildTaskPreview(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {",
		"func (s *taskPreviewService) finalizeTaskPreview(ctx context.Context, task *Task, preview *ListingKitPreview) error {",
	} {
		if strings.Contains(previewServiceContent, needle) {
			t.Fatalf("task_preview_service.go should not contain %q", needle)
		}
	}

	previewSupportSrc, err := os.ReadFile("task_preview_service_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_preview_service_support.go) error = %v", err)
	}
	previewSupportContent := string(previewSupportSrc)
	for _, needle := range []string{
		"func buildTaskPreview(ctx context.Context, task *Task, platform string, decorators taskPreviewDecorationWiring) (*ListingKitPreview, error) {",
		"preview, err := buildListingKitPreview(task, platform)",
		"decorators.decorateSheinCookieAvailabilityPreview(ctx, task, preview)",
		"func finalizeTaskPreview(ctx context.Context, task *Task, preview *ListingKitPreview, decorators taskPreviewDecorationWiring) error {",
		"tasks, err := decorators.listAssetGenerationTasks(ctx, task.ID)",
		"projection := buildAssetGenerationProjection(task.Result, tasks)",
		"decorators.decorateSheinStoreResolutionPreview(ctx, task, preview)",
	} {
		if !strings.Contains(previewSupportContent, needle) {
			t.Fatalf("task_preview_service_support.go should contain %q", needle)
		}
	}

	taskGroupSrc, err := os.ReadFile("service_task_group.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_group.go) error = %v", err)
	}
	taskGroupContent := string(taskGroupSrc)
	for _, needle := range []string{
		"preview     taskPreviewReader",
	} {
		if !strings.Contains(taskGroupContent, needle) {
			t.Fatalf("service_task_group.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"preview     *taskPreviewService",
	} {
		if strings.Contains(taskGroupContent, needle) {
			t.Fatalf("service_task_group.go should not contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_collaborator_mirrors.go"); err == nil {
		t.Fatal("service_collaborator_mirrors.go should be removed after collaborator mirror retirement")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_collaborator_mirrors.go) unexpected error = %v", err)
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

	wiringSupportSrc, err := os.ReadFile("service_task_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_task_wiring_support.go) error = %v", err)
	}
	wiringSupportContent := string(wiringSupportSrc)
	for _, needle := range []string{
		"type taskPreviewDecorationWiring struct {",
		"func buildTaskPreviewDecorationWiring(s *service) taskPreviewDecorationWiring {",
		"type taskPreviewAccessWiring struct {",
		"func buildTaskPreviewAccessWiring(s *service) taskPreviewAccessWiring {",
	} {
		if strings.Contains(wiringSupportContent, needle) {
			t.Fatalf("service_task_wiring_support.go should not contain %q after preview wiring split", needle)
		}
	}

	previewHelperSrc, err := os.ReadFile("task_preview_service_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_preview_service_support.go) error = %v", err)
	}
	previewHelperContent := string(previewHelperSrc)
	for _, needle := range []string{
		"type taskPreviewDecorationWiring struct {",
		"func buildTaskPreviewDecorationWiring(s *service) taskPreviewDecorationWiring {",
		"type taskPreviewAccessWiring struct {",
		"func buildTaskPreviewAccessWiring(s *service) taskPreviewAccessWiring {",
		"listAssetGenerationTasks",
		"[]assetgeneration.Task, error",
	} {
		if !strings.Contains(previewHelperContent, needle) {
			t.Fatalf("task_preview_service_support.go should contain %q", needle)
		}
	}

	baseSrc, err := os.ReadFile("preview_base.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_base.go) error = %v", err)
	}
	baseContent := string(baseSrc)
	for _, needle := range []string{
		"previewdomain.BuildTaskShell(previewdomain.TaskShellInput{",
	} {
		if !strings.Contains(baseContent, needle) {
			t.Fatalf("preview_base.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"previewdomain.BuildProjection(previewdomain.ProjectionInput{",
		"previewdomain.ShellInput{",
	} {
		if strings.Contains(baseContent, needle) {
			t.Fatalf("preview_base.go should not contain %q after task-shell extraction", needle)
		}
	}

	builderSrc, err := os.ReadFile("preview_builder.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder.go) error = %v", err)
	}
	builderContent := string(builderSrc)
	for _, needle := range []string{
		"adaptPreviewDomainHeader(previewdomain.PendingHeader(string(task.Status)))",
	} {
		if !strings.Contains(builderContent, needle) {
			t.Fatalf("preview_builder.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"previewdomain.BuildHeader(previewdomain.HeaderInput{",
		"StatusMessage: previewdomain.StatusMessage(string(task.Status))",
	} {
		if strings.Contains(builderContent, needle) {
			t.Fatalf("preview_builder.go should not contain %q after pending-header extraction", needle)
		}
	}

	if _, err := os.ReadFile("preview_result_projection.go"); err == nil {
		t.Fatal("preview_result_projection.go should be removed after result projection adapter convergence")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(preview_result_projection.go) unexpected error = %v", err)
	}
	taskReadModelSrc, err := os.ReadFile("preview_task_read_model_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_task_read_model_adapter.go) error = %v", err)
	}
	taskReadModelContent := string(taskReadModelSrc)
	for _, needle := range []string{
		"previewdomain.BuildTaskReadModel(previewdomain.TaskReadModelInput{",
		"RequestPlatforms: previewRequestPlatforms(task)",
	} {
		if !strings.Contains(taskReadModelContent, needle) {
			t.Fatalf("preview_task_read_model_adapter.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("preview_domain_input_adapter.go"); err == nil {
		t.Fatal("preview_domain_input_adapter.go should be removed after read-projection preview input ownership move")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(preview_domain_input_adapter.go) unexpected error = %v", err)
	}

	readProjectionSrc, err := os.ReadFile("read_projection_model.go")
	if err != nil {
		t.Fatalf("ReadFile(read_projection_model.go) error = %v", err)
	}
	readProjectionContent := string(readProjectionSrc)
	for _, needle := range []string{
		"PreviewInput                previewdomain.ReadModelInput",
		"PlatformCards               []ListingKitPlatformCard",
		"func (projection *listingKitReadProjection) previewDomainReadModelInput() previewdomain.ReadModelInput {",
	} {
		if !strings.Contains(readProjectionContent, needle) {
			t.Fatalf("read_projection_model.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"previewDomainReadModelInput(result *ListingKitResult)",
		"type listingKitOverviewData struct {",
		"type listingKitResultAttachment struct {",
	} {
		if strings.Contains(readProjectionContent, needle) {
			t.Fatalf("read_projection_model.go should not contain %q after read-projection input convergence", needle)
		}
	}

	for _, file := range []string{
		"read_projection_overview.go",
		"read_projection_overview_stages.go",
		"read_projection_attachment.go",
		"read_projection_attachment_stages.go",
	} {
		if _, err := os.ReadFile(file); err == nil {
			t.Fatalf("%s should be removed after read-projection preview-input convergence", file)
		} else if !os.IsNotExist(err) {
			t.Fatalf("ReadFile(%s) unexpected error = %v", file, err)
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
	if strings.Contains(builderContent, "func buildPreviewPlatformSections(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {") {
		t.Fatalf("preview_builder.go should not contain %q", "func buildPreviewPlatformSections(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {")
	}

	platformsSrc, err := os.ReadFile("preview_builder_platforms.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_platforms.go) error = %v", err)
	}
	platformsContent := string(platformsSrc)
	for _, needle := range []string{
		`listingplatform "task-processor/internal/listing/platform"`,
		"func previewPlatformBuilders() []listingplatform.RegisteredSectionBuilder[*ListingKitResult, *ListingKitPreview] {",
		"func buildPreviewPlatformSections(result *ListingKitResult, preview *ListingKitPreview, selectedPlatform string) error {",
		"return listingplatform.SectionBuilders(previewPlatformRegistrations())",
		"return listingplatform.BuildRegisteredSections(previewPlatformBuilders(), result, preview, selectedPlatform)",
	} {
		if !strings.Contains(platformsContent, needle) {
			t.Fatalf("preview_builder_platforms.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"type previewPlatformBuilder interface {",
		"type previewPlatformBuilderFunc struct {",
		"platformSectionBuilder",
		"platformSectionBuilders(",
		"buildPlatformSections(",
		"if err := builder.build(result, preview, selectedPlatform); err != nil {",
		"listingplatform.BuildAll(",
		"listingplatform.Builder{",
		"previewdomain.PlatformSectionBuilder{",
		"previewdomain.BuildPlatformSections(",
	} {
		if strings.Contains(platformsContent, needle) {
			t.Fatalf("preview_builder_platforms.go should not contain %q after shared platform-section builder extraction", needle)
		}
	}

	if _, err := os.ReadFile("platform_section_builders.go"); err == nil {
		t.Fatal("platform_section_builders.go should be removed after neutral platform builder runner extraction")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(platform_section_builders.go) unexpected error = %v", err)
	}

	runnerSrc, err := os.ReadFile("../listing/platform/registered_builders.go")
	if err != nil {
		t.Fatalf("ReadFile(../listing/platform/registered_builders.go) error = %v", err)
	}
	runnerContent := string(runnerSrc)
	for _, needle := range []string{
		"type SectionRegistration[C, T any] struct {",
		"type RegisteredSectionBuilder[C, T any] struct {",
		"func SectionBuilders[C, T any](registrations []SectionRegistration[C, T]) []RegisteredSectionBuilder[C, T] {",
		"func SupportedSectionRegistrations[C, T any](builds map[string]SectionBuildFunc[C, T]) []SectionRegistration[C, T] {",
		"func BuildRegisteredSections[C, T any](builders []RegisteredSectionBuilder[C, T], context C, target T, selectedPlatform string) error {",
		"return BuildAll(sectionBuilders)",
	} {
		if !strings.Contains(runnerContent, needle) {
			t.Fatalf("../listing/platform/registered_builders.go should contain %q", needle)
		}
	}

	registrySrc, err := os.ReadFile("preview_platform_registry.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_platform_registry.go) error = %v", err)
	}
	registryContent := string(registrySrc)
	if !strings.Contains(registryContent, "return listingplatform.SupportedSectionRegistrations(map[string]listingplatform.SectionBuildFunc[*ListingKitResult, *ListingKitPreview]{") {
		t.Fatal("preview_platform_registry.go should build registrations through listingplatform.SupportedSectionRegistrations")
	}
	if strings.Contains(registryContent, "return []listingplatform.SectionRegistration") {
		t.Fatal("preview_platform_registry.go should not hand-code registration order")
	}

	if _, err := os.ReadFile("preview_platform_sections.go"); err == nil {
		t.Fatal("preview_platform_sections.go should be removed after direct previewdomain section usage")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(preview_platform_sections.go) unexpected error = %v", err)
	}

	applySrc, err := os.ReadFile("preview_platform_apply.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_platform_apply.go) error = %v", err)
	}
	applyContent := string(applySrc)
	for _, needle := range []string{
		"func applyPreviewPlatformSection(selectedPlatform, platform string, available bool, build func()) error {",
		"return applyPlatformSection(selectedPlatform, platform, available, build)",
		"return applyPlatformSection(selectedPlatform, platform, available, func() {",
	} {
		if !strings.Contains(applyContent, needle) {
			t.Fatalf("preview_platform_apply.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		`"task-processor/internal/listing/platform"`,
		"listingplatform.BuildOne(",
		"listingplatform.Section{",
		"previewdomain.BuildPlatformSection(",
		"func adaptPreviewPlatformSectionError(",
	} {
		if strings.Contains(applyContent, needle) {
			t.Fatalf("preview_platform_apply.go should not contain %q after shared platform-section apply extraction", needle)
		}
	}

	applyHelperSrc, err := os.ReadFile("platform_section_apply.go")
	if err != nil {
		t.Fatalf("ReadFile(platform_section_apply.go) error = %v", err)
	}
	applyHelperContent := string(applyHelperSrc)
	for _, needle := range []string{
		"func applyPlatformSection(selectedPlatform, platform string, available bool, build func()) error {",
		"listingplatform.BuildOne(",
		"listingplatform.Section{",
		"UnavailableError: ErrPreviewPlatformUnavailable",
	} {
		if !strings.Contains(applyHelperContent, needle) {
			t.Fatalf("platform_section_apply.go should contain %q", needle)
		}
	}
}

func TestExportPlatformBuilderRegistryUsesNeutralPlatformSectionDispatcher(t *testing.T) {
	t.Parallel()

	platformsSrc, err := os.ReadFile("export_builder_platforms.go")
	if err != nil {
		t.Fatalf("ReadFile(export_builder_platforms.go) error = %v", err)
	}
	platformsContent := string(platformsSrc)
	for _, needle := range []string{
		`listingplatform "task-processor/internal/listing/platform"`,
		"func exportPlatformBuilders() []listingplatform.RegisteredSectionBuilder[*ListingKitResult, *ListingKitExport] {",
		"func buildExportPlatformSections(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {",
		"return listingplatform.SectionBuilders(exportPlatformRegistrations())",
		"return listingplatform.BuildRegisteredSections(exportPlatformBuilders(), result, export, selectedPlatform)",
	} {
		if !strings.Contains(platformsContent, needle) {
			t.Fatalf("export_builder_platforms.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"type exportPlatformBuilder interface {",
		"type exportPlatformBuilderFunc struct {",
		"platformSectionBuilder",
		"platformSectionBuilders(",
		"buildPlatformSections(",
		"if err := builder.build(result, export, selectedPlatform); err != nil {",
		"listingplatform.BuildAll(",
		"listingplatform.Builder{",
		"previewdomain.PlatformSectionBuilder{",
		"previewdomain.BuildPlatformSections(",
		"func buildAmazonExportSection(",
		"func buildSheinExportSection(",
		"func buildTemuExportSection(",
		"func buildWalmartExportSection(",
	} {
		if strings.Contains(platformsContent, needle) {
			t.Fatalf("export_builder_platforms.go should not contain %q after shared platform-section builder extraction", needle)
		}
	}

	sectionSrc, err := os.ReadFile("export_platform_sections.go")
	if err != nil {
		t.Fatalf("ReadFile(export_platform_sections.go) error = %v", err)
	}
	sectionContent := string(sectionSrc)
	for _, needle := range []string{
		"func buildAmazonExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {",
		"func buildSheinExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {",
		"func buildTemuExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {",
		"func buildWalmartExportSection(result *ListingKitResult, export *ListingKitExport, selectedPlatform string) error {",
	} {
		if !strings.Contains(sectionContent, needle) {
			t.Fatalf("export_platform_sections.go should contain %q", needle)
		}
	}

	registrySrc, err := os.ReadFile("export_platform_registry.go")
	if err != nil {
		t.Fatalf("ReadFile(export_platform_registry.go) error = %v", err)
	}
	registryContent := string(registrySrc)
	if !strings.Contains(registryContent, "return listingplatform.SupportedSectionRegistrations(map[string]listingplatform.SectionBuildFunc[*ListingKitResult, *ListingKitExport]{") {
		t.Fatal("export_platform_registry.go should build registrations through listingplatform.SupportedSectionRegistrations")
	}
	if strings.Contains(registryContent, "return []listingplatform.SectionRegistration") {
		t.Fatal("export_platform_registry.go should not hand-code registration order")
	}

	applySrc, err := os.ReadFile("export_platform_apply.go")
	if err != nil {
		t.Fatalf("ReadFile(export_platform_apply.go) error = %v", err)
	}
	applyContent := string(applySrc)
	for _, needle := range []string{
		"func applyExportPlatformSection(selectedPlatform, platform string, available bool, build func()) error {",
		"return applyPlatformSection(selectedPlatform, platform, available, build)",
	} {
		if !strings.Contains(applyContent, needle) {
			t.Fatalf("export_platform_apply.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		`"task-processor/internal/listing/platform"`,
		"listingplatform.BuildOne(",
		"listingplatform.Section{",
		"previewdomain.BuildPlatformSection(",
		"adaptPreviewPlatformSectionError(",
	} {
		if strings.Contains(applyContent, needle) {
			t.Fatalf("export_platform_apply.go should not contain %q after shared platform-section apply extraction", needle)
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

	if _, err := os.ReadFile("preview_platform_selection.go"); err == nil {
		t.Fatal("preview_platform_selection.go should be removed after direct listing platform selection usage")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(preview_platform_selection.go) unexpected error = %v", err)
	}

	cases := []struct {
		file string
		call string
	}{
		{file: "preview_builder_stages.go", call: "listingplatform.ValidateSelectedPlatform(selectedPlatform)"},
		{file: "export_builder.go", call: "listingplatform.ValidateSelectedPlatform(selectedPlatform)"},
		{file: "revision_apply.go", call: "listingplatform.ValidateSelectedPlatform(req.Platform)"},
	}
	for _, tc := range cases {
		src, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
		}
		content := string(src)
		if !strings.Contains(content, tc.call) {
			t.Fatalf("%s should contain %q", tc.file, tc.call)
		}
		if strings.Contains(content, "previewdomain.ValidateSelectedPlatform(") {
			t.Fatalf("%s should not contain previewdomain.ValidateSelectedPlatform after neutral platform registry extraction", tc.file)
		}
	}
}

func TestPreviewPlatformErrorsUseNeutralPlatformPackage(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("preview_errors.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_errors.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		`listingplatform "task-processor/internal/listing/platform"`,
		"ErrUnsupportedPreviewPlatform = listingplatform.ErrUnsupportedPlatform",
		"ErrPreviewPlatformUnavailable = listingplatform.ErrPlatformUnavailable",
		"ErrTaskResultUnavailable = previewdomain.ErrResultUnavailable",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("preview_errors.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"ErrUnsupportedPreviewPlatform = previewdomain.ErrUnsupportedPlatform",
		"ErrPreviewPlatformUnavailable = previewdomain.ErrPlatformUnavailable",
		"ErrTaskResultUnavailable = listingplatform.",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("preview_errors.go should not contain %q after platform error extraction", needle)
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
		"func buildSheinResolutionCacheSummary(pkg *SheinPackage) *SheinResolutionCacheSummary {",
		"return sheinworkspace.BuildResolutionCacheSummary(pkg)",
	} {
		if strings.Contains(mainContent, needle) {
			t.Fatalf("preview_builder_shein.go should not contain %q", needle)
		}
	}

	assertFileAbsent(t, "preview_builder_shein_resolution_cache.go")

	payloadSrc, err := os.ReadFile("preview_builder_shein_payload.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein_payload.go) error = %v", err)
	}
	payloadContent := string(payloadSrc)
	for _, needle := range []string{
		"ResolutionCache:   sheinworkspace.BuildResolutionCacheSummary(pkg),",
		`sheinworkspace "task-processor/internal/marketplace/shein/workspace"`,
	} {
		if !strings.Contains(payloadContent, needle) {
			t.Fatalf("preview_builder_shein_payload.go should contain %q", needle)
		}
	}
}

func TestSheinStoreResolutionSummaryCallsWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("shein_store_resolution_presentation.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_store_resolution_presentation.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"return sheinworkspace.BuildStoreResolutionSummary(",
		`sheinworkspace "task-processor/internal/marketplace/shein/workspace"`,
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("shein_store_resolution_presentation.go should contain %q", needle)
		}
	}
	if strings.Contains(content, "func buildSheinStoreResolutionSummaryValue(") {
		t.Fatalf("shein_store_resolution_presentation.go should not keep %q", "func buildSheinStoreResolutionSummaryValue(")
	}

	previewSrc, err := os.ReadFile("service_shein_store_resolution_preview_support_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_store_resolution_preview_support_helper.go) error = %v", err)
	}
	previewContent := string(previewSrc)
	if !strings.Contains(previewContent, "return sheinworkspace.BuildStoreResolutionSummary(") {
		t.Fatalf("service_shein_store_resolution_preview_support_helper.go should call workspace summary builder directly")
	}
	if strings.Contains(previewContent, "return &SheinStoreResolutionSummary{") {
		t.Fatalf("service_shein_store_resolution_preview_support_helper.go should not inline store resolution summary literals")
	}
}

func TestSheinPreviewPayloadAssemblerLivesOutsideMainSheinPreviewBuilder(t *testing.T) {
	t.Parallel()

	mainSrc, err := os.ReadFile("preview_builder_shein.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein.go) error = %v", err)
	}
	mainContent := string(mainSrc)
	for _, needle := range []string{
		"type sheinPreviewPayloadBodyInput struct {",
		"func buildSheinPreviewPayloadBody(input sheinPreviewPayloadBodyInput) *SheinPreviewPayload {",
		"sheinDisplayTitle(pkg),",
	} {
		if strings.Contains(mainContent, needle) {
			t.Fatalf("preview_builder_shein.go should not contain %q", needle)
		}
	}

	payloadSrc, err := os.ReadFile("preview_builder_shein_payload.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein_payload.go) error = %v", err)
	}
	payloadContent := string(payloadSrc)
	for _, needle := range []string{
		"type sheinPreviewPayloadBodyInput struct {",
		"func buildSheinPreviewPayloadBody(input sheinPreviewPayloadBodyInput) *SheinPreviewPayload {",
		"Headline:",
		"sheinDisplayTitle(pkg),",
		"ResolutionCache:   sheinworkspace.BuildResolutionCacheSummary(pkg),",
		"FinalReview:       buildSheinFinalReviewPayload(pkg, input.canonical, input.readiness),",
	} {
		if !strings.Contains(payloadContent, needle) {
			t.Fatalf("preview_builder_shein_payload.go should contain %q", needle)
		}
	}
}

func TestSheinPreviewReviewSummaryHelperLivesOutsideMainSheinPreviewBuilder(t *testing.T) {
	t.Parallel()

	mainSrc, err := os.ReadFile("preview_builder_shein.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein.go) error = %v", err)
	}
	mainContent := string(mainSrc)
	for _, needle := range []string{
		"func buildSheinPreviewReviewSummary(pkg *sheinpub.Package) (bool, []string) {",
		"needsReview := len(pkg.ReviewNotes) > 0",
		"summary := uniqueStrings(append([]string(nil), pkg.ReviewNotes...))",
	} {
		if strings.Contains(mainContent, needle) {
			t.Fatalf("preview_builder_shein.go should not contain %q", needle)
		}
	}

	assertFileAbsent(t, "preview_builder_shein_review_summary.go")

	mainContent = string(mainSrc)
	for _, needle := range []string{
		"needsReview, summary := sheinworkspace.BuildPreviewReviewSummary(pkg)",
		`sheinworkspace "task-processor/internal/marketplace/shein/workspace"`,
	} {
		if !strings.Contains(mainContent, needle) {
			t.Fatalf("preview_builder_shein.go should contain %q", needle)
		}
	}
}

func TestSheinSourceProductSummaryHelperLivesOutsideMainSheinPreviewBuilder(t *testing.T) {
	t.Parallel()

	mainSrc, err := os.ReadFile("preview_builder_shein.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein.go) error = %v", err)
	}
	mainContent := string(mainSrc)
	for _, needle := range []string{
		"func buildSheinSourceProductSummary(product *canonical.Product) *SheinSourceProductSummary {",
		"return sheinworkspace.BuildSourceProductSummary(product)",
	} {
		if strings.Contains(mainContent, needle) {
			t.Fatalf("preview_builder_shein.go should not contain %q", needle)
		}
	}

	assertFileAbsent(t, "preview_builder_shein_source_product.go")

	payloadSrc, err := os.ReadFile("preview_builder_shein_payload.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein_payload.go) error = %v", err)
	}
	payloadContent := string(payloadSrc)
	for _, needle := range []string{
		"SourceProduct:   sheinworkspace.BuildSourceProductSummary(input.canonical),",
		`sheinworkspace "task-processor/internal/marketplace/shein/workspace"`,
	} {
		if !strings.Contains(payloadContent, needle) {
			t.Fatalf("preview_builder_shein_payload.go should contain %q", needle)
		}
	}
}

func TestSheinPreviewWorkspaceOverviewHelperLivesOutsideMainSheinPreviewBuilder(t *testing.T) {
	t.Parallel()

	mainSrc, err := os.ReadFile("preview_builder_shein.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein.go) error = %v", err)
	}
	mainContent := string(mainSrc)
	for _, needle := range []string{
		"func buildSheinPreviewWorkspaceOverview(statusOverview *sheinworkspace.StatusOverview, submitState *sheinworkspace.SubmitStateInput, repairCenter *SheinRepairCenter) *sheinworkspace.WorkspaceOverview {",
	} {
		if strings.Contains(mainContent, needle) {
			t.Fatalf("preview_builder_shein.go should not contain %q", needle)
		}
	}

	assertFileAbsent(t, "preview_builder_shein_workspace_overview.go")

	for _, needle := range []string{
		"repairState := sheinworkspace.BuildRepairStateInput(repairCenter)",
		"workspaceOverview: sheinworkspace.BuildWorkspaceOverview(statusOverview, submitState, repairState)",
	} {
		if !strings.Contains(mainContent, needle) {
			t.Fatalf("preview_builder_shein.go should contain %q", needle)
		}
	}
}

func TestSheinFinalReviewImageHelpersLiveOutsideMainFinalReviewBuilder(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "preview_builder_shein_final_review_images.go")

	for _, needle := range []string{
		"final.Images = sheinworkspace.BuildFinalReviewImages(pkg.DraftPayload, pkg.FinalSubmissionDraft, pkg.PreviewPayload)",
		`sheinworkspace "task-processor/internal/marketplace/shein/workspace"`,
	} {
		finalReviewSrc, err := os.ReadFile("preview_builder_shein_final_review.go")
		if err != nil {
			t.Fatalf("ReadFile(preview_builder_shein_final_review.go) error = %v", err)
		}
		if !strings.Contains(string(finalReviewSrc), needle) {
			t.Fatalf("preview_builder_shein_final_review.go should contain %q", needle)
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

func TestSheinFinalReviewSKUHelpersLiveOutsideMainFinalReviewBuilder(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "preview_builder_shein_final_review_skus.go")

	for _, needle := range []string{
		"final.SKUs = sheinworkspace.BuildFinalReviewSKUs(pkg.DraftPayload)",
		`sheinworkspace "task-processor/internal/marketplace/shein/workspace"`,
	} {
		finalReviewSrc, err := os.ReadFile("preview_builder_shein_final_review.go")
		if err != nil {
			t.Fatalf("ReadFile(preview_builder_shein_final_review.go) error = %v", err)
		}
		if !strings.Contains(string(finalReviewSrc), needle) {
			t.Fatalf("preview_builder_shein_final_review.go should contain %q", needle)
		}
	}

	finalReviewSrc, err := os.ReadFile("preview_builder_shein_final_review.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein_final_review.go) error = %v", err)
	}
	finalReviewContent := string(finalReviewSrc)

	for _, needle := range []string{
		"func buildSheinFinalReviewSKUs(draft *SheinRequestDraft) []SheinFinalReviewSKU {",
		"func buildSheinFinalReviewSKU(supplierCode string, sku SheinSKUDraft) SheinFinalReviewSKU {",
	} {
		if strings.Contains(finalReviewContent, needle) {
			t.Fatalf("preview_builder_shein_final_review.go should not contain %q", needle)
		}
	}
}

func TestSheinImageUploadPreviewHelpersLiveOutsideSubmitImageRuntime(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "preview_builder_shein_image_upload.go")

	payloadSrc, err := os.ReadFile("preview_builder_shein_payload.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein_payload.go) error = %v", err)
	}
	payloadContent := string(payloadSrc)
	for _, needle := range []string{
		"ImageUpload: sheinworkspace.BuildImageUploadPreflight(",
		`sheinworkspace "task-processor/internal/marketplace/shein/workspace"`,
		"sheinpub.IsUploadedImageURL,",
		"sheinImageUploadCache(pkg)[strings.TrimSpace(sourceURL)]",
		"sheinpub.IsSDSImageURL,",
	} {
		if !strings.Contains(payloadContent, needle) {
			t.Fatalf("preview_builder_shein_payload.go should contain %q", needle)
		}
	}

	submitImagesSrc, err := os.ReadFile("shein_submit_images.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_images.go) error = %v", err)
	}
	submitImagesContent := string(submitImagesSrc)

	for _, needle := range []string{
		"func buildSheinImageUploadPreflight(pkg *SheinPackage) *SheinImageUploadPreflight {",
		"return sheinworkspace.BuildImageUploadPreflight(",
		"isSheinUploadedImageURL,",
		"sheinImageUploadCacheHit,",
		"isSDSImageURL,",
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

	if _, err := os.ReadFile("service_shein_category_search_support.go"); err == nil {
		t.Fatal("service_shein_category_search_support.go should be removed after category search ranking moved to SHEIN workspace")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_shein_category_search_support.go) unexpected error = %v", err)
	}

	adminSrc, err := os.ReadFile("shein_admin_service.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_admin_service.go) error = %v", err)
	}
	adminContent := string(adminSrc)
	for _, needle := range []string{
		"sheinworkspace.SearchCategoryCandidates(tree.Data, trimmedQuery)",
		"func buildSheinCategorySearchCandidates(items []sheinworkspace.CategorySearchCandidate) []SheinCategorySearchCandidate {",
	} {
		if !strings.Contains(adminContent, needle) {
			t.Fatalf("shein_admin_service.go should contain %q", needle)
		}
	}

	workspaceSrc, err := os.ReadFile("../marketplace/shein/workspace/category_search.go")
	if err != nil {
		t.Fatalf("ReadFile(../marketplace/shein/workspace/category_search.go) error = %v", err)
	}
	workspaceContent := string(workspaceSrc)
	for _, needle := range []string{
		"type CategorySearchCandidate struct {",
		"func SearchCategoryCandidates(nodes []sheincategory.CategoryTreeNode, query string) []CategorySearchCandidate {",
		"func categoryMatchScore(path []string, normalizedQuery string, tokens []string) (int, bool) {",
	} {
		if !strings.Contains(workspaceContent, needle) {
			t.Fatalf("workspace category search should contain %q", needle)
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
		"listingsubmission.PreferredSubmitAction(",
	} {
		if !strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_action_normalization_helper.go should keep %q", needle)
		}
	}
	if strings.Contains(helperContent, "func normalizePreferredSheinSubmitAction(action string) string {") {
		t.Fatalf("service_submit_action_normalization_helper.go should delegate preferred submit action normalization to internal/listing/submission")
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
		"identity := RequestIdentityFromContext(ctx)",
		"ctx = WithTenantID(ctx, tenantID)",
		"return WithRequestIdentity(ctx, identity), nil",
	} {
		if !strings.Contains(helperContent, needle) {
			t.Fatalf("service_submit_task_identity_helper.go should contain %q", needle)
		}
	}

	requestIdentitySrc, err := os.ReadFile("request_identity.go")
	if err != nil {
		t.Fatalf("ReadFile(request_identity.go) error = %v", err)
	}
	requestIdentityContent := string(requestIdentitySrc)

	for _, needle := range []string{
		"type RequestIdentity struct {",
		"func WithRequestIdentity(ctx context.Context, identity RequestIdentity) context.Context {",
		"return openaiclient.WithIdentity(ctx, openaiclient.Identity{",
		"func RequestIdentityFromContext(ctx context.Context) RequestIdentity {",
		"identity := openaiclient.IdentityFromContext(ctx)",
	} {
		if !strings.Contains(requestIdentityContent, needle) {
			t.Fatalf("request_identity.go should contain %q", needle)
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

	executionSrc, err := os.ReadFile("task_submission_execution_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_execution_service.go) error = %v", err)
	}
	executionContent := string(executionSrc)
	if strings.Contains(executionContent, "func (s *taskSubmissionExecutionService) resolveSheinSubmitContext(ctx context.Context, task *Task) (context.Context, error) {") {
		t.Fatalf("task_submission_execution_service.go should not keep thin submit-context wrapper; call withSheinSubmitTaskIdentity directly")
	}
	if strings.Contains(executionContent, "func (s *taskSubmissionExecutionService) resolveSheinSubmitRuntime(ctx context.Context, task *Task) (context.Context, int64, error) {") {
		t.Fatalf("task_submission_execution_service.go should not keep thin submit-runtime wrapper; call resolveSheinStoreRuntime directly")
	}

	imageExecutionSrc, err := os.ReadFile("task_submission_execution_images.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_execution_images.go) error = %v", err)
	}
	imageExecutionContent := string(imageExecutionSrc)
	if strings.Contains(imageExecutionContent, "func (s *taskSubmissionExecutionService) resolveSheinImageUploadRuntime(ctx context.Context, task *Task) (context.Context, int64, error) {") {
		t.Fatalf("task_submission_execution_images.go should not keep thin image-upload runtime wrapper; call resolveSheinStoreRuntime directly")
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

	eventsSrc, err := os.ReadFile("service_shein_store_resolution_preview_support_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_store_resolution_preview_support_helper.go) error = %v", err)
	}
	eventsContent := string(eventsSrc)

	if strings.Contains(eventsContent, "func (s *service) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {") {
		t.Fatalf("service_shein_store_resolution_preview_support_helper.go should not contain %q", "func (s *service) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {")
	}

	if !strings.Contains(eventsContent, "func sheinSubmissionEventsWithStoreResolution(events []sheinpub.SubmissionEvent, task *Task) []sheinpub.SubmissionEvent {") {
		t.Fatalf("service_shein_store_resolution_preview_support_helper.go should keep %q", "func sheinSubmissionEventsWithStoreResolution(events []sheinpub.SubmissionEvent, task *Task) []sheinpub.SubmissionEvent {")
	}

	presentationSrc, err := os.ReadFile("shein_store_resolution_presentation.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_store_resolution_presentation.go) error = %v", err)
	}
	presentationContent := string(presentationSrc)
	if !strings.Contains(presentationContent, "func sheinSubmissionStoreResolutionFromSnapshot(snapshot *SheinStoreResolutionSnapshot) *sheinpub.SubmissionStoreResolution {") {
		t.Fatalf("shein_store_resolution_presentation.go should keep %q", "func sheinSubmissionStoreResolutionFromSnapshot(snapshot *SheinStoreResolutionSnapshot) *sheinpub.SubmissionStoreResolution {")
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

	if _, err := os.ReadFile("shein_final_draft.go"); err == nil {
		t.Fatal("shein_final_draft.go should be removed after final draft image wrapper cleanup")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(shein_final_draft.go) unexpected error = %v", err)
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
		"return lifecycle.startSheinPublishWorkflowAttempt(ctx, taskID, task, req, opts)",
		"func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {",
		"client, enabled := resolveSubmissionWorkflowClient(s)",
		"enabled &&",
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

func TestTaskDirectSubmissionSupportUsesSubmissionDomainRunner(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("task_direct_submission_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_direct_submission_support.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func newSheinDirectSubmitFlowRunner(s *taskDirectSubmissionService) *submissiondomain.DirectSubmitFlowService[*Task, *SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *ListingKitPreview] {",
		"return submissiondomain.NewDirectSubmitFlowService(",
		"func newSheinDirectSubmitPayloadStages(s *taskDirectSubmissionService) *submissiondomain.PayloadStageService[*Task, *SheinPackage, *sheinproduct.Product, *sheinpub.SubmitSnapshot] {",
		"return submissiondomain.NewPayloadStageService(",
		"BuildProductAPI: s.loadDirectSubmitProductAPI,",
		"PersistPhase: func(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts submissiondomain.DirectSubmitFlowOptions, phase string) error {",
		"return s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, sheinDirectSubmitOptions{",
		"startedAt: opts.StartedAt,",
		"PrepareProduct: s.prepareDirectSubmitProduct,",
		"return sheinpub.ProductPendingImageUploadCount(product) > 0",
		"UploadImages:     s.uploadPendingDirectSubmitImages,",
		"PreValidate:      s.preValidateDirectSubmitProduct,",
		"SubmitRemote:     s.completeDirectRemoteSubmit,",
		"PersistPhase: func(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], phase string) error {",
		"return s.persistSheinDirectSubmitPhase(ctx, in.TaskID, in.Task, in.Package, sheinDirectSubmitOptions{",
		"PersistSnapshot: func(_ context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], snapshot *sheinpub.SubmitSnapshot) error {",
		"sheinpub.SetSubmissionSnapshot(in.Package, in.Action, in.RequestID, snapshot)",
		"sheinpub.PreValidateSubmitProductWithOptions(product, !sheinpub.SecondarySaleAttributeRequired(in.Package))",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("task_direct_submission_support.go should contain %q", needle)
		}
	}
	if strings.Contains(content, "func (s *taskDirectSubmissionService) persistDirectSubmitPayloadPhase(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], phase string) error {") {
		t.Fatal("task_direct_submission_support.go should not keep thin direct payload phase wrapper")
	}
	if strings.Contains(content, "func (s *taskDirectSubmissionService) persistDirectSubmitPhase(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts submissiondomain.DirectSubmitFlowOptions, phase string) error {") {
		t.Fatal("task_direct_submission_support.go should not keep thin direct flow phase wrapper")
	}
	if strings.Contains(content, "func (s *taskDirectSubmissionService) persistDirectSubmitSnapshot(_ context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], snapshot *sheinpub.SubmitSnapshot) error {") {
		t.Fatal("task_direct_submission_support.go should not keep thin direct payload snapshot wrapper")
	}
	if strings.Contains(content, "s.preValidateSheinSubmitProduct(") {
		t.Fatal("task_direct_submission_support.go should call SHEIN publishing pre-validation directly")
	}
	if strings.Contains(content, "func sheinDirectSubmitNeedsImageUpload(") {
		t.Fatal("task_direct_submission_support.go should not keep a direct submit image upload wrapper")
	}
}

func TestTaskTemporalSubmissionPayloadUsesSubmissionDomainRunner(t *testing.T) {
	t.Parallel()

	supportSrc, err := os.ReadFile("task_temporal_submission_payload_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_temporal_submission_payload_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func newSheinTemporalSubmitPayloadStages(s *taskTemporalSubmissionFlowService) *submissiondomain.PayloadStageService[*Task, *SheinPackage, *sheinproduct.Product, *sheinpub.SubmitSnapshot] {",
		"return submissiondomain.NewPayloadStageService(",
		"PersistPhase: func(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], phase string) error {",
		"return s.persistSheinSubmitPhase(ctx, in.TaskID, in.Task.Result, in.Package, in.Action, in.RequestID, phase)",
		"PreparePayload: s.prepareTemporalSubmitPayload,",
		"PersistSnapshot: func(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], snapshot *sheinpub.SubmitSnapshot) error {",
		"if s.persistence == nil {",
		"return s.persistence.persistSheinSubmitSnapshot(ctx, in.TaskID, in.Task.Result, in.Package, in.Action, in.RequestID, snapshot)",
		"UploadImages: func(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], product *sheinproduct.Product) error {",
		"return s.uploadSheinSubmitImages(ctx, in.Task, in.Package, product)",
		"FinalizeUploaded: s.finalizeTemporalSubmitPayload,",
		"PreValidate: func(_ context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], product *sheinproduct.Product) error {",
		"return sheinpub.PreValidateSubmitProductWithOptions(product, !sheinpub.SecondarySaleAttributeRequired(in.Package))",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_temporal_submission_payload_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"func (s *taskTemporalSubmissionFlowService) persistTemporalSubmitPayloadPhase(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], phase string) error {",
		"func (s *taskTemporalSubmissionFlowService) persistTemporalSubmitPayloadSnapshot(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], snapshot *sheinpub.SubmitSnapshot) error {",
		"func (s *taskTemporalSubmissionFlowService) uploadTemporalSubmitPayloadImages(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], product *sheinproduct.Product) error {",
		"func (s *taskTemporalSubmissionFlowService) preValidateTemporalSubmitPayload(_ context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], product *sheinproduct.Product) error {",
	} {
		if strings.Contains(supportContent, needle) {
			t.Fatalf("task_temporal_submission_payload_support.go should not keep thin payload callback wrapper %q", needle)
		}
	}
	if strings.Contains(supportContent, "s.preValidateSheinSubmitProduct(") {
		t.Fatal("task_temporal_submission_payload_support.go should call SHEIN publishing pre-validation directly")
	}

	payloadSrc, err := os.ReadFile("service_shein_publish_temporal_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_publish_temporal_entrypoints.go) error = %v", err)
	}
	payloadContent := string(payloadSrc)

	for _, needle := range []string{
		"return flow.PrepareSheinPublishPayload(ctx, in)",
		"return flow.UploadSheinPublishImages(ctx, in)",
		"return flow.PreValidateSheinPublish(ctx, in)",
		"return flow.SubmitSheinPublishRemote(ctx, in)",
	} {
		if !strings.Contains(payloadContent, needle) {
			t.Fatalf("service_shein_publish_temporal_entrypoints.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"prepareSheinSubmitPayloadProduct(ctx, in.TaskID, in.Action, in.RequestID, task, pkg, s.prepareSheinSubmitProduct)",
		"finalizeSheinUploadedSubmitPayload(ctx, in.TaskID, in.Action, in.RequestID, task, in, s.resolveSubmitSettings)",
		"return s.preValidateSheinSubmitProduct(pkg, in.Product)",
		"s.payloadStages.Prepare(",
		"s.payloadStages.UploadImages(",
		"s.payloadStages.PreValidate(",
		"s.remoteSubmitter.Submit(",
	} {
		if strings.Contains(payloadContent, needle) {
			t.Fatalf("service_shein_publish_temporal_entrypoints.go should not contain %q after temporal flow extraction", needle)
		}
	}

	if _, err := os.ReadFile("task_temporal_submission_payload.go"); err == nil {
		t.Fatal("task_temporal_submission_payload.go should be removed after service-owned temporal host extraction")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(task_temporal_submission_payload.go) unexpected error = %v", err)
	}

	flowSrc, err := os.ReadFile("task_temporal_submission_flow_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_temporal_submission_flow_service.go) error = %v", err)
	}
	flowContent := string(flowSrc)
	for _, needle := range []string{
		"s.payloadStages.Prepare(",
		"s.payloadStages.UploadImages(",
		"s.payloadStages.PreValidate(",
		"s.remoteSubmitter.Submit(",
	} {
		if !strings.Contains(flowContent, needle) {
			t.Fatalf("task_temporal_submission_flow_service.go should contain %q", needle)
		}
	}
}

func TestTaskSubmissionRemoteSupportUsesSubmissionDomainRunner(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("task_submission_remote_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_remote_support.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func newSheinRemoteSubmitService(",
		"return submissiondomain.NewRemoteSubmitService(",
		"state := prepareSheinRemoteSubmitState(pkg, action, requestID, product, snapshot)",
		"ExecuteAttempt: executeAttempt,",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("task_submission_remote_support.go should contain %q", needle)
		}
	}

	stateSrc, err := os.ReadFile("submit_remote_state_shein.go")
	if err != nil {
		t.Fatalf("ReadFile(submit_remote_state_shein.go) error = %v", err)
	}
	stateContent := string(stateSrc)
	for _, needle := range []string{
		"func prepareSheinRemoteSubmitState(",
		"supplierCode, snapshot := sheinpub.PrepareSubmissionPersistenceInput(",
	} {
		if !strings.Contains(stateContent, needle) {
			t.Fatalf("submit_remote_state_shein.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"sheinSubmitSupplierCode(",
		"setSheinSubmitSupplierCode(",
		"setSheinSubmitSnapshot(",
	} {
		if strings.Contains(stateContent, needle) {
			t.Fatalf("submit_remote_state_shein.go should not contain %q after publishing extraction", needle)
		}
	}
}

func TestTaskSubmissionSuccessPersistenceUsesSubmissionDomainRunner(t *testing.T) {
	t.Parallel()

	supportSrc, err := os.ReadFile("task_submission_success_persistence_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_success_persistence_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func newSheinSubmissionSuccessPersistenceService(",
		"return submissiondomain.NewSuccessPersistenceService(",
		"PersistResultAndPhase:       persistResultAndPhase,",
		"CompleteAttempt:             completeAttempt,",
		"PersistSuccessfulSubmission: persistSuccessfulSubmission,",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_submission_success_persistence_support.go should contain %q", needle)
		}
	}

	stateSrc, err := os.ReadFile("task_submission_state_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_state_service.go) error = %v", err)
	}
	stateContent := string(stateSrc)

	for _, needle := range []string{
		"successRunner := newSheinSubmissionSuccessPersistenceService(",
		"service.resultRunner = submissiondomain.NewResultPersistenceService(",
	} {
		if !strings.Contains(stateContent, needle) {
			t.Fatalf("task_submission_state_service.go should contain %q", needle)
		}
	}

	stateSupportSrc, err := os.ReadFile("task_submission_state_persistence_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_state_persistence_support.go) error = %v", err)
	}
	stateSupportContent := string(stateSupportSrc)

	for _, needle := range []string{
		"return s.resultRunner.Finish(ctx, submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]{",
	} {
		if !strings.Contains(stateSupportContent, needle) {
			t.Fatalf("task_submission_state_persistence_support.go should contain %q", needle)
		}
	}

	temporalSrc, err := os.ReadFile("task_temporal_submission_persistence_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_temporal_submission_persistence_service.go) error = %v", err)
	}
	temporalContent := string(temporalSrc)

	for _, needle := range []string{
		"service.resultRunner = submissiondomain.NewResultPersistenceService(",
	} {
		if !strings.Contains(temporalContent, needle) {
			t.Fatalf("task_temporal_submission_persistence_service.go should contain %q", needle)
		}
	}

	temporalSupportSrc, err := os.ReadFile("task_temporal_submission_persistence_service_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_temporal_submission_persistence_service_support.go) error = %v", err)
	}
	temporalSupportContent := string(temporalSupportSrc)

	for _, needle := range []string{
		"return s.resultRunner.PersistSuccess(ctx, submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]{",
		"func (s *taskTemporalSubmissionPersistenceService) persistTemporalSuccessResultAndPhase(",
		"func (s *taskTemporalSubmissionPersistenceService) completeTemporalSubmitAttempt(",
	} {
		if !strings.Contains(temporalSupportContent, needle) {
			t.Fatalf("task_temporal_submission_persistence_service_support.go should contain %q", needle)
		}
	}

	persistSrc, err := os.ReadFile("service_shein_publish_temporal_entrypoints.go")
	if err != nil {
		t.Fatalf("ReadFile(service_shein_publish_temporal_entrypoints.go) error = %v", err)
	}
	persistContent := string(persistSrc)
	for _, needle := range []string{
		"return persistence.PersistSheinPublishSuccess(ctx, in)",
	} {
		if !strings.Contains(persistContent, needle) {
			t.Fatalf("service_shein_publish_temporal_entrypoints.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"sheinpub.ApplySubmissionPersistenceInput(",
	} {
		if strings.Contains(persistContent, needle) {
			t.Fatalf("service_shein_publish_temporal_entrypoints.go should not contain %q after temporal persistence service extraction", needle)
		}
	}

	if _, err := os.ReadFile("task_temporal_submission_persistence.go"); err == nil {
		t.Fatal("task_temporal_submission_persistence.go should be removed after service-owned temporal host extraction")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(task_temporal_submission_persistence.go) unexpected error = %v", err)
	}
}

func TestTaskSubmissionFailurePersistenceUsesSubmissionDomainRunner(t *testing.T) {
	t.Parallel()

	supportSrc, err := os.ReadFile("task_submission_failure_persistence_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_failure_persistence_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func newSheinSubmissionFailurePersistenceService(",
		"return submissiondomain.NewFailurePersistenceService(",
		"RecordFailure: recordFailure,",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_submission_failure_persistence_support.go should contain %q", needle)
		}
	}

	stateSrc, err := os.ReadFile("task_submission_state_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_state_service.go) error = %v", err)
	}
	stateContent := string(stateSrc)

	for _, needle := range []string{
		"failureRunner := newSheinSubmissionFailurePersistenceService(service.recordFailureState)",
		"service.failureRunner = failureRunner",
	} {
		if !strings.Contains(stateContent, needle) {
			t.Fatalf("task_submission_state_service.go should contain %q", needle)
		}
	}

	stateSupportSrc, err := os.ReadFile("task_submission_state_persistence_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_state_persistence_support.go) error = %v", err)
	}
	stateSupportContent := string(stateSupportSrc)

	for _, needle := range []string{
		"s.failureRunner.PersistFailure(ctx, submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]{",
		"func (s *taskSubmissionStateService) recordFailureState(",
	} {
		if !strings.Contains(stateSupportContent, needle) {
			t.Fatalf("task_submission_state_persistence_support.go should contain %q", needle)
		}
	}

	temporalRootSrc, err := os.ReadFile("task_temporal_submission_persistence_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_temporal_submission_persistence_service.go) error = %v", err)
	}
	temporalSupportSrc, err := os.ReadFile("task_temporal_submission_persistence_service_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_temporal_submission_persistence_service_support.go) error = %v", err)
	}
	temporalContent := string(temporalRootSrc) + "\n" + string(temporalSupportSrc)

	for _, needle := range []string{
		"return s.resultRunner.PersistFailure(ctx, submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]{",
		"recordTemporalFailureState := func(ctx context.Context, in submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]) error {",
		"return service.recordSheinSubmissionFailureForState(ctx, in.TaskID, in.Result, in.Package, in.Action, in.RequestID, in.Phase, in.Err)",
	} {
		if !strings.Contains(temporalContent, needle) {
			t.Fatalf("temporal persistence sources should contain %q", needle)
		}
	}
	if strings.Contains(temporalContent, "func (s *taskTemporalSubmissionPersistenceService) recordTemporalFailureState(") {
		t.Fatal("task_temporal_submission_persistence_service_support.go should not keep thin temporal failure-state wrapper")
	}
}

func TestTaskStudioBatchRunAdapterUsesListingStudioRunner(t *testing.T) {
	t.Parallel()

	adapterSrc, err := os.ReadFile("task_studio_batch_run_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_run_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"func newListingStudioBatchRunService(",
		"repo StudioBatchRunRepository,",
		"studioSessionRepo studioBatchSeedSessionRepository,",
		"startRun func(ctx context.Context, runID string) error,",
		"return studiodomain.NewBatchRunService(studiodomain.BatchRunServiceConfig{",
		"Repo:          studioBatchRunRepositoryAdapter{repo: repo},",
		"SessionRepo:   studioBatchSeedSessionRepositoryAdapter{repo: studioSessionRepo},",
		"StartRun:      startRun,",
		"NewRunID:      uuid.NewString,",
		"RequestUserID: RequestUserIDFromContext,",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_batch_run_adapter.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("task_studio_batch_run_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_run_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"service.ensureRunner()",
		"func (s *taskStudioBatchRunService) ensureRunner() {",
		"s.runner = newListingStudioBatchRunService(s.repo, s.studioSessionRepo, s.startRun)",
		"run, items, err := s.runner.CreateBatchRun(ctx, adaptStudioBatchRunRequest(req))",
		"return adaptStudioBatchRunRecord(run), adaptStudioBatchRunItems(items), nil",
		"run, err := s.runner.GetBatchRun(ctx, runID)",
		"items, err := s.runner.ListBatchRunItems(ctx, runID)",
		"return s.runner.CancelBatchRun(ctx, runID)",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_run_service.go should contain %q", needle)
		}
	}
}

func TestTaskStudioBatchDetailAdapterUsesListingStudioRunner(t *testing.T) {
	t.Parallel()

	adapterSrc, err := os.ReadFile("task_studio_batch_detail_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_detail_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"func newListingStudioBatchDetailService(",
		"repo StudioBatchRepository,",
		"studioSessionRepo studioBatchSeedSessionRepository,",
		"taskLinkRepo StudioBatchTaskLinkRepository,",
		"getTask func(context.Context, string) (*Task, error),",
		"ensureGraph func(context.Context, string) error,",
		"return studiodomain.NewBatchDetailService(studiodomain.BatchDetailServiceConfig[",
		"ResolveWithoutGraph: func(ctx context.Context, batchID string) (*StudioBatchDetail, bool, error) {",
		"return resolveStudioBatchDetailWithoutGraph(ctx, studioSessionRepo, batchID)",
		"EnsureGraph: ensureGraph,",
		"draftUpdatedAt, createdTasks, rejectedTasks, failedTasks, err := loadStudioBatchDraftState(ctx, studioSessionRepo, taskLinkRepo, getTask, batchID)",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_batch_detail_adapter.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("task_studio_batch_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"service.ensureServiceRunner()",
		"return s.serviceRunner.GetDetail(ctx, batchID)",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_service.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("task_studio_batch_runner_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_runner_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func (s *taskStudioBatchService) ensureDetailRunner() {",
		"func (s *taskStudioBatchService) ensureServiceRunner() {",
		"s.detailRunner = newListingStudioBatchDetailService(s.repo, s.studioSessionRepo, s.batchTaskLinkRepo, s.getTask, s.ensureStudioBatchGenerationGraphForResume)",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_studio_batch_runner_support.go should contain %q", needle)
		}
	}

	serviceAdapterSrc, err := os.ReadFile("task_studio_batch_service_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service_adapter.go) error = %v", err)
	}
	serviceAdapterContent := string(serviceAdapterSrc)

	for _, needle := range []string{
		"GetDetail: func(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"return s.detailRunner.GetDetail(ctx, batchID)",
	} {
		if !strings.Contains(serviceAdapterContent, needle) {
			t.Fatalf("task_studio_batch_service_adapter.go should contain %q", needle)
		}
	}
}

func TestTaskStudioBatchReviewAdapterUsesListingStudioRunner(t *testing.T) {
	t.Parallel()

	adapterSrc, err := os.ReadFile("task_studio_batch_review_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_review_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"func newListingStudioBatchReviewService(",
		"repo StudioBatchRepository,",
		"loadDetail func(context.Context, string) (*StudioBatchDetail, error),",
		"currentTime func() time.Time,",
		"return studiodomain.NewBatchDesignReviewService(studiodomain.BatchDesignReviewServiceConfig[StudioBatchDetail]{",
		"EnsureBatchExists: func(ctx context.Context, batchID string) error {",
		"ReplaceReviews: func(ctx context.Context, batchID string, designIDs []string, updatedAt time.Time) error {",
		"LoadDetail:  loadDetail,",
		"CurrentTime: currentTime,",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_batch_review_adapter.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("task_studio_batch_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"service.ensureServiceRunner()",
		"return s.serviceRunner.ApproveDesigns(ctx, batchID, req)",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_service.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("task_studio_batch_runner_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_runner_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func (s *taskStudioBatchService) ensureReviewRunner() {",
		"s.reviewRunner = newListingStudioBatchReviewService(s.repo, s.GetStudioBatchDetail, s.currentTime)",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_studio_batch_runner_support.go should contain %q", needle)
		}
	}

	serviceAdapterSrc, err := os.ReadFile("task_studio_batch_service_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service_adapter.go) error = %v", err)
	}
	serviceAdapterContent := string(serviceAdapterSrc)

	for _, needle := range []string{
		"ApproveDesigns: func(ctx context.Context, batchID string, designIDs []string) (*StudioBatchDetail, error) {",
		"return s.reviewRunner.ApproveDesigns(ctx, batchID, designIDs)",
	} {
		if !strings.Contains(serviceAdapterContent, needle) {
			t.Fatalf("task_studio_batch_service_adapter.go should contain %q", needle)
		}
	}
}

func TestTaskStudioBatchRetryAdapterUsesListingStudioRunner(t *testing.T) {
	t.Parallel()

	adapterSrc, err := os.ReadFile("task_studio_batch_retry_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_retry_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"func newListingStudioBatchRetryPrepareService(",
		"repo StudioBatchRepository,",
		"taskLinkRepo StudioBatchTaskLinkRepository,",
		"loadDetail func(context.Context, string) (*StudioBatchDetail, error),",
		"resetItems func(context.Context, []StudioBatchItemRecord) error,",
		"return studiodomain.NewBatchRetryPrepareService(studiodomain.BatchRetryPrepareServiceConfig[",
		"LoadDetail: func(ctx context.Context, batchID string) (*studioBatchRetryDetailGraph, error) {",
		"links, err := taskLinkRepo.ListStudioBatchTaskLinksByBatchID(ctx, batchID)",
		"SelectItems: selectStudioBatchRetryItems,",
		"ResetItems:  resetItems,",
		"LoadResult:  loadDetail,",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_batch_retry_adapter.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("task_studio_batch_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"service.ensureServiceRunner()",
		"return s.serviceRunner.PrepareRetryItems(ctx, batchID, req)",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_service.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("task_studio_batch_runner_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_runner_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func (s *taskStudioBatchService) ensureRetryRunner() {",
		"s.retryRunner = newListingStudioBatchRetryPrepareService(s.repo, s.batchTaskLinkRepo, s.GetStudioBatchDetail, s.resetStudioBatchRetryItems)",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_studio_batch_runner_support.go should contain %q", needle)
		}
	}

	serviceAdapterSrc, err := os.ReadFile("task_studio_batch_service_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service_adapter.go) error = %v", err)
	}
	serviceAdapterContent := string(serviceAdapterSrc)

	for _, needle := range []string{
		"PrepareRetryItems: func(ctx context.Context, batchID string, itemIDs []string) (*StudioBatchDetail, error) {",
		"return s.retryRunner.PrepareRetryItems(ctx, batchID, itemIDs)",
	} {
		if !strings.Contains(serviceAdapterContent, needle) {
			t.Fatalf("task_studio_batch_service_adapter.go should contain %q", needle)
		}
	}
}

func TestTaskStudioBatchTaskPrepareAdapterUsesListingStudioRunner(t *testing.T) {
	t.Parallel()

	adapterSrc, err := os.ReadFile("task_studio_batch_task_prepare_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_task_prepare_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"func newListingStudioBatchTaskPrepareService(",
		"updateSession func(context.Context, *SheinStudioSession) error,",
		"updateBatch func(context.Context, *StudioBatchRecord) error,",
		"loadResult func(context.Context, string) (*CreateStudioBatchTasksResult, error),",
		"return studiodomain.NewBatchTaskPrepareService(studiodomain.BatchTaskPrepareServiceConfig[",
		"SetPendingDesignIDs: func(session *SheinStudioSession, designIDs []string) {",
		"SetSessionCreating: func(session *SheinStudioSession) {",
		"SetBatchCreating: func(batch *StudioBatchRecord) {",
		"LoadResult:  loadResult,",
		"CurrentTime: currentTime,",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_batch_task_prepare_adapter.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("task_studio_batch_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"service.ensureServiceRunner()",
		"return s.serviceRunner.PrepareCreateTasks(ctx, batchID, req)",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_service.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("task_studio_batch_runner_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_runner_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func (s *taskStudioBatchService) ensureTaskPrepareRunner() {",
		"s.taskPrepareRunner = newListingStudioBatchTaskPrepareService(",
		"func (s *taskStudioBatchService) studioBatchSessionUpdater() func(context.Context, *SheinStudioSession) error {",
		"func (s *taskStudioBatchService) studioBatchUpdater() func(context.Context, *StudioBatchRecord) error {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_studio_batch_runner_support.go should contain %q", needle)
		}
	}

	flowSrc, err := os.ReadFile("task_studio_batch_task_flow_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_task_flow_support.go) error = %v", err)
	}
	flowContent := string(flowSrc)

	if !strings.Contains(flowContent, "func (s *taskStudioBatchService) loadStudioBatchTaskPreparationResult(ctx context.Context, batchID string) (*CreateStudioBatchTasksResult, error) {") {
		t.Fatalf("task_studio_batch_task_flow_support.go should contain task preparation result helper")
	}

	serviceAdapterSrc, err := os.ReadFile("task_studio_batch_service_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service_adapter.go) error = %v", err)
	}
	serviceAdapterContent := string(serviceAdapterSrc)

	for _, needle := range []string{
		"PrepareCreateTasks: func(ctx context.Context, batchID string, designIDs []string) (*CreateStudioBatchTasksResult, error) {",
		"return s.taskPrepareRunner.PrepareTaskCreation(ctx, batchID, listingStudioBatchTaskPrepareState{",
	} {
		if !strings.Contains(serviceAdapterContent, needle) {
			t.Fatalf("task_studio_batch_service_adapter.go should contain %q", needle)
		}
	}
}

func TestTaskStudioBatchTaskResumeAdapterUsesListingStudioRunner(t *testing.T) {
	t.Parallel()

	adapterSrc, err := os.ReadFile("task_studio_batch_task_resume_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_task_resume_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"func newListingStudioBatchTaskResumeService(",
		"updateSession func(context.Context, *SheinStudioSession) error,",
		"updateBatch func(context.Context, *StudioBatchRecord) error,",
		"loadResult func(context.Context, string) (*CreateStudioBatchTasksResult, error),",
		"return studiodomain.NewBatchTaskResumeFinalizeService(studiodomain.BatchTaskResumeFinalizeServiceConfig[",
		"ClearPendingTasks: func(session *SheinStudioSession) {",
		"SetCreatedTasks: func(session *SheinStudioSession, created []SheinStudioCreatedTask) {",
		"session.CreatedTaskIDs = buildCreatedTaskIDs(created)",
		"SetSessionDone: func(session *SheinStudioSession) {",
		"SetBatchDone: func(batch *StudioBatchRecord) {",
		"LoadResult:  loadResult,",
		"CurrentTime: currentTime,",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_batch_task_resume_adapter.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("task_studio_batch_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"service.ensureTaskResumeRunner()",
		"taskResumeRunner   *listingStudioBatchTaskResumeRunner",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_service.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("task_studio_batch_runner_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_runner_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func (s *taskStudioBatchService) ensureTaskResumeRunner() {",
		"s.taskResumeRunner = newListingStudioBatchTaskResumeService(",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_studio_batch_runner_support.go should contain %q", needle)
		}
	}

	flowSrc, err := os.ReadFile("task_studio_batch_task_flow_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_task_flow_support.go) error = %v", err)
	}
	flowContent := string(flowSrc)

	for _, needle := range []string{
		"s.ensureTaskResumeRunner()",
		`return s.taskResumeRunner.FinalizeTaskCreation(ctx, batchID, listingStudioBatchTaskResumeState{`,
	} {
		if !strings.Contains(flowContent, needle) {
			t.Fatalf("task_studio_batch_task_flow_support.go should contain %q", needle)
		}
	}
}

func TestTaskSubmissionServiceConfigsUseSharedSupportWiring(t *testing.T) {
	t.Parallel()

	if _, err := os.ReadFile("service_submit_wiring.go"); err == nil {
		t.Fatal("service_submit_wiring.go should be removed after submit collaborator wiring consolidation")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_submit_wiring.go) unexpected error = %v", err)
	}

	supportSrc, err := os.ReadFile("service_submit_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_wiring_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"type taskSubmissionBaseWiring struct {",
		"type taskSubmissionSupportWiring struct {",
		"func buildTaskSubmissionBaseWiring(s *service) taskSubmissionBaseWiring {",
		"func buildTaskSubmissionBaseWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskSubmissionBaseWiring {",
		"func buildTaskSubmissionSupportWiring(s *service) taskSubmissionSupportWiring {",
		"func buildTaskSubmissionSupportWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskSubmissionSupportWiring {",
		"resolveSheinStoreID      func(context.Context, *Task) (int64, error)",
		"resolveSubmitSettings    func(context.Context, *Task) SheinSettings",
		"support:  buildTaskSubmissionSupportWiringWithAssembly(s, assembly),",
		"currentSheinPricingRule:  s.currentSheinPricingRule,",
		"rememberSheinSubmitted:   s.rememberSheinSubmittedResolution,",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("service_submit_wiring_support.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildTaskSubmissionExecutionServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionExecutionServiceConfig {",
		"func buildTaskSubmissionStateServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionStateServiceConfig {",
		"func resolveSubmissionWorkflowClient(s *service) (SheinPublishWorkflowClient, bool) {",
		"assembly = buildTaskSubmissionAssembly(s)",
		"wiring := buildTaskSubmissionSupportWiring(s)",
	} {
		if strings.Contains(supportContent, needle) {
			t.Fatalf("service_submit_wiring_support.go should not contain %q after resolution/config split", needle)
		}
	}

	resolutionSrc, err := os.ReadFile("service_submit_wiring_resolution_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_wiring_resolution_support.go) error = %v", err)
	}
	resolutionContent := string(resolutionSrc)

	for _, needle := range []string{
		"func resolveSubmissionStoreProfileRepo(s *service) StoreProfileRepository {",
		"func resolveSubmissionWorkflowClient(s *service) (SheinPublishWorkflowClient, bool) {",
		"func buildTaskSubmissionExecutionServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionExecutionServiceConfig {",
		"sheinProductAPIBuilder:   wiring.sheinProductAPIBuilder,",
		"resolveSubmitSettings:    wiring.resolveSubmitSettings,",
		"func buildTaskSubmissionStateServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionStateServiceConfig {",
		"rememberSheinSubmitted: wiring.rememberSheinSubmitted,",
	} {
		if !strings.Contains(resolutionContent, needle) {
			t.Fatalf("service_submit_wiring_resolution_support.go should contain %q", needle)
		}
	}

	managedSrc, err := os.ReadFile("service_submit_managed_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_managed_wiring_support.go) error = %v", err)
	}
	managedContent := string(managedSrc)

	for _, needle := range []string{
		"func buildTaskSubmissionRecoveryServiceConfigWithAssembly(s *service, assembly taskSubmissionAssembly) taskSubmissionRecoveryServiceConfig {",
		"func buildTaskSubmissionServiceConfigWithSupportAndCollaborators(",
		"repo:                            support.repo,",
		"type taskManagedSubmissionConfigWiring struct {",
		"func buildTaskManagedSubmissionConfigWiringWithRecovery(s *service, recovery *taskSubmissionRecoveryService) taskManagedSubmissionConfigWiring {",
	} {
		if !strings.Contains(managedContent, needle) {
			t.Fatalf("service_submit_managed_wiring_support.go should contain %q", needle)
		}
	}
}

func TestTaskTemporalSubmissionFacadeUsesExplicitConfigBuilder(t *testing.T) {
	t.Parallel()

	collaboratorSrc, err := os.ReadFile("service_submit_collaborators.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_collaborators.go) error = %v", err)
	}
	collaboratorContent := string(collaboratorSrc)

	for _, needle := range []string{
		"func (s *service) resolveTaskTemporalSubmissionCollaborators() taskTemporalSubmissionCollaborators {",
		"s.submission.temporalGroup = wiring.resolve(s.submission.temporalGroup)",
		"return s.submission.temporalGroup",
	} {
		if !strings.Contains(collaboratorContent, needle) {
			t.Fatalf("service_submit_collaborators.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("task_temporal_submission_service.go"); err == nil {
		t.Fatal("task_temporal_submission_service.go should be removed after temporal owner delegation split")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(task_temporal_submission_service.go) unexpected error = %v", err)
	}

	wiringSrc, err := os.ReadFile("service_submit_temporal_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_temporal_wiring_support.go) error = %v", err)
	}
	wiringContent := string(wiringSrc)

	for _, needle := range []string{
		"func buildTaskTemporalSubmissionConfigWiring(s *service) taskTemporalSubmissionConfigWiring {",
		"func buildTaskTemporalSubmissionConfigWiringWithPersistence(",
		"config := buildTaskTemporalSubmissionConfigWiring(s)",
		"func buildTaskTemporalSubmissionFlowServiceConfigWithWiring(",
		"func buildTaskTemporalSubmissionRefreshServiceConfigWithWiring(",
	} {
		if !strings.Contains(wiringContent, needle) {
			t.Fatalf("service_submit_temporal_wiring_support.go should contain %q", needle)
		}
	}
}

func TestTaskTemporalSubmissionCollaboratorsShareOneEnsureSeam(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service_submit_collaborators.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_collaborators.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func (s *service) resolveTaskTemporalSubmissionCollaborators() taskTemporalSubmissionCollaborators {",
		"wiring := buildTaskTemporalSubmissionCollaboratorWiring(s)",
		"s.submission.temporalGroup = wiring.resolve(s.submission.temporalGroup)",
		"return s.submission.temporalGroup",
		"s.resolveTaskTemporalSubmissionCollaborators()",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service_submit_collaborators.go should contain %q", needle)
		}
	}

	stageSrc, err := os.ReadFile("service_submit_collaborator_stages.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_collaborator_stages.go) error = %v", err)
	}
	stageContent := string(stageSrc)

	for _, needle := range []string{
		"s.taskTemporalSubmissionPersistenceOrDefault()",
		"s.taskTemporalSubmissionLifecycleOrDefault()",
		"s.taskTemporalSubmissionFlowOrDefault()",
		"s.taskTemporalSubmissionRefreshOrDefault()",
	} {
		if !strings.Contains(stageContent, needle) {
			t.Fatalf("service_submit_collaborator_stages.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("service_submit_temporal_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_temporal_wiring_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"type taskTemporalSubmissionCollaboratorWiring struct {",
		"type taskTemporalSubmissionCollaborators struct {",
		"type taskTemporalSubmissionConfigWiring struct {",
		"func buildTaskTemporalSubmissionConfigWiring(s *service) taskTemporalSubmissionConfigWiring {",
		"func buildTaskTemporalSubmissionCollaboratorWiring(s *service) taskTemporalSubmissionCollaboratorWiring {",
		"func buildTaskTemporalSubmissionConfigWiringWithPersistence(",
		"func (w taskTemporalSubmissionCollaboratorWiring) newFlow(persistence *taskTemporalSubmissionPersistenceService) *taskTemporalSubmissionFlowService {",
		"func (w taskTemporalSubmissionCollaboratorWiring) newRefresh(persistence *taskTemporalSubmissionPersistenceService) *taskTemporalSubmissionRefreshService {",
		"func (w taskTemporalSubmissionCollaboratorWiring) resolve(existing taskTemporalSubmissionCollaborators) taskTemporalSubmissionCollaborators {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("service_submit_temporal_wiring_support.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"type taskTemporalSubmissionFacadeWiring struct {",
		"func buildTaskTemporalSubmissionFacadeWiring(s *service) taskTemporalSubmissionFacadeWiring {",
		"func (w taskTemporalSubmissionCollaboratorWiring) newFacade(",
		"collaborators.facade",
		"assembly = buildTaskSubmissionAssembly(s)",
	} {
		if strings.Contains(supportContent, needle) || strings.Contains(content, needle) {
			t.Fatalf("temporal collaborator wiring should not retain facade seam %q", needle)
		}
	}
}

func TestTaskManagedSubmissionCollaboratorsShareOneEnsureSeam(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service_submit_collaborators.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_collaborators.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func (s *service) resolveTaskManagedSubmissionCollaborators() taskManagedSubmissionCollaborators {",
		"wiring := buildTaskManagedSubmissionCollaboratorWiring(s)",
		"s.submission.managedGroup = wiring.resolve(s.submission.managedGroup)",
		"return s.submission.managedGroup",
		"s.resolveTaskManagedSubmissionCollaborators()",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service_submit_collaborators.go should contain %q", needle)
		}
	}

	stageSrc, err := os.ReadFile("service_submit_collaborator_stages.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_collaborator_stages.go) error = %v", err)
	}
	stageContent := string(stageSrc)

	for _, needle := range []string{
		"s.taskSubmissionRecoveryOrDefault()",
		"s.taskDirectSubmissionOrDefault()",
		"s.taskSubmissionRefreshOrDefault()",
		"s.taskSubmissionOrDefault()",
	} {
		if !strings.Contains(stageContent, needle) {
			t.Fatalf("service_submit_collaborator_stages.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("service_submit_managed_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_managed_wiring_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"type taskManagedSubmissionCollaboratorWiring struct {",
		"type taskManagedSubmissionCollaborators struct {",
		"func buildTaskManagedSubmissionCollaboratorWiring(s *service) taskManagedSubmissionCollaboratorWiring {",
		"func buildTaskManagedSubmissionWiringWithAssemblyAndRecovery(s *service, assembly taskSubmissionAssembly, recovery *taskSubmissionRecoveryService) taskManagedSubmissionWiring {",
		"func (w taskManagedSubmissionCollaboratorWiring) newRecovery() *taskSubmissionRecoveryService {",
		"func (w taskManagedSubmissionCollaboratorWiring) buildManaged(recovery *taskSubmissionRecoveryService) taskManagedSubmissionWiring {",
		"func (w taskManagedSubmissionCollaboratorWiring) newDirect(managed taskManagedSubmissionWiring) *taskDirectSubmissionService {",
		"func (w taskManagedSubmissionCollaboratorWiring) newRefresh(managed taskManagedSubmissionWiring) *taskSubmissionRefreshService {",
		"func (w taskManagedSubmissionCollaboratorWiring) newSubmission(recovery *taskSubmissionRecoveryService, direct *taskDirectSubmissionService) *taskSubmissionService {",
		"func (w taskManagedSubmissionCollaboratorWiring) resolve(existing taskManagedSubmissionCollaborators) taskManagedSubmissionCollaborators {",
		"managed := w.buildManaged(recovery)",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("service_submit_managed_wiring_support.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildTaskSubmissionServiceConfigWithSupportAndCollaborators(",
		"func buildTaskSubmissionRefreshServiceConfigWithWiring(wiring taskManagedSubmissionWiring) taskSubmissionRefreshServiceConfig {",
		"func buildTaskDirectSubmissionServiceConfigWithWiring(wiring taskManagedSubmissionWiring) taskDirectSubmissionServiceConfig {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("service_submit_managed_wiring_support.go should contain %q", needle)
		}
	}

	if strings.Contains(supportContent, "assembly = buildTaskSubmissionAssembly(s)") {
		t.Fatal("managed submission wiring should complete the provided assembly instead of rebuilding it")
	}
}

func TestTaskSubmissionCoreCollaboratorsShareOneEnsureSeam(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service_submit_collaborators.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_collaborators.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func (s *service) resolveTaskSubmissionCoreCollaborators() taskSubmissionCoreCollaborators {",
		"wiring := buildTaskSubmissionCoreCollaboratorWiring(s)",
		"s.submission.coreGroup = wiring.resolve(s.submission.coreGroup)",
		"return s.submission.coreGroup",
		"s.resolveTaskSubmissionCoreCollaborators()",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service_submit_collaborators.go should contain %q", needle)
		}
	}

	stageSrc, err := os.ReadFile("service_submit_collaborator_stages.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_collaborator_stages.go) error = %v", err)
	}
	stageContent := string(stageSrc)

	for _, needle := range []string{
		"s.taskSubmissionExecutionOrDefault()",
		"s.taskSubmissionStateOrDefault()",
	} {
		if !strings.Contains(stageContent, needle) {
			t.Fatalf("service_submit_collaborator_stages.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("service_submit_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_wiring_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"type taskSubmissionCoreCollaboratorWiring struct {",
		"type taskSubmissionCoreCollaborators struct {",
		"func buildTaskSubmissionCoreCollaboratorWiring(s *service) taskSubmissionCoreCollaboratorWiring {",
		"func (w taskSubmissionCoreCollaboratorWiring) newExecution() *taskSubmissionExecutionService {",
		"func (w taskSubmissionCoreCollaboratorWiring) newState() *taskSubmissionStateService {",
		"func (w taskSubmissionCoreCollaboratorWiring) resolve(existing taskSubmissionCoreCollaborators) taskSubmissionCoreCollaborators {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("service_submit_wiring_support.go should contain %q", needle)
		}
	}

	if strings.Contains(supportContent, "func buildTaskSubmissionCoreCollaboratorWiring(s *service) taskSubmissionCoreCollaboratorWiring {\n\tbase := buildTaskSubmissionBaseWiring(s)") {
		t.Fatal("core submission collaborator wiring must not build base wiring because base assembly bindings resolve core collaborators")
	}
}

func TestTaskSubmitTaskRecoveryCollaboratorsShareOneEnsureSeam(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service_submit_collaborators.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_collaborators.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func (s *service) resolveTaskSubmitTaskRecoveryCollaborators() taskSubmitTaskRecoveryCollaborators {",
		"wiring := buildTaskSubmitTaskRecoveryCollaboratorWiring(s)",
		"s.submission.recoveryGroup = wiring.resolve(s.submission.recoveryGroup)",
		"return s.submission.recoveryGroup",
		"s.resolveTaskSubmitTaskRecoveryCollaborators()",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service_submit_collaborators.go should contain %q", needle)
		}
	}

	stageSrc, err := os.ReadFile("service_submit_collaborator_stages.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_collaborator_stages.go) error = %v", err)
	}
	stageContent := string(stageSrc)

	for _, needle := range []string{
		"s.taskRecoveryOrDefault()",
		"s.taskRequeueOrDefault()",
	} {
		if !strings.Contains(stageContent, needle) {
			t.Fatalf("service_submit_collaborator_stages.go should contain %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("service_submit_wiring_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_wiring_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"type taskSubmitTaskRecoveryCollaboratorWiring struct {",
		"type taskSubmitTaskRecoveryCollaborators struct {",
		"func buildTaskSubmitTaskRecoveryCollaboratorWiring(s *service) taskSubmitTaskRecoveryCollaboratorWiring {",
		"func (w taskSubmitTaskRecoveryCollaboratorWiring) newTaskRecovery() *taskRecoveryService {",
		"func (w taskSubmitTaskRecoveryCollaboratorWiring) newTaskRequeue() *taskRequeueService {",
		"func (w taskSubmitTaskRecoveryCollaboratorWiring) resolve(existing taskSubmitTaskRecoveryCollaborators) taskSubmitTaskRecoveryCollaborators {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("service_submit_wiring_support.go should contain %q", needle)
		}
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
