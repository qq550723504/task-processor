package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/zitadelprovision"
)

const (
	defaultAPIName  = "ListingKit Local API"
	defaultOIDCName = "ListingKit Local OIDC"
	defaultCallback = "http://localhost:3000/api/zitadel-auth/callback"
	defaultLogout   = "http://localhost:3000"

	runtimeIssuer     = "ZITADEL_ISSUER_URL"
	runtimeProject    = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID"
	runtimeManagement = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_MANAGEMENT_TOKEN"
	runtimeAPIClient  = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID"
	runtimeAPISecret  = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET"
	runtimeOIDCClient = "ZITADEL_CLIENT_ID"
	runtimeOIDCSecret = "ZITADEL_CLIENT_SECRET"
	runtimeCallback   = "ZITADEL_REDIRECT_URI"
	runtimeLogout     = "ZITADEL_POST_LOGOUT_REDIRECT_URI"
	runtimeAuthz      = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTHZ_REQUIRED"
	runtimeScopes     = "ZITADEL_SCOPES"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(args) == 0 {
		return errors.New("a subcommand is required: provision or authorize")
	}
	switch args[0] {
	case "provision":
		return runProvision(ctx, args[1:], stdout, stderr)
	case "authorize":
		return runAuthorize(ctx, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func runProvision(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("provision", flag.ContinueOnError)
	flags.SetOutput(stderr)
	issuerURL := flags.String("issuer-url", "", "local ZITADEL issuer URL")
	projectID := flags.String("project-id", "", "existing ListingKit project ID")
	projectName := flags.String("project-name", "ListingKit", "ListingKit project name")
	createProject := flags.Bool("create-project", false, "create the project when it does not exist")
	managementTokenFile := flags.String("management-token-file", "", "file containing the ZITADEL management token")
	runtimeFile := flags.String("runtime-file", "", "generated local runtime env file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *issuerURL == "" || *managementTokenFile == "" || *runtimeFile == "" {
		return errors.New("-issuer-url, -management-token-file and -runtime-file are required")
	}
	managementToken, err := readSecretFile(*managementTokenFile)
	if err != nil {
		return fmt.Errorf("read management token file: %w", err)
	}
	oldRuntime, err := readRuntimeFileIfPresent(*runtimeFile)
	if err != nil {
		return fmt.Errorf("read runtime file: %w", err)
	}
	result, err := zitadelprovision.ProvisionLocalApplications(ctx, zitadelprovision.Config{
		IssuerURL:       *issuerURL,
		ManagementToken: managementToken,
		ProjectID:       *projectID,
		ProjectName:     *projectName,
		CreateProject:   *createProject,
	}, zitadelprovision.LocalApplicationConfig{
		APIName:                defaultAPIName,
		OIDCName:               defaultOIDCName,
		RedirectURIs:           []string{defaultCallback},
		PostLogoutRedirectURIs: []string{defaultLogout},
	})
	if err != nil {
		return err
	}
	values := map[string]string{
		runtimeIssuer:     strings.TrimSpace(*issuerURL),
		runtimeProject:    result.ProjectID,
		runtimeManagement: managementToken,
		runtimeAPIClient:  result.APIClientID,
		runtimeAPISecret:  result.APIClientSecret,
		runtimeOIDCClient: result.OIDCClientID,
		runtimeOIDCSecret: result.OIDCClientSecret,
		runtimeCallback:   defaultCallback,
		runtimeLogout:     defaultLogout,
		runtimeAuthz:      "true",
		runtimeScopes:     strings.Join(result.RecommendedScopes, " "),
	}
	for _, key := range []string{runtimeManagement, runtimeAPISecret, runtimeOIDCSecret} {
		if values[key] == "" {
			values[key] = oldRuntime[key]
		}
		if values[key] == "" {
			return fmt.Errorf("generated runtime value %s is unavailable; rerun after recreating the application", key)
		}
	}
	if err := writeRuntimeFile(*runtimeFile, values); err != nil {
		return fmt.Errorf("write runtime file: %w", err)
	}
	fmt.Fprintf(stdout, "status=ok phase=provision project_id=%s runtime_file=%s\n", result.ProjectID, filepath.Clean(*runtimeFile))
	return nil
}

func runAuthorize(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("authorize", flag.ContinueOnError)
	flags.SetOutput(stderr)
	tokenFile := flags.String("token-file", "", "file containing the browser bearer token")
	runtimeFile := flags.String("runtime-file", "", "generated local runtime env file")
	grantAdmin := flags.Bool("grant-admin", false, "also grant listingkit_admin")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *tokenFile == "" || *runtimeFile == "" {
		return errors.New("-token-file and -runtime-file are required")
	}
	runtime, err := readRuntimeFile(*runtimeFile)
	if err != nil {
		return fmt.Errorf("read runtime file: %w", err)
	}
	for _, key := range []string{runtimeIssuer, runtimeProject, runtimeManagement, runtimeAPIClient, runtimeAPISecret} {
		if strings.TrimSpace(runtime[key]) == "" {
			return fmt.Errorf("runtime file is missing %s", key)
		}
	}
	browserToken, err := readSecretFile(*tokenFile)
	if err != nil {
		return fmt.Errorf("read browser token file: %w", err)
	}
	identity, err := zitadel.NewVerifier(zitadel.Config{
		IssuerURL:    runtime[runtimeIssuer],
		ClientID:     runtime[runtimeAPIClient],
		ClientSecret: runtime[runtimeAPISecret],
	}).Verify(ctx, browserToken)
	if err != nil {
		return err
	}
	additionalRole := ""
	if *grantAdmin {
		additionalRole = "listingkit_admin"
	}
	if err := zitadelprovision.GrantLocalOperator(ctx, zitadelprovision.Config{
		IssuerURL:       runtime[runtimeIssuer],
		ManagementToken: runtime[runtimeManagement],
		ProjectID:       runtime[runtimeProject],
	}, additionalRole, identity); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "status=ok phase=authorize project_id=%s\n", runtime[runtimeProject])
	return nil
}

func readSecretFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("file path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", errors.New("file is empty")
	}
	return value, nil
}

func readRuntimeFileIfPresent(path string) (map[string]string, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	return readRuntimeFile(path)
}

func readRuntimeFile(path string) (map[string]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("file path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string)
	for lineNumber, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid runtime entry at line %d", lineNumber+1)
		}
		values[strings.TrimSpace(key)] = value
	}
	return values, nil
}

func writeRuntimeFile(path string, values map[string]string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("file path is required")
	}
	for key, value := range values {
		if strings.ContainsAny(key, "=\r\n") || strings.ContainsAny(value, "\r\n") {
			return errors.New("runtime values cannot contain newlines")
		}
	}
	keys := []string{
		runtimeIssuer, runtimeProject, runtimeManagement, runtimeAPIClient, runtimeAPISecret,
		runtimeOIDCClient, runtimeOIDCSecret, runtimeCallback, runtimeLogout, runtimeAuthz, runtimeScopes,
	}
	var content strings.Builder
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		content.WriteString(key)
		content.WriteByte('=')
		content.WriteString(value)
		content.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content.String()), 0o600)
}
