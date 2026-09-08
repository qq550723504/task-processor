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
}

func readGreenfieldPolicyDocument(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
