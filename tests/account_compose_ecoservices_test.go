package tests

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEcoservicesRetainedInstallerRequiresOriginalManifestsAndPrivateKeys(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer execution")
	}
	for _, variant := range []string{"retained", "changed source", "changed runtime", "missing root", "missing access key", "missing runtime key"} {
		t.Run(variant, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"base", "state", "runtime", "object-root", "object-runtime", "bin"} {
				if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			base := []byte(`{"productCollections":true,"dataServices":{}}`)
			manifest := []byte(`{"ecoservices":{"enabled":true,"nonPaymentOnly":true}}`)
			files := map[string][]byte{
				"base/current-application.json": base, "runtime/current-application.json": manifest,
				"state/.complete":               []byte("ecoservices-qualification-v1"),
				"state/source-manifest.sha256":  []byte(fmt.Sprintf("%x", sha256.Sum256(base))),
				"state/runtime-manifest.sha256": []byte(fmt.Sprintf("%x", sha256.Sum256(manifest))),
				"object-root/root-password":     []byte("fixture"), "object-runtime/access-key": []byte("fixture"), "object-runtime/secret-key": []byte("fixture"),
			}
			switch variant {
			case "changed source":
				files["base/current-application.json"] = []byte(`{"changed":true}`)
			case "changed runtime":
				files["runtime/current-application.json"] = []byte(`{"changed":true}`)
			case "missing root":
				delete(files, "object-root/root-password")
			case "missing access key":
				delete(files, "object-runtime/access-key")
			case "missing runtime key":
				delete(files, "object-runtime/secret-key")
			}
			for name, data := range files {
				if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			mutation := filepath.Join(root, "unexpected-provision")
			for _, name := range []string{"psql", "openssl", "ecoservices-schema-init"} {
				writeAccountComposeExecutable(t, filepath.Join(root, "bin", name), "#!/bin/sh\nprintf called > "+shellQuote(mutation)+"\nexit 97\n")
			}
			input, err := os.ReadFile(filepath.Join("..", "deployments", "docker", "account-compose", "ecoservices-init.sh"))
			if err != nil {
				t.Fatal(err)
			}
			var pairs []string
			for _, dir := range []string{"object-runtime", "object-root", "base", "state", "runtime"} {
				pairs = append(pairs, "/"+dir+"/", filepath.Join(root, dir)+"/")
			}
			script := filepath.Join(root, "installer.sh")
			if err := os.WriteFile(script, []byte(strings.NewReplacer(pairs...).Replace(string(input))), 0700); err != nil {
				t.Fatal(err)
			}
			command := exec.Command("sh", script)
			command.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := command.CombinedOutput()
			if (err == nil) != (variant == "retained") {
				t.Fatalf("retained guard failed: %v %s", err, out)
			}
			if _, err := os.Stat(mutation); err == nil {
				t.Fatal("retained branch attempted provisioning")
			}
		})
	}
}
