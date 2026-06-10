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
		"newTaskTemporalSubmissionAdapter(",
		"buildSettingsAdminServiceConfig(",
		"buildSheinAdminServiceConfig(",
		"buildTaskSubmissionServiceConfig(",
		"buildTaskSubmissionRefreshServiceConfig(",
		"buildTaskSubmissionExecutionServiceConfig(",
		"buildTaskTemporalSubmissionAdapterConfig(",
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
			file: "service_admin_wiring.go",
			needles: []string{
				"func buildSettingsAdminServiceConfig(s *service) settingsAdminServiceConfig {",
				"func buildSheinAdminServiceConfig(s *service) sheinAdminServiceConfig {",
			},
		},
		{
			file: "service_submit_wiring.go",
			needles: []string{
				"func buildTaskSubmissionServiceConfig(s *service) taskSubmissionServiceConfig {",
				"func buildTaskSubmissionRefreshServiceConfig(s *service) taskSubmissionRefreshServiceConfig {",
				"func buildTaskSubmissionExecutionServiceConfig(s *service) taskSubmissionExecutionServiceConfig {",
				"func buildTaskTemporalSubmissionAdapterConfig(s *service) taskTemporalSubmissionAdapterConfig {",
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

	src, err := os.ReadFile("service_process.go")
	if err != nil {
		t.Fatalf("ReadFile(service_process.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"func taskNeedsReviewReason(result *ListingKitResult) string {",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service_process.go should contain %q", needle)
		}
	}
	if strings.Contains(content, "return buildListingKitProcessFlow(s).run(ctx, task, log)") {
		t.Fatalf("service_process.go should not contain %q after facade split", "return buildListingKitProcessFlow(s).run(ctx, task, log)")
	}

	for _, needle := range []string{
		"s.repo.MarkProcessing(",
		"s.runWorkflow(",
		"s.persistProcessFailure(",
		"s.persistProcessSuccess(",
		"deriveProcessTerminalStatus(",
		"applyProcessTerminalResult(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("service_process.go should not contain %q", needle)
		}
	}
}
