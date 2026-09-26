package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const accountComposeStateJQHelper = "ACCOUNT_COMPOSE_STATE_JQ_HELPER"

func TestAccountComposeRetainedStateBackfillRuntime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell is required to execute the account-compose OpenTofu init script")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("POSIX shell is required to execute the account-compose OpenTofu init script")
	}

	t.Run("extracts only the managed root operator ID", func(t *testing.T) {
		const expectedID = "bootstrap-user-123"
		state := retainedStateFixture(map[string]any{
			"resources": []any{
				map[string]any{"mode": "managed", "type": "zitadel_human_user", "name": "operator", "instances": []any{map[string]any{"attributes": map[string]any{"id": expectedID, "email": "private@example.test"}}}},
				map[string]any{"mode": "managed", "type": "zitadel_human_user", "name": "viewer", "instances": []any{map[string]any{"attributes": map[string]any{"id": "other-user-secret"}}}},
				map[string]any{"mode": "data", "type": "zitadel_human_user", "name": "operator", "instances": []any{map[string]any{"attributes": map[string]any{"id": "data-source-secret"}}}},
				map[string]any{"mode": "managed", "type": "zitadel_human_user", "name": "operator", "module": "module.accounts", "instances": []any{map[string]any{"attributes": map[string]any{"id": "module-user-secret"}}}},
				map[string]any{"mode": "managed", "type": "zitadel_human_user", "name": "operator", "instances": []any{map[string]any{"index_key": 0, "attributes": map[string]any{"id": "indexed-user-secret"}}}},
				map[string]any{"mode": "managed", "type": "zitadel_human_user", "name": "operator", "instances": []any{map[string]any{"deposed_key": "old-instance", "attributes": map[string]any{"id": "deposed-user-secret"}}}},
			},
		})
		stdout, stderr, runtimeDir := runAccountComposeRetainedStateBackfill(t, sh, state, "")
		got, err := os.ReadFile(filepath.Join(runtimeDir, "bootstrap-user-id"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != expectedID+"\n" {
			t.Fatal("runtime bootstrap identity does not match the unique operator ID")
		}
		info, err := os.Stat(filepath.Join(runtimeDir, "bootstrap-user-id"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("runtime bootstrap identity permissions = %o, want 600", info.Mode().Perm())
		}
		for _, output := range []string{stdout, stderr} {
			for _, secret := range []string{expectedID, "other-user-secret", "data-source-secret", "module-user-secret", "indexed-user-secret", "deposed-user-secret", "private@example.test"} {
				if strings.Contains(output, secret) {
					t.Fatalf("script output leaked retained state value %q", secret)
				}
			}
		}
	})

	for _, tc := range []struct {
		name  string
		state map[string]any
	}{
		{name: "missing operator", state: retainedStateFixture(map[string]any{"resources": []any{}})},
		{name: "missing ID", state: retainedStateFixture(map[string]any{"resources": []any{operatorResource(map[string]any{})}})},
		{name: "empty ID", state: retainedStateFixture(map[string]any{"resources": []any{operatorResource(map[string]any{"id": ""})}})},
		{name: "invalid ID", state: retainedStateFixture(map[string]any{"resources": []any{operatorResource(map[string]any{"id": "bad\nidentity"})}})},
		{name: "ambiguous operator instances", state: retainedStateFixture(map[string]any{"resources": []any{operatorResource(map[string]any{"id": "operator-one"}), operatorResource(map[string]any{"id": "operator-two"})}})},
		{name: "indexed operator", state: retainedStateFixture(map[string]any{"resources": []any{map[string]any{"mode": "managed", "type": "zitadel_human_user", "name": "operator", "instances": []any{map[string]any{"index_key": 0, "attributes": map[string]any{"id": "indexed-user"}}}}}})},
	} {
		t.Run("fails closed: "+tc.name, func(t *testing.T) {
			stdout, stderr, runtimeDir := runAccountComposeRetainedStateBackfill(t, sh, tc.state, "")
			if stdout != "" {
				t.Fatalf("failed backfill stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "could not recover bootstrap user identity") {
				t.Fatalf("failed backfill stderr = %q, want generic recovery error", stderr)
			}
			if _, err := os.Stat(filepath.Join(runtimeDir, "bootstrap-user-id")); !os.IsNotExist(err) {
				t.Fatalf("failed backfill left runtime identity file behind, stat err=%v", err)
			}
			if _, err := os.Stat(filepath.Join(runtimeDir, "bootstrap-user-id.tmp")); !os.IsNotExist(err) {
				t.Fatalf("failed backfill left temporary identity file behind, stat err=%v", err)
			}
		})
	}

	t.Run("invalid JSON fails closed without exposing parser diagnostics", func(t *testing.T) {
		stdout, stderr, runtimeDir := runAccountComposeRetainedStateBackfill(t, sh, nil, `{"secret":"sensitive-state-value"`)
		if stdout != "" || !strings.Contains(stderr, "could not recover bootstrap user identity") {
			t.Fatalf("invalid state output = stdout %q, stderr %q", stdout, stderr)
		}
		if strings.Contains(stderr, "sensitive-state-value") {
			t.Fatal("invalid state parser diagnostic leaked state content")
		}
		if _, err := os.Stat(filepath.Join(runtimeDir, "bootstrap-user-id")); !os.IsNotExist(err) {
			t.Fatalf("invalid state left runtime identity file behind, stat err=%v", err)
		}
	})
}

func TestAccountComposeStateJQMockHelper(t *testing.T) {
	if os.Getenv(accountComposeStateJQHelper) != "1" {
		return
	}
	args := os.Args
	separator := -1
	for i, arg := range args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 || len(args) <= separator+2 {
		os.Exit(2)
	}
	filter := args[separator+2]
	for _, required := range []string{`.resources`, `"managed"`, `"zitadel_human_user"`, `"operator"`, `.attributes.id`, `index_key`, `deposed_key`} {
		if !strings.Contains(filter, required) {
			os.Exit(2)
		}
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	var document struct {
		Resources []struct {
			Mode      string `json:"mode"`
			Type      string `json:"type"`
			Name      string `json:"name"`
			Module    string `json:"module"`
			Instances []struct {
				IndexKey   json.RawMessage `json:"index_key"`
				DeposedKey string          `json:"deposed_key"`
				Attributes struct {
					ID any `json:"id"`
				} `json:"attributes"`
			} `json:"instances"`
		} `json:"resources"`
	}
	if json.Unmarshal(input, &document) != nil {
		fmt.Fprintln(os.Stderr, "private parser failure: sensitive-state-value")
		os.Exit(2)
	}
	ids := make([]string, 0, 1)
	for _, resource := range document.Resources {
		if resource.Mode != "managed" || resource.Type != "zitadel_human_user" || resource.Name != "operator" || resource.Module != "" {
			continue
		}
		for _, instance := range resource.Instances {
			if len(instance.IndexKey) != 0 && string(instance.IndexKey) != "null" || instance.DeposedKey != "" {
				continue
			}
			id, ok := instance.Attributes.ID.(string)
			if !ok || len(id) == 0 || len(id) > 256 || !validBootstrapStateID(id) {
				fmt.Fprintln(os.Stderr, "private parser failure: sensitive-state-value")
				os.Exit(2)
			}
			ids = append(ids, id)
		}
	}
	if len(ids) != 1 {
		fmt.Fprintln(os.Stderr, "private parser failure: sensitive-state-value")
		os.Exit(2)
	}
	_, _ = fmt.Fprintln(os.Stdout, ids[0])
	os.Exit(0)
}

func runAccountComposeRetainedStateBackfill(t *testing.T, sh string, state map[string]any, rawState string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runtimeDir := filepath.Join(root, "runtime")
	frontendDir := filepath.Join(root, "frontend")
	trustedCA := filepath.Join(root, "trusted-ca")
	inputs := filepath.Join(root, "inputs")
	patDir := filepath.Join(root, "pat")
	for _, dir := range []string{stateDir, runtimeDir, frontendDir, filepath.Join(trustedCA), inputs, patDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{filepath.Join(trustedCA, "root-ca.pem"), filepath.Join(inputs, "operator-password"), filepath.Join(inputs, "viewer-password"), filepath.Join(inputs, "insufficient-password"), filepath.Join(patDir, "iam-owner.pat")} {
		if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(stateDir, ".terraform-complete"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(stateDir, "terraform.tfstate")
	contents := []byte(rawState)
	if rawState == "" {
		var err error
		contents, err = json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(statePath, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tofuLog := filepath.Join(root, "tofu-called")
	writeAccountComposeExecutable(t, filepath.Join(binDir, "tofu"), "#!/bin/sh\nprintf 'called' > "+shellQuote(tofuLog)+"\nexit 97\n")
	writeAccountComposeExecutable(t, filepath.Join(binDir, "jq"), "#!/bin/sh\nstate_file=\nfor arg do state_file=$arg; done\nexec "+shellQuote(os.Args[0])+" -test.run=^TestAccountComposeStateJQMockHelper$ -- \"$@\" < \"$state_file\"\n")

	script, err := os.ReadFile(filepath.Join("..", "deployments", "docker", "account-compose", "tofu-init.sh"))
	if err != nil {
		t.Fatal(err)
	}
	replacer := strings.NewReplacer(
		"state=/state", "state="+shellQuote(stateDir),
		"terraform_source=/terraform-source", "terraform_source="+shellQuote(filepath.Join(root, "terraform-source")),
		"trusted_ca=/private/ca", "trusted_ca="+shellQuote(trustedCA),
		"tofu_inputs=/tofu-inputs", "tofu_inputs="+shellQuote(inputs),
		"runtime=/runtime", "runtime="+shellQuote(runtimeDir),
		"frontend=/frontend", "frontend="+shellQuote(frontendDir),
	)
	scriptPath := filepath.Join(root, "tofu-init.sh")
	if err := os.WriteFile(scriptPath, []byte(replacer.Replace(string(script))), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(sh, scriptPath)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"BOOTSTRAP_PAT_FILE="+filepath.Join(patDir, "iam-owner.pat"),
		"ACCOUNT_IDENTITY_PORT=8443",
		"ACCOUNT_APPLICATION_PORT=8080",
		accountComposeStateJQHelper+"=1",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), "", runtimeDir
	}
	if _, tofuErr := os.Stat(tofuLog); tofuErr == nil {
		t.Fatalf("completed-state backfill invoked tofu, output=%s", output)
	}
	return "", string(output), runtimeDir
}

func retainedStateFixture(fields map[string]any) map[string]any {
	state := map[string]any{"version": 4, "terraform_version": "1.12.6", "serial": 7, "lineage": "fixture"}
	for key, value := range fields {
		state[key] = value
	}
	return state
}

func operatorResource(attributes map[string]any) map[string]any {
	return map[string]any{
		"mode": "managed", "type": "zitadel_human_user", "name": "operator",
		"instances": []any{map[string]any{"attributes": attributes}},
	}
}

func validBootstrapStateID(value string) bool {
	for i, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || (i > 0 && strings.ContainsRune("-_.:", r)) || (i > 0 && r == '_') {
			continue
		}
		return false
	}
	return value != ""
}

func writeAccountComposeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
