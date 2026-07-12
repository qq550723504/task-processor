package listingkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestPodExecutionSupportBoundary(t *testing.T) {
	root := parsePodExecutionBoundaryFile(t, "pod_execution.go")
	policy := parsePodExecutionBoundaryFile(t, "pod_execution_policy_support.go")

	assertPodExecutionImport(t, root, "task-processor/internal/product/sourcing/sdspod")
	assertPodExecutionDelegates(t, root, "derivePodExecutionSummary", "DeriveExecution")
	assertPodExecutionDelegates(t, root, "normalizePodExecutionSummary", "NormalizeExecution")
	assertPodExecutionDelegates(t, policy, "podSubmissionBlocked", "SubmissionBlocked")
	assertPodExecutionDelegates(t, policy, "podReadinessMessage", "ReadinessMessage")

	for _, file := range []*ast.File{root, policy} {
		assertPodExecutionRetiredHelpersAbsent(t, file)
	}

	auditContent, err := os.ReadFile("pod_execution_audit_support.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, snippet := range []string{
		"func podExecutionEqual(left *PodExecutionSummary, right *PodExecutionSummary) bool {",
		"func normalizePodExecutionAuditHistory(items []PodExecutionAuditEvent) []PodExecutionAuditEvent {",
		"func recordPodExecutionAudit(before *PodExecutionSummary, after *PodExecutionSummary, updatedAt time.Time) {",
	} {
		if !strings.Contains(string(auditContent), snippet) {
			t.Fatalf("audit support file missing seam %q", snippet)
		}
	}
}

func parsePodExecutionBoundaryFile(t *testing.T, path string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func assertPodExecutionImport(t *testing.T, file *ast.File, want string) {
	t.Helper()
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if path == want {
			return
		}
	}
	t.Fatalf("%s should import %q", file.Name.Name, want)
}

func assertPodExecutionDelegates(t *testing.T, file *ast.File, functionName, selectorName string) {
	t.Helper()
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Name == nil || function.Name.Name != functionName {
			continue
		}
		delegates := false
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel == nil || selector.Sel.Name != selectorName {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if ok && identifier.Name == "sdspod" {
				delegates = true
				return false
			}
			return true
		})
		if delegates {
			return
		}
		t.Fatalf("%s should call sdspod.%s", functionName, selectorName)
	}
	t.Fatalf("missing %s", functionName)
}

func assertPodExecutionRetiredHelpersAbsent(t *testing.T, file *ast.File) {
	t.Helper()
	retired := map[string]struct{}{
		"inferPodStatusFromSDS":              {},
		"inferActivePodStatusFromChildTasks": {},
		"inferPodStatusFromChildTasks":       {},
		"mapSDSStatusToPODStatus":            {},
		"podFailureStatusForMode":            {},
	}
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Name == nil {
			continue
		}
		if _, found := retired[function.Name.Name]; found {
			t.Fatalf("retired root policy helper %s remains in %s", function.Name.Name, file.Name.Name)
		}
	}
}
