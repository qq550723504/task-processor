// Package skill owns reusable method packages, never identity or execution.
package skill

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

const (
	MaxFileBytes    = 16 << 10
	MaxPackageBytes = 48 << 10
	MaxFiles        = 16
	MaxRequestBytes = 96 << 10
)

var ErrInvalid = errors.New("SKILL_INVALID")
var ErrUnsupported = errors.New("SKILL_UNSUPPORTED_FIELD")

type File struct {
	Path string `json:"path"`
	Text string `json:"text"`
}
type Package struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Target      string `json:"target,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Digest      string `json:"digest"`
	Files       []File `json:"files"`
}
type manifest struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	License     string            `yaml:"license"`
	Metadata    map[string]string `yaml:"metadata"`
}

var namePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func supportedPath(p string) bool {
	if p == "SKILL.md" {
		return true
	}
	parts := strings.Split(p, "/")
	if len(parts) != 2 || parts[1] == "" || strings.ContainsAny(p, "\\:%?#") || path.Clean(p) != p {
		return false
	}
	if parts[0] != "references" && parts[0] != "assets" {
		return false
	}
	if len(p) > 256 || !utf8.ValidString(p) || strings.IndexFunc(p, unicode.IsControl) >= 0 {
		return false
	}
	ext := path.Ext(parts[1])
	return ext == ".md" || ext == ".txt" || parts[0] == "assets" && ext == ".json"
}
func textOK(s string) bool {
	return utf8.ValidString(s) && strings.IndexFunc(s, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) < 0
}

func parseManifest(source string) (manifest, error) {
	var m manifest
	if !strings.HasPrefix(source, "---\n") {
		return m, ErrInvalid
	}
	remaining := source[4:]
	end := strings.Index(remaining, "\n---\n")
	if end < 0 || strings.TrimSpace(remaining[end+5:]) == "" {
		return m, ErrInvalid
	}
	raw := []byte(remaining[:end])
	var node yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	if decoder.Decode(&node) != nil {
		return m, ErrInvalid
	}
	var extra yaml.Node
	if decoder.Decode(&extra) != io.EOF {
		return m, ErrInvalid
	}
	var check func(*yaml.Node) bool
	check = func(n *yaml.Node) bool {
		if n.Kind == yaml.AliasNode || n.Anchor != "" || n.Tag != "" && n.Tag != "!!str" && n.Tag != "!!map" {
			return false
		}
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				key := n.Content[i]
				if key.Tag != "!!str" || seen[key.Value] {
					return false
				}
				seen[key.Value] = true
			}
		}
		for _, c := range n.Content {
			if !check(c) {
				return false
			}
		}
		return true
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode || !check(&node) {
		return m, ErrInvalid
	}
	root := node.Content[0]
	for i := 0; i < len(root.Content); i += 2 {
		switch root.Content[i].Value {
		case "name", "description", "license", "metadata":
		default:
			return m, fmt.Errorf("%w: %s", ErrUnsupported, root.Content[i].Value)
		}
	}
	strict := yaml.NewDecoder(bytes.NewReader(raw))
	strict.KnownFields(true)
	if strict.Decode(&m) != nil || len(m.Name) > 64 || !namePattern.MatchString(m.Name) || strings.TrimSpace(m.Description) != m.Description || utf8.RuneCountInString(m.Description) < 1 || utf8.RuneCountInString(m.Description) > 1024 || !textOK(m.Description) || len(m.License) > 512 || !textOK(m.License) || len(m.Metadata) > 16 {
		return m, ErrInvalid
	}
	for k, v := range m.Metadata {
		if k == "" || len(k) > 128 || len(v) > 512 || !textOK(k) || !textOK(v) {
			return m, ErrInvalid
		}
	}
	return m, nil
}

func validateLinks(file File, files map[string]bool) error {
	source := []byte(file.Text)
	doc := goldmark.DefaultParser().Parse(text.NewReader(source))
	return ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var destination []byte
		switch n := node.(type) {
		case *ast.Image, *ast.RawHTML, *ast.HTMLBlock:
			return ast.WalkStop, ErrInvalid
		case *ast.Link:
			destination = n.Destination
		case *ast.AutoLink:
			destination = n.URL(source)
		}
		if len(destination) == 0 {
			return ast.WalkContinue, nil
		}
		u, e := url.Parse(string(destination))
		if e != nil || u.IsAbs() || u.Host != "" || u.RawQuery != "" || strings.ContainsAny(string(destination), "\\:%") || strings.HasPrefix(u.Path, "/") {
			return ast.WalkStop, ErrInvalid
		}
		for _, segment := range strings.Split(u.Path, "/") {
			if segment == ".." || segment == "." {
				return ast.WalkStop, ErrInvalid
			}
		}
		if u.Path != "" && !files[path.Join(path.Dir(file.Path), u.Path)] {
			return ast.WalkStop, ErrInvalid
		}
		return ast.WalkContinue, nil
	})
}

// ValidatePackage canonicalizes a complete, bounded text bundle. The digest
// covers every file; neither the YAML author nor a self-reported version is authority.
func ValidatePackage(files []File) (Package, error) {
	var p Package
	if len(files) < 1 || len(files) > MaxFiles {
		return p, ErrInvalid
	}
	p.Files = append([]File(nil), files...)
	sort.Slice(p.Files, func(i, j int) bool { return p.Files[i].Path < p.Files[j].Path })
	names := map[string]bool{}
	folded := map[string]bool{}
	total := 0
	for i := range p.Files {
		f := &p.Files[i]
		f.Text = strings.ReplaceAll(f.Text, "\r\n", "\n")
		fold := strings.ToLower(f.Path)
		if !supportedPath(f.Path) || folded[fold] || len(f.Text) == 0 || len(f.Text) > MaxFileBytes || !textOK(f.Text) {
			return Package{}, ErrInvalid
		}
		names[f.Path], folded[fold] = true, true
		total += len(f.Path) + len(f.Text)
		if total > MaxPackageBytes {
			return Package{}, ErrInvalid
		}
		if path.Ext(f.Path) == ".json" && !json.Valid([]byte(f.Text)) {
			return Package{}, ErrInvalid
		}
	}
	if !names["SKILL.md"] {
		return Package{}, ErrInvalid
	}
	for _, file := range p.Files {
		if file.Path == "SKILL.md" {
			m, e := parseManifest(file.Text)
			if e != nil {
				return Package{}, e
			}
			p.Name, p.Description = m.Name, m.Description
			// Metadata remains opaque unless an exact supported method marker
			// opts in. Never invalidate an existing package's arbitrary metadata.
			if target := m.Metadata["listingkit-method-v1"]; target == TitleTarget || target == ImageTarget {
				p.Target = target
				p.DisplayName = m.Metadata["listingkit-display-name"]
			}
		}
		if path.Ext(file.Path) == ".md" {
			if e := validateLinks(file, names); e != nil {
				return Package{}, e
			}
		}
	}
	raw, e := json.Marshal(p.Files)
	if e != nil || len(raw) > MaxPackageBytes {
		return Package{}, ErrInvalid
	}
	sum := sha256.Sum256(raw)
	p.Digest = hex.EncodeToString(sum[:])
	return p, nil
}

const TitleTarget = "product-title"
const ImageTarget = "product-image"

// Supports describes content compatibility, never execution authority. Current
// unmarked methods keep their existing title consumer; images require opt-in.
func (p Package) Supports(target string) bool {
	return target == TitleTarget && (p.Target == "" || p.Target == TitleTarget) || target == ImageTarget && p.Target == ImageTarget
}
