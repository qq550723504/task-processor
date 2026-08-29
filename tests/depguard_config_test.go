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

func TestAICapabilityDepguardUsesStrictAllowlist(t *testing.T) {
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

	if !strings.Contains(config, "list-mode: strict") {
		t.Errorf("%s must make aicapability_boundaries a strict allowlist", configPath)
	}

	for _, packagePath := range []string{
		"$gostd",
		"gorm.io/gorm",
		"task-processor/internal/aicapability",
	} {
		pattern := fmt.Sprintf(`- "%s"`, packagePath)
		if !strings.Contains(config, pattern) {
			t.Errorf("%s must allow %s for AI capability contracts", configPath, packagePath)
		}
	}
}

func TestImageAgentEffectPolicyDepguardUsesStrictAllowlist(t *testing.T) {
	configPath := filepath.Join("..", ".golangci.yml")
	config := readRepositoryText(t, configPath)
	start := strings.Index(config, "      imageagent_effectpolicy_boundaries:\n")
	if start == -1 {
		t.Fatalf("%s must define imageagent_effectpolicy_boundaries", configPath)
	}
	end := strings.Index(config[start+1:], "\n      source_handoff_legacy_http:")
	if end == -1 {
		t.Fatalf("%s must keep imageagent_effectpolicy_boundaries before source_handoff_legacy_http", configPath)
	}
	config = config[start : start+1+end]

	for _, required := range []string{
		"list-mode: strict",
		`- "**/internal/imageagent/effectpolicy/*.go"`,
		`- "**/internal/imageagent/effectpolicy/**/*.go"`,
		`- "$gostd"`,
		`- "task-processor/internal/imageagent"`,
	} {
		if !strings.Contains(config, required) {
			t.Errorf("%s imageagent effect policy boundary must contain %s", configPath, required)
		}
	}

	for _, forbidden := range []string{"gorm.io/", "go.temporal.io/", "internal/imageagent/store"} {
		if strings.Contains(config, forbidden) {
			t.Errorf("%s imageagent effect policy boundary must not allow %s", configPath, forbidden)
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

func TestZitadelAuthRuntimeDepguardPatternCoversListingKitPackageTree(t *testing.T) {
	configPath := filepath.Join("..", ".golangci.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}

	config := string(content)
	start := strings.Index(config, "      authruntime_zitadel_listingkit:\n")
	if start == -1 {
		t.Fatalf("%s must define authruntime_zitadel_listingkit", configPath)
	}
	end := strings.Index(config[start+1:], "\n      alibaba1688_listingkit_root:")
	if end == -1 {
		t.Fatalf("%s must keep authruntime_zitadel_listingkit before alibaba1688_listingkit_root", configPath)
	}
	config = config[start : start+1+end]

	for _, suffix := range []string{"$", "/"} {
		pattern := fmt.Sprintf(`- pkg: "task-processor/internal/listingkit%s"`, suffix)
		if !strings.Contains(config, pattern) {
			t.Errorf("%s must contain depguard package-tree pattern %s", configPath, pattern)
		}
	}
}

func TestCmdProductionDepguardPatternCoversDomainAndInfraTrees(t *testing.T) {
	configPath := filepath.Join("..", ".golangci.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}

	config := string(content)
	start := strings.Index(config, "      cmd_domain_dependencies:\n")
	if start == -1 {
		t.Fatalf("%s must define cmd_domain_dependencies", configPath)
	}
	end := strings.Index(config[start+1:], "\n      cmd_legacy_app_compatibility:")
	if end == -1 {
		t.Fatalf("%s must keep cmd_domain_dependencies before cmd_legacy_app_compatibility", configPath)
	}
	config = config[start : start+1+end]

	for _, packagePath := range []string{
		"task-processor/internal/amazon",
		"task-processor/internal/amazonlisting",
		"task-processor/internal/asset",
		"task-processor/internal/catalog",
		"task-processor/internal/infra",
		"task-processor/internal/listingkit",
		"task-processor/internal/marketplace",
		"task-processor/internal/productenrich",
		"task-processor/internal/productimage",
		"task-processor/internal/publishing",
		"task-processor/internal/shein",
		"task-processor/internal/temu",
		"task-processor/internal/workspace",
	} {
		for _, suffix := range []string{"$", "/"} {
			pattern := fmt.Sprintf(`- pkg: "%s%s"`, packagePath, suffix)
			if !strings.Contains(config, pattern) {
				t.Errorf("%s must contain depguard package-tree pattern %s", configPath, pattern)
			}
		}
	}
}

func TestTemporalRuntimeDepguardPatternCoversHTTPAPIPackageTrees(t *testing.T) {
	configPath := filepath.Join("..", ".golangci.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}

	config := string(content)
	start := strings.Index(config, "      temporal_runtime_httpapi:\n")
	if start == -1 {
		t.Fatalf("%s must define temporal_runtime_httpapi", configPath)
	}
	end := strings.Index(config[start+1:], "\n      authruntime_zitadel_listingkit:")
	if end == -1 {
		t.Fatalf("%s must keep temporal_runtime_httpapi before authruntime_zitadel_listingkit", configPath)
	}
	config = config[start : start+1+end]

	for _, packagePath := range []string{
		"task-processor/internal/app/httpapi",
		"task-processor/internal/listingkit/httpapi",
	} {
		for _, suffix := range []string{"$", "/"} {
			pattern := fmt.Sprintf(`- pkg: "%s%s"`, packagePath, suffix)
			if !strings.Contains(config, pattern) {
				t.Errorf("%s must contain depguard package-tree pattern %s", configPath, pattern)
			}
		}
	}
}

func TestDomainAppHTTPAPIDepguardPatternCoversBusinessTrees(t *testing.T) {
	configPath := filepath.Join("..", ".golangci.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}

	config := string(content)
	start := strings.Index(config, "      domain_app_httpapi_boundaries:\n")
	if start == -1 {
		t.Fatalf("%s must define domain_app_httpapi_boundaries", configPath)
	}
	end := strings.Index(config[start+1:], "\n      platform_registration_boundaries:")
	if end == -1 {
		t.Fatalf("%s must keep domain_app_httpapi_boundaries before platform_registration_boundaries", configPath)
	}
	config = config[start : start+1+end]

	for _, packagePath := range []string{
		"amazon",
		"amazonlisting",
		"asset",
		"catalog",
		"listing",
		"listingkit",
		"marketplace",
		"pricing",
		"productenrich",
		"productimage",
		"publishing",
		"sds",
		"shein",
		"temu",
	} {
		for _, pattern := range []string{
			fmt.Sprintf(`- "**/internal/%s/*.go"`, packagePath),
			fmt.Sprintf(`- "**/internal/%s/**/*.go"`, packagePath),
		} {
			if !strings.Contains(config, pattern) {
				t.Errorf("%s must cover domain package files with %s", configPath, pattern)
			}
		}
	}

	for _, suffix := range []string{"$", "/"} {
		pattern := fmt.Sprintf(`- pkg: "task-processor/internal/app/httpapi%s"`, suffix)
		if !strings.Contains(config, pattern) {
			t.Errorf("%s must deny app/httpapi package-tree pattern %s", configPath, pattern)
		}
	}
}
