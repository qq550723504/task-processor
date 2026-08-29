package imageagentacceptance

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	RuntimeDatabaseDSNKey       = "LISTINGKIT_ACCEPTANCE_DATABASE_DSN"
	RuntimeEnvironmentMarkerKey = "LISTINGKIT_ACCEPTANCE_ENVIRONMENT_MARKER"
	RuntimeComposeProjectKey    = "LISTINGKIT_ACCEPTANCE_COMPOSE_PROJECT"
	RuntimeIssuerURLKey         = "ZITADEL_ISSUER_URL"
	RuntimeAPIClientIDKey       = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID"
	RuntimeAPIClientSecretKey   = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET"
)

var runtimeConfigFields = map[string]func(*RuntimeConfig, string){
	RuntimeDatabaseDSNKey: func(config *RuntimeConfig, value string) { config.DatabaseDSN = value },
	RuntimeEnvironmentMarkerKey: func(config *RuntimeConfig, value string) {
		config.EnvironmentMarker = value
	},
	RuntimeComposeProjectKey: func(config *RuntimeConfig, value string) { config.ComposeProject = value },
	RuntimeIssuerURLKey:      func(config *RuntimeConfig, value string) { config.IssuerURL = value },
	RuntimeAPIClientIDKey:    func(config *RuntimeConfig, value string) { config.APIClientID = value },
	RuntimeAPIClientSecretKey: func(config *RuntimeConfig, value string) {
		config.APIClientSecret = value
	},
}

// RuntimeConfig contains only the values generated for the local acceptance
// environment. It deliberately does not model the broader application env.
type RuntimeConfig struct {
	DatabaseDSN       string
	EnvironmentMarker string
	ComposeProject    string
	IssuerURL         string
	APIClientID       string
	APIClientSecret   string
}

// LoadRuntimeConfig reads and validates the generated acceptance runtime file.
func LoadRuntimeConfig(path string) (RuntimeConfig, error) {
	if strings.TrimSpace(path) == "" {
		return RuntimeConfig{}, errors.New("acceptance runtime file path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("read acceptance runtime file: %w", err)
	}
	return ParseRuntimeConfig(data)
}

// ParseRuntimeConfig parses the dotenv-style bytes emitted for acceptance.
func ParseRuntimeConfig(data []byte) (RuntimeConfig, error) {
	var config RuntimeConfig
	seen := make(map[string]struct{}, len(runtimeConfigFields))
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimPrefix(string(data), "\ufeff")))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return RuntimeConfig{}, fmt.Errorf("invalid acceptance runtime entry at line %d", lineNumber)
		}
		setter, ok := runtimeConfigFields[key]
		if !ok {
			return RuntimeConfig{}, fmt.Errorf("unsupported acceptance runtime field %q", key)
		}
		if _, duplicate := seen[key]; duplicate {
			return RuntimeConfig{}, fmt.Errorf("duplicate acceptance runtime field %q", key)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = strings.Trim(value[1:len(value)-1], " ")
		}
		if value == "" {
			return RuntimeConfig{}, fmt.Errorf("acceptance runtime field %q is empty", key)
		}
		setter(&config, value)
		seen[key] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return RuntimeConfig{}, fmt.Errorf("read acceptance runtime entries: %w", err)
	}
	for key := range runtimeConfigFields {
		if _, ok := seen[key]; !ok {
			return RuntimeConfig{}, fmt.Errorf("acceptance runtime field %q is required", key)
		}
	}
	return config, nil
}
