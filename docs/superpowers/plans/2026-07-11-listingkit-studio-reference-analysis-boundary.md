# ListingKit Studio Reference Analysis Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Move deterministic Studio reference-analysis interpretation and safety policy from root ListingKit into internal/listing/studio/referenceanalysis without changing public behavior or the running SDS POD flow.

**Architecture:** Keep internal/listingkit as the imperative shell for request validation, image URL resolution, AI calls, warnings, DTOs, and public error translation. Add a standard-library-only functional core that accepts raw AI responses and returns a style brief, sanitized prompt, and policy flags.

**Tech Stack:** Go 1.26+, encoding/json, errors, regexp, strings, existing Go tests, and repository import-boundary tests.

## Global Constraints

- Preserve AnalyzeStudioReferenceStyle signatures, HTTP DTOs, output ordering, punctuation, warning strings, and public error strings.
- Do not modify SDS synchronization, generation, login, or retirement behavior.
- Do not modify Temporal workflows, retries, persistence ordering, task state, AI prompts, AI clients, URL handling, or object-store behavior.
- The new package may import only the Go standard library.
- Move algorithms mechanically. Vocabulary and safety-policy changes require a separate behavior change.
- Use TDD and keep every task independently passing.

---

## File Map

Create:

- internal/listing/studio/referenceanalysis/model.go — public result/errors and private analysis models.
- internal/listing/studio/referenceanalysis/vocabulary.go — existing regexes and vocabularies.
- internal/listing/studio/referenceanalysis/interpret.go — public pipeline, parsing, brief, and prompt construction.
- internal/listing/studio/referenceanalysis/sanitize.go — abstraction, fallback, sanitization, and unsafe-signal policy.
- internal/listing/studio/referenceanalysis/interpret_test.go — exact output and policy characterization.
- internal/listing/studio/referenceanalysis/boundary_guard_test.go — standard-library-only guard.

Modify:

- internal/listingkit/studio_reference_analysis.go — retain I/O orchestration and delegate pure interpretation.
- internal/listingkit/studio_reference_analysis_test.go — retain facade, error, URL, AI failure, and warning mapping coverage.
- docs/refactoring/listingkit-boundary-checkpoint.md — record the new owner.

Do not create an internal/listingkit/referenceanalysis compatibility package.

---

### Task 1: Establish the Pure Package Contract

**Files:**
- Create: internal/listing/studio/referenceanalysis/model.go
- Create: internal/listing/studio/referenceanalysis/interpret.go
- Create: internal/listing/studio/referenceanalysis/interpret_test.go
- Create: internal/listing/studio/referenceanalysis/boundary_guard_test.go

**Interfaces:**
- Consumes: raw strings returned by promptDiversifier.AnalyzeImage.
- Produces: Interpret(rawAnalyses []string) (Result, error), Result, ErrNoInput, ErrNoSafeDirection, ErrEmptyPrompt.

- [ ] **Step 1: Write failing contract tests**

Create interpret_test.go:

~~~go
package referenceanalysis

import (
	"errors"
	"testing"
)

func TestInterpretRejectsEmptyInput(t *testing.T) {
	_, err := Interpret(nil)
	if !errors.Is(err, ErrNoInput) {
		t.Fatalf("Interpret(nil) error = %v, want ErrNoInput", err)
	}
}

func TestInterpretRejectsWhitespaceOnlyInput(t *testing.T) {
	_, err := Interpret([]string{"  ", "\n"})
	if !errors.Is(err, ErrNoInput) {
		t.Fatalf("Interpret(whitespace) error = %v, want ErrNoInput", err)
	}
}
~~~

- [ ] **Step 2: Verify the tests fail**

Run:

~~~powershell
go test ./internal/listing/studio/referenceanalysis -run TestInterpretRejects -count=1
~~~

Expected: compilation fails because the API does not exist.

- [ ] **Step 3: Add the contract**

Create model.go:

~~~go
package referenceanalysis

import "errors"

var (
	ErrNoInput         = errors.New("reference analysis input is empty")
	ErrNoSafeDirection = errors.New("reference analysis has no reusable safe style direction")
	ErrEmptyPrompt     = errors.New("reference analysis generated an empty prompt")
)

type Result struct {
	StyleBrief        string
	SanitizedPrompt   string
	HadUnsafeInput    bool
	HadMalformedInput bool
}

type imageAnalysis struct {
	Motif            string   `json:"motif,omitempty"`
	Palette          []string `json:"palette,omitempty"`
	Composition      string   `json:"composition,omitempty"`
	Typography       string   `json:"typography,omitempty"`
	Density          string   `json:"density,omitempty"`
	ProductFit       string   `json:"product_fit,omitempty"`
	Mood             string   `json:"mood,omitempty"`
	GarmentPlacement string   `json:"garment_placement,omitempty"`
	Avoid            []string `json:"avoid,omitempty"`
	Raw              string   `json:"-"`
}

type abstractedAnalysis struct {
	Motif            string
	Palette          []string
	Composition      []string
	Typography       string
	Density          string
	ProductFit       string
	Mood             string
	GarmentPlacement string
	HadUnsafe        bool
	HadMalformed     bool
}
~~~

Create interpret.go:

~~~go
package referenceanalysis

import "strings"

func Interpret(rawAnalyses []string) (Result, error) {
	for _, raw := range rawAnalyses {
		if strings.TrimSpace(raw) != "" {
			return Result{}, ErrNoSafeDirection
		}
	}
	return Result{}, ErrNoInput
}
~~~

- [ ] **Step 4: Add the dependency guard**

Create boundary_guard_test.go:

~~~go
package referenceanalysis

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageImportsOnlyStandardLibrary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(".", entry.Name())
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, """)
			if strings.Contains(importPath, ".") ||
				strings.HasPrefix(importPath, "task-processor/") {
				t.Errorf("%s imports non-standard package %s", path, importPath)
			}
		}
	}
}
~~~

- [ ] **Step 5: Format, test, and commit**

~~~powershell
gofmt -w internal/listing/studio/referenceanalysis/*.go
go test ./internal/listing/studio/referenceanalysis -count=1
git add internal/listing/studio/referenceanalysis
git commit -m "refactor: define studio reference analysis contract"
~~~

Expected: PASS, then one contract commit.

---

### Task 2: Move Deterministic Interpretation and Safety Policy

**Files:**
- Create: internal/listing/studio/referenceanalysis/vocabulary.go
- Create: internal/listing/studio/referenceanalysis/sanitize.go
- Modify: internal/listing/studio/referenceanalysis/interpret.go
- Modify: internal/listing/studio/referenceanalysis/interpret_test.go
- Source: internal/listingkit/studio_reference_analysis.go

**Interfaces:**
- Consumes: []string raw AI responses.
- Produces: complete Interpret behavior with exact existing output text and policy flags.

- [ ] **Step 1: Add an exact-output test**

Append to interpret_test.go:

~~~go
func TestInterpretPreservesExactStructuredOutput(t *testing.T) {
	raw := `{"motif":"Retro Flowers","palette":["Cream","Cherry Red"],"composition":"Centered Badge","typography":"Old English","density":"Clean Layering","product_fit":"Vintage Streetwear"}`

	got, err := Interpret([]string{raw})
	if err != nil {
		t.Fatalf("Interpret() error = %v", err)
	}
	const wantBrief = "Reference style cues. Motif family: retro flowers. " +
		"Palette direction: cream, cherry red. " +
		"Composition family: centered composition, badge composition. " +
		"Typography feel: old english. Visual density: clean layering. " +
		"Product fit: vintage streetwear."
	const wantPrompt = "Create an original POD artwork with a commercially proven graphic style direction. " +
		"Motif direction: retro flowers. Palette direction: cream, cherry red. " +
		"Composition direction: centered composition, badge composition. " +
		"Typography feel: old english. Visual density: clean layering. " +
		"Product fit: vintage streetwear. Keep all graphics brand-neutral, " +
		"use fresh custom wording if text appears, avoid recognizable characters or people, " +
		"and use a clearly original layout."
	if got.StyleBrief != wantBrief {
		t.Fatalf("StyleBrief = %q, want %q", got.StyleBrief, wantBrief)
	}
	if got.SanitizedPrompt != wantPrompt {
		t.Fatalf("SanitizedPrompt = %q, want %q", got.SanitizedPrompt, wantPrompt)
	}
	if got.HadUnsafeInput || got.HadMalformedInput {
		t.Fatalf("flags = unsafe:%t malformed:%t, want false/false",
			got.HadUnsafeInput, got.HadMalformedInput)
	}
}
~~~

- [ ] **Step 2: Add policy cases**

Add strings to the test imports and append:

~~~go
func TestInterpretPreservesPolicyFlags(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		wantContains  []string
		wantAbsent    []string
		wantUnsafe    bool
		wantMalformed bool
		wantErr       error
	}{
		{
			name: "protected structured fields",
			raw: `{"motif":"Hello Kitty bow","typography":"Old English","avoid":["Adidas trefoil logo","exact slogan Just Do It"]}`,
			wantContains: []string{"old english", "clearly original layout"},
			wantAbsent:   []string{"hello kitty", "adidas", "trefoil", "just do it"},
			wantUnsafe:   true,
		},
		{
			name:          "safe malformed cues",
			raw:           "distressed serif, clean layering, vintage streetwear",
			wantContains:  []string{"distressed serif", "clean layering", "vintage streetwear"},
			wantMalformed: true,
		},
		{
			name: "no safe direction",
			raw: `{"motif":"Hello Kitty","palette":["Nike"],"composition":"same exact layout","typography":"Taylor Swift signature quote","density":"Mickey portrait","product_fit":"Adidas logo","avoid":["Just Do It slogan"]}`,
			wantErr: ErrNoSafeDirection,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Interpret([]string{tt.raw})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Interpret() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			combined := strings.ToLower(got.StyleBrief + " " + got.SanitizedPrompt)
			for _, want := range tt.wantContains {
				if !strings.Contains(combined, want) {
					t.Fatalf("output = %q, want %q", combined, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(combined, absent) {
					t.Fatalf("output = %q, must not contain %q", combined, absent)
				}
			}
			if got.HadUnsafeInput != tt.wantUnsafe ||
				got.HadMalformedInput != tt.wantMalformed {
				t.Fatalf("flags = %t/%t, want %t/%t",
					got.HadUnsafeInput, got.HadMalformedInput,
					tt.wantUnsafe, tt.wantMalformed)
			}
		})
	}
}
~~~

- [ ] **Step 3: Verify the new cases fail**

~~~powershell
go test ./internal/listing/studio/referenceanalysis -run TestInterpretPreserves -count=1
~~~

Expected: FAIL with ErrNoSafeDirection from the minimal implementation.

- [ ] **Step 4: Move declarations mechanically**

Create vocabulary.go by moving these declarations unchanged from studio_reference_analysis.go:

- studioWordPattern through studioRepeatedWhitespacePattern.
- studioSafeDescriptorWords.
- motif, palette, typography, density, and product-fit phrase/word vocabularies.
- studioSafeTitleCasePhraseSet.

Retain every regex, protected term, map entry, slice value, and order. Rename private identifiers only after all tests pass; no content edits are allowed.

- [ ] **Step 5: Move sanitization helpers mechanically**

Create sanitize.go and move the exact bodies of:

- abstractStudioReferenceAnalyses through abstractStudioReferenceAnalysis.
- mergeStudioAbstractedReferenceAnalysis through applyStructuredStudioFallbacks.
- all abstractStudio* field functions.
- sanitizeStudioStylePhrase through recoverMalformedStudioReferenceField.
- splitMalformedStudioReferenceFragments through containsStudioVocabularyPhrase.
- studioReferenceContainsUnsafeSignals through isMalformedStudioReferenceAnalysis.

Change only the private model names:

~~~go
studioReferenceImageAnalysis -> imageAnalysis
studioAbstractedReferenceAnalysis -> abstractedAnalysis
~~~

Keep regexp and strings as the only imports.

- [ ] **Step 6: Implement the complete public pipeline**

Replace interpret.go with:

~~~go
package referenceanalysis

import (
	"encoding/json"
	"strings"
)

func Interpret(rawAnalyses []string) (Result, error) {
	parsed := make([]imageAnalysis, 0, len(rawAnalyses))
	for _, raw := range rawAnalyses {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		parsed = append(parsed, parseImageAnalysis(raw))
	}
	if len(parsed) == 0 {
		return Result{}, ErrNoInput
	}
	abstracted := abstractStudioReferenceAnalyses(parsed)
	if !hasStudioReusableSafeStyleDirection(abstracted) {
		return Result{}, ErrNoSafeDirection
	}
	result := Result{
		StyleBrief:        buildStudioReferenceStyleBrief(abstracted),
		SanitizedPrompt:   buildSanitizedStudioReferencePrompt(abstracted),
		HadUnsafeInput:    studioReferenceContainsUnsafeSignals(parsed, abstracted),
		HadMalformedInput: studioReferenceContainsMalformedFallback(abstracted),
	}
	if strings.TrimSpace(result.SanitizedPrompt) == "" {
		return Result{}, ErrEmptyPrompt
	}
	return result, nil
}

func parseImageAnalysis(raw string) imageAnalysis {
	cleaned := strings.TrimSpace(raw)
	var analysis imageAnalysis
	if err := json.Unmarshal([]byte(cleaned), &analysis); err != nil {
		return imageAnalysis{Raw: cleaned}
	}
	analysis.Raw = cleaned
	return analysis
}
~~~

Move these five functions unchanged into interpret.go:

- buildStudioReferenceStyleBrief.
- buildSanitizedStudioReferencePrompt.
- collectStudioReferenceFragments.
- collectStudioReferencePalettes.
- collectStudioReferenceCompositionFragments.

Preserve literal strings, ordering, de-duplication, punctuation, and whitespace.

- [ ] **Step 7: Format, test, and commit**

~~~powershell
gofmt -w internal/listing/studio/referenceanalysis/*.go
go test ./internal/listing/studio/referenceanalysis -count=1
go test ./internal/listingkit -run TestAnalyzeStudioReferenceStyle -count=1
git add internal/listing/studio/referenceanalysis
git commit -m "refactor: extract studio reference analysis policy"
~~~

Expected: both packages PASS. ListingKit still uses its original policy at this checkpoint.

---

### Task 3: Switch the ListingKit Facade

**Files:**
- Modify: internal/listingkit/studio_reference_analysis.go
- Modify: internal/listingkit/studio_reference_analysis_test.go

**Interfaces:**
- Consumes: referenceanalysis.Interpret.
- Produces: unchanged AnalyzeStudioReferenceStyle public behavior.

- [ ] **Step 1: Preserve facade warning tests**

Keep these existing tests as facade-level mapping checks:

- TestAnalyzeStudioReferenceStyleSanitizesSingleReferencePrompt.
- TestAnalyzeStudioReferenceStyleDerivesFallbackForSafeOffVocabularyMalformedText.
- TestAnalyzeStudioReferenceStyleReturnsFailureWhenSingleReferenceAnalysisFails.
- TestAnalyzeStudioReferenceStyleErrorsWhenNoSafeSignalsSurvive.

Ensure the first two assert the exact existing warning strings:

~~~go
"已移除品牌、Logo、原文案或过于接近原图的描述。"
"部分参考图返回了非结构化分析结果，仅保留可安全复用的风格提示。"
~~~

- [ ] **Step 2: Run the facade tests before integration**

~~~powershell
go test ./internal/listingkit -run "TestAnalyzeStudioReferenceStyle(SanitizesSingleReferencePrompt|DerivesFallbackForSafeOffVocabularyMalformedText|ReturnsFailureWhenSingleReferenceAnalysisFails|ErrorsWhenNoSafeSignalsSurvive)" -count=1
~~~

Expected: PASS.

- [ ] **Step 3: Delegate interpretation**

Add:

~~~go
referenceanalysis "task-processor/internal/listing/studio/referenceanalysis"
~~~

Keep request validation, URL resolution, analysisPrompt construction, and AI calls unchanged. Replace parsed private values with raw strings, then call the new package:

~~~go
rawAnalyses := make([]string, 0, len(urls))
for _, imageURL := range resolvedURLs {
	raw, err := s.promptDiversifier.AnalyzeImage(ctx, imageURL, analysisPrompt)
	if err != nil {
		warnings = append(warnings,
			fmt.Sprintf("参考图分析失败：%s", compactStudioGenerationError(err)))
		continue
	}
	rawAnalyses = append(rawAnalyses, raw)
}
if len(rawAnalyses) == 0 {
	return nil, fmt.Errorf(
		"reference_analysis_failed: no reference image could be analyzed")
}

interpreted, err := referenceanalysis.Interpret(rawAnalyses)
if err != nil {
	switch {
	case errors.Is(err, referenceanalysis.ErrNoSafeDirection):
		return nil, fmt.Errorf(
			"reference_analysis_failed: no reusable safe style direction extracted")
	case errors.Is(err, referenceanalysis.ErrEmptyPrompt):
		return nil, fmt.Errorf(
			"reference_analysis_failed: generated reference brief is empty")
	case errors.Is(err, referenceanalysis.ErrNoInput):
		return nil, fmt.Errorf(
			"reference_analysis_failed: no reference image could be analyzed")
	default:
		return nil, fmt.Errorf("reference_analysis_failed: %w", err)
	}
}
if interpreted.HadUnsafeInput {
	warnings = append(warnings,
		"已移除品牌、Logo、原文案或过于接近原图的描述。")
}
if interpreted.HadMalformedInput {
	warnings = append(warnings,
		"部分参考图返回了非结构化分析结果，仅保留可安全复用的风格提示。")
}
return &StudioReferenceAnalysisResponse{
	ReferenceStyleBrief: interpreted.StyleBrief,
	SanitizedPrompt:     interpreted.SanitizedPrompt,
	Warnings:            warnings,
}, nil
~~~

- [ ] **Step 4: Remove duplicate ListingKit ownership**

Delete from studio_reference_analysis.go:

- policy regexes and vocabularies;
- studioReferenceImageAnalysis and studioAbstractedReferenceAnalysis;
- parseStudioReferenceImageAnalysis;
- the five output/collection functions moved to interpret.go;
- every abstraction, fallback, sanitization, and unsafe-signal helper moved to sanitize.go.

Remove encoding/json and regexp imports. Retain errors for errors.Is and existing URL/store logic.

Do not move buildStudioReferenceAnalysisPrompt or any URL/upload helper.

- [ ] **Step 5: Verify and commit**

~~~powershell
gofmt -w internal/listingkit/studio_reference_analysis.go internal/listingkit/studio_reference_analysis_test.go
go test ./internal/listing/studio/referenceanalysis -count=1
go test ./internal/listingkit -run TestAnalyzeStudioReferenceStyle -count=1
go test ./internal/listingkit/... -count=1
git add internal/listingkit/studio_reference_analysis.go internal/listingkit/studio_reference_analysis_test.go
git commit -m "refactor: delegate reference analysis policy"
~~~

Expected: all commands PASS. Any SDS, workflow, submission, or Studio regression blocks the commit.

---

### Task 4: Document and Verify the Ownership Move

**Files:**
- Modify: docs/refactoring/listingkit-boundary-checkpoint.md

**Interfaces:**
- Consumes: completed package and facade delegation.
- Produces: current architecture checkpoint and repository verification evidence.

- [ ] **Step 1: Update the checkpoint**

Add:

~~~markdown
### internal/listing/studio/referenceanalysis

Owns platform-neutral interpretation and safety policy for Studio
reference-image analysis, including structured/malformed result parsing,
reusable style abstraction, protected-identity filtering, and sanitized
brief/prompt construction.

Guardrail:

- internal/listing/studio/referenceanalysis uses only the Go standard library
  and must not import ListingKit, marketplace, runtime, infrastructure, HTTP,
  SDS, or external SDK packages.

Root internal/listingkit retains request validation, image URL and upload
resolution, AI invocation, compatibility DTOs, warning text, and public
error translation.
~~~

- [ ] **Step 2: Run hygiene and focused verification**

~~~powershell
gofmt -w internal/listing/studio/referenceanalysis/*.go internal/listingkit/studio_reference_analysis.go internal/listingkit/studio_reference_analysis_test.go
git diff --check
go test ./internal/listing/studio/... -count=1
go test ./internal/listingkit/... -count=1
go test ./tests/... -count=1
~~~

Expected: no diff errors and all tests PASS.

- [ ] **Step 3: Run repository-wide verification**

~~~powershell
go test ./... -count=1
~~~

Expected: PASS. If an unrelated known instability occurs, rerun its package once, report exact output, and do not claim repository-wide success.

- [ ] **Step 4: Confirm scope**

~~~powershell
git diff --name-only HEAD~3
git status --short
~~~

Allowed production scope:

~~~text
internal/listing/studio/referenceanalysis/*
internal/listingkit/studio_reference_analysis.go
internal/listingkit/studio_reference_analysis_test.go
docs/refactoring/listingkit-boundary-checkpoint.md
~~~

No SDS, Temporal, persistence, marketplace submission, DTO, or object-store file may appear.

- [ ] **Step 5: Commit documentation**

~~~powershell
git add docs/refactoring/listingkit-boundary-checkpoint.md
git commit -m "docs: record reference analysis ownership"
~~~

---

## Final Acceptance Checklist

- [ ] Interpret owns parsing, abstraction, sanitization, and policy flags.
- [ ] The new package imports only the Go standard library.
- [ ] ListingKit retains request validation, URL resolution, AI calls, DTOs, warnings, and error translation.
- [ ] Existing output, warning, and public error behavior is unchanged.
- [ ] SDS POD, Temporal, persistence, and submission files are untouched.
- [ ] Studio, ListingKit, architecture, and repository-wide tests pass.
- [ ] The active ListingKit checkpoint documents the new owner.
