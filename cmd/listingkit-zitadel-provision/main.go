package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/zitadelprovision"
)

const (
	defaultIssuerURL      = "http://localhost:8080"
	localAPIApplication   = "ListingKit Local API"
	localOIDCApplication  = "ListingKit Local OIDC"
	localRedirectURI      = "http://localhost:3000/api/zitadel-auth/callback"
	localPostLogoutURI    = "http://localhost:3000"
	managementTokenEnvKey = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_MANAGEMENT_TOKEN"
	projectIDEnvKey       = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID"
	apiClientIDEnvKey     = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID"
	apiClientSecretEnvKey = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if len(args) == 0 {
		return errors.New("subcommand is required: provision or authorize")
	}

	switch args[0] {
	case "provision":
		return runProvision(ctx, args[1:], stdout, stderr)
	case "authorize":
		return runAuthorize(ctx, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown subcommand %q: want provision or authorize", args[0])
	}
}

func runProvision(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("provision", flag.ContinueOnError)
	flags.SetOutput(stderr)

	issuerURL := envOrDefault("ZITADEL_ISSUER_URL", defaultIssuerURL)
	orgID := envOrDefault("ZITADEL_ORG_ID", "")
	projectID := os.Getenv(projectIDEnvKey)
	projectName := envOrDefault("LISTINGKIT_ZITADEL_PROJECT_NAME", "ListingKit")
	createProject := true
	managementTokenFile := ""
	runtimeFile := ""
	flags.StringVar(&issuerURL, "issuer-url", issuerURL, "ZITADEL issuer URL")
	flags.StringVar(&orgID, "org-id", orgID, "ZITADEL management organization ID")
	flags.StringVar(&projectID, "project-id", projectID, "ListingKit ZITADEL project ID")
	flags.StringVar(&projectName, "project-name", projectName, "ListingKit ZITADEL project name")
	flags.BoolVar(&createProject, "create-project", createProject, "create the project when it does not exist")
	flags.StringVar(&managementTokenFile, "management-token-file", managementTokenFile, "file containing the ZITADEL management token")
	flags.StringVar(&runtimeFile, "runtime-file", runtimeFile, "generated runtime environment file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("unexpected arguments after provision flags: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(managementTokenFile) == "" {
		return errors.New("-management-token-file is required")
	}
	if strings.TrimSpace(runtimeFile) == "" {
		return errors.New("-runtime-file is required")
	}
	if err := validateAcceptanceRuntimePath(runtimeFile); err != nil {
		return err
	}

	existingRuntime, err := readRuntimeEnv(runtimeFile)
	if err != nil {
		return err
	}
	if runtimeFileExists(runtimeFile) {
		for _, key := range []string{apiClientSecretEnvKey, "ZITADEL_CLIENT_SECRET"} {
			if strings.TrimSpace(existingRuntime[key]) == "" {
				return fmt.Errorf("existing runtime file is missing %s; refusing to mutate ZITADEL", key)
			}
		}
	}
	managementToken, err := readSecretFile(managementTokenFile, "management token")
	if err != nil {
		return err
	}

	result, err := zitadelprovision.ProvisionLocalApplications(ctx, zitadelprovision.Config{
		IssuerURL:       strings.TrimSpace(issuerURL),
		ManagementToken: managementToken,
		OrgID:           strings.TrimSpace(orgID),
		ProjectID:       strings.TrimSpace(projectID),
		ProjectName:     strings.TrimSpace(projectName),
		CreateProject:   createProject,
	}, zitadelprovision.LocalApplicationConfig{
		APIName:                localAPIApplication,
		OIDCName:               localOIDCApplication,
		RedirectURIs:           []string{localRedirectURI},
		PostLogoutRedirectURIs: []string{localPostLogoutURI},
	})
	if err != nil {
		return fmt.Errorf("provision local ZITADEL applications: %w", err)
	}

	runtime, err := runtimeValues(existingRuntime, issuerURL, managementToken, orgID, result)
	if err != nil {
		return err
	}
	if err := writeRuntimeEnv(runtimeFile, runtime); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "status=ok phase=provision")
	return nil
}

func runAuthorize(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("authorize", flag.ContinueOnError)
	flags.SetOutput(stderr)

	tokenFile := ""
	runtimeFile := ""
	grantAdmin := false
	flags.StringVar(&tokenFile, "token-file", tokenFile, "file containing the browser access token")
	flags.StringVar(&runtimeFile, "runtime-file", runtimeFile, "runtime environment file generated by provision")
	flags.BoolVar(&grantAdmin, "grant-admin", grantAdmin, "also grant the ListingKit admin role")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("unexpected arguments after authorize flags: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(tokenFile) == "" {
		return errors.New("-token-file is required")
	}
	if strings.TrimSpace(runtimeFile) == "" {
		return errors.New("-runtime-file is required")
	}
	if err := validateAcceptanceRuntimePath(runtimeFile); err != nil {
		return err
	}

	runtime, err := readRuntimeEnv(runtimeFile)
	if err != nil {
		return err
	}
	browserToken, err := readSecretFile(tokenFile, "browser token")
	if err != nil {
		return err
	}
	issuerURL, err := requiredRuntimeValue(runtime, "ZITADEL_ISSUER_URL")
	if err != nil {
		return err
	}
	projectID, err := requiredRuntimeValue(runtime, projectIDEnvKey)
	if err != nil {
		return err
	}
	managementToken, err := requiredRuntimeValue(runtime, managementTokenEnvKey)
	if err != nil {
		return err
	}
	apiClientID, err := requiredRuntimeValue(runtime, apiClientIDEnvKey)
	if err != nil {
		return err
	}
	apiClientSecret, err := requiredRuntimeValue(runtime, apiClientSecretEnvKey)
	if err != nil {
		return err
	}

	verifier := zitadel.NewVerifier(zitadel.Config{
		IssuerURL:    issuerURL,
		ClientID:     apiClientID,
		ClientSecret: apiClientSecret,
	})
	identity, err := verifier.Verify(ctx, browserToken)
	if err != nil {
		return fmt.Errorf("verify browser token: %w", err)
	}

	additionalRole := ""
	if grantAdmin {
		additionalRole = "listingkit_admin"
	}
	if err := zitadelprovision.GrantLocalOperator(ctx, zitadelprovision.Config{
		IssuerURL:       issuerURL,
		ManagementToken: managementToken,
		OrgID:           strings.TrimSpace(runtime["ZITADEL_ORG_ID"]),
		ProjectID:       projectID,
	}, additionalRole, identity); err != nil {
		return fmt.Errorf("grant local operator authorization: %w", err)
	}
	fmt.Fprintln(stdout, "status=ok phase=authorize")
	return nil
}

func runtimeValues(existing map[string]string, issuerURL, managementToken, orgID string, result zitadelprovision.LocalApplicationResult) (map[string]string, error) {
	runtime := make(map[string]string)
	for _, key := range []string{managementTokenEnvKey, apiClientSecretEnvKey, "ZITADEL_CLIENT_SECRET", "ZITADEL_ORG_ID"} {
		if value := strings.TrimSpace(existing[key]); value != "" {
			runtime[key] = value
		}
	}
	runtime["ZITADEL_ISSUER_URL"] = strings.TrimSpace(issuerURL)
	if runtime["ZITADEL_ISSUER_URL"] == "" {
		return nil, errors.New("issuer URL is required")
	}
	apiSecret, err := preserveSecret(result.APIClientSecret, existing, apiClientSecretEnvKey)
	if err != nil {
		return nil, err
	}
	oidcSecret, err := preserveSecret(result.OIDCClientSecret, existing, "ZITADEL_CLIENT_SECRET")
	if err != nil {
		return nil, err
	}
	runtime[projectIDEnvKey] = strings.TrimSpace(result.ProjectID)
	runtime[managementTokenEnvKey] = managementToken
	runtime["TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_APP_ID"] = result.APIAppID
	runtime[apiClientIDEnvKey] = result.APIClientID
	runtime[apiClientSecretEnvKey] = apiSecret
	runtime["TASK_PROCESSOR_LISTINGKIT_ZITADEL_OIDC_APP_ID"] = result.OIDCAppID
	runtime["ZITADEL_CLIENT_ID"] = result.OIDCClientID
	runtime["ZITADEL_CLIENT_SECRET"] = oidcSecret
	runtime["ZITADEL_REDIRECT_URI"] = localRedirectURI
	runtime["ZITADEL_POST_LOGOUT_REDIRECT_URI"] = localPostLogoutURI
	runtime["ZITADEL_SCOPES"] = strings.Join(result.RecommendedScopes, " ")
	runtime["TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTHZ_REQUIRED"] = "true"
	runtime["TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES"] = "listingkit_viewer,listingkit_operator,listingkit_admin,platform_admin"
	if strings.TrimSpace(orgID) != "" {
		runtime["ZITADEL_ORG_ID"] = strings.TrimSpace(orgID)
	}
	return runtime, nil
}

func preserveSecret(generated string, existing map[string]string, key string) (string, error) {
	if strings.TrimSpace(generated) != "" {
		return generated, nil
	}
	if value := strings.TrimSpace(existing[key]); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("runtime secret %s is unavailable", key)
}

func readSecretFile(path, description string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s file: %w", description, err)
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", fmt.Errorf("%s file is empty", description)
	}
	return value, nil
}

func readRuntimeEnv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read runtime file: %w", err)
	}

	values := make(map[string]string)
	for lineNumber, line := range strings.Split(strings.TrimPrefix(string(data), "\ufeff"), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("invalid runtime env line %d", lineNumber+1)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			if value[0] == '"' {
				if unquoted, err := strconv.Unquote(value); err == nil {
					value = unquoted
				}
			} else {
				value = value[1 : len(value)-1]
			}
		}
		values[key] = value
	}
	return values, nil
}

func runtimeFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func validateAcceptanceRuntimePath(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return errors.New("runtime file path is invalid")
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(absolute)), "/")
	for index := 0; index+1 < len(parts); index++ {
		if parts[index] == ".local" && parts[index+1] == "image-agent-acceptance" {
			return rejectSymlinkPath(absolute)
		}
	}
	return errors.New("runtime file must be under .local/image-agent-acceptance")
}

func rejectSymlinkPath(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return errors.New("runtime file path is invalid")
	}
	current := filepath.VolumeName(absolute) + string(filepath.Separator)
	remainder := strings.TrimPrefix(absolute, current)
	for _, part := range strings.Split(remainder, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return fmt.Errorf("inspect runtime file path: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("runtime file path must not contain symlinks")
		}
	}
	return nil
}

func writeRuntimeEnv(path string, values map[string]string) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create runtime directory: %w", err)
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var content strings.Builder
	for _, key := range keys {
		value := values[key]
		if strings.ContainsAny(key, "\r\n=") || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("invalid runtime env value for %s", key)
		}
		content.WriteString(key)
		content.WriteByte('=')
		content.WriteString(value)
		content.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o600); err != nil {
		return fmt.Errorf("write runtime file: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("protect runtime file: %w", err)
	}
	return nil
}

func requiredRuntimeValue(values map[string]string, key string) (string, error) {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return "", fmt.Errorf("runtime file is missing %s", key)
	}
	return value, nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
