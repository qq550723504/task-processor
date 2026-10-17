package skill

import (
	"context"
	"embed"
	"io/fs"
	"strings"
	"task-processor/internal/agent"
)

// Release material is installed explicitly into the existing Skill owner.
// Reads and execution never fall back to this embedded bundle.
//
//go:embed official/*.md
var officialFiles embed.FS

type OfficialPackage struct {
	Key     string
	Package Package
}

func OfficialPackages() ([]OfficialPackage, error) {
	entries, err := fs.ReadDir(officialFiles, "official")
	if err != nil {
		return nil, err
	}
	out := make([]OfficialPackage, 0, len(entries))
	for _, entry := range entries {
		data, err := officialFiles.ReadFile("official/" + entry.Name())
		if err != nil {
			return nil, err
		}
		p, err := ValidatePackage([]File{{Path: "SKILL.md", Text: string(data)}})
		if err != nil {
			return nil, err
		}
		out = append(out, OfficialPackage{Key: "official-method-v1:" + strings.TrimSuffix(entry.Name(), ".md"), Package: p})
	}
	return out, nil
}

type OfficialInstallResult struct {
	Installed []agent.SkillVersionRef `json:"installed"`
	Existing  []agent.SkillVersionRef `json:"existing"`
}

type OfficialInstaller interface {
	InstallOfficial(context.Context, Principal, ResolvePrincipal) (OfficialInstallResult, error)
}
