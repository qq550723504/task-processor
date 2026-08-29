package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlatformRegistrationDepguardPatternsRespectPackageBoundaries(t *testing.T) {
	configPath := filepath.Join("..", ".golangci.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}

	config := string(content)
	start := strings.Index(config, "      platform_registration_boundaries:\n")
	if start == -1 {
		t.Fatalf("%s must define platform_registration_boundaries", configPath)
	}
	end := strings.Index(config[start+1:], "\n      aicapability_boundaries:")
	if end == -1 {
		t.Fatalf("%s must keep platform_registration_boundaries before aicapability_boundaries", configPath)
	}
	config = config[start : start+1+end]

	for _, packagePath := range []string{
		"task-processor/internal/app/httpapi",
		"task-processor/internal/asset",
		"task-processor/internal/catalog",
		"task-processor/internal/listingkit",
		"task-processor/internal/marketplace",
		"task-processor/internal/productimage",
		"task-processor/internal/publishing",
		"task-processor/internal/workspace",
	} {
		for _, suffix := range []string{"$", "/"} {
			pattern := fmt.Sprintf(`- pkg: "%s%s"`, packagePath, suffix)
			if !strings.Contains(config, pattern) {
				t.Errorf("%s must contain depguard package-boundary pattern %s", configPath, pattern)
			}
		}
	}
}

func TestAICapabilityDepguardPatternsRespectPackageBoundaries(t *testing.T) {
	configPath := filepath.Join("..", ".golangci.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}

	config := string(content)
	start := strings.Index(config, "      aicapability_boundaries:\n")
	if start == -1 {
		t.Fatalf("%s must define aicapability_boundaries", configPath)
	}
	end := strings.Index(config[start+1:], "\n      source_handoff_legacy_http:")
	if end == -1 {
		t.Fatalf("%s must keep aicapability_boundaries before source_handoff_legacy_http", configPath)
	}
	config = config[start : start+1+end]

	for _, packagePath := range []string{
		"github.com/sashabaranov/go-openai",
		"task-processor/internal/app",
		"task-processor/internal/amazon",
		"task-processor/internal/amazonlisting",
		"task-processor/internal/asset",
		"task-processor/internal/catalog",
		"task-processor/internal/core/config",
		"task-processor/internal/httpbootstrap",
		"task-processor/internal/httproute",
		"task-processor/internal/infra/clients",
		"task-processor/internal/infra/httpx",
		"task-processor/internal/integration/openai",
		"task-processor/internal/listing",
		"task-processor/internal/listingkit",
		"task-processor/internal/marketplace",
		"task-processor/internal/product",
		"task-processor/internal/productenrich",
		"task-processor/internal/productimage",
		"task-processor/internal/prompt",
		"task-processor/internal/promptmgmt",
		"task-processor/internal/pricing",
		"task-processor/internal/publishing",
		"task-processor/internal/shein",
		"task-processor/internal/sds",
		"task-processor/internal/temu",
		"task-processor/internal/workspace",
	} {
		for _, suffix := range []string{"$", "/"} {
			pattern := fmt.Sprintf(`- pkg: "%s%s"`, packagePath, suffix)
			if !strings.Contains(config, pattern) {
				t.Errorf("%s must contain depguard package-boundary pattern %s", configPath, pattern)
			}
		}
	}
}

func TestAlibaba1688CrawlerDepguardPatternKeepsHTTPAPIAdapterException(t *testing.T) {
	configPath := filepath.Join("..", ".golangci.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}

	config := string(content)
	start := strings.Index(config, "      alibaba1688_listingkit_root:\n")
	if start == -1 {
		t.Fatalf("%s must define alibaba1688_listingkit_root", configPath)
	}
	end := strings.Index(config[start+1:], "\n  govet:")
	if end == -1 {
		t.Fatalf("%s must keep alibaba1688_listingkit_root before govet", configPath)
	}
	config = config[start : start+1+end]

	if !strings.Contains(config, `- pkg: "task-processor/internal/listingkit$"`) {
		t.Errorf("%s must deny the exact ListingKit root facade", configPath)
	}
	if strings.Contains(config, `- pkg: "task-processor/internal/listingkit/"`) {
		t.Errorf("%s must keep ListingKit subpackage adapters outside the root-facade guard", configPath)
	}
}
