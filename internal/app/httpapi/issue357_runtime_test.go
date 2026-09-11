//go:build issue357

package httpapi

// Opt-in local composition. Every request uses current production modules;
// the harness itself is absent from production builds and default registrations.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/zitadelprovision"
)

type issue357Config struct {
	RunID                 string `json:"runId"`
	Issuer                string `json:"issuer"`
	IssuerPort            int    `json:"issuerPort"`
	WebOrigin             string `json:"webOrigin"`
	GoPort                int    `json:"goPort"`
	DatabasePort          int    `json:"databasePort"`
	DatabaseHost          string `json:"databaseHost"`
	DatabaseName          string `json:"databaseName"`
	DatabaseUser          string `json:"databaseUser"`
	DatabasePassword      string `json:"databasePassword"`
	ReaderPassword        string `json:"readerPassword,omitempty"`
	SourceRuntimePassword string `json:"sourceRuntimePassword,omitempty"`
	RuntimeMode           string `json:"runtimeMode,omitempty"`
	ProjectID             string `json:"projectId"`
	APIClientID           string `json:"apiClientId"`
	APIClientSecret       string `json:"apiClientSecret"`
	OrganizationA         string `json:"organizationA"`
	OrganizationB         string `json:"organizationB"`
	OrganizationC         string `json:"organizationC"`
	ManagementToken       string `json:"managementToken,omitempty"`
}

type issue357Allocation struct {
	RunID         string            `json:"runId"`
	ProjectID     string            `json:"projectId"`
	APIClientID   string            `json:"apiClientId"`
	Ports         map[string]int    `json:"ports"`
	Origins       map[string]string `json:"origins"`
	Organizations map[string]struct {
		ID string `json:"id"`
	} `json:"organizations"`
}

func (c issue357Config) validateAssigned(m issue357Allocation) error {
	if c.validate() != nil || c.RunID != m.RunID || c.ProjectID != m.ProjectID || c.Issuer != m.Origins["issuer"] || c.WebOrigin != m.Origins["web"] || c.IssuerPort != m.Ports["issuer"] || c.GoPort != m.Ports["go"] || c.DatabasePort != m.Ports["database"] || c.APIClientID != "" && c.APIClientID != m.APIClientID {
		return errors.New("ASSIGNED_RESOURCE_MISMATCH")
	}
	if c.OrganizationA != m.Organizations["A"].ID || c.OrganizationB != m.Organizations["B"].ID || c.OrganizationC != m.Organizations["C"].ID {
		return errors.New("ASSIGNED_RESOURCE_MISMATCH")
	}
	return nil
}

func (c issue357Config) validate() error {
	id, err := uuid.Parse(c.RunID)
	if err != nil || id.String() != c.RunID || id.Version() != 4 {
		return errors.New("INVALID_RUN")
	}
	if c.Issuer != fmt.Sprintf("http://localhost:%d", c.IssuerPort) || c.DatabaseHost != "127.0.0.1" || c.DatabaseName != "issue357" || (c.DatabaseUser != "issue357" && c.DatabaseUser != "commercial_reader" && c.DatabaseUser != "source_account_runtime") || (c.RuntimeMode != "" && c.RuntimeMode != "current-application") {
		return errors.New("INVALID_RUN")
	}
	seen := map[int]bool{}
	for _, p := range []int{c.IssuerPort, c.GoPort, c.DatabasePort} {
		if p <= 1024 || p > 65535 || seen[p] {
			return errors.New("INVALID_RUN")
		}
		seen[p] = true
	}
	return nil
}
func issue357ReadConfig(t *testing.T) (issue357Config, string) {
	t.Helper()
	path := os.Getenv("ISSUE357_INPUT_FILE")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "private input required")
	var cfg issue357Config
	require.NoError(t, json.Unmarshal(data, &cfg))
	require.NoError(t, cfg.validate())
	directory := filepath.Join(os.TempDir(), "task-processor-issue357", cfg.RunID)
	require.Equal(t, filepath.Clean(directory), filepath.Dir(path), "input must be run owned")
	// The manifest holds public allocations only; bootstrap/password material
	// lives in separate owner files and is never loaded by the serving process.
	allocationBytes, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	require.NoError(t, err)
	var allocation issue357Allocation
	require.NoError(t, json.Unmarshal(allocationBytes, &allocation))
	require.NoError(t, cfg.validateAssigned(allocation))
	return cfg, directory
}
func issue357Write(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
}

func TestIssue357Provision(t *testing.T) {
	c, dir := issue357ReadConfig(t)
	client, err := zitadelprovision.NewLoopbackOnlyHTTPClient(c.Issuer)
	require.NoError(t, err)
	disabled := false
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	result, err := zitadelprovision.ProvisionLocalApplications(ctx, zitadelprovision.Config{
		IssuerURL: c.Issuer, OrgID: c.OrganizationA, ProjectID: c.ProjectID, ProjectName: "issue357-" + c.RunID,
		ManagementToken: c.ManagementToken, CreateProject: true, HasProjectCheck: &disabled, ProjectRoleCheck: &disabled, HTTPClient: client,
	}, zitadelprovision.LocalApplicationConfig{
		APIName: "Issue357 API", OIDCName: "Issue357 Console", LocalOrigin: c.WebOrigin, EnableRefreshToken: true,
		RedirectURIs: []string{c.WebOrigin + "/api/auth/callback/zitadel"}, PostLogoutRedirectURIs: []string{c.WebOrigin},
	})
	// Do not log returned provider errors or credentials from setup.
	require.True(t, err == nil, "provider application setup failed; no response payload logged")
	issue357Write(t, filepath.Join(dir, "applications.json"), result)
	t.Log("official application provisioning completed")
}

func issue357Database(t *testing.T, c issue357Config) *gorm.DB {
	t.Helper()
	require.NoError(t, c.validate())
	// All identifiers and passwords are generated by the launcher, never caller DSNs.
	dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=%s password=%s dbname=issue357 sslmode=disable", c.DatabasePort, c.DatabaseUser, c.DatabasePassword)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.True(t, err == nil, "private database connection failed")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(2)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return db
}
func TestIssue357Seed(t *testing.T) {
	c, dir := issue357ReadConfig(t)
	require.Equal(t, "issue357", c.DatabaseUser)
	db := issue357Database(t, c)
	var existing int64
	require.NoError(t, db.Raw("SELECT count(*) FROM pg_tables WHERE schemaname='public'").Scan(&existing).Error)
	require.Zero(t, existing, "partial or existing database must never be reseeded")
	require.NoError(t, listingsubscription.AutoMigrateRepository(db))
	repo := listingsubscription.NewGormRepository(db)
	service, err := listingsubscription.NewService(repo)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = service.UpsertPlan(ctx, listingsubscription.PlanInput{Code: "paid_pilot", Name: "Issue357 synthetic pilot", Active: true, Modules: []listingsubscription.PlanModuleInput{{ModuleCode: listingsubscription.ModuleListingKit, Limits: map[string]int{"listingkit_generations_succeeded": 0}}}}, "issue357-setup")
	require.NoError(t, err)
	ledger := listingsubscription.NewGormUsageLedger(repo)
	now := time.Now().UTC()
	for i, org := range []string{c.OrganizationB, c.OrganizationC} {
		require.NotEmpty(t, org)
		_, err = service.ApplyPlan(ctx, org, listingsubscription.PlanApplyInput{PlanCode: "paid_pilot", Status: listingsubscription.StatusActive}, "issue357-setup")
		require.NoError(t, err)
		for n := 0; n <= i; n++ {
			key := uuid.NewString()
			reservation, e := ledger.Reserve(ctx, listingsubscription.ReserveUsageInput{TenantID: org, ModuleCode: listingsubscription.ModuleListingKit, Metric: "listingkit_generations_succeeded", Quantity: 1, PeriodKey: now.Format("2006-01"), SourceType: "issue357_setup", SourceID: key, IdempotencyKey: key, OccurredAt: now})
			require.NoError(t, e)
			_, e = ledger.Commit(ctx, reservation.Event.EventID)
			require.NoError(t, e)
		}
	}
	require.NotContains(t, c.ReaderPassword, "'")
	require.NoError(t, db.Exec("CREATE ROLE commercial_reader LOGIN PASSWORD '"+c.ReaderPassword+"'").Error)
	require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA public TO commercial_reader").Error)
	if c.RuntimeMode == "current-application" {
		require.NoError(t, db.Exec("GRANT SELECT ON TABLE public.saas_tenant_subscriptions, public.saas_plans, public.saas_tenant_entitlements, public.saas_usage_buckets TO commercial_reader").Error)
	} else {
		require.NoError(t, db.Exec("GRANT SELECT ON ALL TABLES IN SCHEMA public TO commercial_reader").Error)
	}
	require.NoError(t, db.Exec("ALTER ROLE commercial_reader SET default_transaction_read_only=on").Error)
	require.NoError(t, db.Exec("ALTER ROLE commercial_reader SET statement_timeout='10s'").Error)
	snapshot := issue357Snapshot(t, db)
	issue357Write(t, filepath.Join(dir, "baseline.json"), map[string]any{"digest": snapshot, "setupWrites": true})
	t.Log("synthetic setup complete; baseline captured")
}

func TestIssue357GrantSourceAccountRuntime(t *testing.T) {
	c, _ := issue357ReadConfig(t)
	require.Equal(t, "current-application", c.RuntimeMode)
	require.Equal(t, "issue357", c.DatabaseUser)
	require.NotEmpty(t, c.SourceRuntimePassword)
	require.NotContains(t, c.SourceRuntimePassword, "'")
	db := issue357Database(t, c)
	require.NoError(t, db.Exec("REVOKE CREATE ON SCHEMA public FROM PUBLIC").Error)
	require.NoError(t, db.Exec("REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC").Error)
	require.NoError(t, db.Exec("CREATE ROLE source_account_runtime LOGIN PASSWORD '"+c.SourceRuntimePassword+"'").Error)
	require.NoError(t, db.Exec("GRANT CONNECT ON DATABASE issue357 TO source_account_runtime").Error)
	require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA public TO source_account_runtime").Error)
	require.NoError(t, db.Exec("GRANT SELECT, INSERT, UPDATE ON TABLE public.source_account_resources TO source_account_runtime").Error)
	require.NoError(t, db.Exec("GRANT SELECT, INSERT ON TABLE public.source_account_operations TO source_account_runtime").Error)
	require.NoError(t, db.Exec("ALTER ROLE source_account_runtime SET statement_timeout='10s'").Error)
	var resourceCount, operationCount int64
	require.NoError(t, db.Table("source_account_resources").Count(&resourceCount).Error)
	require.NoError(t, db.Table("source_account_operations").Count(&operationCount).Error)
	require.Zero(t, resourceCount)
	require.Zero(t, operationCount)
	t.Log("source account runtime role granted only current SA1 DML permissions")
}

func TestIssue357SourceAccountSnapshot(t *testing.T) {
	c, dir := issue357ReadConfig(t)
	require.Equal(t, "current-application", c.RuntimeMode)
	require.Equal(t, "issue357", c.DatabaseUser)
	db := issue357Database(t, c)
	var resources, operations int64
	require.NoError(t, db.Table("source_account_resources").Count(&resources).Error)
	require.NoError(t, db.Table("source_account_operations").Count(&operations).Error)
	issue357Write(t, filepath.Join(dir, "source-account-snapshot.json"), map[string]any{
		"resources": resources, "operations": operations, "observedAt": time.Now().UTC(),
	})
}
func issue357Snapshot(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw("SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename LIKE 'saas_%' ORDER BY tablename").Scan(&tables).Error)
	require.NotEmpty(t, tables)
	var rows []string
	for _, table := range tables {
		var values []string
		require.NoError(t, db.Raw(fmt.Sprintf(`SELECT row_to_json(t)::text || ':' || xmin::text FROM %q t`, table)).Scan(&values).Error)
		sort.Strings(values)
		rows = append(rows, table+":"+strings.Join(values, "|"))
	}
	sum := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return hex.EncodeToString(sum[:])
}
func TestIssue357Snapshot(t *testing.T) {
	c, dir := issue357ReadConfig(t)
	if c.RuntimeMode == "current-application" {
		require.Equal(t, "issue357", c.DatabaseUser)
	} else {
		require.Equal(t, "commercial_reader", c.DatabaseUser)
	}
	db := issue357Database(t, c)
	snapshot := issue357Snapshot(t, db)
	var baseline struct {
		Digest string `json:"digest"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "baseline.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &baseline))
	issue357Write(t, filepath.Join(dir, "zero-write.json"), map[string]any{"before": baseline.Digest, "after": snapshot, "passed": baseline.Digest == snapshot, "checkedAt": time.Now().UTC()})
	require.Equal(t, baseline.Digest, snapshot, "commercial writes detected")
	t.Log("ZERO_WRITE: all saas values and xmin unchanged")
}
func TestIssue357Serve(t *testing.T) {
	c, dir := issue357ReadConfig(t)
	require.Equal(t, "commercial_reader", c.DatabaseUser)
	require.Empty(t, c.ManagementToken)
	require.Empty(t, c.ReaderPassword)
	cfg := &config.Config{Workbench: config.WorkbenchConfig{Enabled: true}, Database: &config.DatabaseConfig{Host: c.DatabaseHost, Port: c.DatabasePort, User: c.DatabaseUser, Password: c.DatabasePassword, Database: c.DatabaseName, MaxConnections: 4, MaxIdleConnections: 2}}
	cfg.ListingKit.Zitadel = config.ListingKitZitadelConfig{IssuerURL: c.Issuer, AuthorizationAPIURL: c.Issuer, ProjectID: c.ProjectID, ClientID: c.APIClientID, ClientSecret: c.APIClientSecret}
	log := logrus.New()
	log.SetOutput(io.Discard)
	wb, err := buildDefaultWorkbenchContextModule(cfg, log)
	require.NoError(t, err)
	commercial, err := buildCommercialReadModule(cfg, log)
	require.NoError(t, err)
	defer func() { require.NoError(t, commercial.closer()) }()
	reg := kernelmodule.NewRegistry()
	require.NoError(t, wb.module.Register(reg))
	require.NoError(t, commercial.module.Register(reg))
	require.Len(t, reg.Routes(), 5, "only current account, organization context and commercial routes")
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", c.GoPort, reg.Routes(), *wb.authDependencies)
	server.ReadHeaderTimeout = 5 * time.Second
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(c.GoPort)))
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	issue357Write(t, filepath.Join(dir, "go-ready.json"), map[string]any{"port": c.GoPort, "realProvider": true})
	for {
		if _, e := os.Stat(filepath.Join(dir, "stop-go")); e == nil {
			break
		}
		select {
		case e := <-done:
			t.Fatalf("server exited: %v", e)
		case <-time.After(200 * time.Millisecond):
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, server.Shutdown(ctx))
	require.ErrorIs(t, <-done, http.ErrServerClosed)
	t.Log("Go server drained and stopped")
}
