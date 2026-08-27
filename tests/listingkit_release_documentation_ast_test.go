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

		document := goldmark.DefaultParser().Parse(text.NewReader(source))
		inReleaseWorkflowSection := false
		if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			switch typed := node.(type) {
			case *ast.Heading:
				heading := strings.TrimSpace(string(typed.Text(source)))
				if typed.Level <= 3 {
					inReleaseWorkflowSection = heading == "Release workflows"
				}
			case *ast.FencedCodeBlock:
				if !inReleaseWorkflowSection {
					return ast.WalkContinue, nil
				}
				var block strings.Builder
				for index := 0; index < typed.Lines().Len(); index++ {
					segment := typed.Lines().At(index)
					block.Write(segment.Value(source))
				}
				content := strings.ToLower(block.String())
				if strings.Contains(block.String(), "KUBE_CONFIG") {
					t.Errorf("%s release-workflow code block advertises a long-lived kubeconfig", relativePath)
				}
				if strings.Contains(content, "kubectl") && containsAny(content, " apply ", " patch ", " rollout restart ", " set image ", " delete ", " scale ", " create ") {
					t.Errorf("%s release-workflow code block advertises a direct production mutation", relativePath)
				}
			}
			return ast.WalkContinue, nil
		}); err != nil {
			t.Fatalf("walk Markdown AST for %s: %v", relativePath, err)
		}
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
