package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingDSNAndInstallerFailureDoNotLeakSecrets(t *testing.T) {
	calls := 0
	install := func(context.Context, string) error { calls++; return errors.New("password=private-secret") }
	var output bytes.Buffer
	if code := run(nil, &output, install); code == 0 || calls != 0 {
		t.Fatal("missing DSN must fail without dispatch")
	}
	path := filepath.Join(t.TempDir(), "dsn with spaces.txt")
	if err := os.WriteFile(path, []byte("private-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-dsn-file", path}, &output, install); code == 0 || calls != 1 {
		t.Fatal("installer failure must return nonzero")
	}
	if bytes.Contains(output.Bytes(), []byte("private-secret")) {
		t.Fatal("secret leaked")
	}
}

func TestPowerShellWrapperPreservesArgumentsAndFailure(t *testing.T) {
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("PowerShell is unavailable")
	}
	script, err := filepath.Abs("../../scripts/referral-schema-init.ps1")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	dsn := filepath.Join(dir, "dsn with spaces.txt")
	capture := filepath.Join(dir, "arguments.json")
	if err = os.WriteFile(dsn, []byte("private-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"0", "17"} {
		cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command", `function global:go { [IO.File]::WriteAllText($env:REFERRAL_CAPTURE, ($args | ConvertTo-Json -Compress)); $global:LASTEXITCODE = [int]$env:REFERRAL_STATUS }; & $env:REFERRAL_SCRIPT -DsnFile $env:REFERRAL_DSN -TimeoutSeconds 2`)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "REFERRAL_SCRIPT="+script, "REFERRAL_DSN="+dsn, "REFERRAL_CAPTURE="+capture, "REFERRAL_STATUS="+status)
		output, e := cmd.CombinedOutput()
		if (e == nil) != (status == "0") {
			t.Fatalf("wrong wrapper exit: %v", e)
		}
		if strings.Contains(string(output), "private-secret") {
			t.Fatal("wrapper leaked DSN")
		}
		data, e := os.ReadFile(capture)
		if e != nil {
			t.Fatal(e)
		}
		var args []string
		if json.Unmarshal(data, &args) != nil || len(args) != 6 || args[3] != dsn || args[5] != "2s" {
			t.Fatalf("argument forwarding %s", data)
		}
	}
}
func TestDSNFileAndDeadlineForwarded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dsn with spaces.txt")
	if err := os.WriteFile(path, []byte("explicit-dsn\n"), 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	var output bytes.Buffer
	code := run([]string{"-dsn-file", path, "-timeout", "2s"}, &output, func(ctx context.Context, dsn string) error {
		called = true
		if dsn != "explicit-dsn" {
			t.Error("wrong DSN")
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("missing deadline")
		}
		return nil
	})
	if code != 0 || !called {
		t.Fatal("explicit target not forwarded")
	}
}
