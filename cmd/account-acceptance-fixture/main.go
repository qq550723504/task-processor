package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"task-processor/internal/zitadelprovision"
)

const (
	stateFile               = "/state/manifest.json"
	managementTokenFile     = "/zitadel/iac/iam-owner.pat"
	runtimeDirectory        = "/runtime"
	defaultOrganizationAID  = "910000000000000001"
	defaultOrganizationBID  = "910000000000000002"
	acceptanceOrganizationA = "ListingKit Acceptance Organization A"
	acceptanceOrganizationB = "ListingKit Acceptance Organization B"
	viewerLogin             = "local-acceptance-viewer@localhost"
	insufficientLogin       = "local-acceptance-insufficient@localhost"
)

type manifest struct {
	SchemaVersion      int                    `json:"schemaVersion"`
	Status             string                 `json:"status"`
	IssuerURL          string                 `json:"issuerUrl"`
	HomeOrganizationID string                 `json:"homeOrganizationId"`
	ProjectID          string                 `json:"projectId"`
	OperatorUserID     string                 `json:"operatorUserId"`
	Organizations      []manifestOrganization `json:"organizations"`
	Identities         []manifestIdentity     `json:"identities"`
}

type manifestOrganization struct {
	Name     string   `json:"name"`
	ID       string   `json:"id"`
	RoleKeys []string `json:"operatorRoleKeys"`
}

type manifestIdentity struct {
	Name             string   `json:"name"`
	Login            string   `json:"login"`
	UserID           string   `json:"userId"`
	OrganizationName string   `json:"organization"`
	RoleKeys         []string `json:"roleKeys"`
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	identityPort := strings.TrimSpace(os.Getenv("ACCOUNT_IDENTITY_PORT"))
	if identityPort == "" {
		return errors.New("ACCOUNT_IDENTITY_PORT is required")
	}
	issuerURL := "https://localhost:" + identityPort
	managementToken, err := readRequiredFile(managementTokenFile)
	if err != nil {
		return fmt.Errorf("read management token: %w", err)
	}
	projectID, err := readRuntime("project-id")
	if err != nil {
		return err
	}
	operatorUserID, err := readRuntime("bootstrap-user-id")
	if err != nil {
		return err
	}
	viewerUserID, err := readRuntime("viewer-user-id")
	if err != nil {
		return err
	}
	insufficientUserID, err := readRuntime("insufficient-user-id")
	if err != nil {
		return err
	}
	homeOrganizationID, err := readRuntime("signup-org-id")
	if err != nil {
		return err
	}

	previous, err := readManifest()
	if err != nil {
		return err
	}
	if previous.Status != "" && (previous.IssuerURL != issuerURL || previous.ProjectID != projectID || previous.HomeOrganizationID != homeOrganizationID) {
		return errors.New("existing fixture manifest belongs to a different isolated runtime")
	}
	organizationIDs := make([]string, 2)
	if len(previous.Organizations) == 2 {
		organizationIDs[0] = strings.TrimSpace(previous.Organizations[0].ID)
		organizationIDs[1] = strings.TrimSpace(previous.Organizations[1].ID)
	} else {
		organizationIDs[0] = defaultOrganizationAID
		organizationIDs[1] = defaultOrganizationBID
	}
	client, err := zitadelprovision.NewLoopbackOnlyHTTPClient(issuerURL)
	if err != nil {
		return err
	}
	result, err := zitadelprovision.ProvisionLocalMultiOrganizationAcceptance(ctx, zitadelprovision.Config{
		IssuerURL: issuerURL, ManagementToken: managementToken, OrgID: homeOrganizationID,
		ProjectID: projectID, AcceptanceOrganizationIDs: organizationIDs, HTTPClient: client,
	}, zitadelprovision.MultiOrganizationAcceptanceSpec{
		UserID: operatorUserID,
		Organizations: []zitadelprovision.AcceptanceOrganizationSpec{
			{Name: acceptanceOrganizationA, RoleKeys: []string{"listingkit_admin"}, ProjectRoleKeys: []string{"listingkit_admin", "listingkit_operator", "listingkit_viewer"}},
			{Name: acceptanceOrganizationB, RoleKeys: []string{"listingkit_viewer"}},
		},
		AdditionalAuthorizations: []zitadelprovision.AcceptanceAuthorizationSpec{
			{UserID: viewerUserID, OrganizationName: acceptanceOrganizationA, RoleKeys: []string{"listingkit_viewer"}},
			{UserID: insufficientUserID, OrganizationName: acceptanceOrganizationA, RoleKeys: []string{"listingkit_operator"}},
		},
	})
	if err != nil {
		return fmt.Errorf("provision acceptance organizations and grants: %w", err)
	}
	if len(result.Organizations) != 2 || len(result.AdditionalAuthorizations) != 2 {
		return errors.New("acceptance provisioner returned an incomplete fixture")
	}
	output := manifest{
		SchemaVersion:      1,
		Status:             "ready",
		IssuerURL:          issuerURL,
		HomeOrganizationID: homeOrganizationID,
		ProjectID:          projectID,
		OperatorUserID:     operatorUserID,
		Organizations: []manifestOrganization{
			{Name: result.Organizations[0].OrganizationName, ID: result.Organizations[0].OrganizationID, RoleKeys: result.Organizations[0].RoleKeys},
			{Name: result.Organizations[1].OrganizationName, ID: result.Organizations[1].OrganizationID, RoleKeys: result.Organizations[1].RoleKeys},
		},
		Identities: []manifestIdentity{
			{Name: "operator", Login: "local-bootstrap-operator@localhost", UserID: operatorUserID, OrganizationName: acceptanceOrganizationA + " / " + acceptanceOrganizationB, RoleKeys: []string{"listingkit_admin", "listingkit_viewer"}},
			{Name: "viewer", Login: viewerLogin, UserID: viewerUserID, OrganizationName: acceptanceOrganizationA, RoleKeys: []string{"listingkit_viewer"}},
			{Name: "insufficient-role", Login: insufficientLogin, UserID: insufficientUserID, OrganizationName: acceptanceOrganizationA, RoleKeys: []string{"listingkit_operator"}},
		},
	}
	if err := writeManifest(output); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "status=ready organizations=2 identities=3")
	return nil
}

func readRuntime(name string) (string, error) {
	value, err := readRequiredFile(filepath.Join(runtimeDirectory, name))
	if err != nil {
		return "", fmt.Errorf("read runtime %s: %w", name, err)
	}
	return value, nil
}

func readRequiredFile(path string) (string, error) {
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

func readManifest() (manifest, error) {
	data, err := os.ReadFile(stateFile)
	if errors.Is(err, os.ErrNotExist) {
		return manifest{}, nil
	}
	if err != nil {
		return manifest{}, fmt.Errorf("read existing fixture manifest: %w", err)
	}
	var value manifest
	if err := json.Unmarshal(data, &value); err != nil {
		return manifest{}, fmt.Errorf("decode existing fixture manifest: %w", err)
	}
	if value.SchemaVersion != 1 || value.Status != "ready" || len(value.Organizations) != 2 {
		return manifest{}, errors.New("existing fixture manifest is not a valid version 1 ready manifest")
	}
	return value, nil
}

func writeManifest(value manifest) error {
	data, err := encodeManifest(value)
	if err != nil {
		return fmt.Errorf("encode fixture manifest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o700); err != nil {
		return fmt.Errorf("create fixture state directory: %w", err)
	}
	temporary := stateFile + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write fixture manifest: %w", err)
	}
	if err := os.Rename(temporary, stateFile); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish fixture manifest: %w", err)
	}
	return nil
}

func encodeManifest(value manifest) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
