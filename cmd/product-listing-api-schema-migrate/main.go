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
	currentManifest := flag.String("current-application-manifest", "", "absolute private current-application manifest required with -grant-image-agent-runtime")
	flag.Parse()

	if *grantImageAgentRuntime {
		if err := productlistingschemamigrate.GrantImageAgentRuntime(context.Background(), *configPath, *currentManifest); err != nil {
			exitf("grant image agent runtime permissions: %v", err)
		}
		fmt.Printf("image agent runtime permissions granted using %s\n", *configPath)
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
