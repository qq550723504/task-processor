# SDS POD Test Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (\`- [ ]\`) syntax for tracking.

**Goal:** Lock the SDS POD style-refresh behavior and canonical metadata boundary with direct, AST-backed regression tests.

**Architecture:** Production code remains unchanged. A ListingKit revision test exercises the real refresh entrypoint and asserts canonical variant results. Shared test helpers parse Go syntax using the standard library; the SDS boundary test uses them to inspect declarations, imports, and selector call expressions rather than comments or raw source strings.

**Tech Stack:** Go 1.26+, \`go/ast\`, \`go/parser\`, \`go/token\`, existing ListingKit fixtures.

## Global Constraints

- Change test code only; do not alter production behavior, DTOs, JSON contracts, \`sdspod.ApplyCanonical\`, workflow ordering, image behavior, or SHEIN payload behavior.
- Keep \`go.work.sum\` untouched and unstaged.
- Scan every non-test Go file directly in \`internal/listingkit\`; do not recurse into subdirectories.
- The forbidden helper is exactly \`applyStudioStyleDimension\`.
- Require the exact import \`task-processor/internal/product/sourcing/sdspod\` and a real \`sdspod.ApplyCanonical\` call expression.
- The direct refresh test asserts \`ai_style\`, trimmed StyleName, and the full current trace: detail \`SDS studio AI style dimension\`, confidence \`0.94\`, inferred false, needs review false.

---

## File Map

- Modify: \`internal/listingkit/service_revision_test.go\` — real revision-refresh style regression.
- Modify: \`internal/listingkit/phase10_task_generation_action_boundary_test.go\` — reusable AST parsing and inspection helpers plus their syntax-level regression test.
- Modify: \`internal/listingkit/phase6_workflow_studio_sds_metadata_support_boundary_test.go\` — use AST helpers for the SDS metadata delegation boundary.

### Task 1: Add Direct Revision Refresh Style Characterization

**Files:**
- Modify: \`internal/listingkit/service_revision_test.go\`

**Interfaces:**
- Consumes: \`(*service).refreshSheinDerivedState(task *Task, req *ApplyRevisionRequest)\`.
- Produces: a regression test proving the existing style-only \`sdspod.ApplyCanonical\` delegation remains observable through the real refresh path.

- [ ] **Step 1: Add the real-path characterization test**

Append this test after \`TestRefreshSheinDerivedStateClearsAttributeCacheForRegeneration\` and add \`"reflect"\` to the import block:

~~~go
func TestRefreshSheinDerivedStateAppliesSDSStyleToEveryCanonicalVariant(t *testing.T) {
	styleName := "  Studio B6C753EB  "
	task := &Task{
		Request: &GenerateRequest{Options: &GenerateOptions{
			SDS: &SDSSyncOptions{StyleName: styleName},
		}},
		Result: &ListingKitResult{
			CanonicalProduct: &canonical.Product{Variants: []canonical.Variant{
				{SKU: "SKU-1"},
				{SKU: "SKU-2"},
			}},
			Shein: &SheinPackage{RequestDraft: &SheinRequestDraft{}},
		},
	}

	(&service{}).refreshSheinDerivedState(task, &ApplyRevisionRequest{
		Platform: "shein",
		Shein:    &SheinRevisionInput{RegenerateAttributes: true},
	})

	wantTrace := canonical.FieldTrace{
		Sources: []canonical.Source{{
			Type:   canonical.SourceDerived,
			Detail: "SDS studio AI style dimension",
		}},
		Confidence:  0.94,
		IsInferred:  false,
		NeedsReview: false,
	}
	for i, variant := range task.Result.CanonicalProduct.Variants {
		attribute, ok := variant.Attributes["ai_style"]
		if !ok {
			t.Fatalf("variant[%d] ai_style missing", i)
		}
		if attribute.Value != "Studio B6C753EB" {
			t.Fatalf("variant[%d] ai_style = %q, want Studio B6C753EB", i, attribute.Value)
		}
		if !reflect.DeepEqual(attribute.Trace, wantTrace) {
			t.Fatalf("variant[%d] ai_style trace = %+v, want %+v", i, attribute.Trace, wantTrace)
		}
	}
}
~~~

- [ ] **Step 2: Run the characterization test**

Run:

~~~powershell
go test ./internal/listingkit -run TestRefreshSheinDerivedStateAppliesSDSStyleToEveryCanonicalVariant -count=1
~~~

Expected: PASS. This is a characterization test for already-shipped behavior; no production change is in scope, so its initial green output establishes the current oracle.

- [ ] **Step 3: Format, run related tests, and commit**

~~~powershell
gofmt -w internal/listingkit/service_revision_test.go
go test ./internal/listingkit -run "TestRefreshSheinDerivedState" -count=1
git add internal/listingkit/service_revision_test.go
git commit -m "test: cover sds style refresh delegation"
~~~

Expected: PASS and a test-only commit.

### Task 2: Replace SDS Metadata String Guards with AST Guards

**Files:**
- Modify: \`internal/listingkit/phase10_task_generation_action_boundary_test.go\`
- Modify: \`internal/listingkit/phase6_workflow_studio_sds_metadata_support_boundary_test.go\`

**Interfaces:**
- Produces test helpers: \`listingKitProductionGoFiles(t *testing.T) []string\`, \`parseListingKitGoFile(t *testing.T, path string) *ast.File\`, \`hasFunctionDeclaration(file *ast.File, name string) bool\`, \`hasImportPath(file *ast.File, want string) bool\`, and \`hasSelectorCall(file *ast.File, receiver, name string) bool\`.

- [ ] **Step 1: Add the failing selector-call test**

In \`phase10_task_generation_action_boundary_test.go\`, add \`"strconv"\` to imports and append:

~~~go
func TestHasSelectorCallIgnoresCommentsAndStrings(t *testing.T) {
	t.Parallel()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "fixture.go", `package fixture
// sdspod.ApplyCanonical(product, metadata)
var note = "sdspod.ApplyCanonical(product, metadata)"
func apply() {}
`, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if hasSelectorCall(file, "sdspod", "ApplyCanonical") {
		t.Fatal("comment or string should not satisfy selector-call detection")
	}
}
~~~

- [ ] **Step 2: Verify RED**

~~~powershell
go test ./internal/listingkit -run TestHasSelectorCallIgnoresCommentsAndStrings -count=1
~~~

Expected: compile failure because `hasSelectorCall` does not yet exist.

- [ ] **Step 3: Implement minimal AST helpers**

Append to `phase10_task_generation_action_boundary_test.go`:

~~~go
func listingKitProductionGoFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.): %v", err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		paths = append(paths, entry.Name())
	}
	return paths
}

func parseListingKitGoFile(t *testing.T, path string) *ast.File {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
	}
	return file
}

func hasFunctionDeclaration(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if ok && funcDecl.Name != nil && funcDecl.Name.Name == name {
			return true
		}
	}
	return false
}

func hasImportPath(file *ast.File, want string) bool {
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err == nil && path == want {
			return true
		}
	}
	return false
}

func hasSelectorCall(file *ast.File, receiver, name string) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil || selector.Sel.Name != name {
			return true
		}
		identifier, ok := selector.X.(*ast.Ident)
		if ok && identifier.Name == receiver {
			found = true
			return false
		}
		return true
	})
	return found
}
~~~

- [ ] **Step 4: Verify helper GREEN**

~~~powershell
gofmt -w internal/listingkit/phase10_task_generation_action_boundary_test.go
go test ./internal/listingkit -run TestHasSelectorCallIgnoresCommentsAndStrings -count=1
~~~

Expected: PASS.

- [ ] **Step 5: Replace the SDS delegation string checks**

In `TestWorkflowStudioSDSMetadataSupportBoundary`, remove the two `applyStudioStyleDimension` substring exclusions and replace the final `adapterSource` string assertion with:

~~~go
for _, path := range listingKitProductionGoFiles(t) {
	if hasFunctionDeclaration(parseListingKitGoFile(t, path), "applyStudioStyleDimension") {
		t.Fatalf("%s should not declare retired applyStudioStyleDimension", path)
	}
}

adapterFile := parseListingKitGoFile(t, "sds_canonical_metadata.go")
if !hasImportPath(adapterFile, "task-processor/internal/product/sourcing/sdspod") {
	t.Fatal("sds_canonical_metadata.go should import sdspod")
}
if !hasSelectorCall(adapterFile, "sdspod", "ApplyCanonical") {
	t.Fatal("sds_canonical_metadata.go should call sdspod.ApplyCanonical")
}
~~~

- [ ] **Step 6: Run targeted architecture checks and commit**

~~~powershell
gofmt -w internal/listingkit/phase10_task_generation_action_boundary_test.go internal/listingkit/phase6_workflow_studio_sds_metadata_support_boundary_test.go
go test ./internal/listingkit -run "TestHasSelectorCallIgnoresCommentsAndStrings|TestWorkflowStudioSDSMetadataSupportBoundary" -count=1
git add internal/listingkit/phase10_task_generation_action_boundary_test.go internal/listingkit/phase6_workflow_studio_sds_metadata_support_boundary_test.go
git commit -m "test: harden sds metadata boundary guard"
~~~

Expected: PASS and a test-only commit.

### Task 3: Verify the Complete Test-Only Slice

**Files:**
- Verify: the three files changed in Tasks 1 and 2.

- [ ] **Step 1: Check formatting and scope**

~~~powershell
gofmt -w internal/listingkit/service_revision_test.go internal/listingkit/phase10_task_generation_action_boundary_test.go internal/listingkit/phase6_workflow_studio_sds_metadata_support_boundary_test.go
git diff --check master...HEAD
git diff --name-only master...HEAD
git diff -- go.work.sum
~~~

Expected: no whitespace errors; code scope is the three test files plus this design and plan; `go.work.sum` has no diff.

- [ ] **Step 2: Run ListingKit verification**

~~~powershell
go test ./internal/listingkit/... -count=1
go vet ./internal/listingkit/...
~~~

Expected: PASS.

- [ ] **Step 3: Record final state**

~~~powershell
git status --short
git log --oneline master..HEAD
~~~

Expected: a clean feature worktree with the design, plan, and two test commits.
