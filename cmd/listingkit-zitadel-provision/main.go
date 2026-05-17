package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"task-processor/internal/zitadelprovision"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "listingkit-zitadel-provision: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := zitadelprovision.Config{}
	flag.StringVar(&cfg.IssuerURL, "issuer-url", env("ZITADEL_ISSUER_URL"), "ZITADEL issuer URL")
	flag.StringVar(&cfg.ManagementToken, "token", env("ZITADEL_MANAGEMENT_TOKEN"), "ZITADEL Management API bearer token")
	flag.StringVar(&cfg.OrgID, "org-id", env("ZITADEL_ORG_ID"), "optional ZITADEL organization id")
	flag.StringVar(&cfg.ProjectID, "project-id", env("LISTINGKIT_ZITADEL_PROJECT_ID"), "existing ZITADEL project id")
	flag.StringVar(&cfg.ProjectName, "project-name", envDefault("LISTINGKIT_ZITADEL_PROJECT_NAME", "ListingKit"), "ZITADEL project name")
	flag.BoolVar(&cfg.CreateProject, "create-project", false, "create the ZITADEL project when project-name is not found")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := zitadelprovision.Provision(ctx, cfg)
	if err != nil {
		return err
	}
	printResult(result)
	return nil
}

func printResult(result zitadelprovision.Result) {
	fmt.Printf("ListingKit ZITADEL project: %s (%s)\n", result.ProjectName, result.ProjectID)
	fmt.Println()
	fmt.Println("Roles:")
	for _, role := range result.Roles {
		status := "created"
		if role.Existed {
			status = "exists"
		}
		fmt.Printf("- %s: %s\n", role.Role.Key, status)
	}
	fmt.Println()
	fmt.Println("Recommended environment:")
	fmt.Printf("LISTINGKIT_ZITADEL_PROJECT_ID=%s\n", result.ProjectID)
	fmt.Printf("LISTINGKIT_ZITADEL_ALLOWED_ROLES=%s\n", strings.Join(result.AllowedRoles, ","))
	fmt.Printf("ZITADEL_SCOPES=%s\n", strings.Join(result.RecommendedScopes, " "))
}

func env(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func envDefault(name string, fallback string) string {
	if value := env(name); value != "" {
		return value
	}
	return fallback
}
