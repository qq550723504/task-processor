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
	profileLocation, err := linuxFilesystemLocationFromMountInfo(profileRoot, data)
	if err != nil {
		return false, err
	}
	receiptLocation, err := linuxFilesystemLocationFromMountInfo(receiptParent, data)
	if err != nil {
		return false, err
	}
	if profileLocation.device != receiptLocation.device {
		return false, nil
	}
	return insidePath(profileLocation.path, receiptLocation.path), nil
}

func linuxFilesystemLocationFromMountInfo(path string, data []byte) (linuxMountLocation, error) {
	path = filepath.Clean(path)
	entries, err := parseLinuxMountInfo(data)
	if err != nil {
		return linuxMountLocation{}, err
	}
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
		entries = append(entries, linuxMountEntry{
			device:     fields[2],
			root:       decodeLinuxMountInfoPath(fields[3]),
			mountPoint: decodeLinuxMountInfoPath(fields[4]),
		})
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
