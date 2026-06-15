package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestInterfacesFilesOwnDependencyAndServiceFamilies(t *testing.T) {
	t.Parallel()

	rootSrc, err := os.ReadFile("interfaces.go")
	if err != nil {
		t.Fatalf("ReadFile(interfaces.go) error = %v", err)
	}
	rootContent := string(rootSrc)

	for _, needle := range []string{
		"type Service interface {",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("interfaces.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"type Repository interface {",
		"type ProductService interface {",
		"type TaskLifecycleService interface {",
		"type StoreAdminService interface {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("interfaces.go should not contain %q after interface split", needle)
		}
	}

	depSrc, err := os.ReadFile("interfaces_dependencies.go")
	if err != nil {
		t.Fatalf("ReadFile(interfaces_dependencies.go) error = %v", err)
	}
	depContent := string(depSrc)

	for _, needle := range []string{
		"type TaskSubmitter interface{ Submit(taskID string) error }",
		"type ProductService interface {",
		"type Repository interface {",
		"type CanonicalProductCacheRepository interface {",
		"type WorkflowClientConfigurer interface {",
	} {
		if !strings.Contains(depContent, needle) {
			t.Fatalf("interfaces_dependencies.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("interfaces_services.go")
	if err != nil {
		t.Fatalf("ReadFile(interfaces_services.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"type TaskLifecycleService interface {",
		"type TaskRecoveryService interface {",
		"type GenerationTaskService interface {",
		"type StoreAdminService interface {",
		"type InternalListingKitService interface {",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("interfaces_services.go should contain %q", needle)
		}
	}
}
