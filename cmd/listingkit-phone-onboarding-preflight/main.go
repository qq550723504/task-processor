// listingkit-phone-onboarding-preflight is an isolated, interactive ZITADEL
// Login V2 feasibility probe. It deliberately loads no application runtime
// configuration and does not create ListingKit tenants, roles, or subscriptions.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"task-processor/internal/listingkit/phoneonboardingpreflight"

	"golang.org/x/term"
)

const preflightTimeout = 5 * time.Minute

type secretReader func(string, *os.File, io.Writer) (string, error)
type clientFactory func(phoneonboardingpreflight.ClientConfig) (phoneonboardingpreflight.Client, error)

func main() {
	if err := run(context.Background(), os.Getenv, readSecret, phoneonboardingpreflight.NewClient, os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func run(parent context.Context, getenv func(string) string, read secretReader, newClient clientFactory, input *os.File, stdout, stderr io.Writer) error {
	phone, err := readSecretValue("phone: ", read, input, stdout, stderr)
	if err != nil {
		return err
	}

	client, err := newClient(phoneonboardingpreflight.ClientConfig{
		IssuerURL:         getenv("ZITADEL_ISSUER_URL"),
		ProvisioningToken: getenv("ZITADEL_PREFLIGHT_PROVISION_TOKEN"),
		SessionToken:      getenv("ZITADEL_PREFLIGHT_LOGIN_TOKEN"),
	})
	if err != nil {
		return failed(stderr)
	}
	runner, err := phoneonboardingpreflight.NewRunner(client, rand.Reader, time.Now, stdout)
	if err != nil {
		return failed(stderr)
	}
	ctx, cancel := context.WithTimeout(parent, preflightTimeout)
	defer cancel()
	attempt, err := runner.Start(ctx, phone)
	if err != nil {
		return failed(stderr)
	}

	code, err := readSecretValue("verification code: ", read, input, stdout, stderr)
	if err != nil {
		return err
	}
	if _, err := runner.Verify(ctx, attempt, code); err != nil {
		return failed(stderr)
	}
	return nil
}

func readSecretValue(prompt string, read secretReader, input *os.File, stdout, stderr io.Writer) (string, error) {
	value, err := read(prompt, input, stdout)
	if err != nil || strings.TrimSpace(value) == "" {
		return "", failed(stderr)
	}
	return value, nil
}

func failed(stderr io.Writer) error {
	_, _ = fmt.Fprintln(stderr, "preflight failed")
	return errors.New("preflight failed")
}

func readSecret(prompt string, input *os.File, output io.Writer) (string, error) {
	if _, err := fmt.Fprint(output, prompt); err != nil {
		return "", err
	}
	value, err := term.ReadPassword(int(input.Fd()))
	_, _ = fmt.Fprintln(output)
	if err != nil {
		return "", errors.New("secure input failed")
	}
	return strings.TrimSpace(string(value)), nil
}
