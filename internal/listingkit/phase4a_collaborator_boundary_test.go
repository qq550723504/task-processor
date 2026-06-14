package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestServiceRootFileKeepsCollaboratorWiringOutOfServiceRoot(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("ReadFile(service.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"newSettingsAdminService(",
		"newSheinAdminService(",
		"newTaskSubmissionService(",
		"newTaskSubmissionRefreshService(",
		"newTaskSubmissionExecutionService(",
		"newTaskTemporalSubmissionLifecycleService(",
		"newTaskTemporalSubmissionFlowService(",
		"newTaskTemporalSubmissionPersistenceService(",
		"newTaskTemporalSubmissionRefreshService(",
		"buildSettingsAdminServiceConfig(",
		"buildSheinAdminServiceConfig(",
		"buildTaskSubmissionServiceConfig(",
		"buildTaskSubmissionRefreshServiceConfig(",
		"buildTaskSubmissionExecutionServiceConfig(",
		"buildTaskTemporalSubmissionLifecycleServiceConfig(",
		"buildTaskTemporalSubmissionFlowServiceConfig(",
		"buildTaskTemporalSubmissionPersistenceServiceConfig(",
		"buildTaskTemporalSubmissionRefreshServiceConfig(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("service.go should not contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *service) SetTaskSubmitter(submitter TaskSubmitter) {",
		"func (s *service) ConfigureSheinPublishWorkflowClient(client SheinPublishWorkflowClient, enabled bool) {",
		"func (s *service) currentSheinSubmitSettings() SheinSettings {",
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

func TestCollaboratorWiringFilesOwnExplicitBuilders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		file    string
		needles []string
	}{
		{
			file: "service_admin_wiring_support.go",
			needles: []string{
				"func buildSettingsAdminServiceConfigWithWiring(wiring settingsAdminWiring) settingsAdminServiceConfig {",
				"func buildSheinAdminServiceConfigWithWiring(wiring sheinAdminWiring) sheinAdminServiceConfig {",
			},
		},
		{
			file: "service_submit_wiring_support.go",
			needles: []string{
				"func buildTaskSubmissionServiceConfigWithSupportAndCollaborators(",
				"func buildTaskSubmissionRefreshServiceConfigWithWiring(wiring taskManagedSubmissionWiring) taskSubmissionRefreshServiceConfig {",
				"func buildTaskSubmissionExecutionServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionExecutionServiceConfig {",
				"func buildTaskTemporalSubmissionLifecycleServiceConfigWithWiring(wiring taskTemporalSubmissionWiring) taskTemporalSubmissionLifecycleServiceConfig {",
				"func buildTaskTemporalSubmissionFlowServiceConfigWithWiring(",
				"func buildTaskTemporalSubmissionPersistenceServiceConfigWithWiring(wiring taskTemporalSubmissionWiring) taskTemporalSubmissionPersistenceServiceConfig {",
				"func buildTaskTemporalSubmissionRefreshServiceConfigWithWiring(",
			},
		},
		{
			file: "service_submit_wiring_support.go",
			needles: []string{
				"func buildTaskManagedSubmissionWiring(s *service) taskManagedSubmissionWiring {",
				"func buildTaskManagedSubmissionWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskManagedSubmissionWiring {",
				"func buildTaskManagedSubmissionWiringWithAssemblyAndRecovery(s *service, assembly taskSubmissionAssembly, recovery *taskSubmissionRecoveryService) taskManagedSubmissionWiring {",
				"func buildTaskManagedSubmissionCollaboratorWiring(s *service) taskManagedSubmissionCollaboratorWiring {",
				"func buildTaskSubmissionCoreCollaboratorWiring(s *service) taskSubmissionCoreCollaboratorWiring {",
				"func buildTaskSubmitTaskRecoveryCollaboratorWiring(s *service) taskSubmitTaskRecoveryCollaboratorWiring {",
				"func buildTaskSubmissionBaseWiring(s *service) taskSubmissionBaseWiring {",
				"func buildTaskSubmissionBaseWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskSubmissionBaseWiring {",
				"func buildTaskSubmissionSupportWiring(s *service) taskSubmissionSupportWiring {",
				"func buildTaskSubmissionSupportWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskSubmissionSupportWiring {",
				"func buildTaskTemporalSubmissionWiring(s *service) taskTemporalSubmissionWiring {",
				"func buildTaskTemporalSubmissionCollaboratorWiring(s *service) taskTemporalSubmissionCollaboratorWiring {",
				"func buildTaskTemporalSubmissionWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskTemporalSubmissionWiring {",
			},
		},
		{
			file: "service_studio_wiring_support.go",
			needles: []string{
				"func buildTaskStudioSessionWiring(s *service) taskStudioSessionWiring {",
				"func (w taskStudioSessionWiring) newSessionRunner() *listingStudioSessionRunner {",
				"func (w taskStudioSessionWiring) newBatchDraftRunner() *listingStudioBatchDraftRunner {",
				"func buildTaskStudioBatchServiceWiring(s *service) taskStudioBatchServiceWiring {",
				"func (w taskStudioBatchServiceWiring) newDetailRunner() *listingStudioBatchDetailRunner {",
				"func (w taskStudioBatchServiceWiring) newReviewRunner() *listingStudioBatchReviewRunner {",
				"func buildTaskStudioBatchRunWiring(s *service) taskStudioBatchRunWiring {",
				"func (w taskStudioBatchRunWiring) newServiceRunner(startRun func(context.Context, string) error) *studiodomain.BatchRunService {",
				"func (w taskStudioBatchRunWiring) newCompletionRunner(now func() time.Time) *listingStudioBatchRunCompletionRunner {",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
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

func TestServiceProcessFileUsesExplicitFlowSeam(t *testing.T) {
	t.Parallel()

	facadeSrc, err := os.ReadFile("service_process_entry.go")
	if err != nil {
		t.Fatalf("ReadFile(service_process_entry.go) error = %v", err)
	}
	facadeContent := string(facadeSrc)

	for _, needle := range []string{
		"return buildListingKitProcessFlow(s).run(ctx, task, log)",
	} {
		if !strings.Contains(facadeContent, needle) {
			t.Fatalf("service_process_entry.go should contain %q", needle)
		}
	}

	if _, err := os.ReadFile("service_process_review_helper.go"); err == nil {
		t.Fatal("service_process_review_helper.go should be removed after process persistence merge")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_process_review_helper.go) unexpected error = %v", err)
	}

	if _, err := os.ReadFile("service_process_review.go"); err == nil {
		t.Fatal("service_process_review.go should be removed after process review helper rename")
	} else if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(service_process_review.go) unexpected error = %v", err)
	}
}
