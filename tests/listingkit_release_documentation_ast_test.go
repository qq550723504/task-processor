package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

func TestListingKitReleaseDocumentationUsesWorkflowOnlyProductionExamples(t *testing.T) {
	t.Parallel()

	policyPath := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "release-authority", "release-policy.yaml")
	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		ReleaseAuthority struct {
			Documentation struct {
				Paths              []string `yaml:"paths"`
				CanonicalWorkflows []string `yaml:"canonicalWorkflows"`
			} `yaml:"documentation"`
		} `yaml:"releaseAuthority"`
	}
	if err := yaml.Unmarshal(policyBytes, &policy); err != nil {
		t.Fatal(err)
	}

	for _, relativePath := range policy.ReleaseAuthority.Documentation.Paths {
		source, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(relativePath)))
		if err != nil {
			t.Fatalf("read supported release document %s: %v", relativePath, err)
		}
		for _, workflow := range policy.ReleaseAuthority.Documentation.CanonicalWorkflows {
			if !strings.Contains(string(source), workflow) {
				t.Errorf("%s does not link canonical workflow %s", relativePath, workflow)
			}
		}

		violations, err := listingKitReleaseDocumentationFenceViolations(source)
		if err != nil {
			t.Fatalf("walk Markdown AST for %s: %v", relativePath, err)
		}
		for _, violation := range violations {
			t.Errorf("%s %s", relativePath, violation)
		}
	}
}

func TestListingKitReleaseDocumentationRejectsMutationFenceUnderAnyHeading(t *testing.T) {
	t.Parallel()

	source := []byte("## Emergency recovery\n\n```bash\nkubectl -n task-processor patch deployment product-listing-api --patch '{}'\n```\n")
	violations, err := listingKitReleaseDocumentationFenceViolations(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) == 0 {
		t.Fatal("direct production mutation outside the Release workflows heading must be rejected")
	}
}

func TestListingKitReleaseAuthorityCIIncludesWorkbenchReadmePath(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On struct {
			Push struct {
				Paths []string `yaml:"paths"`
			} `yaml:"push"`
			PullRequest struct {
				Paths []string `yaml:"paths"`
			} `yaml:"pull_request"`
		} `yaml:"on"`
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatal(err)
	}
	want := "deployments/kubernetes/listingkit-workbench/README.md"
	for trigger, paths := range map[string][]string{
		"push":         workflow.On.Push.Paths,
		"pull_request": workflow.On.PullRequest.Paths,
	} {
		if !containsString(paths, want) {
			t.Errorf("ListingKit CI %s paths must include supported release README %q", trigger, want)
		}
	}
}

func listingKitReleaseDocumentationFenceViolations(source []byte) ([]string, error) {
	document := goldmark.DefaultParser().Parse(text.NewReader(source))
	var violations []string
	err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if typed, ok := node.(*ast.FencedCodeBlock); ok {
			var block strings.Builder
			for index := 0; index < typed.Lines().Len(); index++ {
				segment := typed.Lines().At(index)
				block.Write(segment.Value(source))
			}
			content := strings.ToLower(block.String())
			if strings.Contains(block.String(), "KUBE_CONFIG") {
				violations = append(violations, "code block advertises a long-lived kubeconfig")
			}
			for _, command := range listingKitKubectlCommands(content) {
				mutates := containsAny(command, " apply ", " patch ", " rollout restart ", " set image ", " delete ", " scale ") ||
					(strings.Contains(command, " create ") && !strings.Contains(command, "--dry-run=client"))
				if mutates {
					violations = append(violations, "code block advertises a direct production mutation")
					break
				}
			}
		}
		return ast.WalkContinue, nil
	})
	return violations, err
}

func listingKitKubectlCommands(block string) []string {
	lines := strings.Split(block, "\n")
	var commands []string
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		kubectlIndex := strings.Index(line, "kubectl")
		if kubectlIndex < 0 {
			continue
		}
		command := line[kubectlIndex:]
		for hasCommandContinuation(line) && index+1 < len(lines) {
			index++
			line = strings.TrimSpace(lines[index])
			command += " " + line
		}
		commands = append(commands, " "+strings.Join(strings.Fields(command), " ")+" ")
	}
	return commands
}

func hasCommandContinuation(line string) bool {
	return strings.HasSuffix(line, "\\") || strings.HasSuffix(line, "`") || strings.HasSuffix(line, "|")
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
