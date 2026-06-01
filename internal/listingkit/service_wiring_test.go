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
	if impl.taskSubmission == nil {
		t.Fatal("expected taskSubmission to be initialized")
	}
	if impl.taskSubmissionRecovery == nil {
		t.Fatal("expected taskSubmissionRecovery to be initialized")
	}
	if impl.taskSubmissionExecution == nil {
		t.Fatal("expected taskSubmissionExecution to be initialized")
	}
	if impl.taskSubmissionState == nil {
		t.Fatal("expected taskSubmissionState to be initialized")
	}
	if impl.taskDirectSubmission == nil {
		t.Fatal("expected taskDirectSubmission to be initialized")
	}
	if impl.taskTemporalSubmissionAdapter == nil {
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
	if svc.taskSubmission == nil {
		t.Fatal("expected taskSubmission to be initialized")
	}
	if svc.taskSubmissionRecovery == nil {
		t.Fatal("expected taskSubmissionRecovery to be initialized")
	}
	if svc.taskSubmissionExecution == nil {
		t.Fatal("expected taskSubmissionExecution to be initialized")
	}
	if svc.taskSubmissionState == nil {
		t.Fatal("expected taskSubmissionState to be initialized")
	}
	if svc.taskDirectSubmission == nil {
		t.Fatal("expected taskDirectSubmission to be initialized")
	}

	svc.initializeTemporalCollaborators()
	if svc.taskTemporalSubmissionAdapter == nil {
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
		"func NewService(config *ServiceConfig) (Service, error) {",
		"func newServiceWithConfig(config *ServiceConfig) *service {",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("service.go should keep %q", needle)
		}
	}
}

func TestAdminCollaboratorFilesUseExplicitWiringBuilders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		file         string
		builderCall  string
		inlineConfig string
	}{
		{
			name:         "settings admin",
			file:         "settings_admin_service.go",
			builderCall:  "buildSettingsAdminServiceConfig(s)",
			inlineConfig: "newSettingsAdminService(settingsAdminServiceConfig{",
		},
		{
			name:         "shein admin",
			file:         "shein_admin_service.go",
			builderCall:  "buildSheinAdminServiceConfig(s)",
			inlineConfig: "newSheinAdminService(sheinAdminServiceConfig{",
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

			if !strings.Contains(content, tc.builderCall) {
				t.Fatalf("%s should contain %q", tc.file, tc.builderCall)
			}
			if strings.Contains(content, tc.inlineConfig) {
				t.Fatalf("%s should not contain %q", tc.file, tc.inlineConfig)
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
			name: "submit services",
			file: "service_submit.go",
			builderCalls: []string{
				"buildTaskSubmissionServiceConfig(s)",
				"buildTaskSubmissionExecutionServiceConfig(s)",
			},
			inlineConfig: []string{
				"newTaskSubmissionService(taskSubmissionServiceConfig{",
				"newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{",
			},
		},
		{
			name: "direct submit service",
			file: "service_submit_direct.go",
			builderCalls: []string{
				"buildTaskDirectSubmissionServiceConfig(s)",
			},
			inlineConfig: []string{
				"newTaskDirectSubmissionService(taskDirectSubmissionServiceConfig{",
			},
		},
		{
			name: "temporal submission adapter",
			file: "service_submit_temporal_adapter.go",
			builderCalls: []string{
				"buildTaskTemporalSubmissionAdapterConfig(s)",
			},
			inlineConfig: []string{
				"newTaskTemporalSubmissionAdapter(taskTemporalSubmissionAdapterConfig{",
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

func TestSubmitRuntimeContextFilesUseExplicitResolverSeam(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		file    string
		needles []string
	}{
		{
			name: "submit store context",
			file: "service_submit_store_context.go",
			needles: []string{
				"buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)",
			},
		},
		{
			name: "shein store client",
			file: "service_shein_store_client.go",
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
