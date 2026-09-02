package runtimepath

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// NamespaceForDirectory returns a stable, path-safe namespace for one installation directory.
func NamespaceForDirectory(directory string) string {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory = "unknown"
	}
	directory = filepath.Clean(directory)
	if absoluteDirectory, err := filepath.Abs(directory); err == nil {
		directory = absoluteDirectory
	}
	digest := sha256.Sum256([]byte(directory))
	return hex.EncodeToString(digest[:8])
}

// InstallationNamespace identifies the current checkout or installation.
func InstallationNamespace() string {
	executable, err := os.Executable()
	if err == nil {
		return namespaceForExecutable(executable)
	}
	directory, err := os.Getwd()
	if err != nil {
		directory = "unknown"
	}
	return NamespaceForDirectory(directory)
}

func namespaceForExecutable(executable string) string {
	if resolvedExecutable, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolvedExecutable
	}
	return NamespaceForDirectory(filepath.Dir(executable))
}

// NamespacedTempPath returns a path under the system temp directory isolated by installation.
func NamespacedTempPath(parts ...string) string {
	components := make([]string, 0, len(parts)+3)
	components = append(components, os.TempDir(), "task-processor")
	components = append(components, parts...)
	components = append(components, InstallationNamespace())
	return filepath.Join(components...)
}
