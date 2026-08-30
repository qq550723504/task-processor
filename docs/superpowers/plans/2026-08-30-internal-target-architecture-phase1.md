# Internal 目标架构迁移阶段 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 消除默认日志初始化对源码目录的文件系统副作用，将受维护日志配置统一到仓库根 `.local/logs`，并让结构测试检查实际文件系统。

**Architecture:** logger 库默认只输出控制台，文件输出必须由 app 显式配置。受维护配置只允许使用 `.local/logs`；结构测试遍历真实目录。当前忽略产物只移动到仓库根 `.local` 隔离区，不删除。

**Tech Stack:** Go 1.26、logrus、Viper、`gopkg.in/yaml.v3`、Go `testing`、PowerShell、Git

**Spec:** `docs/superpowers/specs/2026-08-30-internal-target-architecture-migration-design.md`

## Global Constraints

- 本计划只实现设计的阶段 1；后续架构阶段分别编写实施计划。
- 不删除当前工作区产物；只允许移动到 `.local/legacy-internal-artifacts/2026-08-30`。
- logger 库和测试默认必须为 stdout-only；文件日志必须由 app 显式配置。
- 本阶段不改变 HTTP、RabbitMQ、Temporal、数据库契约。
- 不引入新日志库或依赖分析器。
- 每个提交前运行聚焦测试和 `git diff --check`。

---

### Task 1: 让默认 logger 不产生文件

**Files:**
- Modify: `internal/core/logger/manager.go:43-56`
- Modify: `internal/core/logger/manager_test.go:55-65,149-158`
- Modify: `internal/core/config/common_types.go:97-108`
- Modify: `internal/core/config/common_types_test.go`

**Interfaces:**
- Consumes: `NewLogManager(*LogConfig) *LogManager`、`GetGlobalLogger(string) *logrus.Entry`。
- Produces: `logger.DefaultLogConfig().OutputFile == ""`、`config.DefaultLogConfig().FilePath == ""`；显式非空 `OutputFile` 行为保持不变。

- [ ] **Step 1: 写默认构造失败测试**

在 `internal/core/logger/manager_test.go` 增加：

```go
func TestDefaultLogManagerDoesNotCreateRuntimeFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	manager := NewLogManager(nil)
	t.Cleanup(func() { _ = manager.Close() })

	manager.GetLogger("default-no-file").Info("stdout only")
	if _, err := os.Stat("tmp"); !os.IsNotExist(err) {
		t.Fatalf("default logger created a runtime directory: %v", err)
	}
}
```

- [ ] **Step 2: 运行并确认 RED**

Run:

```powershell
go test ./internal/core/logger -run TestDefaultLogManagerDoesNotCreateRuntimeFiles -count=1 -v
```

Expected: FAIL，包含 `default logger created a runtime directory`。

- [ ] **Step 3: 写延迟全局初始化失败测试**

```go
func TestLazyGlobalLoggerDoesNotCreateRuntimeFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	previous := globalLogManager
	globalLogManager = nil
	t.Cleanup(func() {
		if globalLogManager != nil {
			_ = globalLogManager.Close()
		}
		globalLogManager = previous
	})

	GetGlobalLogger("lazy-no-file").Info("stdout only")
	if _, err := os.Stat("tmp"); !os.IsNotExist(err) {
		t.Fatalf("lazy global logger created a runtime directory: %v", err)
	}
}
```

- [ ] **Step 4: 运行并确认 RED**

Run:

```powershell
go test ./internal/core/logger -run TestLazyGlobalLoggerDoesNotCreateRuntimeFiles -count=1 -v
```

Expected: FAIL，包含 `lazy global logger created a runtime directory`。

- [ ] **Step 5: 实现 stdout-only 默认值**

在 `internal/core/logger/manager.go` 保持其他字段不变，只将：

```go
OutputFile: "",
```

在 `internal/core/config/common_types.go` 保持其他字段不变，只将：

```go
FilePath: "",
```

将 `TestDefaultLogConfig` 的路径断言改为：

```go
assert.Empty(t, config.OutputFile)
```

在 `internal/core/config/common_types_test.go` 增加：

```go
func TestDefaultLogConfigDoesNotEnableFileOutput(t *testing.T) {
	cfg := DefaultLogConfig()
	if cfg.Output != "stdout" {
		t.Fatalf("Output = %q, 期望 stdout", cfg.Output)
	}
	if cfg.FilePath != "" {
		t.Fatalf("FilePath = %q, 默认配置不能启用文件输出", cfg.FilePath)
	}
}
```

- [ ] **Step 6: 运行 GREEN 验证**

Run:

```powershell
go test ./internal/core/logger ./internal/core/config -count=1
```

Expected: PASS；现有 `TestLogManagerWithFile` 同时证明显式文件日志仍可用。

- [ ] **Step 7: 提交**

```powershell
git diff --check
git add internal/core/logger/manager.go internal/core/logger/manager_test.go internal/core/config/common_types.go internal/core/config/common_types_test.go
git commit -m "fix(logging): keep default logger stdout only"
```

---

### Task 2: 将受维护日志配置限制在 `.local/logs`

**Files:**
- Create: `tests/logging_config_test.go`
- Modify: `config/config-dev.yaml:5-17`
- Modify: `config/config-test.yaml:5-8`
- Modify: `config/config-prod.yaml:5-17`

**Interfaces:**
- Consumes: YAML `logging.file` 与 `logging.split_by_level[].file`。
- Produces: 所有非空受维护日志路径都以 `.local/logs/` 开头。

- [ ] **Step 1: 写配置边界失败测试**

创建 `tests/logging_config_test.go`：

```go
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type maintainedLoggingFileConfig struct {
	Logging struct {
		File         string `yaml:"file"`
		SplitByLevel []struct {
			File string `yaml:"file"`
		} `yaml:"split_by_level"`
	} `yaml:"logging"`
}

func TestMaintainedLoggingFilesStayUnderLocalRuntimeRoot(t *testing.T) {
	for _, name := range []string{"config-dev.yaml", "config-test.yaml", "config-prod.yaml"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "config", name))
			if err != nil {
				t.Fatal(err)
			}
			var cfg maintainedLoggingFileConfig
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatal(err)
			}
			paths := []string{cfg.Logging.File}
			for _, split := range cfg.Logging.SplitByLevel {
				paths = append(paths, split.File)
			}
			for _, path := range paths {
				if path == "" {
					continue
				}
				clean := filepath.ToSlash(filepath.Clean(path))
				if !strings.HasPrefix(clean, ".local/logs/") {
					t.Errorf("logging path %q must stay under .local/logs", path)
				}
			}
		})
	}
}
```

- [ ] **Step 2: 运行并确认 RED**

```powershell
go test ./tests -run TestMaintainedLoggingFilesStayUnderLocalRuntimeRoot -count=1 -v
```

Expected: FAIL，报告 `tmp/logs/app.log` 和开发配置的分级日志路径。

- [ ] **Step 3: 更新 YAML 路径**

三个配置的主日志均改为：

```yaml
file: ".local/logs/app.log"
```

开发和生产配置的分级日志依次使用：

```yaml
file: .local/logs/error.log
file: .local/logs/warn.log
file: .local/logs/info.log
file: .local/logs/debug.log
```

测试配置没有分级日志时不新增配置。

- [ ] **Step 4: 运行 GREEN 验证**

```powershell
go test ./tests -run TestMaintainedLoggingFilesStayUnderLocalRuntimeRoot -count=1
go test ./internal/app/runtime/listing -run TestApplyLoggingConfigFromConfig_WritesToConfiguredFile -count=1 -v
```

Expected: PASS，显式 `t.TempDir` 日志仍能写入。

- [ ] **Step 5: 提交**

```powershell
git diff --check
git add tests/logging_config_test.go config/config-dev.yaml config/config-test.yaml config/config-prod.yaml
git commit -m "chore(logging): keep configured logs under local runtime root"
```

---

### Task 3: 让产物护栏扫描实际文件系统

**Files:**
- Modify: `tests/repository_structure_test.go:3-10,141-143,182-227`

**Interfaces:**
- Consumes: `containsLocalArtifactPathPart(string) bool`。
- Produces: `localArtifactPaths(repoRoot, pathspec string) ([]string, error)`，返回最外层产物目录及独立产物文件，并跳过已命中的目录子树。

- [ ] **Step 1: 写 ignored 文件扫描失败测试**

```go
func TestLocalArtifactPathsInspectIgnoredFilesystemEntries(t *testing.T) {
	repoRoot := t.TempDir()
	artifactDir := filepath.Join(repoRoot, "internal", "example", "tmp", "logs")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "app.log"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := localArtifactPaths(repoRoot, "internal")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"internal/example/tmp"}
	if !slices.Equal(paths, want) {
		t.Fatalf("localArtifactPaths() = %v, want %v", paths, want)
	}
}
```

- [ ] **Step 2: 运行并确认 RED**

```powershell
go test ./tests -run TestLocalArtifactPathsInspectIgnoredFilesystemEntries -count=1 -v
```

Expected: FAIL，编译错误包含 `undefined: localArtifactPaths`。

- [ ] **Step 3: 实现扫描器**

imports 增加 `io/fs` 和 `sort`，然后增加：

```go
func localArtifactPaths(repoRoot, pathspec string) ([]string, error) {
	root := filepath.Join(repoRoot, filepath.FromSlash(normalizeRepoRelativePath(pathspec)))
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if !containsLocalArtifactPathPart(relative) {
			return nil
		}
		paths = append(paths, relative)
		if entry.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}
```

- [ ] **Step 4: 将护栏切换到扫描器**

```go
func assertNoLocalArtifactPaths(t *testing.T, pathspec string) {
	t.Helper()
	repoRootBytes, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := strings.TrimSpace(string(repoRootBytes))
	paths, err := localArtifactPaths(repoRoot, pathspec)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		t.Errorf("%s is a local artifact path under %s; keep runtime files under .local instead", path, pathspec)
	}
}
```

保留 `assertNoTrackedLocalArtifacts`，它继续负责 Git 索引约束。

- [ ] **Step 5: 验证 helper GREEN、真实仓库 RED**

```powershell
go test ./tests -run TestLocalArtifactPathsInspectIgnoredFilesystemEntries -count=1 -v
go test ./tests -run TestInternalPackagesContainNoLocalArtifacts -count=1 -v
```

Expected: helper PASS；真实仓库测试 FAIL，并报告当前 `internal` 下的 `tmp` 或 `.local` 路径。

- [ ] **Step 6: 可恢复地隔离现有产物**

使用一个 PowerShell 进程解析并校验源、目标绝对路径。目录名按结构规则匹配，独立文件同时覆盖 `__debug_bin*` 与 `.exe`；只移动没有产物目录祖先的最外层目标：

```powershell
$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (git rev-parse --show-toplevel)).Path
$sourceRoot = (Resolve-Path (Join-Path $repoRoot 'internal')).Path
$destinationRoot = Join-Path $repoRoot '.local\legacy-internal-artifacts\2026-08-30'
if (Test-Path -LiteralPath $destinationRoot) {
    throw "destination already exists: $destinationRoot"
}

$directoryNames = @('.local', 'logs', 'tmp', 'bin', 'dev-logs', 'playwright-cli', 'node_modules', 'result')
$candidates = @(Get-ChildItem -LiteralPath $sourceRoot -Recurse -Force | Where-Object {
    ($_.PSIsContainer -and $_.Name -in $directoryNames) -or
    (-not $_.PSIsContainer -and ($_.Name.StartsWith('__debug_bin', [StringComparison]::Ordinal) -or $_.Extension -ieq '.exe'))
})
$targets = @($candidates | Where-Object {
    $candidatePath = $_.FullName
    -not ($candidates | Where-Object {
        $_.PSIsContainer -and
        $_.FullName -ne $candidatePath -and
        $candidatePath.StartsWith($_.FullName + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)
    })
})

foreach ($target in $targets) {
    $source = (Resolve-Path -LiteralPath $target.FullName).Path
    if (-not $source.StartsWith($sourceRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
        throw "source escaped internal: $source"
    }
    $relative = [IO.Path]::GetRelativePath($sourceRoot, $source)
    $destination = Join-Path $destinationRoot $relative
    if (-not $destination.StartsWith($destinationRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
        throw "destination escaped quarantine: $destination"
    }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $destination) | Out-Null
    Move-Item -LiteralPath $source -Destination $destination
}

Write-Output ("moved_artifact_roots={0} destination={1}" -f $targets.Count, $destinationRoot)
```

Expected: 输出移动数量和隔离目录；匹配的目录和独立文件都被保留，没有内容被删除。

- [ ] **Step 7: 运行真实护栏 GREEN**

```powershell
go test ./tests -run 'Test(InternalPackagesContainNoLocalArtifacts|LocalArtifactPathsInspectIgnoredFilesystemEntries|ProductionEntrypointsContainNoLocalArtifacts|HackSupportAreasContainNoLocalArtifacts|ToolsContainNoLocalArtifacts)$' -count=1 -v
```

Expected: PASS。

- [ ] **Step 8: 提交**

```powershell
git diff --check
git add tests/repository_structure_test.go
git commit -m "test(structure): inspect actual runtime artifacts"
```

隔离目录保持 Git 忽略，不进入提交。

---

### Task 4: 更新规则并完成阶段验证

**Files:**
- Modify: `docs/development/repository-structure.md:55-59,89-93`

**Interfaces:**
- Consumes: Task 1 的 stdout-only 默认、Task 2 的 `.local/logs` 配置、Task 3 的实际扫描护栏。
- Produces: 阶段 2 可依赖的无源码目录副作用日志基线。

- [ ] **Step 1: 更新文档**

在 `.local/` 规则下增加：

```markdown
- 日志库和测试默认只输出到 stdout；只有 app 运行装配可以通过显式配置启用文件日志。
- 仓库内受维护的相对日志路径统一位于 `.local/logs/`；测试文件输出使用 `t.TempDir()`。
```

在 `internal/*` 规则下增加：

```markdown
- `TestInternalPackagesContainNoLocalArtifacts` 检查实际文件系统，包括 Git 忽略路径；`.gitignore` 不能作为在源码包下保留运行态文件的依据。
```

- [ ] **Step 2: 运行聚焦测试**

```powershell
go test ./internal/core/logger ./internal/core/config ./internal/app/runtime/listing ./internal/app/runtime/listingcontrol ./internal/app/runtime/listingscheduler -count=1
go test ./tests -run 'Test(InternalPackagesContainNoLocalArtifacts|LocalArtifactPathsInspectIgnoredFilesystemEntries|MaintainedLoggingFilesStayUnderLocalRuntimeRoot|ProductionEntrypointsContainNoLocalArtifacts|HackSupportAreasContainNoLocalArtifacts|ToolsContainNoLocalArtifacts)$' -count=1
```

Expected: PASS。

- [ ] **Step 3: 运行完整架构测试**

```powershell
go test ./tests -count=1
```

Expected: PASS。若命令超时或受环境依赖阻塞，记录准确命令、耗时和最后输出，不报告完整测试通过。

- [ ] **Step 4: 运行仓库级测试**

```powershell
go test ./... -count=1
```

Expected: PASS。若环境集成测试无法运行，记录具体 package、测试名和错误，不用聚焦测试替代全量结论。

- [ ] **Step 5: 提交文档**

```powershell
git diff --check
git add docs/development/repository-structure.md
git commit -m "docs(structure): define runtime log ownership"
```

- [ ] **Step 6: 核验阶段提交与状态**

```powershell
git log -4 --oneline
git status --short
```

Expected: 最近四个提交依次覆盖默认 logger、受维护配置、实际文件系统护栏和结构文档；工作区干净。随后依据总体设计编写阶段 2 的独立实施计划。
