//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"golang.org/x/sys/windows"
)

var windowsSIDPattern = regexp.MustCompile(`S-\d-(?:\d+-)+\d+`)
var windowsACEIdentityPattern = regexp.MustCompile(`;;;([^\)]+)\)`)

func protectPrivateFile(path string) error {
	whoamiOutput, err := exec.Command("whoami.exe", "/user", "/fo", "csv", "/nh").Output()
	if err != nil {
		return fmt.Errorf("resolve current Windows SID: %w", err)
	}
	currentSID := windowsSIDPattern.FindString(string(whoamiOutput))
	if currentSID == "" {
		return errors.New("resolve current Windows SID: SID was not returned")
	}
	if output, err := exec.Command("icacls.exe", path, "/inheritance:r", "/grant:r",
		"*"+currentSID+":(F)", "*S-1-5-18:(F)", "*S-1-5-32-544:(F)").CombinedOutput(); err != nil {
		return fmt.Errorf("protect Windows runtime file ACL: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	command := exec.Command("pwsh.exe", "-NoProfile", "-NonInteractive", "-Command",
		"[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); (Get-Acl -LiteralPath $env:LISTINGKIT_ACL_VERIFY_PATH).Sddl")
	command.Env = append(os.Environ(), "LISTINGKIT_ACL_VERIFY_PATH="+path)
	sddlOutput, err := command.Output()
	if err != nil {
		return fmt.Errorf("verify Windows runtime file ACL: %w", err)
	}
	sddl := strings.TrimSpace(string(sddlOutput))
	if !strings.Contains(sddl, "D:P") {
		return errors.New("Windows runtime file ACL inheritance is not disabled")
	}
	allowed := map[string]bool{
		currentSID: true, "SY": true, "BA": true,
		"S-1-5-18": true, "S-1-5-32-544": true,
	}
	identities := windowsACEIdentityPattern.FindAllStringSubmatch(sddl, -1)
	if len(identities) == 0 {
		return errors.New("Windows runtime file ACL does not contain explicit principals")
	}
	for _, identity := range identities {
		if !allowed[identity[1]] {
			return errors.New("Windows runtime file ACL contains an unexpected principal")
		}
	}
	return nil
}

func replacePrivateFile(source string, target string) error {
	sourcePointer, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return fmt.Errorf("encode private temporary file path: %w", err)
	}
	targetPointer, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return fmt.Errorf("encode private runtime file path: %w", err)
	}
	return windows.MoveFileEx(
		sourcePointer,
		targetPointer,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}

func isUnsafePathComponent(path string, info os.FileInfo) (bool, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		return true, nil
	}
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, fmt.Errorf("encode runtime file path: %w", err)
	}
	attributes, err := windows.GetFileAttributes(pathPointer)
	if err != nil {
		return false, fmt.Errorf("read runtime file attributes: %w", err)
	}
	return attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}
