package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/playwright-community/playwright-go"
)

const defaultPlaywrightBrowser = "chromium"

func browsersFromEnv(getenv func(string) string) []string {
	raw := strings.TrimSpace(getenv("PLAYWRIGHT_BROWSERS"))
	if raw == "" {
		return []string{defaultPlaywrightBrowser}
	}

	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	browsers := make([]string, 0, len(fields))
	for _, field := range fields {
		browser := strings.TrimSpace(field)
		if browser != "" {
			browsers = append(browsers, browser)
		}
	}
	if len(browsers) == 0 {
		return []string{defaultPlaywrightBrowser}
	}
	return browsers
}

func run() error {
	browsers := browsersFromEnv(os.Getenv)
	if err := playwright.Install(&playwright.RunOptions{Browsers: browsers}); err != nil {
		return fmt.Errorf("install playwright browsers %v: %w", browsers, err)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
