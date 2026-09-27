// Command accept-1688-browser performs the one-off real-network acceptance for
// the anonymous public 1688 browser provider (architecture design A5).
//
// It is a throwaway acceptance runner, not production code: it drives the real
// provider against a real 1688 detail page, then proves the resulting untrusted
// evidence maps through the current owner into a publishable envelope.
//
// Usage:
//
//	go run ./hack/debug/accept-1688-browser -offer 965933437579 [-browser <path>]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	browser "task-processor/internal/integration/acquisition/a1688/browser"
	"task-processor/internal/product/sourcing"
)

func main() {
	offer := flag.String("offer", "", "1688 offer id (digits only) or a full detail.1688.com URL")
	browserPath := flag.String("browser", "", "path to the Chromium binary")
	headless := flag.Bool("headless", true, "run headless")
	timeout := flag.Duration("timeout", 120*time.Second, "acquisition budget")
	flag.Parse()

	if *offer == "" {
		fmt.Fprintln(os.Stderr, "-offer is required")
		os.Exit(2)
	}
	if *browserPath == "" {
		fmt.Fprintln(os.Stderr, "-browser is required (the fingerprint Chromium or a playwright chromium)")
		os.Exit(2)
	}

	input := *offer
	if len(input) < 20 {
		input = "https://detail.1688.com/offer/" + input + ".html"
	}
	source, err := sourcing.Canonical1688Source(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid source:", err)
		os.Exit(2)
	}

	client := browser.New(browser.Options{
		ExecutablePath: *browserPath,
		Headless:       *headless,
		Budget:         *timeout,
		AllowedOrigins: browser.DefaultAllowedOrigins,
	})

	ctx, cancel := context.WithTimeout(context.Background(), *timeout+30*time.Second)
	defer cancel()
	started := time.Now()
	evidence, err := client.Acquire(ctx, source)
	elapsed := time.Since(started)

	fmt.Printf("RESULT=%s duration=%s offer=%s\n", resultWord(err), elapsed.Round(time.Millisecond), source.OfferID)
	if err != nil {
		fmt.Printf("ERROR=%v\n", err)
		return
	}

	if evidence.Title != nil {
		fmt.Printf("TITLE=%s\n", *evidence.Title)
	}
	fmt.Printf("IMAGES=%d ATTRIBUTES=%d VARIANTS=%d PRICEFACTS=%d\n",
		len(evidence.Images), len(evidence.Attributes), len(evidence.Variants), len(evidence.PriceFacts))
	if len(evidence.PriceFacts) > 0 {
		amount := evidence.PriceFacts[0].Amount
		currency := ""
		if evidence.PriceFacts[0].Currency != nil {
			currency = *evidence.PriceFacts[0].Currency
		}
		fmt.Printf("PRICE=%s %s\n", amount, currency)
	}
	if len(evidence.Images) > 0 {
		fmt.Printf("FIRST_IMAGE=%s\n", evidence.Images[0].URL)
	}
	if len(evidence.Attributes) > 0 {
		fmt.Printf("FIRST_ATTR=%s=%s\n", evidence.Attributes[0].Name, evidence.Attributes[0].Value)
	}
	fmt.Printf("CONTENT_SHA256=%s\n", evidence.ContentSHA256)
	fmt.Printf("PARSER=%s\n", evidence.ParserVersion)

	// The real proof: the untrusted evidence must map through the current owner.
	envelope, err := sourcing.MapAcquisitionEvidence(source, evidence, sourcing.AcquisitionChannelPublicBrowser, "op-acceptance")
	if err != nil {
		fmt.Printf("MAP=FAILED error=%v\n", err)
		os.Exit(1)
	}
	key, version, err := sourcing.PublicationIdentity(envelope)
	if err != nil {
		fmt.Printf("IDENTITY=FAILED error=%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("MAP=OK\nPRODUCT_KEY=%s\nVERSION=%s\n", key, version)
	fmt.Printf("MISSING_FACTS=%d WARNINGS=%d\n", len(envelope.MissingFacts), len(envelope.Warnings))
}

func resultWord(err error) string {
	if err != nil {
		return "FAIL"
	}
	return "OK"
}
