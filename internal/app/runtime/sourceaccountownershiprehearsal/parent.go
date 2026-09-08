package sourceaccountownershiprehearsal

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	rehearsalImage = "postgres:18-alpine"
	rehearsalLabel = "task-processor.source-account-b2"
)

type rehearsalScenario string

const (
	scenarioNormal                  rehearsalScenario = "normal"
	scenarioCommitResponseLoss      rehearsalScenario = "commit-response-loss"
	scenarioCommitLossTargetDrift   rehearsalScenario = "commit-response-loss-target-drift"
	scenarioInspectPermissionDenied rehearsalScenario = "inspect-permission-denied"
	scenarioSchemaPermissionDenied  rehearsalScenario = "schema-permission-denied"
	scenarioPreparePermissionDenied rehearsalScenario = "prepare-permission-denied"
	scenarioReceiptPermissionDenied rehearsalScenario = "receipt-permission-denied"
	scenarioMappingDrift            rehearsalScenario = "mapping-drift"
	scenarioTargetDrift             rehearsalScenario = "target-drift"
)

func validRehearsalScenario(scenario rehearsalScenario) bool {
	switch scenario {
	case scenarioNormal, scenarioCommitResponseLoss, scenarioCommitLossTargetDrift, scenarioInspectPermissionDenied,
		scenarioSchemaPermissionDenied, scenarioPreparePermissionDenied,
		scenarioReceiptPermissionDenied, scenarioMappingDrift, scenarioTargetDrift:
		return true
	default:
		return false
	}
}

type roleCredentials struct {
	name     string
	password string
}

type rehearsalRuntime struct {
	invocation  string
	storage     rehearsalStorage
	container   *tcpostgres.PostgresContainer
	containerID string
	adminDSN    string
	source      databaseIdentity
	metadata    databaseIdentity
	roles       map[string]roleCredentials
}

func runRehearsal(ctx context.Context, scenario rehearsalScenario, stdout, stderr io.Writer) int {
	ctx, cancel := boundedRehearsalContext(ctx)
	defer cancel()
	storage, err := newRehearsalStorage(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "rehearsal storage admission denied")
		return ExitAdmissionDenied
	}
	runtimeState := &rehearsalRuntime{storage: storage}
	exitCode := ExitDependencyFailure
	summary := ""
	if err = runtimeState.start(ctx); err != nil {
		fmt.Fprintln(stderr, "could not start task-owned PostgreSQL rehearsal resource")
	} else {
		exitCode, summary = runtimeState.execute(ctx, scenario, stderr)
	}
	cleanupErr := runtimeState.cleanup()
	if cleanupErr != nil {
		fmt.Fprintln(stderr, "task-owned rehearsal cleanup failed")
		return ExitDependencyFailure
	}
	if exitCode != ExitSuccess {
		return exitCode
	}
	fmt.Fprintln(stdout, summary)
	fmt.Fprintln(stdout, "cleanup=PASS")
	fmt.Fprintln(stdout, "ISOLATED_REHEARSAL_PASS real_authority=NOT_RUN real_environment=NOT_AUTHORIZED c_d=NOT_RUN")
	return ExitSuccess
}

func boundedRehearsalContext(parent context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := parent.Deadline(); ok && time.Until(deadline) <= 10*time.Minute {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, 10*time.Minute)
}

func (runtimeState *rehearsalRuntime) start(ctx context.Context) error {
	invocation, err := newNonce()
	if err != nil {
		return err
	}
	password, err := newNonce()
	if err != nil {
		return err
	}
	runtimeState.invocation = invocation

	postgresContainer, err := tcpostgres.Run(ctx,
		rehearsalImage,
		tcpostgres.WithDatabase("source_b2"),
		tcpostgres.WithUsername("issue364_admin"),
		tcpostgres.WithPassword(password),
		tcpostgres.BasicWaitStrategies(),
		testcontainers.WithLabels(map[string]string{rehearsalLabel: "issue-364-rehearsal", rehearsalLabel + ".invocation": invocation}),
		testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) {
			port := network.MustParsePort("5432/tcp")
			hostConfig.PortBindings = network.PortMap{
				port: {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}},
			}
		}),
	)
	if err != nil {
		return err
	}
	runtimeState.container = postgresContainer
	runtimeState.containerID = postgresContainer.GetContainerID()
	if runtimeState.containerID == "" {
		return errors.New("task-owned container has no immutable ID")
	}
	if err = runtimeState.verifyLiveContainer(ctx); err != nil {
		return err
	}
	adminDSN, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return err
	}
	adminDSN, err = canonicalLoopbackDSN(adminDSN)
	if err != nil {
		return err
	}
	runtimeState.adminDSN = adminDSN
	return runtimeState.setupFixture(ctx)
}

func canonicalLoopbackDSN(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" {
		address, parseErr := netip.ParseAddr(host)
		if parseErr != nil || !address.IsLoopback() {
			return "", errors.New("Testcontainers PostgreSQL endpoint is not loopback")
		}
	}
	if parsed.Port() == "" {
		return "", errors.New("Testcontainers PostgreSQL endpoint has no allocated port")
	}
	parsed.Host = "127.0.0.1:" + parsed.Port()
	return parsed.String(), nil
}

func (runtimeState *rehearsalRuntime) verifyLiveContainer(ctx context.Context) error {
	if runtimeState.container == nil || runtimeState.container.GetContainerID() != runtimeState.containerID {
		return errors.New("task-owned container handle mismatch")
	}
	inspection, err := runtimeState.container.Inspect(ctx)
	if err != nil {
		return fmt.Errorf("inspect task-owned container: %w", err)
	}
	port := network.MustParsePort("5432/tcp")
	if inspection.ID != runtimeState.containerID || inspection.Config == nil || inspection.State == nil || !inspection.State.Running ||
		inspection.Config.Image != rehearsalImage || inspection.NetworkSettings == nil ||
		inspection.Config.Labels[rehearsalLabel] != "issue-364-rehearsal" ||
		inspection.Config.Labels[rehearsalLabel+".invocation"] != runtimeState.invocation {
		return errors.New("task-owned container inspection mismatch")
	}
	bindings := inspection.NetworkSettings.Ports[port]
	if len(bindings) != 1 || !bindings[0].HostIP.IsLoopback() || bindings[0].HostIP.String() != "127.0.0.1" {
		return errors.New("task-owned container is not bound to one IPv4 loopback endpoint")
	}
	if _, err = strconv.ParseUint(bindings[0].HostPort, 10, 16); err != nil {
		return errors.New("task-owned container has an invalid allocated port")
	}
	if runtimeState.adminDSN != "" {
		parsed, parseErr := url.Parse(runtimeState.adminDSN)
		if parseErr != nil || parsed.Port() != bindings[0].HostPort {
			return errors.New("task-owned container endpoint changed")
		}
	}
	return nil
}

func (runtimeState *rehearsalRuntime) setupFixture(ctx context.Context) error {
	admin, err := openDatabase(ctx, runtimeState.adminDSN)
	if err != nil {
		return err
	}
	defer admin.Close()

	roleNames := []string{"b2_inspect", "b2_schema", "b2_prepare", "b2_receipt"}
	runtimeState.roles = make(map[string]roleCredentials, len(roleNames))
	for _, name := range roleNames {
		password, nonceErr := newNonce()
		if nonceErr != nil {
			return nonceErr
		}
		runtimeState.roles[name] = roleCredentials{name: name, password: password}
		if _, err = admin.ExecContext(ctx, fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT", name, password)); err != nil {
			return fmt.Errorf("create synthetic rehearsal role: %w", err)
		}
	}
	if _, err = admin.ExecContext(ctx, "CREATE DATABASE metadata_b2 OWNER issue364_admin"); err != nil {
		return fmt.Errorf("create synthetic metadata database: %w", err)
	}

	sourceStatements := []string{
		`CREATE TABLE public.source_account (
			id BIGINT PRIMARY KEY, tenant_id BIGINT NOT NULL, platform VARCHAR(32) NOT NULL,
			label VARCHAR(128), profile_ref VARCHAR(256) NOT NULL, proxy_ref VARCHAR(256), login_url TEXT,
			status SMALLINT NOT NULL, deleted SMALLINT NOT NULL, last_verified_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`INSERT INTO public.source_account
			(id, tenant_id, platform, label, profile_ref, proxy_ref, login_url, status, deleted, last_verified_at, created_at, updated_at)
		 VALUES
			(7, 101, '1688', 'enabled', 'profile-7', 'proxy-7', 'https://example.invalid/7', 1, 0, '2026-09-08T00:00:00Z', '2026-09-01T00:00:00Z', '2026-09-02T00:00:00Z'),
			(8, 102, '1688', 'disabled', 'profile-8', NULL, NULL, 0, 0, NULL, '2026-09-01T00:00:00Z', '2026-09-02T00:00:00Z'),
			(9, 103, '1688', 'deleted', 'profile-9', NULL, NULL, 1, 1, NULL, '2026-09-01T00:00:00Z', '2026-09-02T00:00:00Z'),
			(99, 999, 'amazon', 'protected', 'amazon-profile', NULL, NULL, 1, 0, NULL, '2026-09-01T00:00:00Z', '2026-09-02T00:00:00Z')`,
		`CREATE TABLE public.b2_protected (id BIGINT PRIMARY KEY, value TEXT NOT NULL)`,
		`INSERT INTO public.b2_protected (id, value) VALUES (1, 'must-remain')`,
		`CREATE FUNCTION public.b2_hold_source_freeze() RETURNS void
			LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public
			AS $body$ BEGIN LOCK TABLE public.source_account IN SHARE MODE; END $body$`,
		`REVOKE ALL ON FUNCTION public.b2_hold_source_freeze() FROM PUBLIC`,
		`GRANT CONNECT ON DATABASE source_b2 TO b2_inspect, b2_schema, b2_prepare, b2_receipt`,
		`GRANT USAGE ON SCHEMA public TO b2_inspect, b2_prepare, b2_receipt`,
		`GRANT CREATE, USAGE ON SCHEMA public TO b2_schema`,
		`GRANT SELECT ON public.source_account TO b2_inspect, b2_prepare, b2_receipt`,
		`GRANT UPDATE ON public.source_account TO b2_prepare`,
		`GRANT EXECUTE ON FUNCTION public.b2_hold_source_freeze() TO b2_prepare, b2_receipt`,
		`GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO b2_inspect, b2_schema, b2_prepare, b2_receipt`,
	}
	if err = executeStatements(ctx, admin, sourceStatements); err != nil {
		return err
	}

	metadataDSN, err := dsnForRole(runtimeState.adminDSN, "metadata_b2", "issue364_admin", adminPassword(runtimeState.adminDSN))
	if err != nil {
		return err
	}
	metadataAdmin, err := openDatabase(ctx, metadataDSN)
	if err != nil {
		return err
	}
	defer metadataAdmin.Close()
	metadataStatements := []string{
		`CREATE SCHEMA projections`,
		`CREATE TABLE projections.org_metadata2 (org_id VARCHAR(128) NOT NULL, key VARCHAR(128) NOT NULL, value BYTEA NOT NULL, sequence BIGINT NOT NULL, owner_removed BOOLEAN NOT NULL)`,
		`INSERT INTO projections.org_metadata2 (org_id, key, value, sequence, owner_removed) VALUES
			('org-b2-enabled', 'yudao_tenant_id', convert_to('101', 'UTF8'), 1, false),
			('org-b2-disabled', 'yudao_tenant_id', convert_to('102', 'UTF8'), 2, false),
			('org-b2-deleted', 'yudao_tenant_id', convert_to('103', 'UTF8'), 3, false)`,
		`CREATE FUNCTION projections.b2_hold_metadata_freeze() RETURNS void
			LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, projections
			AS $body$ BEGIN LOCK TABLE projections.org_metadata2 IN SHARE MODE; END $body$`,
		`REVOKE ALL ON FUNCTION projections.b2_hold_metadata_freeze() FROM PUBLIC`,
		`GRANT CONNECT ON DATABASE metadata_b2 TO b2_inspect, b2_prepare, b2_receipt`,
		`GRANT USAGE ON SCHEMA projections TO b2_inspect, b2_prepare, b2_receipt`,
		`GRANT SELECT ON projections.org_metadata2 TO b2_inspect, b2_prepare, b2_receipt`,
		`GRANT EXECUTE ON FUNCTION projections.b2_hold_metadata_freeze() TO b2_prepare, b2_receipt`,
		`GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO b2_inspect, b2_prepare, b2_receipt`,
	}
	if err = executeStatements(ctx, metadataAdmin, metadataStatements); err != nil {
		return err
	}
	runtimeState.source, err = readDatabaseIdentity(ctx, admin)
	if err != nil {
		return err
	}
	runtimeState.metadata, err = readDatabaseIdentity(ctx, metadataAdmin)
	return err
}

func adminPassword(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.User == nil {
		return ""
	}
	password, _ := parsed.User.Password()
	return password
}

func executeStatements(ctx context.Context, db *sql.DB, statements []string) error {
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize synthetic PostgreSQL rehearsal: %w", err)
		}
	}
	return nil
}

func openDatabase(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, errors.New("invalid synthetic PostgreSQL configuration")
	}
	db.SetMaxOpenConns(8)
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, errors.New("synthetic PostgreSQL connection failed")
	}
	return db, nil
}

func readDatabaseIdentity(ctx context.Context, db *sql.DB) (databaseIdentity, error) {
	var identity databaseIdentity
	err := db.QueryRowContext(ctx, `SELECT (pg_control_system()).system_identifier::text, current_database(), oid::bigint FROM pg_database WHERE datname = current_database()`).
		Scan(&identity.SystemIdentifier, &identity.Database, &identity.OID)
	if err != nil {
		return databaseIdentity{}, fmt.Errorf("read synthetic PostgreSQL identity: %w", err)
	}
	if !validDatabaseIdentity(identity) {
		return databaseIdentity{}, errors.New("invalid synthetic PostgreSQL identity")
	}
	return identity, nil
}

func dsnForRole(base, database, username, password string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	parsed.User = url.UserPassword(username, password)
	parsed.Path = "/" + database
	result := parsed.String()
	if err = validateLoopbackDSN(result, database); err != nil {
		return "", err
	}
	return result, nil
}

func (runtimeState *rehearsalRuntime) grantFor(stage string) (stageGrant, error) {
	roleName := map[string]string{
		stageInspect:       "b2_inspect",
		stageInstallSchema: "b2_schema",
		stagePrepare:       "b2_prepare",
		stageReceiptRead:   "b2_receipt",
	}[stage]
	role, ok := runtimeState.roles[roleName]
	if !ok {
		return stageGrant{}, errors.New("fixed rehearsal role is missing")
	}
	sourceDSN, err := dsnForRole(runtimeState.adminDSN, runtimeState.source.Database, role.name, role.password)
	if err != nil {
		return stageGrant{}, err
	}
	metadataDSN := ""
	if stage != stageInstallSchema {
		metadataDSN, err = dsnForRole(runtimeState.adminDSN, runtimeState.metadata.Database, role.name, role.password)
		if err != nil {
			return stageGrant{}, err
		}
	}
	return stageGrant{
		Invocation:  runtimeState.invocation,
		Stage:       stage,
		Container:   runtimeState.containerID,
		Source:      runtimeState.source,
		Metadata:    runtimeState.metadata,
		SourceDSN:   sourceDSN,
		MetadataDSN: metadataDSN,
		SourceID:    "issue364/rehearsal/source",
		MetadataID:  "issue364/rehearsal/metadata",
		ProfileRoot: runtimeState.storage.ProfileRoot,
		ReceiptPath: runtimeState.storage.ReceiptPath,
	}, nil
}

func (runtimeState *rehearsalRuntime) execute(ctx context.Context, scenario rehearsalScenario, stderr io.Writer) (int, string) {
	if scenario == scenarioInspectPermissionDenied {
		if err := runtimeState.executeAsAdmin(ctx, runtimeState.source.Database, `REVOKE SELECT ON public.source_account FROM b2_inspect`); err != nil {
			return ExitDependencyFailure, ""
		}
	}
	inspectGrant, err := runtimeState.grantFor(stageInspect)
	if err != nil {
		return ExitDependencyFailure, ""
	}
	inspectResult, err := runtimeState.runStage(ctx, inspectGrant)
	if err != nil || inspectResult.Code != ExitSuccess || inspectResult.Preflight == nil {
		fmt.Fprintln(stderr, "isolated inspect failed")
		return stageFailureCode(inspectResult, err), ""
	}
	pinned := *inspectResult.Preflight

	if scenario == scenarioSchemaPermissionDenied {
		if err = runtimeState.executeAsAdmin(ctx, runtimeState.source.Database, `REVOKE CREATE ON SCHEMA public FROM b2_schema`); err != nil {
			return ExitDependencyFailure, ""
		}
	}
	schemaGrant, err := runtimeState.grantFor(stageInstallSchema)
	if err != nil {
		return ExitDependencyFailure, ""
	}
	schemaResult, err := runtimeState.runStage(ctx, schemaGrant)
	if err != nil || schemaResult.Code != ExitSuccess {
		fmt.Fprintln(stderr, "explicit isolated schema install failed")
		return stageFailureCode(schemaResult, err), ""
	}
	if err = runtimeState.grantPreparedTables(ctx); err != nil {
		fmt.Fprintln(stderr, "isolated prepared-table role setup failed")
		return ExitDependencyFailure, ""
	}
	if scenario == scenarioMappingDrift {
		if err = runtimeState.executeAsAdmin(ctx, runtimeState.metadata.Database, `DELETE FROM projections.org_metadata2 WHERE org_id = 'org-b2-disabled'`); err != nil {
			return ExitDependencyFailure, ""
		}
	}
	if scenario == scenarioPreparePermissionDenied {
		if err = runtimeState.executeAsAdmin(ctx, runtimeState.source.Database, `REVOKE INSERT ON public.organization_source_accounts FROM b2_prepare`); err != nil {
			return ExitDependencyFailure, ""
		}
	}

	prepareGrant, err := runtimeState.grantFor(stagePrepare)
	if err != nil {
		return ExitDependencyFailure, ""
	}
	prepareGrant.Preflight = pinned
	prepareGrant.IdempotencyKey = "issue364-b2-rehearsal-v1"
	prepareGrant.DropResponseAfterWrite = scenario == scenarioCommitResponseLoss || scenario == scenarioCommitLossTargetDrift
	prepareResult, prepareErr := runtimeState.runStage(ctx, prepareGrant)
	unknown := prepareErr != nil || prepareResult.Code == ExitOutcomeUnknown
	if !unknown && prepareResult.Code != ExitSuccess {
		fmt.Fprintf(stderr, "isolated prepare rejected (%s)\n", prepareResult.Status)
		return stageFailureCode(prepareResult, prepareErr), ""
	}
	if (scenario == scenarioTargetDrift && !unknown) || scenario == scenarioCommitLossTargetDrift {
		if err = runtimeState.executeAsAdmin(ctx, runtimeState.source.Database, `UPDATE public.organization_source_accounts SET organization_id = 'org-b2-tampered' WHERE id = 7`); err != nil {
			return ExitDependencyFailure, ""
		}
	}
	if scenario == scenarioReceiptPermissionDenied && !unknown {
		if err = runtimeState.executeAsAdmin(ctx, runtimeState.source.Database, `REVOKE SELECT ON public.source_account_ownership_migration_receipts FROM b2_receipt`); err != nil {
			return ExitDependencyFailure, ""
		}
	}

	readGrant, err := runtimeState.grantFor(stageReceiptRead)
	if err != nil {
		return ExitDependencyFailure, ""
	}
	readGrant.Preflight = pinned
	readGrant.IdempotencyKey = prepareGrant.IdempotencyKey
	readResult, readErr := runtimeState.runStage(ctx, readGrant)
	if readErr != nil {
		fmt.Fprintln(stderr, "fresh-process prepared receipt read could not confirm outcome")
		return ExitOutcomeUnknown, ""
	}
	if readResult.Code == ExitNotPrepared {
		fmt.Fprintln(stderr, "fresh-process prepared receipt read confirmed not prepared")
		return ExitNotPrepared, ""
	}
	if readResult.Code != ExitSuccess || readResult.Prepared == nil {
		fmt.Fprintln(stderr, "fresh-process prepared receipt read failed")
		if unknown {
			return ExitOutcomeUnknown, ""
		}
		if readResult.Code == ExitConflictOrDrift {
			return ExitConflictOrDrift, ""
		}
		return ExitOutcomeUnknown, ""
	}
	if !unknown && prepareResult.Prepared != nil && prepareResult.Prepared.RequestSHA256 != readResult.Prepared.RequestSHA256 {
		fmt.Fprintln(stderr, "fresh-process receipt identity mismatch")
		return ExitConflictOrDrift, ""
	}
	if err = runtimeState.verifyProtectedState(ctx); err != nil {
		fmt.Fprintln(stderr, "isolated protected state changed")
		return ExitConflictOrDrift, ""
	}
	summary := fmt.Sprintf("inspect=PASS schema_install=PASS prepare=PASS new_process_receipt_read=PASS accounts=%d sha256=%s", readResult.Prepared.AccountCount, readResult.Prepared.RequestSHA256)
	if unknown {
		summary += " recovered_unknown=PASS"
	}
	return ExitSuccess, summary
}

func (runtimeState *rehearsalRuntime) executeAsAdmin(ctx context.Context, database, statement string) error {
	dsn, err := dsnForRole(runtimeState.adminDSN, database, "issue364_admin", adminPassword(runtimeState.adminDSN))
	if err != nil {
		return err
	}
	db, err := openDatabase(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, statement)
	return err
}

func stageFailureCode(result stageResult, err error) int {
	if result.Code != 0 {
		return result.Code
	}
	if err != nil {
		return ExitDependencyFailure
	}
	return ExitDependencyFailure
}

func (runtimeState *rehearsalRuntime) grantPreparedTables(ctx context.Context) error {
	admin, err := openDatabase(ctx, runtimeState.adminDSN)
	if err != nil {
		return err
	}
	defer admin.Close()
	return executeStatements(ctx, admin, []string{
		`GRANT USAGE ON SCHEMA public TO b2_prepare, b2_receipt`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON public.organization_source_accounts, public.source_account_ownership_migration_receipts TO b2_prepare`,
		`GRANT SELECT ON public.organization_source_accounts, public.source_account_ownership_migration_receipts TO b2_receipt`,
	})
}

func (runtimeState *rehearsalRuntime) verifyProtectedState(ctx context.Context) error {
	admin, err := openDatabase(ctx, runtimeState.adminDSN)
	if err != nil {
		return err
	}
	defer admin.Close()
	var protected string
	if err = admin.QueryRowContext(ctx, `SELECT value FROM public.b2_protected WHERE id = 1`).Scan(&protected); err != nil || protected != "must-remain" {
		return errors.New("protected SQL fixture changed")
	}
	var totalAccounts, exactAccounts int
	if err = admin.QueryRowContext(ctx, `SELECT count(*), count(*) FILTER (WHERE
		(id = 7 AND tenant_id = 101 AND platform = '1688' AND label = 'enabled' AND profile_ref = 'profile-7' AND status = 1 AND deleted = 0) OR
		(id = 8 AND tenant_id = 102 AND platform = '1688' AND label = 'disabled' AND profile_ref = 'profile-8' AND status = 0 AND deleted = 0) OR
		(id = 9 AND tenant_id = 103 AND platform = '1688' AND label = 'deleted' AND profile_ref = 'profile-9' AND status = 1 AND deleted = 1) OR
		(id = 99 AND tenant_id = 999 AND platform = 'amazon' AND label = 'protected' AND profile_ref = 'amazon-profile' AND status = 1 AND deleted = 0))
		FROM public.source_account`).Scan(&totalAccounts, &exactAccounts); err != nil || totalAccounts != 4 || exactAccounts != 4 {
		return errors.New("synthetic source rows changed")
	}
	for _, account := range [][2]string{{"101", "7"}, {"102", "8"}, {"103", "9"}} {
		marker, readErr := os.ReadFile(filepath.Join(runtimeState.storage.ProfileRoot, account[0], account[1], "profile.marker"))
		if readErr != nil || string(marker) != "synthetic-b2-profile\n" {
			return errors.New("synthetic profile content changed")
		}
	}
	return nil
}

func (runtimeState *rehearsalRuntime) runStage(ctx context.Context, grant stageGrant) (stageResult, error) {
	if err := runtimeState.verifyLiveContainer(ctx); err != nil {
		return stageResult{}, err
	}
	executable, err := os.Executable()
	if err != nil {
		return stageResult{}, err
	}
	command := exec.CommandContext(ctx, executable, internalStageFlagPrefix+grant.Stage)
	stdin, err := command.StdinPipe()
	if err != nil {
		return stageResult{}, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return stageResult{}, err
	}
	command.Stderr = io.Discard
	if err = command.Start(); err != nil {
		return stageResult{}, err
	}
	reader := bufio.NewReaderSize(stdout, maxProtocolFrameBytes+2)
	var challenge stageChallenge
	if err = decodeFrame(reader, &challenge); err != nil {
		_ = stdin.Close()
		_ = command.Wait()
		return stageResult{}, fmt.Errorf("child admission challenge failed")
	}
	if challenge.Stage != grant.Stage || challenge.ChildPID != command.Process.Pid || challenge.ParentPID != os.Getpid() || !boundedToken(challenge.Nonce, 64) {
		_ = stdin.Close()
		_ = command.Wait()
		return stageResult{}, errors.New("child admission challenge mismatch")
	}
	if err = runtimeState.verifyLiveContainer(ctx); err != nil {
		_ = stdin.Close()
		_ = command.Wait()
		return stageResult{}, err
	}
	grant.Nonce = challenge.Nonce
	grant.ChildPID = challenge.ChildPID
	grant.ParentPID = challenge.ParentPID
	if err = encodeFrame(stdin, grant); err != nil {
		_ = stdin.Close()
		_ = command.Wait()
		return stageResult{}, err
	}
	var result stageResult
	decodeErr := decodeFrame(reader, &result)
	_ = stdin.Close()
	waitErr := command.Wait()
	if decodeErr != nil {
		var exitError *exec.ExitError
		if errors.As(waitErr, &exitError) {
			code := exitError.ExitCode()
			if code == ExitInvalidInput || code == ExitAdmissionDenied || code == ExitOutcomeUnknown ||
				code == ExitNotPrepared || code == ExitConflictOrDrift || code == ExitDependencyFailure {
				return stageResult{Code: code, Status: "child-rejected-before-result"}, nil
			}
		}
		if grant.Stage == stagePrepare {
			return stageResult{Code: ExitOutcomeUnknown, Status: "unknown"}, fmt.Errorf("prepare child response unavailable")
		}
		return stageResult{}, fmt.Errorf("child stage response unavailable")
	}
	if result.Invocation != grant.Invocation || result.Stage != grant.Stage || result.Nonce != grant.Nonce {
		return stageResult{}, errors.New("child stage response identity mismatch")
	}
	if waitErr != nil {
		if result.Code == 0 {
			return stageResult{}, errors.New("child stage exited inconsistently")
		}
		return result, nil
	}
	if result.Code != ExitSuccess {
		return result, nil
	}
	return result, nil
}

func (runtimeState *rehearsalRuntime) cleanup() error {
	var cleanupErrors []error
	if runtimeState.container != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		if err := runtimeState.container.Terminate(ctx); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
		cancel()
	}
	if runtimeState.storage.Root != "" {
		if err := runtimeState.storage.cleanup(); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	return errors.Join(cleanupErrors...)
}
