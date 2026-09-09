package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGreenfieldNoLegacyMigrationPolicyIsConsistent(t *testing.T) {
	decision := readGreenfieldPolicyDocument(t, filepath.Join("..", "docs", "product", "greenfield-no-legacy-migration.md"))
	for _, required := range []string{
		"PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08",
		"全新安装、空业务数据",
		"旧数据迁移",
		"旧 ID 映射",
		"旧 Service 包装",
		"tenantbridge consumer",
		"双读、双写、同步或第二事实源",
		"具体、当前的外部可观察契约证据",
	} {
		if !strings.Contains(decision, required) {
			t.Errorf("greenfield decision must state %q", required)
		}
	}

	for _, path := range []string{
		filepath.Join("..", "AGENTS.md"),
		filepath.Join("..", "docs", "engineering", "issue-driven-development.md"),
		filepath.Join("..", "docs", "refactoring", "legacy-hard-cut-policy.md"),
		filepath.Join("..", "docs", "refactoring", "legacy-register.md"),
	} {
		if !strings.Contains(readGreenfieldPolicyDocument(t, path), "PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08") {
			t.Errorf("%s must link the current greenfield policy", filepath.ToSlash(path))
		}
	}

	for _, path := range []string{
		filepath.Join("..", "docs", "product", "issue30-clean-slate-cutover.md"),
		filepath.Join("..", "docs", "superpowers", "specs", "2026-09-05-source-account-organization-cutover.md"),
		filepath.Join("..", "docs", "superpowers", "specs", "2026-09-05-issue-30-sourcing-cutover.md"),
		filepath.Join("..", "docs", "operations", "source-account-ownership-preflight.md"),
		filepath.Join("..", "docs", "refactoring", "2026-09-05-issue-30-prepared-slices.md"),
	} {
		contents := readGreenfieldPolicyDocument(t, path)
		if !strings.Contains(contents, "SUPERSEDED") || !strings.Contains(contents, "PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08") {
			t.Errorf("%s must be an explicitly superseded historical entrypoint", filepath.ToSlash(path))
		}
	}

	for path, forbidden := range map[string]string{
		filepath.Join("..", "docs", "refactoring", "legacy-register.md"):        "A one-time migration/backfill or explicit cutover is preferred",
		filepath.Join("..", "docs", "refactoring", "legacy-hard-cut-policy.md"): "If a migration requires an explicitly bounded transition",
	} {
		if strings.Contains(readGreenfieldPolicyDocument(t, path), forbidden) {
			t.Errorf("%s must not retain the superseded migration authorization %q", filepath.ToSlash(path), forbidden)
		}
	}

	moduleMap := readGreenfieldPolicyDocument(t, filepath.Join("..", "docs", "refactoring", "module-target-mapping.md"))
	for _, required := range []string{
		"PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08",
		"current package areas to current target owners",
		"no legacy numeric mapping, old-data migration/backfill or cutover",
		"current Console, account, plans and review surfaces",
	} {
		if !strings.Contains(moduleMap, required) {
			t.Errorf("active module map must state %q", required)
		}
	}
	for _, forbidden := range []string{
		"#301: freeze new consumers, migrate/backfill/cut over",
		"#30 owns active 1688 handoff cutover",
		"after controlled acceptance",
	} {
		if strings.Contains(moduleMap, forbidden) {
			t.Errorf("active module map must not retain the superseded instruction %q", forbidden)
		}
	}

	for path, required := range map[string][]string{
		filepath.Join("..", "docs", "product", "product-sourcing-mvp-plan.md"): {
			"PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08",
			"fresh installation and empty business data",
			"It does not require legacy profile reuse, environment acceptance or old-execution cutover gates.",
			"Do not build migration, old-ID mapping, wrapper, fallback, dual-read, dual-write, synchronization or second-fact machinery.",
		},
		filepath.Join("..", "docs", "architecture", "README.md"): {
			"PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08",
			"greenfield-no-legacy-migration.md",
		},
		filepath.Join("..", "docs", "product", "product-sourcing-handoff.md"): {
			"PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08",
			"fresh installation and empty business data",
			"it does not authorize migration, profile reuse, wrappers, mappings or cutover machinery.",
			"historical evidence only; it is not a prerequisite for a fresh-install source path.",
			"The new path has no old product/asset/task state, legacy profile or late-result dependency.",
		},
		filepath.Join("..", "docs", "refactoring", "current-refactoring-status.md"): {
			"PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08",
			"greenfield-no-legacy-migration.md",
			"profile reuse and cutover are not current gates.",
		},
		filepath.Join("..", "docs", "refactoring", "legacy-register.md"): {
			"PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08",
			"current entitlement facts are created by their current owners, not carried over",
			"Do not restore task-count quota as resource authority or migrate legacy quota values.",
		},
		filepath.Join("..", "docs", "architecture", "project-boundaries.md"): {
			"PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08",
			"fresh installation and empty business data",
			"does not authorize legacy profile reuse, migration or cutover",
		},
		filepath.Join("..", "docs", "architecture", "auth-and-tenancy.md"): {
			"PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08",
			"legacy profile preservation is not a current requirement",
		},
		filepath.Join("..", "README.md"): {
			"PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08",
			"greenfield-no-legacy-migration.md",
			"历史 profile 和 cutover gate 不是当前验收前置。",
		},
	} {
		contents := strings.Join(strings.Fields(readGreenfieldPolicyDocument(t, path)), " ")
		for _, requirement := range required {
			if !strings.Contains(contents, requirement) {
				t.Errorf("active sourcing authority %s must state %q", filepath.ToSlash(path), requirement)
			}
		}
	}

	for path, forbidden := range map[string]string{
		filepath.Join("..", "docs", "refactoring", "legacy-register.md"):            "migration facts needed to preserve valid current value",
		filepath.Join("..", "docs", "product", "product-sourcing-handoff.md"):      "#30/#307 own separately approved cutover",
		filepath.Join("..", "docs", "refactoring", "current-refactoring-status.md"): "#30 historical data and cutover",
	} {
		if strings.Contains(readGreenfieldPolicyDocument(t, path), forbidden) {
			t.Errorf("active authority %s must not retain superseded instruction %q", filepath.ToSlash(path), forbidden)
		}
	}

	policy := readGreenfieldPolicyDocument(t, filepath.Join("..", "docs", "refactoring", "legacy-hard-cut-policy.md"))
	if !strings.Contains(policy, "only a new explicit product decision from the user") {
		t.Error("Hard-Cut policy must reserve any change to the greenfield prohibition to a new explicit user product decision")
	}

	agents := readGreenfieldPolicyDocument(t, filepath.Join("..", "AGENTS.md"))
	if strings.Contains(agents, "必须先停下来建立显式、可评审的 Exception") {
		t.Error("AGENTS must not retain an executable legacy compatibility Exception path")
	}
	if !strings.Contains(agents, "只有用户新的明确产品决定可以改变本禁令") {
		t.Error("AGENTS must reserve any change to the greenfield prohibition to a new explicit user product decision")
	}
}

func readGreenfieldPolicyDocument(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
