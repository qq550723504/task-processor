package main

import (
	"log"
	"os"

	"github.com/playwright-community/playwright-go"
)

func main() {
	if err := playwright.Install(&playwright.RunOptions{
		SkipInstallBrowsers: true,
	}); err != nil {
		log.Fatalf("install playwright driver failed: %v", err)
	}

	log.Printf("playwright driver installed, PLAYWRIGHT_DRIVER_PATH=%s", os.Getenv("PLAYWRIGHT_DRIVER_PATH"))
}
