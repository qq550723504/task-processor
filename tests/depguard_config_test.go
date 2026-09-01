package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPhase2TargetDirectionDepguardRulesCoverApprovedBoundaries(t *testing.T) {
	configPath := filepath.Join("..", ".golangci.yml")
	rules := loadDepguardRules(t, configPath)
	targetRule := requireDepguardRule(t, rules, "target_domain_concrete_infrastructure")
	platformRule := requireDepguardRule(t, rules, "platform_domain_dependencies")
	targetFiles := stringSet(targetRule.Files)
	platformFiles := stringSet(platformRule.Files)
	targetDeny := depguardDenyPackageSet(targetRule)
	platformDeny := depguardDenyPackageSet(platformRule)

	domains := []string{
		"listing", "product", "marketplace", "agent", "knowledge",
		"resourcecatalog", "commercial", "ledger", "organization",
	}
	for _, domain := range domains {
		for _, glob := range []string{
			fmt.Sprintf("**/internal/%s/*.go", domain),
			fmt.Sprintf("**/internal/%s/**/*.go", domain),
		} {
			if _, ok := targetFiles[glob]; !ok {
				t.Errorf("target_domain_concrete_infrastructure must cover %s", glob)
			}
		}

		packagePath := "task-processor/internal/" + domain
		for _, suffix := range []string{"$", "/"} {
			pattern := packagePath + suffix
			if _, ok := platformDeny[pattern]; !ok {
				t.Errorf("platform_domain_dependencies must deny %s", pattern)
			}
		}
	}

	for _, glob := range []string{
		"**/internal/platform/*.go",
		"**/internal/platform/**/*.go",
	} {
		if _, ok := platformFiles[glob]; !ok {
			t.Errorf("platform_domain_dependencies must cover %s", glob)
		}
	}

	for _, packagePath := range []string{
		"task-processor/internal/platform",
		"task-processor/internal/integration",
		"task-processor/internal/infra",
		"task-processor/internal/app",
		"gorm.io",
		"go.temporal.io",
		"go.opentelemetry.io",
		"github.com/open-feature",
		"github.com/aws",
		"github.com/redis",
		"github.com/rabbitmq",
	} {
		for _, suffix := range []string{"$", "/"} {
			pattern := packagePath + suffix
			if _, ok := targetDeny[pattern]; !ok {
				t.Errorf("target_domain_concrete_infrastructure must deny %s", pattern)
			}
		}
	}
}

func TestPhase3ProductDomainBoundaryDepguardRuleCoversOnlyTargetSubpackages(t *testing.T) {
	rules := loadDepguardRules(t, filepath.Join("..", ".golangci.yml"))
	rule := requireDepguardRule(t, rules, "phase3_product_domain_boundaries")
	files := stringSet(rule.Files)
	denied := depguardDenyPackageSet(rule)

	for _, glob := range []string{
		"**/internal/product/catalog/**/*.go",
		"**/internal/product/sourcing/**/*.go",
		"**/internal/product/enrichment/**/*.go",
		"**/internal/product/asset/**/*.go",
		"**/internal/product/image/**/*.go",
	} {
		if _, ok := files[glob]; !ok {
			t.Errorf("phase3_product_domain_boundaries must cover %s", glob)
		}
	}

	for _, packagePath := range []string{
		"task-processor/internal/app",
		"task-processor/internal/platform",
		"task-processor/internal/integration",
		"gorm.io/gorm",
		"go.temporal.io",
		"github.com/aws/aws-sdk-go-v2",
		"github.com/sashabaranov/go-openai",
	} {
		if _, ok := denied[packagePath]; !ok {
			t.Errorf("phase3_product_domain_boundaries must deny %s", packagePath)
		}
	}
}

func TestDepguardRuleParsingUsesYAMLSemantics(t *testing.T) {
	rules := parseDepguardRules(t, []byte(`
linters-settings:
  depguard:
    rules:
      semantic_rule:
        files:
          # - "**/internal/commented/*.go"
          - '**/internal/single-quoted/*.go'
        deny:
          # - pkg: "task-processor/internal/commented$"
          - pkg: task-processor/internal/unquoted$
`))
	rule := requireDepguardRule(t, rules, "semantic_rule")
	files := stringSet(rule.Files)
	denied := depguardDenyPackageSet(rule)

	if _, ok := files["**/internal/commented/*.go"]; ok {
		t.Fatal("commented file glob must not be a semantic depguard entry")
	}
	if _, ok := denied["task-processor/internal/commented$"]; ok {
		t.Fatal("commented deny package must not be a semantic depguard entry")
	}
	if _, ok := files["**/internal/single-quoted/*.go"]; !ok {
		t.Fatal("single-quoted file glob must retain its YAML value")
	}
	if _, ok := denied["task-processor/internal/unquoted$"]; !ok {
		t.Fatal("unquoted deny package must retain its YAML value")
	}
}

func TestPhase2RetiredRuntimePathsHavePermanentDepguardRules(t *testing.T) {
	rules := loadDepguardRules(t, filepath.Join("..", ".golangci.yml"))
	rule := requireDepguardRule(t, rules, "phase2_retired_runtime_paths")
	files := stringSet(rule.Files)
	denied := depguardDenyPackageSet(rule)

	for _, glob := range []string{"**/internal/*.go", "**/internal/**/*.go"} {
		if _, ok := files[glob]; !ok {
			t.Errorf("phase2_retired_runtime_paths must cover production files with %s", glob)
		}
	}
	for _, path := range []string{
		"task-processor/internal/core/lifecycle",
		"task-processor/internal/infra/database",
		"task-processor/internal/infra/redisclient",
		"task-processor/internal/infra/lock",
		"task-processor/internal/infra/rabbitmq",
		"task-processor/internal/infra/worker",
		"task-processor/internal/infra/clients/openai",
		"task-processor/internal/infra/clients/geminiimage",
		"task-processor/internal/infra/clients/grsai",
		"task-processor/internal/infra/storage",
		"task-processor/internal/infra/resilience",
		"task-processor/internal/infra/metrics",
		"task-processor/internal/infra/monitoring",
		"task-processor/internal/pkg/safeimagehttp",
		"task-processor/internal/pkg/hashx",
		"task-processor/internal/pkg/mathx",
		"task-processor/internal/pkg/ptr",
		"task-processor/internal/pkg/strx",
		"task-processor/internal/pkg/timex",
	} {
		for _, suffix := range []string{"$", "/"} {
			if _, ok := denied[path+suffix]; !ok {
				t.Errorf("phase2_retired_runtime_paths must deny %s", path+suffix)
			}
		}
	}
}

type depguardConfig struct {
	LintersSettings struct {
		Depguard struct {
			Rules map[string]depguardRule `yaml:"rules"`
		} `yaml:"depguard"`
	} `yaml:"linters-settings"`
}

type depguardRule struct {
	Files []string       `yaml:"files"`
	Deny  []depguardDeny `yaml:"deny"`
}

type depguardDeny struct {
	Package string `yaml:"pkg"`
}

func loadDepguardRules(t *testing.T, configPath string) map[string]depguardRule {
	t.Helper()
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}
	return parseDepguardRules(t, content)
}

func parseDepguardRules(t *testing.T, content []byte) map[string]depguardRule {
	t.Helper()
	var config depguardConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		t.Fatalf("parse depguard YAML: %v", err)
	}
	return config.LintersSettings.Depguard.Rules
}

func requireDepguardRule(t *testing.T, rules map[string]depguardRule, ruleName string) depguardRule {
	t.Helper()
	rule, ok := rules[ruleName]
	if !ok {
		t.Fatalf(".golangci.yml must define %s", ruleName)
	}
	return rule
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func depguardDenyPackageSet(rule depguardRule) map[string]struct{} {
	result := make(map[string]struct{}, len(rule.Deny))
	for _, deny := range rule.Deny {
		result[deny.Package] = struct{}{}
	}
	return result
}

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
		"task-processor/internal/product/catalog",
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
		"task-processor/internal/product/catalog",
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
		"product/catalog",
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

func TestInfrastructureBusinessDepguardPatternCoversInfrastructureTrees(t *testing.T) {
	configPath := filepath.Join("..", ".golangci.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}

	config := string(content)
	start := strings.Index(config, "      infrastructure_business_boundaries:\n")
	if start == -1 {
		t.Fatalf("%s must define infrastructure_business_boundaries", configPath)
	}
	end := strings.Index(config[start+1:], "\n      domain_app_httpapi_boundaries:")
	if end == -1 {
		t.Fatalf("%s must keep infrastructure_business_boundaries before domain_app_httpapi_boundaries", configPath)
	}
	config = config[start : start+1+end]

	for _, packagePath := range []string{"infra", "integration", "platform", "platformbase", "platformtask"} {
		for _, pattern := range []string{
			fmt.Sprintf(`- "**/internal/%s/*.go"`, packagePath),
			fmt.Sprintf(`- "**/internal/%s/**/*.go"`, packagePath),
		} {
			if !strings.Contains(config, pattern) {
				t.Errorf("%s must cover infrastructure package files with %s", configPath, pattern)
			}
		}
	}

	for _, packagePath := range []string{
		"amazon",
		"amazonlisting",
		"asset",
		"product/catalog",
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
		"workspace",
	} {
		for _, suffix := range []string{"$", "/"} {
			pattern := fmt.Sprintf(`- pkg: "task-processor/internal/%s%s"`, packagePath, suffix)
			if !strings.Contains(config, pattern) {
				t.Errorf("%s must deny business package-tree pattern %s", configPath, pattern)
			}
		}
	}
}

func TestProjectBoundaryListingKitDepguardPatternCoversDomainTrees(t *testing.T) {
	configPath := filepath.Join("..", ".golangci.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}

	config := string(content)
	start := strings.Index(config, "      project_boundary_listingkit:\n")
	if start == -1 {
		t.Fatalf("%s must define project_boundary_listingkit", configPath)
	}
	end := strings.Index(config[start+1:], "\n      infrastructure_business_boundaries:")
	if end == -1 {
		t.Fatalf("%s must keep project_boundary_listingkit before infrastructure_business_boundaries", configPath)
	}
	config = config[start : start+1+end]

	for _, packagePath := range []string{
		"amazon",
		"asset",
		"product/catalog",
		"infra",
		"integration",
		"marketplace",
		"platform",
		"product/sourcing",
		"productimage",
		"publishing",
		"shein",
		"temu",
		"workspace",
	} {
		for _, pattern := range []string{
			fmt.Sprintf(`- "**/internal/%s/*.go"`, packagePath),
			fmt.Sprintf(`- "**/internal/%s/**/*.go"`, packagePath),
		} {
			if !strings.Contains(config, pattern) {
				t.Errorf("%s must cover project-boundary package files with %s", configPath, pattern)
			}
		}
	}

	for _, suffix := range []string{"$", "/"} {
		pattern := fmt.Sprintf(`- pkg: "task-processor/internal/listingkit%s"`, suffix)
		if !strings.Contains(config, pattern) {
			t.Errorf("%s must deny ListingKit package-tree pattern %s", configPath, pattern)
		}
	}
}
