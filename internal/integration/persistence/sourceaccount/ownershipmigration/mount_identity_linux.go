//go:build linux

package ownershipmigration

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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
	profileLocation, err := linuxFilesystemLocation(profileRoot, entries)
	if err != nil {
		return false, err
	}
	receiptLocation, err := linuxFilesystemLocation(receiptParent, entries)
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
		if insidePath(profileRoot, entry.mountPoint) && entry.device == receiptLocation.device && insidePath(entry.root, receiptLocation.path) {
			return true, nil
		}
	}
	return false, nil
}

func linuxFilesystemLocation(path string, entries []linuxMountEntry) (linuxMountLocation, error) {
	path = filepath.Clean(path)
	var selected *linuxMountEntry
	for i := range entries {
		entry := &entries[i]
		if !insidePath(entry.mountPoint, path) {
			continue
		}
		if selected == nil || len(entry.mountPoint) > len(selected.mountPoint) {
			selected = entry
		}
	}
	if selected == nil {
		return linuxMountLocation{}, fmt.Errorf("path is not covered by mountinfo")
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

func parseLinuxMountInfo(data []byte) ([]linuxMountEntry, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	entries := make([]linuxMountEntry, 0, 32)
	seenMountPoints := map[string]bool{}
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
		// Stacked entries at one mountpoint do not give this bounded resolver an
		// unambiguous visible mount. Refuse rather than selecting by input order.
		if !filepath.IsAbs(entry.root) || !filepath.IsAbs(entry.mountPoint) || seenMountPoints[entry.mountPoint] {
			return nil, fmt.Errorf("invalid or ambiguous mountinfo paths")
		}
		seenMountPoints[entry.mountPoint] = true
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
