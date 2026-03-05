# Panic Usage Auto-Fix Script
# Automatically fixes improper panic usage in the project

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Blue
Write-Host "   Panic Usage Auto-Fix Tool" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""

# Backup directory
$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$backupDir = "backups/panic_fix_$timestamp"
New-Item -ItemType Directory -Path $backupDir -Force | Out-Null

Write-Host "Creating backup to: $backupDir" -ForegroundColor Yellow
Write-Host ""

# Fix counter
$fixedCount = 0

# 1. Fix ValidateOrPanic definition
Write-Host "[1] Fixing ValidateOrPanic method..." -ForegroundColor Yellow
$validatorFile = "internal/core/config/validator.go"

if (Test-Path $validatorFile) {
    # Backup original file
    Copy-Item $validatorFile -Destination $backupDir
    
    # Check if fix is needed
    if (Select-String -Path $validatorFile -Pattern "ValidateOrPanic" -Quiet) {
        Write-Host "Info: Modifying $validatorFile" -ForegroundColor Blue
        Write-Host "   - Mark ValidateOrPanic as deprecated"
        Write-Host "   - Add ValidateWithError method"
        
        Write-Host "! Manual fix required for this file" -ForegroundColor Yellow
        Write-Host ""
    }
}

# 2. Find and list files that need fixing
Write-Host "[2] Scanning files that need fixing..." -ForegroundColor Yellow

# Find files using ValidateOrPanic
$filesWithValidate = Get-ChildItem -Path . -Recurse -Include *.go | 
    Select-String -Pattern "\.ValidateOrPanic\(\)" | 
    Select-Object -ExpandProperty Path -Unique

if ($filesWithValidate) {
    Write-Host "Files with ValidateOrPanic calls:" -ForegroundColor Red
    $filesWithValidate | ForEach-Object { Write-Host "  $_" }
    Write-Host ""
}

# Find files using panic (exclude legitimate uses)
$filesWithPanic = Get-ChildItem -Path . -Recurse -Include *.go -Exclude *_test.go | 
    Select-String -Pattern "panic\(" | 
    Select-Object -ExpandProperty Path -Unique | 
    Where-Object { 
        $_ -notmatch "internal[\\/]core[\\/]errors[\\/]errors\.go" -and 
        $_ -notmatch "internal[\\/]core[\\/]config[\\/]validator\.go"
    }

if ($filesWithPanic) {
    Write-Host "Files that may need panic usage review:" -ForegroundColor Red
    $filesWithPanic | ForEach-Object { Write-Host "  $_" }
    Write-Host ""
}

# 3. Generate fix template
Write-Host "[3] Generating fix template..." -ForegroundColor Yellow

$fixTemplate = @"
# Panic Fix Template

## 1. ValidateOrPanic Fix

### Before:
``````go
func Initialize() {
    config.ValidateOrPanic()
    // ...
}
``````

### After:
``````go
func Initialize() error {
    if err := config.Validate(); err != nil {
        return fmt.Errorf("config validation failed: %w", err)
    }
    // ...
    return nil
}
``````

## 2. Runtime Panic Fix

### Before:
``````go
func ProcessTask(task *Task) {
    if task == nil {
        panic("task is nil")
    }
    // ...
}
``````

### After:
``````go
func ProcessTask(task *Task) error {
    if task == nil {
        return errors.New(errors.ErrCodeValidation, "task cannot be nil")
    }
    // ...
    return nil
}
``````

## 3. Goroutine Panic Fix

### Before:
``````go
go func() {
    // Code that may panic
    doSomething()
}()
``````

### After:
``````go
errors.SafeGo(func() {
    // Code that may panic
    doSomething()
}, logger)
``````

## 4. Must/MustValue Usage Check

### Correct Usage (only in main or init):
``````go
func main() {
    config := errors.MustValue(loadConfig())
    // ...
}
``````

### Incorrect Usage (in business code):
``````go
func ProcessData() {
    data := errors.MustValue(fetchData()) // X Don't do this
    // ...
}
``````

### Should be:
``````go
func ProcessData() error {
    data, err := fetchData()
    if err != nil {
        return fmt.Errorf("failed to fetch data: %w", err)
    }
    // ...
    return nil
}
``````
"@

Set-Content -Path "$backupDir/fix_template.md" -Value $fixTemplate
Write-Host "OK Fix template generated: $backupDir/fix_template.md" -ForegroundColor Green
Write-Host ""

# 4. Generate fix checklist
Write-Host "[4] Generating fix checklist..." -ForegroundColor Yellow

$filesWithValidateList = if ($filesWithValidate) { $filesWithValidate -join "`n" } else { "None" }
$filesWithPanicList = if ($filesWithPanic) { $filesWithPanic -join "`n" } else { "None" }

$fixChecklist = @"
# Panic Fix Checklist

Generated: $(Get-Date)

## Files to Fix

### ValidateOrPanic Calls
$filesWithValidateList

### Possible Panic Usage
$filesWithPanicList

## Fix Steps

1. [ ] Backup completed (location: $backupDir)
2. [ ] Modify internal/core/config/validator.go
   - [ ] Mark ValidateOrPanic as deprecated
   - [ ] Add ValidateWithError method
3. [ ] Fix all ValidateOrPanic calls
4. [ ] Check and fix runtime panic
5. [ ] Add panic recovery for goroutines
6. [ ] Run tests to verify fixes
7. [ ] Run check_panic_usage.ps1 to verify

## Verification Commands

``````powershell
# Compile check
go build ./...

# Run tests
go test ./...

# Check panic usage
./scripts/check_panic_usage.ps1
``````
"@

Set-Content -Path "$backupDir/fix_checklist.md" -Value $fixChecklist
Write-Host "OK Fix checklist generated: $backupDir/fix_checklist.md" -ForegroundColor Green
Write-Host ""

# 5. Summary
Write-Host "========================================" -ForegroundColor Blue
Write-Host "   Fix Preparation Complete" -ForegroundColor Blue
Write-Host "========================================" -ForegroundColor Blue
Write-Host ""
Write-Host "OK Backup directory: $backupDir" -ForegroundColor Green
Write-Host "OK Fix template: $backupDir/fix_template.md" -ForegroundColor Green
Write-Host "OK Fix checklist: $backupDir/fix_checklist.md" -ForegroundColor Green
Write-Host ""
Write-Host "Next Steps:" -ForegroundColor Yellow
Write-Host "   1. View fix template: cat $backupDir/fix_template.md"
Write-Host "   2. View fix checklist: cat $backupDir/fix_checklist.md"
Write-Host "   3. Manually fix files (refer to template)"
Write-Host "   4. Run check script: ./scripts/check_panic_usage.ps1"
Write-Host ""
