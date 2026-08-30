$scriptPath = Join-Path $PSScriptRoot "image-agent-local-acceptance.ps1"

function Import-AcceptanceFunction {
    param([string]$Name)

    $tokens = $null
    $errors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) {
        throw "Unable to parse ${scriptPath}: $($errors[0].Message)"
    }
    $function = $ast.FindAll({
        param($node)
            $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $Name
    }, $true) | Select-Object -First 1
    if ($null -eq $function) {
        throw "${Name} was not found in ${scriptPath}"
    }
    $definition = $function.Extent.Text -replace "^function\s+$([regex]::Escape($Name))", "function global:${Name}"
    Invoke-Expression $definition
}

Describe "image-agent-local-acceptance seed routing" {
    BeforeEach {
        Import-AcceptanceFunction -Name "Invoke-Seed"
        $global:SourceUrl = "https://example.com/source.png"
        $global:StyleUrl = ""
        $global:TokenFile = Join-Path $TestDrive "token.txt"
        $global:runtimeFile = Join-Path $TestDrive "runtime.env"
        Set-Content -LiteralPath $global:TokenFile -Value "test-token"
        $global:capturedGoArguments = @()
        function global:Assert-LocalSourceUrl { param([string]$Url, [string]$Name) }
        function global:Invoke-GoCommand { param([string[]]$Arguments) $global:capturedGoArguments = $Arguments }
    }

    AfterEach {
        Remove-Item Function:\global:Invoke-Seed,Function:\global:Assert-LocalSourceUrl,Function:\global:Invoke-GoCommand -ErrorAction SilentlyContinue
        Remove-Variable SourceUrl,StyleUrl,TokenFile,runtimeFile,capturedGoArguments -Scope Global -ErrorAction SilentlyContinue
    }

    It "invokes the application-owned Image Agent seed command" {
        Invoke-Seed

        $global:capturedGoArguments.Count | Should Be 8
        $global:capturedGoArguments[0] | Should Be "run"
        $global:capturedGoArguments[1] | Should Be "./internal/app/runtime/imageagentacceptance/cmd"
        $global:capturedGoArguments[2] | Should Be "-runtime-file"
        $global:capturedGoArguments[3] | Should Be $global:runtimeFile
        $global:capturedGoArguments[4] | Should Be "-token-file"
        $global:capturedGoArguments[5] | Should Be $global:TokenFile
        $global:capturedGoArguments[6] | Should Be "-source-url"
        $global:capturedGoArguments[7] | Should Be $global:SourceUrl
    }

    It "appends the optional style URL without changing required arguments" {
        $global:StyleUrl = "https://example.com/style.png"

        Invoke-Seed

        $global:capturedGoArguments.Count | Should Be 10
        $global:capturedGoArguments[8] | Should Be "-style-url"
        $global:capturedGoArguments[9] | Should Be $global:StyleUrl
    }
}

Describe "image-agent-local-acceptance authorization startup" {
    BeforeEach {
        Import-AcceptanceFunction -Name "Invoke-Authorize"
        $global:TokenFile = Join-Path $TestDrive "user-token.txt"
        $global:acceptanceRoot = $TestDrive
        $global:runtimeFile = Join-Path $TestDrive "runtime.env"
        Set-Content -LiteralPath $global:TokenFile -Value "test-token"
        $global:authorizationEvents = @()
        $global:apiRequiredReadiness = $false
        function global:Invoke-GoCommand { param([string[]]$Arguments) $global:authorizationEvents += "authorize" }
        function global:Import-ProvisionedRuntime { $global:authorizationEvents += "runtime" }
        function global:Start-LocalApi {
            param([switch]$RequireReadiness)
            $global:apiRequiredReadiness = $RequireReadiness.IsPresent
            $global:authorizationEvents += "api"
        }
        function global:Start-LocalWorker { $global:authorizationEvents += "worker" }
    }

    AfterEach {
        Remove-Item Function:\global:Invoke-Authorize,Function:\global:Invoke-GoCommand,Function:\global:Import-ProvisionedRuntime,Function:\global:Start-LocalApi,Function:\global:Start-LocalWorker -ErrorAction SilentlyContinue
        Remove-Variable TokenFile,acceptanceRoot,runtimeFile,authorizationEvents,apiRequiredReadiness -Scope Global -ErrorAction SilentlyContinue
    }

    It "restarts a ready API with authorized tenant settings before the worker" {
        Invoke-Authorize

        $global:authorizationEvents -join "," | Should Be "authorize,runtime,api,worker"
        $global:apiRequiredReadiness | Should Be $true
    }
}
