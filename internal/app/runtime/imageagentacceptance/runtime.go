package imageagentacceptanceruntime

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"strings"

	"gorm.io/gorm"

	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/listingkit/imageagentacceptance"
	listingkitstore "task-processor/internal/listingkit/store"
)

// Run assembles and executes the local Image Agent acceptance seed workflow.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("listingkit-image-agent-acceptance-seed", flag.ContinueOnError)
	flags.SetOutput(stderr)
	runtimeFile := flags.String("runtime-file", "", "generated local acceptance runtime file")
	tokenFile := flags.String("token-file", "", "file containing the browser bearer token")
	sourceURL := flags.String("source-url", "", "public HTTPS source image URL")
	styleURL := flags.String("style-url", "", "optional public HTTPS style image URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return errors.New("unexpected positional arguments")
	}
	if strings.TrimSpace(*runtimeFile) == "" || strings.TrimSpace(*tokenFile) == "" || strings.TrimSpace(*sourceURL) == "" {
		return errors.New("-runtime-file, -token-file and -source-url are required")
	}
	runtime, err := imageagentacceptance.LoadRuntimeConfig(*runtimeFile)
	if err != nil {
		return err
	}
	token, err := readToken(*tokenFile)
	if err != nil {
		return err
	}
	concreteGuard := imageagentacceptance.NewEnvironmentGuard(imageagentacceptance.EnvironmentProbes{
		ComposeProject: dockerComposeProjectProbe,
	})
	db, err := concreteGuard.Verify(ctx, runtime)
	if err != nil {
		return err
	}
	if db == nil {
		return errors.New("acceptance environment guard returned no database")
	}

	// The concrete guard has already checked Compose identity, database name and
	// marker. Pass its verified handle through the Seed guard adapter so Seed can
	// own the handle lifetime without opening a second database connection.
	result, err := imageagentacceptance.Seed(ctx, verifiedGuard{db: db}, zitadel.NewVerifier(zitadel.Config{
		IssuerURL:    runtime.IssuerURL,
		ClientID:     runtime.APIClientID,
		ClientSecret: runtime.APIClientSecret,
	}), listingkitstore.NewTaskRepository(db), imageagentacceptance.SeedRequest{
		Runtime:   runtime,
		Token:     token,
		SourceURL: *sourceURL,
		StyleURL:  *styleURL,
	})
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(struct {
		TaskID       string `json:"task_id"`
		TenantID     string `json:"tenant_id"`
		UserID       string `json:"user_id"`
		WorkspaceURL string `json:"workspace_url"`
	}{result.TaskID, result.TenantID, result.UserID, result.WorkspaceURL})
}

type verifiedGuard struct{ db *gorm.DB }

func (g verifiedGuard) Verify(context.Context, imageagentacceptance.RuntimeConfig) (*gorm.DB, error) {
	return g.db, nil
}

func readToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("read acceptance token file failed")
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", errors.New("acceptance token file is empty")
	}
	return token, nil
}

func dockerComposeProjectProbe(ctx context.Context, config imageagentacceptance.RuntimeConfig) (bool, error) {
	project := strings.TrimSpace(config.ComposeProject)
	if project == "" {
		return false, errors.New("Compose project is required")
	}
	command := exec.CommandContext(
		ctx,
		"docker",
		"ps",
		"--filter", "label=com.docker.compose.project="+project,
		"--filter", "label=com.docker.compose.service="+imageagentacceptance.ComposePostgresService,
		"--format", "{{.ID}}",
	)
	output, err := command.Output()
	if err != nil {
		return false, errors.New("docker Compose project probe failed")
	}
	containerIDs := strings.Fields(string(output))
	if len(containerIDs) != 1 {
		return false, nil
	}
	inspect := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .NetworkSettings.Ports}}", containerIDs[0])
	portOutput, err := inspect.Output()
	if err != nil {
		return false, errors.New("docker Compose PostgreSQL port probe failed")
	}
	return validatePostgresBindings(portOutput), nil
}

func validatePostgresBindings(data []byte) bool {
	var ports map[string][]struct {
		HostIP   string `json:"HostIp"`
		HostPort string `json:"HostPort"`
	}
	if err := json.Unmarshal(data, &ports); err != nil {
		return false
	}
	bindings := ports["5432/tcp"]
	return len(bindings) == 1 &&
		bindings[0].HostIP == "127.0.0.1" &&
		bindings[0].HostPort == imageagentacceptance.DatabasePort
}
