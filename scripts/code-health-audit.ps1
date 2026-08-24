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
    $lookup = @($name)
    if ($name -eq "npx.cmd") { $lookup += "npx" }
    foreach ($candidate in $lookup) {
        $command = Get-Command $candidate -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($command) { return $command.Source }
    }
    throw "Unable to locate required tool: $name"
}

function New-DeadcodePlans {
    $plans = @()
    if ($Mode -in @("All", "Go", "Verify")) {
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
                        ModuleMode = "mod"
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
    $moduleFiles = @()
    if ($plan.PSObject.Properties.Name -contains "ModuleMode" -and $plan.ModuleMode) {
        $psi.Environment["GOFLAGS"] = "-mod=$($plan.ModuleMode)"
        foreach ($name in @("go.mod", "go.sum")) {
            $path = Join-Path $plan.WorkingDirectory $name
            $moduleFiles += [pscustomobject]@{ Path = $path; Exists = (Test-Path -LiteralPath $path); Bytes = if (Test-Path -LiteralPath $path) { [IO.File]::ReadAllBytes($path) } else { $null } }
        }
    }
    foreach ($argument in $plan.Arguments) { [void]$psi.ArgumentList.Add([string]$argument) }
    if ($plan.PSObject.Properties.Name -contains "GoOS" -and $plan.GoOS) { $psi.Environment["GOOS"] = $plan.GoOS }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $psi
    $started = Get-Date
    try {
        if (-not $process.Start()) { throw "Failed to start $($plan.FilePath)" }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        $process.WaitForExit()
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        Set-Content -LiteralPath $outputPath -Value $stdout -NoNewline
        $stderrPath = $null
        if (-not [string]::IsNullOrEmpty($stderr)) {
            $stderrPath = "$outputPath.stderr"
            Set-Content -LiteralPath $stderrPath -Value $stderr -NoNewline
        }
        return [pscustomobject]@{
            name = $plan.Name
            command = $plan.FilePath
            arguments = @($plan.Arguments)
            working_directory = [IO.Path]::GetRelativePath($repoRoot, $plan.WorkingDirectory)
            goos = if ($plan.PSObject.Properties.Name -contains "GoOS") { $plan.GoOS } else { $null }
            test_mode = if ($plan.PSObject.Properties.Name -contains "TestMode") { $plan.TestMode } else { $null }
            module_mode = if ($plan.PSObject.Properties.Name -contains "ModuleMode") { $plan.ModuleMode } else { $null }
            output_path = [IO.Path]::GetRelativePath($repoRoot, $outputPath)
            stderr_path = if ($stderrPath) { [IO.Path]::GetRelativePath($repoRoot, $stderrPath) } else { $null }
            exit_code = $process.ExitCode
            started_at = $started.ToUniversalTime().ToString("o")
            finished_at = (Get-Date).ToUniversalTime().ToString("o")
        }
    } finally {
        foreach ($moduleFile in $moduleFiles) {
            if ($moduleFile.Exists) { [IO.File]::WriteAllBytes($moduleFile.Path, $moduleFile.Bytes) }
            elseif (Test-Path -LiteralPath $moduleFile.Path) { [IO.File]::Delete($moduleFile.Path) }
        }
    }
}

function Install-Deadcode {
    $gobin = (& (Get-ToolPath "go") env GOBIN).Trim()
    if ([string]::IsNullOrWhiteSpace($gobin)) {
        $gobin = Join-Path ((& (Get-ToolPath "go") env GOPATH).Trim()) "bin"
    }
    $name = if ($IsWindows) { "deadcode.exe" } else { "deadcode" }
    $path = Join-Path $gobin $name
    & (Get-ToolPath "go") install "golang.org/x/tools/cmd/deadcode@$($config.deadcode_version)"
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $path)) {
        throw "Unable to install pinned deadcode $($config.deadcode_version)"
    }
    return $path
}

function Get-KnipIssueCount($knip) {
    $count = 0
    if ($null -ne $knip.files) { $count += @($knip.files).Count }
    foreach ($issue in @($knip.issues)) {
        if ($null -eq $issue) { continue }
        foreach ($property in @("file", "files", "dependencies", "devDependencies", "exports", "types")) {
            if ($issue.PSObject.Properties.Name -notcontains $property) { continue }
            $value = $issue.$property
            if ($value -is [System.Collections.IEnumerable] -and $value -isnot [string]) {
                $count += @($value).Count
            } elseif ($null -ne $value) {
                $count++
            }
        }
    }
    return $count
}

$goPlans = @(New-DeadcodePlans)
$frontendPlans = @()
$clonePlans = @()
if ($Mode -in @("All", "Frontend", "Verify")) {
    $frontendPlans = @([pscustomobject]@{
        Name = "knip"
        FilePath = (Get-ToolPath "npx.cmd")
        WorkingDirectory = Join-Path $repoRoot "web/listingkit-ui"
        Arguments = @("--yes", "knip@$($config.knip_version)", "--config", "knip.jsonc", "--reporter", "json", "--no-exit-code")
        OutputName = "knip.json"
    })
}
if ($Mode -in @("All", "Clones", "Verify")) {
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
    Write-Output "- verify: go test ./... -run ^$"
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
        $deadcodePath = Install-Deadcode
        foreach ($plan in $goPlans) {
            $plan.FilePath = $deadcodePath
            $packageArgs = if ($plan.TestMode) { @($plan.Arguments | Select-Object -Skip 4) } else { @($plan.Arguments | Select-Object -Skip 3) }
            $plan.Arguments = @("-json") + $(if ($plan.TestMode) { @("-test") } else { @() }) + $packageArgs
        }
    }
    if ($Mode -in @("All", "Go", "Frontend", "Clones", "Verify")) {
        $verify = [pscustomobject]@{ Name = "verify-go-compile"; FilePath = (Get-ToolPath "go"); WorkingDirectory = $repoRoot; Arguments = @("test", "./...", "-run", "^$"); OutputName = "baseline-go-test.txt" }
        $record = Invoke-ProcessPlan $verify (Join-Path $runDir $verify.OutputName)
        $manifest.commands += $record
        if ($record.exit_code -ne 0) { throw "Go compilation baseline failed (exit $($record.exit_code))" }
    }
    foreach ($plan in $allPlans) {
        if ($plan.Name -eq "jscpd") {
            $jscpdConfig = [ordered]@{ minLines = $config.clone_min_lines; minTokens = $config.clone_min_tokens; ignore = @($config.clone_ignore); reporters = @("ai") }
            $jscpdConfig | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $runDir "jscpd.json")
            $plan.Arguments = @("--yes", "jscpd@$($config.jscpd_version)", "--config", (Join-Path $runDir "jscpd.json"), "--reporters", "ai") + @($config.clone_paths)
        }
        $record = Invoke-ProcessPlan $plan (Join-Path $runDir $plan.OutputName)
        $manifest.commands += $record
        if ($record.exit_code -ne 0) { $manifest.failures += "$($plan.Name) exited $($record.exit_code)" }
        if ($Mode -eq "Verify" -and $plan.Name -eq "knip" -and $record.exit_code -eq 0) {
            $knip = Get-Content -Raw (Join-Path $runDir $plan.OutputName) | ConvertFrom-Json
            $issueCount = Get-KnipIssueCount $knip
            if ($issueCount -gt 0) { $manifest.failures += "knip reported $issueCount unclassified findings" }
        }
    }
    $tracked = & (Get-ToolPath "git") -C $repoRoot ls-files -- scripts tools hack/debug
    $manifest.support_candidates = @($tracked | Where-Object { $_ -match '\.(ps1|sh|mjs|js|cmd|bat|go)$' })
    if ($Summarize) {
        $records = @($manifest.commands | Where-Object { $_.name -like "deadcode-*" -and $_.exit_code -eq 0 })
        $groups = @{}
        foreach ($record in $records) {
            $scope = $record.name -replace '^deadcode-(.+)-(windows|linux)-(prod|test)$', '$1'
            if (-not $groups.ContainsKey($scope)) { $groups[$scope] = @() }
            $groups[$scope] += $record
        }
        $summary = @()
        foreach ($scope in $groups.Keys) {
            $scopeRecords = @($groups[$scope])
            if ($scopeRecords.Count -lt 2) { continue }
            $occurrences = @{}
            foreach ($record in $scopeRecords) {
                $seen = @{}
                $file = Join-Path $repoRoot $record.output_path
                if (-not (Test-Path $file)) { continue }
                try {
                    $items = Get-Content -Raw $file | ConvertFrom-Json
                    foreach ($pkg in @($items)) {
                        foreach ($fn in @($pkg.Funcs)) {
                            $identity = "$($pkg.Path)|$($fn.Name)|$($fn.Position.File)"
                            $seen[$identity] = [ordered]@{ package = $pkg.Path; name = $fn.Name; file = $fn.Position.File; line = $fn.Position.Line }
                        }
                    }
                } catch { }
                foreach ($identity in $seen.Keys) {
                    if (-not $occurrences.ContainsKey($identity)) { $occurrences[$identity] = @() }
                    $occurrences[$identity] += [pscustomobject]@{ record = $record.name; finding = $seen[$identity] }
                }
            }
            foreach ($identity in $occurrences.Keys) {
                $matches = @($occurrences[$identity])
                if ($matches.Count -eq $scopeRecords.Count) {
                    $finding = $matches[0].finding
                    $summary += [ordered]@{
                        package = $finding.package
                        name = $finding.name
                        file = $finding.file
                        line = $finding.line
                        scope = $scope
                        report_count = $matches.Count
                        sources = @($matches | ForEach-Object { $_.record })
                    }
                }
            }
        }
        if ($summary.Count -eq 0) {
            $summary = @()
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
