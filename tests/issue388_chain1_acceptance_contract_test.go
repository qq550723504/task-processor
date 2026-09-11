package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssue388Chain1AcceptanceAssemblyContract(t *testing.T) {
	repositoryRoot := filepath.Clean("..")
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(path)))
		require.NoError(t, err, "required #388 acceptance artifact %s", path)
		return string(raw)
	}

	harness := read("internal/app/httpapi/chain1_local_product_acceptance_test.go")
	orchestrationDriver := read("internal/imageagent/temporal/acceptancetest/driver.go")
	assembly := harness + orchestrationDriver
	for _, required := range []string{
		"productsourcing.NewInternalProducer",
		"NewProductReviewApplication",
		"NewImageAgentOrganizationApplication",
		"imageagenttemporal.NewOrganizationClient",
		"imageagenttemporal.NewWorker",
		"WorkerWireModeOrganization",
		"imageagentworker.OrganizationExecutionAuthorizer",
		"imageagenttools.NewProductImageSlotExecutor",
		"imageagentpolicy.LoadEmbeddedResolver",
		"NewSheinRecordApplication",
	} {
		require.Contains(t, assembly, required, "#388 must compose the current owner %s", required)
	}
	for _, forbidden := range []string{
		"CommitApproval(",
		"INSERT INTO product_approved_assets",
		"INSERT INTO listing_shein_records",
		"INSERT INTO store_center_stores",
		"tenantbridge",
		"ProfileResolver: imagePorts",
	} {
		require.NotContains(t, harness, forbidden, "#388 must not seed or reintroduce %s", forbidden)
	}

	script := read("scripts/chain1-local-product-acceptance.ps1")
	for _, required := range []string{
		"CHAIN1_ACCEPTANCE_DSN",
		"CHAIN1_TEMPORAL_ADDRESS",
		"TestChain1LocalProductAcceptance",
		"docker compose",
		"CHAIN1_STAGE",
		"git rev-parse --show-toplevel",
		"git status --porcelain=v1 --untracked-files=all",
		"requires a clean git index and worktree",
		"@(\"resources\", \"business-chain\", \"contract\", \"cleanup\", \"acceptance\")",
	} {
		require.Contains(t, script, required)
	}
	require.NotContains(t, script, "ISSUE376_TEST_DSN")
	require.NotContains(t, script, "ISSUE382_TEST_DSN")

	compose := read("deployments/docker/chain1-acceptance/docker-compose.yml")
	require.Contains(t, compose, "chain1-postgres")
	require.Contains(t, compose, "chain1-temporal")
	require.Contains(t, compose, "CHAIN1_DB_PORT:-17443")
	require.Contains(t, compose, "CHAIN1_TEMPORAL_PORT:-17333")
	require.NotContains(t, strings.ToLower(compose), "zitadel")

	documentation := read("docs/development/chain1-local-product-acceptance.md")
	for _, required := range []string{"PASS", "FAIL", "SKIP", "NOT_RUN", "controlled external", "real PostgreSQL", "real Temporal"} {
		require.Contains(t, documentation, required)
	}
}

func TestIssue388Chain1ScriptExecutionReceipts(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell command-shim execution contract is Windows-specific")
	}

	tests := []struct {
		name        string
		dirty       string
		environment map[string]string
		wantSuccess bool
		wantStages  map[string]string
		wantDocker  int
		wantGo      int
		wantHead    bool
	}{
		{
			name:        "clean",
			wantSuccess: true,
			wantStages:  chain1Stages("PASS", "PASS", "PASS", "PASS", "PASS"),
			wantDocker:  3,
			wantGo:      2,
			wantHead:    true,
		},
		{
			name:       "staged change",
			dirty:      "staged",
			wantStages: chain1Stages("NOT_RUN", "NOT_RUN", "NOT_RUN", "NOT_RUN", "FAIL"),
		},
		{
			name:       "unstaged tracked change",
			dirty:      "unstaged",
			wantStages: chain1Stages("NOT_RUN", "NOT_RUN", "NOT_RUN", "NOT_RUN", "FAIL"),
		},
		{
			name:       "untracked Go change",
			dirty:      "untracked",
			wantStages: chain1Stages("NOT_RUN", "NOT_RUN", "NOT_RUN", "NOT_RUN", "FAIL"),
		},
		{
			name:        "git identity query fails",
			environment: map[string]string{"CHAIN1_FAIL_GIT": "1"},
			wantStages:  chain1Stages("NOT_RUN", "NOT_RUN", "NOT_RUN", "NOT_RUN", "FAIL"),
		},
		{
			name:        "initial Compose cleanup fails",
			environment: map[string]string{"CHAIN1_FAIL_INITIAL_DOWN": "1"},
			wantStages:  chain1Stages("FAIL", "NOT_RUN", "NOT_RUN", "PASS", "FAIL"),
			wantDocker:  2,
			wantHead:    true,
		},
		{
			name:        "Compose startup fails",
			environment: map[string]string{"CHAIN1_FAIL_UP": "1"},
			wantStages:  chain1Stages("FAIL", "NOT_RUN", "NOT_RUN", "PASS", "FAIL"),
			wantDocker:  3,
			wantHead:    true,
		},
		{
			name:        "business chain fails before contract",
			environment: map[string]string{"CHAIN1_FAIL_BUSINESS": "1"},
			wantStages:  chain1Stages("PASS", "FAIL", "NOT_RUN", "PASS", "FAIL"),
			wantDocker:  3,
			wantGo:      1,
			wantHead:    true,
		},
		{
			name:        "final cleanup fails",
			environment: map[string]string{"CHAIN1_FAIL_FINAL_DOWN": "1"},
			wantStages:  chain1Stages("PASS", "PASS", "PASS", "FAIL", "FAIL"),
			wantDocker:  3,
			wantGo:      2,
			wantHead:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := runChain1ScriptFixture(t, test.dirty, test.environment)
			if test.wantSuccess {
				require.NoError(t, result.err, result.output)
			} else {
				require.Error(t, result.err, "a failed or unidentifiable run must return non-zero\n%s", result.output)
			}
			require.Equal(t, test.wantStages, finalChain1Stages(result.output), result.output)
			require.Equal(t, test.wantDocker, countChain1Calls(result.calls, "docker "), result.calls)
			require.Equal(t, test.wantGo, countChain1Calls(result.calls, "go "), result.calls)
			if test.wantHead {
				require.Contains(t, result.output, "CHAIN1_HEAD "+result.head)
			} else {
				require.NotContains(t, result.output, "CHAIN1_HEAD ")
			}
		})
	}
}

type chain1ScriptResult struct {
	output string
	calls  string
	head   string
	err    error
}

func chain1Stages(resources, business, contract, cleanup, acceptance string) map[string]string {
	return map[string]string{
		"resources":      resources,
		"business-chain": business,
		"contract":       contract,
		"cleanup":        cleanup,
		"acceptance":     acceptance,
	}
}

func finalChain1Stages(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 3 && fields[0] == "CHAIN1_STAGE" {
			result[fields[1]] = fields[2]
		}
	}
	return result
}

func countChain1Calls(calls, prefix string) int {
	count := 0
	for _, line := range strings.Split(strings.ReplaceAll(calls, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			count++
		}
	}
	return count
}

func runChain1ScriptFixture(t *testing.T, dirty string, environment map[string]string) chain1ScriptResult {
	t.Helper()

	temporaryRoot := t.TempDir()
	repository := filepath.Join(temporaryRoot, "repository")
	shimDirectory := filepath.Join(temporaryRoot, "shims")
	require.NoError(t, os.MkdirAll(filepath.Join(repository, "scripts"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repository, "deployments", "docker", "chain1-acceptance"), 0o755))
	require.NoError(t, os.MkdirAll(shimDirectory, 0o755))

	sourceScript, err := os.ReadFile(filepath.Join("..", "scripts", "chain1-local-product-acceptance.ps1"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repository, "scripts", "chain1-local-product-acceptance.ps1"), sourceScript, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repository, "deployments", "docker", "chain1-acceptance", "docker-compose.yml"), []byte("services: {}\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repository, "fixture.go"), []byte("package fixture\n"), 0o600))

	runChain1Git(t, repository, "init", "-q")
	runChain1Git(t, repository, "config", "user.email", "chain1@example.invalid")
	runChain1Git(t, repository, "config", "user.name", "CHAIN-1 Test")
	runChain1Git(t, repository, "add", ".")
	runChain1Git(t, repository, "commit", "-q", "-m", "fixture")
	head := strings.TrimSpace(runChain1Git(t, repository, "rev-parse", "HEAD"))

	switch dirty {
	case "staged":
		require.NoError(t, os.WriteFile(filepath.Join(repository, "staged.go"), []byte("package fixture\n"), 0o600))
		runChain1Git(t, repository, "add", "staged.go")
	case "unstaged":
		require.NoError(t, os.WriteFile(filepath.Join(repository, "fixture.go"), []byte("package fixture\n\nconst dirty = true\n"), 0o600))
	case "untracked":
		require.NoError(t, os.WriteFile(filepath.Join(repository, "untracked.go"), []byte("package fixture\n"), 0o600))
	case "":
	default:
		t.Fatalf("unknown dirty fixture %q", dirty)
	}

	callLog := filepath.Join(temporaryRoot, "calls.log")
	downMarker := filepath.Join(temporaryRoot, "down.marker")
	dockerShim := `@echo off
echo docker %*>>"%CHAIN1_TEST_CALL_LOG%"
set "chain1_args= %* "
echo %chain1_args%| findstr /C:" down " >nul
if not errorlevel 1 (
  if not exist "%CHAIN1_TEST_DOWN_MARKER%" (
    >"%CHAIN1_TEST_DOWN_MARKER%" echo initial
    if "%CHAIN1_FAIL_INITIAL_DOWN%"=="1" exit /b 23
  ) else (
    if "%CHAIN1_FAIL_FINAL_DOWN%"=="1" exit /b 23
  )
)
echo %chain1_args%| findstr /C:" up " >nul
if not errorlevel 1 if "%CHAIN1_FAIL_UP%"=="1" exit /b 23
exit /b 0
`
	goShim := `@echo off
echo go %*>>"%CHAIN1_TEST_CALL_LOG%"
set "chain1_args= %* "
echo %chain1_args%| findstr /C:"TestChain1LocalProductAcceptance" >nul
if not errorlevel 1 if "%CHAIN1_FAIL_BUSINESS%"=="1" exit /b 17
exit /b 0
`
	require.NoError(t, os.WriteFile(filepath.Join(shimDirectory, "docker.cmd"), []byte(strings.ReplaceAll(dockerShim, "\n", "\r\n")), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(shimDirectory, "go.cmd"), []byte(strings.ReplaceAll(goShim, "\n", "\r\n")), 0o600))
	if environment["CHAIN1_FAIL_GIT"] == "1" {
		gitShim := "@echo off\r\necho git %*>>\"%CHAIN1_TEST_CALL_LOG%\"\r\nexit /b 42\r\n"
		require.NoError(t, os.WriteFile(filepath.Join(shimDirectory, "git.cmd"), []byte(gitShim), 0o600))
	}

	command := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(repository, "scripts", "chain1-local-product-acceptance.ps1"))
	command.Env = append(os.Environ(),
		"PATH="+shimDirectory+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CHAIN1_TEST_CALL_LOG="+callLog,
		"CHAIN1_TEST_DOWN_MARKER="+downMarker,
	)
	for key, value := range environment {
		command.Env = append(command.Env, fmt.Sprintf("%s=%s", key, value))
	}
	output, commandErr := command.CombinedOutput()
	calls, readErr := os.ReadFile(callLog)
	if !os.IsNotExist(readErr) {
		require.NoError(t, readErr)
	}
	return chain1ScriptResult{output: string(output), calls: string(calls), head: head, err: commandErr}
}

func runChain1Git(t *testing.T, repository string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, arguments...)...)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "git %s failed: %s", strings.Join(arguments, " "), output)
	return string(output)
}
