package sheinlogin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var profileTrimDirs = []string{
	filepath.Join("Default", "Cache"),
	filepath.Join("Default", "Code Cache"),
	filepath.Join("Default", "GPUCache"),
	filepath.Join("Default", "ShaderCache"),
	filepath.Join("Default", "Service Worker", "CacheStorage"),
	filepath.Join("Default", "Session Storage"),
	"Crashpad",
	"GrShaderCache",
	"GraphiteDawnCache",
}

var profileLockFiles = []string{
	"SingletonLock",
	"SingletonCookie",
	"SingletonSocket",
}

func resolveProfileDir(root string, tenantID, storeID int64) (string, error) {
	profileDir := filepath.Join(strings.TrimSpace(root), fmt.Sprintf("%d", tenantID), fmt.Sprintf("%d", storeID))
	if !filepath.IsAbs(profileDir) {
		absDir, absErr := filepath.Abs(profileDir)
		if absErr != nil {
			return "", absErr
		}
		profileDir = absDir
	}
	return profileDir, nil
}

func isProfileInUseError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "profile appears to be in use") ||
		strings.Contains(message, "processsingleton") ||
		strings.Contains(message, "profile directory") ||
		strings.Contains(message, "singletonlock")
}

func clearProfileLockFiles(profileDir string) bool {
	cleared := false
	for _, name := range profileLockFiles {
		path := filepath.Join(profileDir, name)
		if err := os.Remove(path); err == nil || os.IsNotExist(err) {
			if err == nil {
				cleared = true
			}
			continue
		}
	}
	return cleared
}

func terminateProfileBrowserProcesses(profileDir string) int {
	profileText := strings.ToLower(strings.TrimSpace(profileDir))
	if profileText == "" {
		return 0
	}
	if runtime.GOOS == "windows" {
		return terminateWindowsProfileProcesses(profileText)
	}
	return terminatePosixProfileProcesses(profileText)
}

func terminateWindowsProfileProcesses(profileText string) int {
	script := strings.ReplaceAll(`
$profile = '__PROFILE__'
$matches = Get-CimInstance Win32_Process |
  Where-Object {
    $_.CommandLine -and
    ($_.Name -match 'chrome|chromium') -and
    ($_.CommandLine.ToLowerInvariant().Contains($profile))
  }
$count = 0
foreach ($process in $matches) {
  try {
    Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
    $count += 1
  } catch {}
}
Write-Output $count
`, "__PROFILE__", strings.ReplaceAll(profileText, "'", "''"))
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return 0
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	var count int
	_, _ = fmt.Sscanf(last, "%d", &count)
	return count
}

func terminatePosixProfileProcesses(profileText string) int {
	cmd := exec.Command("ps", "-eo", "pid=,args=")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	count := 0
	currentPID := os.Getpid()
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pidText, command, found := strings.Cut(line, " ")
		if !found {
			continue
		}
		commandLower := strings.ToLower(command)
		if !strings.Contains(commandLower, profileText) || (!strings.Contains(commandLower, "chrome") && !strings.Contains(commandLower, "chromium")) {
			continue
		}
		var pid int
		if _, err := fmt.Sscanf(pidText, "%d", &pid); err != nil || pid == currentPID {
			continue
		}
		process, procErr := os.FindProcess(pid)
		if procErr != nil {
			continue
		}
		if killErr := process.Kill(); killErr == nil {
			count++
		}
	}
	return count
}

func trimProfileDir(profileDir string) int {
	removed := 0
	for _, relativeDir := range profileTrimDirs {
		target := filepath.Join(profileDir, relativeDir)
		if _, err := os.Stat(target); err != nil {
			continue
		}
		if err := os.RemoveAll(target); err == nil {
			removed++
		}
	}
	return removed
}
