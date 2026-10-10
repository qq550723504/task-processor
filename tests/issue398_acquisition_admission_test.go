package tests

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const issue398Database = "task-processor/internal/platform/database"
const issue398HTTP = "task-processor/internal/integration/httpimage"
const issue605SourceStorage = "task-processor/internal/integration/s3"

func issue398CurrentLeafImporter(importer, target string) bool {
	if importer == "task-processor/internal/app/productsourcing" && target == issue398Database {
		return true
	}
	if importer == "task-processor/internal/app/runtime/currentapplication" && target == issue605SourceStorage {
		return true
	}
	if target != issue398HTTP {
		return false
	}
	switch importer {
	case "task-processor/internal/integration/acquisition/a1688", "task-processor/internal/app/productsourcing", "task-processor/internal/app/supplychain", "task-processor/internal/integration/shein", "task-processor/internal/app/runtime/currentapplication", "task-processor/internal/app/pod", "task-processor/internal/integration/sds":
		return true
	}
	return false
}

func issue398AcquisitionAPIViolations(sources []listingKitImageBoundarySource) ([]string, error) {
	// This current-edge register does not exempt directories from the legacy
	// scans. Parse all tracked production text, including other OS/build tags.
	rules := []struct {
		root, target string
		files        map[string][]string
	}{
		{"internal/app/productsourcing", issue398Database, map[string][]string{"internal/app/productsourcing/acquisition_initialization.go": {"OpenExistingWritableContext", "Close", "Config"}}},
		{"internal/integration/acquisition/a1688", issue398HTTP, map[string][]string{"internal/integration/acquisition/a1688/public.go": {"NewPublicImageHTTPClient"}}},
		{"cmd/product-acquisition-init", "task-processor/internal/app/productsourcing", map[string][]string{"cmd/product-acquisition-init/main.go": {"InitializeAcquisitionDatabase"}}},
		// #399/#561 constructor edges; #605 adds only current Collection publication.
		// #612 frozen full-image design adds only its explicitly authorized read-only receipt constructor.
		{"internal/app/httpapi/product_acquisition_application.go", "task-processor/internal/app/productsourcing", map[string][]string{"internal/app/httpapi/product_acquisition_application.go": {"NewPublicAcquisition", "NewBrowserAcquisition", "NewBrowserPublicAcquisition", "PublishedAcquisition", "NewPublishedAcquisitionReader", "NewScopedPublishedAcquisitionReader", "NewAcquisitionChargeOwner", "AcquisitionPublicationOption", "WithCollectionPublication"}}},
		// #605 frozen CURRENT helpers: exact files/APIs, all source text, ceiling=8.
		{"internal/app/productsourcing", issue398HTTP, map[string][]string{
			"internal/app/productsourcing/source_media.go": {"InspectGeneratedArtifact", "ValidatePublicHTTPSURL"},
		}},
		{"internal/app/runtime/currentapplication", issue605SourceStorage, map[string][]string{"internal/app/runtime/currentapplication/source_media_storage.go": {"NewClient", "ClientConfig", "NewUploaderWithOptions", "UploaderOptions", "AdaptLogrus", "ArtifactStorageCapabilities", "ArtifactStorageMode"}}},
		{"internal/app/runtime/currentapplication", issue398HTTP, map[string][]string{"internal/app/runtime/currentapplication/source_media_storage.go": {"ValidatePublicHTTPSURL"}}},
		{"internal/app/supplychain", issue398HTTP, map[string][]string{"internal/app/supplychain/image_probe.go": {"NewPublicImageHTTPClient", "ValidatePublicHTTPSURL", "Download", "InspectGeneratedArtifact"}}},
		{"internal/integration/shein", issue398HTTP, map[string][]string{"internal/integration/shein/official_goods.go": {"ValidatePublicHTTPSURL"}}},
		// #622 frozen design reuses these current SSRF-safe and pure image leaves.
		// Historical ceiling remains eight; all OS/build-tag text and sibling APIs are still checked.
		{"internal/app/pod", issue398HTTP, map[string][]string{
			"internal/app/pod/application.go": {"NewPublicImageHTTPClient"},
			"internal/app/pod/inputs.go":      {"Download", "InspectGeneratedArtifact"},
		}},
		{"internal/integration/sds", issue398HTTP, map[string][]string{"internal/integration/sds/mutations.go": {"InspectGeneratedArtifact"}}},
		{"internal/app/httpapi/collection_source_media.go", "task-processor/internal/app/productsourcing", map[string][]string{"internal/app/httpapi/collection_source_media.go": {"SourceMedia", "SourceMediaStorage"}}},
		{"internal/app/httpapi/product_collection_application.go", "task-processor/internal/app/productsourcing", map[string][]string{"internal/app/httpapi/product_collection_application.go": {"NewOwnProductWriter", "NewPublishedAcquisitionReader"}}},
		{"internal/app/httpapi/supply_chain_agents.go", "task-processor/internal/app/productsourcing", map[string][]string{"internal/app/httpapi/supply_chain_agents.go": {"NewExecutionPublicationGateway", "NewTransactionReader"}}},
		{"internal/app/runtime/currentapplication", "task-processor/internal/app/productsourcing", map[string][]string{"internal/app/runtime/currentapplication/source_media_storage.go": {"SourceMediaStorage"}, "internal/app/runtime/currentapplication/runtime.go": {"SourceMediaStorage"}}},
	}
	var violations []string
	for _, source := range sources {
		if !issue30ProductionGoSource(source.path) {
			continue
		}
		path := filepath.ToSlash(source.path)
		for _, rule := range rules {
			if path != rule.root && !strings.HasPrefix(path, rule.root+"/") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, source.text, 0)
			if err != nil {
				return nil, err
			}
			for _, imp := range file.Imports {
				target, err := decodeGoImportPath(imp.Path.Value)
				if err != nil {
					return nil, err
				}
				if !importMatchesPrefix(target, rule.target) {
					continue
				}
				apis, admitted := rule.files[path]
				if !admitted || target != rule.target {
					violations = append(violations, path+" has an unadmitted current import: "+target)
					continue
				}
				alias := filepath.Base(target)
				if imp.Name != nil {
					alias = imp.Name.Name
				}
				if alias == "." || alias == "_" {
					violations = append(violations, path+" obscures an admitted leaf API")
					continue
				}
				ast.Inspect(file, func(node ast.Node) bool {
					selector, ok := node.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					qualifier, ok := selector.X.(*ast.Ident)
					if !ok || qualifier.Name != alias || qualifier.Obj != nil {
						return true
					}
					for _, allowed := range apis {
						if selector.Sel.Name == allowed {
							return true
						}
					}
					violations = append(violations, path+" uses an unadmitted leaf API: "+selector.Sel.Name)
					return true
				})
			}
		}
	}
	return violations, nil
}

func TestIssue398ProducerImportAdmissionRejectsOtherFiles(t *testing.T) {
	root := t.TempDir()
	allowed := map[string]struct{}{}
	for _, name := range []string{"product_acquisition_application.go", "main.go"} {
		path := filepath.Join(root, name)
		allowed[path] = struct{}{}
		require.NoError(t, os.WriteFile(path, []byte(`package fixture; import p "task-processor/internal/app/productsourcing"; var use = p.NewPublicAcquisition`), 0600))
	}
	for _, name := range []string{"other_http.go", "other_command.go", "main.go.fake.go", "consumer_resource_charges.go"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(`package fixture; import alias "task-processor/internal/app/productsourcing"; var use = alias.NewPublicAcquisition`), 0600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "browser_capture_application.go"), []byte(`package fixture; import alias "task-processor/internal/app/productsourcing"; var use = alias.NewBrowserAcquisition`), 0600))
	violations, err := findBannedImportViolations(root, []string{`"task-processor/internal/app/productsourcing"`}, allowed, true)
	require.NoError(t, err)
	require.Len(t, violations, 5)
}

func TestIssue398CurrentLeafEdgesAreExact(t *testing.T) {
	for _, tc := range []struct {
		importer, target string
		want             bool
	}{
		{"task-processor/internal/app/productsourcing", issue398Database, true},
		{"task-processor/internal/integration/acquisition/a1688", issue398HTTP, true},
		{"task-processor/internal/app/productsourcing/child", issue398Database, false},
		{"task-processor/internal/app/productsourcingfake", issue398Database, false},
		{"task-processor/internal/app/productsourcing", issue398Database + "/child", false},
		{"task-processor/internal/app/productsourcing", issue398HTTP, true},
		{"task-processor/internal/integration/acquisition/a1688/child", issue398HTTP, false},
		{"task-processor/internal/integration/acquisition/a1688", issue398Database, false},
		{"task-processor/internal/worker", issue398HTTP, false},
		{"task-processor/internal/app/pod", issue398HTTP, true},
		{"task-processor/internal/app/pod/child", issue398HTTP, false},
		{"task-processor/internal/integration/sds", issue398HTTP, true},
		{"task-processor/internal/integration/sdslegacy", issue398HTTP, false},
	} {
		require.Equal(t, tc.want, issue398CurrentLeafImporter(tc.importer, tc.target), "%s -> %s", tc.importer, tc.target)
	}
}

func TestIssue398CurrentLeafAndInitializerAPIGuard(t *testing.T) {
	for _, tc := range []struct {
		name, path, target, symbol string
		want                       bool
	}{
		{"database open", "internal/app/productsourcing/acquisition_initialization.go", issue398Database, "OpenExistingWritableContext", true},
		{"database close", "internal/app/productsourcing/acquisition_initialization.go", issue398Database, "Close", true},
		{"database config", "internal/app/productsourcing/acquisition_initialization.go", issue398Database, "Config", true},
		{"database wrong API", "internal/app/productsourcing/acquisition_initialization.go", issue398Database, "Open", false},
		{"database shared API", "internal/app/productsourcing/acquisition_initialization.go", issue398Database, "OpenShared", false},
		{"database sibling", "internal/app/productsourcing/other.go", issue398Database, "Close", false},
		{"database nested", "internal/app/productsourcing/child/acquisition_initialization.go", issue398Database, "Close", false},
		{"database target subpackage", "internal/app/productsourcing/acquisition_initialization.go", issue398Database + "/child", "Close", false},
		{"source media validator", "internal/app/runtime/currentapplication/source_media_storage.go", issue398HTTP, "ValidatePublicHTTPSURL", true},
		{"source media legacy provider", "internal/app/runtime/currentapplication/source_media_storage.go", issue398HTTP, "NewProductImageDownloader", false},
		{"source media sibling", "internal/app/productsourcing/other.go", issue398HTTP, "ValidatePublicHTTPSURL", false},
		{"probe current reader", "internal/app/supplychain/image_probe.go", issue398HTTP, "Download", true},
		{"probe sibling", "internal/app/supplychain/other.go", issue398HTTP, "Download", false},
		{"POD exact image reader", "internal/app/pod/inputs.go", issue398HTTP, "Download", true},
		{"POD legacy API denied", "internal/app/pod/inputs.go", issue398HTTP, "NewProductImageDownloader", false},
		{"POD sibling denied", "internal/app/pod/other.go", issue398HTTP, "Download", false},
		{"SDS exact thumbnail inspection", "internal/integration/sds/mutations.go", issue398HTTP, "InspectGeneratedArtifact", true},
		{"SDS sibling denied", "internal/integration/sds/other.go", issue398HTTP, "InspectGeneratedArtifact", false},
		{"official exact validator", "internal/integration/shein/official_goods.go", issue398HTTP, "ValidatePublicHTTPSURL", true},
		{"official sibling", "internal/integration/shein/other.go", issue398HTTP, "ValidatePublicHTTPSURL", false},
		{"storage current constructor", "internal/app/runtime/currentapplication/source_media_storage.go", issue605SourceStorage, "NewUploaderWithOptions", true},
		{"storage sibling", "internal/app/runtime/currentapplication/other.go", issue605SourceStorage, "NewUploaderWithOptions", false},
		{"storage unadmitted API", "internal/app/runtime/currentapplication/source_media_storage.go", issue605SourceStorage, "Upload", false},
		{"runtime storage port", "internal/app/runtime/currentapplication/source_media_storage.go", "task-processor/internal/app/productsourcing", "SourceMediaStorage", true},
		{"runtime producer forbidden", "internal/app/runtime/currentapplication/config.go", "task-processor/internal/app/productsourcing", "NewPublicAcquisition", false},
		{"HTTP leaf", "internal/integration/acquisition/a1688/public.go", issue398HTTP, "NewPublicImageHTTPClient", true},
		{"HTTP wrong API", "internal/integration/acquisition/a1688/public.go", issue398HTTP, "Fetch", false},
		{"HTTP sibling", "internal/integration/acquisition/a1688/other.go", issue398HTTP, "NewPublicImageHTTPClient", false},
		{"HTTP nested", "internal/integration/acquisition/a1688/child/public.go", issue398HTTP, "NewPublicImageHTTPClient", false},
		{"HTTP target subpackage", "internal/integration/acquisition/a1688/public.go", issue398HTTP + "/child", "NewPublicImageHTTPClient", false},
		{"initializer", "cmd/product-acquisition-init/main.go", "task-processor/internal/app/productsourcing", "InitializeAcquisitionDatabase", true},
		{"initializer publish", "cmd/product-acquisition-init/main.go", "task-processor/internal/app/productsourcing", "Publish", false},
		{"initializer build", "cmd/product-acquisition-init/main.go", "task-processor/internal/app/productsourcing", "NewPublicAcquisition", false},
		{"initializer sibling", "cmd/product-acquisition-init/other.go", "task-processor/internal/app/productsourcing", "InitializeAcquisitionDatabase", false},
		{"initializer target subpackage", "cmd/product-acquisition-init/main.go", "task-processor/internal/app/productsourcing/httpapi", "InitializeAcquisitionDatabase", false},
		{"module build", "internal/app/httpapi/product_acquisition_application.go", "task-processor/internal/app/productsourcing", "NewPublicAcquisition", true},
		{"Browser module build", "internal/app/httpapi/product_acquisition_application.go", "task-processor/internal/app/productsourcing", "NewBrowserAcquisition", true},
		{"module cannot publish", "internal/app/httpapi/product_acquisition_application.go", "task-processor/internal/app/productsourcing", "Publish", false},
		{"Browser module target subpackage", "internal/app/httpapi/product_acquisition_application.go", "task-processor/internal/app/productsourcing/httpapi", "NewBrowserAcquisition", false},
		{"initializer cannot build Browser", "cmd/product-acquisition-init/main.go", "task-processor/internal/app/productsourcing", "NewBrowserAcquisition", false},
		{"module exact receipt read", "internal/app/httpapi/product_acquisition_application.go", "task-processor/internal/app/productsourcing", "PublishedAcquisition", true},
		{"module exact published reader", "internal/app/httpapi/product_acquisition_application.go", "task-processor/internal/app/productsourcing", "NewPublishedAcquisitionReader", true},
		{"image module exact scoped receipt reader", "internal/app/httpapi/product_acquisition_application.go", "task-processor/internal/app/productsourcing", "NewScopedPublishedAcquisitionReader", true},
		{"Collection cannot consume image scoped constructor", "internal/app/httpapi/product_collection_application.go", "task-processor/internal/app/productsourcing", "NewScopedPublishedAcquisitionReader", false},
		{"module exact charge proof owner", "internal/app/httpapi/product_acquisition_application.go", "task-processor/internal/app/productsourcing", "NewAcquisitionChargeOwner", true},
		{"module target subpackage", "internal/app/httpapi/product_acquisition_application.go", "task-processor/internal/app/productsourcing/httpapi", "NewPublicAcquisition", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Alias/function-value form also covers tagged and non-host sources.
			body := fmt.Sprintf("//go:build linux && custom\n\npackage fixture; import leaf %q; var use = leaf.%s", tc.target, tc.symbol)
			violations, err := issue398AcquisitionAPIViolations([]listingKitImageBoundarySource{{path: tc.path, text: body}})
			require.NoError(t, err)
			require.Equal(t, tc.want, len(violations) == 0, "%v", violations)
		})
	}
	for _, alias := range []string{".", "_"} {
		body := fmt.Sprintf("package fixture; import %s %q; var use = Open", alias, issue398Database)
		violations, err := issue398AcquisitionAPIViolations([]listingKitImageBoundarySource{{path: "internal/app/productsourcing/acquisition_initialization.go", text: body}})
		require.NoError(t, err)
		require.NotEmpty(t, violations, "dot/blank imports cannot bypass symbol admission")
		body = fmt.Sprintf("package fixture; import %s %q; var use = NewBrowserAcquisition", alias, "task-processor/internal/app/productsourcing")
		violations, err = issue398AcquisitionAPIViolations([]listingKitImageBoundarySource{{path: "internal/app/httpapi/product_acquisition_application.go", text: body}})
		require.NoError(t, err)
		require.NotEmpty(t, violations, "dot/blank imports cannot obscure the Browser constructor")
	}
}

func TestIssue398TrackedCurrentLeafAPIsStayAdmitted(t *testing.T) {
	sources := trackedProductionTextSources(t, []string{"."}, issue30ProductionGoSource)
	violations, err := issue398AcquisitionAPIViolations(sources)
	require.NoError(t, err)
	require.Empty(t, violations)
}

func TestIssue398InitializerWrapperDelegatesAndPropagatesExit(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("PowerShell is required for the operational wrapper behavior test")
	}
	script, err := filepath.Abs(filepath.Join("..", "scripts", "product-acquisition-init.ps1"))
	require.NoError(t, err)
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	manifest := filepath.Join(t.TempDir(), "private manifest.json")
	for _, code := range []int{0, 37} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			command := fmt.Sprintf("function global:go { $args | ConvertTo-Json -Compress; $global:LASTEXITCODE=%d }; & %s -Config %s -ConfirmEmptyDatabase issue398_empty; exit $LASTEXITCODE", code, quote(script), quote(manifest))
			out, err := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-Command", command).CombinedOutput()
			if code == 0 {
				require.NoError(t, err, string(out))
			} else {
				var exit *exec.ExitError
				require.ErrorAs(t, err, &exit, string(out))
				require.Equal(t, code, exit.ExitCode())
			}
			var args []string
			require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(string(out))), &args), string(out))
			require.Equal(t, []string{"run", "./cmd/product-acquisition-init", "-config", manifest, "-confirm-empty-database", "issue398_empty"}, args)
		})
	}
	command := "function global:go { throw 'must not execute' }; & " + quote(script) + " -Config " + quote(manifest)
	_, err = exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-Command", command).CombinedOutput()
	require.Error(t, err, "confirmation cannot default to an implicit initialization")
}
