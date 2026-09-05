//go:build linux

package ownershipmigration

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type linuxMountLocation struct {
	device string
	path   string
}

type linuxMountEntry struct {
	device     string
	root       string
	mountPoint string
}

func receiptParentAliasesProfileSubtree(profileRoot, receiptParent string) (bool, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false, err
	}
	return receiptParentAliasesProfileSubtreeFromMountInfo(profileRoot, receiptParent, data)
}

func receiptParentAliasesProfileSubtreeFromMountInfo(profileRoot, receiptParent string, data []byte) (bool, error) {
	entries, err := parseLinuxMountInfo(data)
	if err != nil {
		return false, err
	}
	mounts := indexLinuxMounts(entries)
	profileLocation, err := linuxFilesystemLocation(profileRoot, mounts)
	if err != nil {
		return false, err
	}
	receiptLocation, err := linuxFilesystemLocation(receiptParent, mounts)
	if err != nil {
		return false, err
	}
	if profileLocation.device == receiptLocation.device && insidePath(profileLocation.path, receiptLocation.path) {
		return true, nil
	}
	// Each mount below the protected root introduces another backing subtree,
	// possibly on a different device. Inspect mount metadata, never profile files.
	// One pass over mounts avoids per-account or pairwise filesystem scans.
	for _, entry := range entries {
		if !insidePath(profileRoot, entry.mountPoint) {
			continue
		}
		if !filepath.IsAbs(entry.root) {
			return false, fmt.Errorf("cannot resolve protected mount backing path")
		}
		if entry.device == receiptLocation.device && insidePath(entry.root, receiptLocation.path) {
			return true, nil
		}
	}
	return false, nil
}

type linuxMountChoice struct {
	entry     linuxMountEntry
	ambiguous bool
}

func indexLinuxMounts(entries []linuxMountEntry) map[string]linuxMountChoice {
	index := make(map[string]linuxMountChoice, len(entries))
	for _, entry := range entries {
		choice, exists := index[entry.mountPoint]
		if !exists {
			choice.entry = entry
		} else if entry.device != choice.entry.device || entry.root != choice.entry.root {
			choice.ambiguous = true
		}
		index[entry.mountPoint] = choice
	}
	return index
}

func linuxFilesystemLocation(path string, mounts map[string]linuxMountChoice) (linuxMountLocation, error) {
	path = filepath.Clean(path)
	var choice linuxMountChoice
	found := false
	for p := path; ; p = filepath.Dir(p) {
		if choice, found = mounts[p]; found || filepath.Dir(p) == p {
			break
		}
	}
	if !found {
		return linuxMountLocation{}, fmt.Errorf("path is not covered by mountinfo")
	}
	selected := choice.entry
	if choice.ambiguous || !filepath.IsAbs(selected.root) {
		return linuxMountLocation{}, fmt.Errorf("invalid or ambiguous backing location")
	}
	relative, err := filepath.Rel(selected.mountPoint, path)
	if err != nil {
		return linuxMountLocation{}, err
	}
	underlying := filepath.Clean(selected.root)
	if relative != "." {
		underlying = filepath.Join(underlying, relative)
	}
	if !filepath.IsAbs(underlying) {
		underlying = string(os.PathSeparator) + underlying
	}
	return linuxMountLocation{device: selected.device, path: filepath.Clean(underlying)}, nil
}

func validateProfileSubtrees(ctx context.Context, accounts []AccountEvidence) error {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return fmt.Errorf("cannot inspect profile mount identities")
	}
	return validateProfileSubtreesFromMountInfo(ctx, accounts, data)
}

func validateProfileSubtreesFromMountInfo(ctx context.Context, accounts []AccountEvidence, data []byte) error {
	entries, err := parseLinuxMountInfo(data)
	if err != nil {
		return err
	}
	mounts := indexLinuxMounts(entries)
	owners := make(map[string]int64, len(accounts))
	type ownedSubtree struct {
		device, path string
		accountID    int64
	}
	subtrees := make([]ownedSubtree, 0, len(accounts)+len(entries))
	for _, account := range accounts {
		if err := ctx.Err(); err != nil {
			return err
		}
		location, err := linuxFilesystemLocation(account.ProfileDirectory, mounts)
		if err != nil {
			return err
		}
		owners[filepath.Clean(account.ProfileDirectory)] = account.Previous.ID
		subtrees = append(subtrees, ownedSubtree{location.device, strings.TrimSuffix(location.path, "/") + "/", account.Previous.ID})
	}
	// Assign each nested mount to its containing account with ancestor map
	// lookups, not a scan of accounts for each mount. Profile contents stay opaque.
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		for p := entry.mountPoint; ; p = filepath.Dir(p) {
			if owner, ok := owners[p]; ok {
				if !filepath.IsAbs(entry.root) {
					return fmt.Errorf("cannot resolve protected profile mount")
				}
				subtrees = append(subtrees, ownedSubtree{entry.device, strings.TrimSuffix(filepath.Clean(entry.root), "/") + "/", owner})
				break
			}
			if filepath.Dir(p) == p {
				break
			}
		}
	}
	// Trailing separators make descendants contiguous in lexical order without
	// confusing similar prefixes. Keep the broadest range for one account; any
	// differently owned range inside it is overlap. O((accounts+mounts) log n),
	// plus bounded path-depth lookups; never cross-account pairwise comparisons.
	sort.Slice(subtrees, func(i, j int) bool {
		if subtrees[i].device != subtrees[j].device {
			return subtrees[i].device < subtrees[j].device
		}
		return subtrees[i].path < subtrees[j].path
	})
	var covering ownedSubtree
	for _, subtree := range subtrees {
		if err := ctx.Err(); err != nil {
			return err
		}
		if covering.device == subtree.device && strings.HasPrefix(subtree.path, covering.path) {
			if covering.accountID != subtree.accountID {
				return fmt.Errorf("source accounts %d and %d have overlapping browser profile subtrees", covering.accountID, subtree.accountID)
			}
			continue
		}
		covering = subtree
	}
	return nil
}

func parseLinuxMountInfo(data []byte) ([]linuxMountEntry, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	entries := make([]linuxMountEntry, 0, 32)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			return nil, fmt.Errorf("malformed mountinfo")
		}
		separator := -1
		for i := 6; i < len(fields); i++ {
			if fields[i] == "-" {
				separator = i
				break
			}
		}
		if separator < 0 || separator+3 >= len(fields) {
			return nil, fmt.Errorf("malformed mountinfo")
		}
		entry := linuxMountEntry{
			device:     fields[2],
			root:       decodeLinuxMountInfoPath(fields[3]),
			mountPoint: decodeLinuxMountInfoPath(fields[4]),
		}
		// Unrelated mounts may legitimately have opaque roots (e.g. nsfs) or
		// stacked mountpoints. Resolve backing-path ambiguity only where needed.
		if !filepath.IsAbs(entry.mountPoint) {
			return nil, fmt.Errorf("invalid mountinfo mountpoint")
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("empty mountinfo")
	}
	return entries, nil
}

func decodeLinuxMountInfoPath(value string) string {
	return strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	).Replace(value)
}
