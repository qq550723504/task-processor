package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	productlistingschemamigrate "task-processor/internal/app/runtime/productlistingschemamigrate"
)

func main() {
	configPath := flag.String("config", "config/config-dev.yaml", "config file path")
	grantImageAgentRuntime := flag.Bool("grant-image-agent-runtime", false, "grant only the pre-provisioned image_agent_runtime role on the existing ImageAgent owner database; no schema migration")
	grantImageAgentWorkerRuntime := flag.Bool("grant-image-agent-worker-runtime", false, "grant only the pre-provisioned image_agent_worker_runtime role on the existing ImageAgent owner database; no schema migration")
	initOrganizationImageAgent := flag.Bool("initialize-organization-image-agent", false, "initialize only a new empty Organization ImageAgent owner database; requires explicit owner config and current manifest")
	currentManifest := flag.String("current-application-manifest", "", "absolute private current-application manifest required with -grant-image-agent-runtime")
	flag.Parse()
	if (*grantImageAgentRuntime && *initOrganizationImageAgent) || (*grantImageAgentWorkerRuntime && *initOrganizationImageAgent) || (*grantImageAgentRuntime && *grantImageAgentWorkerRuntime) {
		exitf("image agent initialization and runtime grants are separate explicit operations")
	}
	if *initOrganizationImageAgent {
		if err := productlistingschemamigrate.InitializeOrganizationImageAgent(context.Background(), *configPath, *currentManifest); err != nil {
			exitf("initialize organization image agent owner: %v", err)
		}
		fmt.Println("organization image agent owner initialized")
		return
	}

	if *grantImageAgentRuntime {
		if err := productlistingschemamigrate.GrantImageAgentRuntime(context.Background(), *configPath, *currentManifest); err != nil {
			exitf("grant image agent runtime permissions: %v", err)
		}
		fmt.Printf("image agent runtime permissions granted using %s\n", *configPath)
		return
	}
	if *grantImageAgentWorkerRuntime {
		if err := productlistingschemamigrate.GrantImageAgentWorkerRuntime(context.Background(), *configPath, *currentManifest); err != nil {
			exitf("grant image agent worker runtime permissions: %v", err)
		}
		fmt.Printf("image agent worker runtime permissions granted using %s\n", *configPath)
		return
	}
	if err := productlistingschemamigrate.Run(context.Background(), *configPath); err != nil {
		exitf("migrate product listing API schema: %v", err)
	}

	fmt.Printf("product-listing-api schema migration completed using %s\n", *configPath)
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
