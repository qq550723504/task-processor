[CmdletBinding()]
param(
    [ValidateSet("All", "Go", "Frontend", "Clones", "Verify")]
    [string]$Mode = "All",
    [switch]$ListOnly,
    [switch]$Summarize
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$configPath = Join-Path $PSScriptRoot "code-health-audit.config.json"
$config = Get-Content -Raw $configPath | ConvertFrom-Json

function Get-ToolPath([string]$name) {
    $command = Get-Command $name -ErrorAction Stop | Select-Object -First 1
    return $command.Source
}

function New-DeadcodePlans {
    $plans = @()
    if ($Mode -in @("All", "Go")) {
        foreach ($goos in $config.target_goos) {
            foreach ($testMode in @($false, $true)) {
                $plans += [pscustomobject]@{
                    Name = "deadcode-root-$goos-$(if ($testMode) { 'test' } else { 'prod' })"
                    FilePath = (Get-ToolPath "go")
                    WorkingDirectory = $repoRoot
                    Arguments = @("run", "golang.org/x/tools/cmd/deadcode@$($config.deadcode_version)", "-json") + $(if ($testMode) { @("-test") } else { @() }) + $config.root_patterns
                    GoOS = $goos
                    TestMode = $testMode
                    OutputName = "deadcode-root-$goos-$(if ($testMode) { 'test' } else { 'prod' }).json"
                }
            }
        }
        foreach ($module in $config.nested_modules) {
            $modulePath = Join-Path $repoRoot $module
            if (-not (Test-Path (Join-Path $modulePath "go.mod"))) { continue }
            foreach ($goos in $config.target_goos) {
                foreach ($testMode in @($false, $true)) {
                    $plans += [pscustomobject]@{
                        Name = "deadcode-$($module.Replace('/', '-'))-$goos-$(if ($testMode) { 'test' } else { 'prod' })"
                        FilePath = (Get-ToolPath "go")
                        WorkingDirectory = $modulePath
                        Arguments = @("run", "golang.org/x/tools/cmd/deadcode@$($config.deadcode_version)", "-json") + $(if ($testMode) { @("-test") } else { @() }) + @("./...")
                        GoOS = $goos
                        TestMode = $testMode
                        OutputName = "deadcode-$($module.Replace('/', '-'))-$goos-$(if ($testMode) { 'test' } else { 'prod' }).json"
                    }
                }
            }
        }
    }
    return $plans
}

function Invoke-ProcessPlan($plan, [string]$outputPath) {
    $psi = [Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $plan.FilePath
    $psi.WorkingDirectory = $plan.WorkingDirectory
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    foreach ($argument in $plan.Arguments) { [void]$psi.ArgumentList.Add([string]$argument) }
    if ($plan.PSObject.Properties.Name -contains "GoOS" -and $plan.GoOS) { $psi.Environment["GOOS"] = $plan.GoOS }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $psi
    $started = Get-Date
    if (-not $process.Start()) { throw "Failed to start $($plan.FilePath)" }
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $process.WaitForExit()
    $stdout = $stdoutTask.GetAwaiter().GetResult()
    $stderr = $stderrTask.GetAwaiter().GetResult()
    $text = if ([string]::IsNullOrEmpty($stderr)) { $stdout } else { "$stdout`n$stderr" }
    Set-Content -LiteralPath $outputPath -Value $text -NoNewline
    return [pscustomobject]@{
        name = $plan.Name
        command = $plan.FilePath
        arguments = @($plan.Arguments)
        working_directory = [IO.Path]::GetRelativePath($repoRoot, $plan.WorkingDirectory)
        goos = if ($plan.PSObject.Properties.Name -contains "GoOS") { $plan.GoOS } else { $null }
        test_mode = if ($plan.PSObject.Properties.Name -contains "TestMode") { $plan.TestMode } else { $null }
        output_path = [IO.Path]::GetRelativePath($repoRoot, $outputPath)
        exit_code = $process.ExitCode
        started_at = $started.ToUniversalTime().ToString("o")
        finished_at = (Get-Date).ToUniversalTime().ToString("o")
    }
}

$goPlans = @(New-DeadcodePlans)
$frontendPlans = @()
$clonePlans = @()
if ($Mode -in @("All", "Frontend")) {
    $frontendPlans = @([pscustomobject]@{
        Name = "knip"
        FilePath = (Get-ToolPath "npx.cmd")
        WorkingDirectory = Join-Path $repoRoot "web/listingkit-ui"
        Arguments = @("--yes", "knip@$($config.knip_version)", "--config", "knip.jsonc", "--reporter", "json", "--no-exit-code")
        OutputName = "knip.json"
    })
}
if ($Mode -in @("All", "Clones")) {
    $clonePlans = @([pscustomobject]@{
        Name = "jscpd"
        FilePath = (Get-ToolPath "npx.cmd")
        WorkingDirectory = $repoRoot
        Arguments = @("--yes", "jscpd@$($config.jscpd_version)", "--reporters", "ai") + @($config.clone_paths)
        OutputName = "jscpd.txt"
    })
}

$allPlans = @($goPlans + $frontendPlans + $clonePlans)
if ($ListOnly) {
    Write-Output "code-health-audit mode=$Mode (read-only plan)"
    foreach ($plan in $allPlans) { Write-Output ("- {0}: {1} [{2}]" -f $plan.Name, $plan.FilePath, ($plan.Arguments -join " ")) }
    if ($Mode -in @("All", "Verify")) { Write-Output "- verify: go test ./... -run ^$" }
    exit 0
}

$outputRoot = Join-Path $repoRoot $config.output_root
New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
$runName = (Get-Date).ToUniversalTime().ToString("yyyyMMdd-HHmmssfff")
$runDir = Join-Path $outputRoot $runName
New-Item -ItemType Directory -Force -Path $runDir | Out-Null
$manifest = [ordered]@{
    schema_version = 1
    mode = $Mode
    started_at = (Get-Date).ToUniversalTime().ToString("o")
    run_directory = [IO.Path]::GetRelativePath($repoRoot, $runDir)
    commands = @()
    support_candidates = @()
    failures = @()
}

try {
    if ($Mode -in @("All", "Go", "Verify")) {
        $verify = [pscustomobject]@{ Name = "verify-go-compile"; FilePath = (Get-ToolPath "go"); WorkingDirectory = $repoRoot; Arguments = @("test", "./...", "-run", "^$"); OutputName = "baseline-go-test.txt" }
        $record = Invoke-ProcessPlan $verify (Join-Path $runDir $verify.OutputName)
        $manifest.commands += $record
        if ($record.exit_code -ne 0) { throw "Go compilation baseline failed (exit $($record.exit_code))" }
    }
    if ($Mode -ne "Verify") {
        foreach ($plan in $allPlans) {
            if ($plan.Name -eq "jscpd") {
                $jscpdConfig = [ordered]@{ minLines = $config.clone_min_lines; minTokens = $config.clone_min_tokens; ignore = @($config.clone_ignore); reporters = @("ai") }
                $jscpdConfig | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $runDir "jscpd.json")
                $plan.Arguments = @("--yes", "jscpd@$($config.jscpd_version)", "--config", (Join-Path $runDir "jscpd.json"), "--reporters", "ai") + @($config.clone_paths)
            }
            $record = Invoke-ProcessPlan $plan (Join-Path $runDir $plan.OutputName)
            $manifest.commands += $record
            if ($record.exit_code -ne 0) { $manifest.failures += "$($plan.Name) exited $($record.exit_code)" }
        }
    }
    $tracked = & (Get-ToolPath "git") -C $repoRoot ls-files -- scripts tools hack/debug
    $manifest.support_candidates = @($tracked | Where-Object { $_ -match '\.(ps1|sh|mjs|js|cmd|bat|go)$' })
    if ($Summarize) {
        $summary = @()
        foreach ($record in $manifest.commands | Where-Object { $_.name -like "deadcode-*" -and $_.exit_code -eq 0 }) {
            $file = Join-Path $repoRoot $record.output_path
            if (-not (Test-Path $file)) { continue }
            try {
                $items = Get-Content -Raw $file | ConvertFrom-Json
                foreach ($pkg in @($items)) {
                    foreach ($fn in @($pkg.Funcs)) {
                        $summary += [ordered]@{ package = $pkg.Path; name = $fn.Name; file = $fn.Position.File; line = $fn.Position.Line; source = $record.name }
                    }
                }
            } catch { }
        }
        $summary | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $runDir "deadcode-intersection.json")
    }
} catch {
    $manifest.failures += $_.Exception.Message
} finally {
    $manifest.finished_at = (Get-Date).ToUniversalTime().ToString("o")
    $manifest | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $runDir "manifest.json")
    Set-Content -LiteralPath (Join-Path $outputRoot "latest-run.txt") -Value ([IO.Path]::GetRelativePath($repoRoot, $runDir))
}

if ($manifest.failures.Count -gt 0) { throw ($manifest.failures -join "; ") }
Write-Output ([IO.Path]::GetRelativePath($repoRoot, $runDir))
