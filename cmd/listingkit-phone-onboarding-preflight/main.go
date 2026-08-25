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
	"os/signal"
	"strings"
	"time"

	"task-processor/internal/listingkit/phoneonboardingpreflight"
)

const preflightTimeout = 5 * time.Minute

var errNonProductionConfirmation = errors.New("--non-production confirmation is required")

type secretReader func(string, *os.File, io.Writer) (string, error)
type clientFactory func(phoneonboardingpreflight.ClientConfig) (phoneonboardingpreflight.Client, error)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Getenv, readSecret, phoneonboardingpreflight.NewClient, os.Stdin, os.Stdout, os.Stderr, os.Args[1:], preflightTimeout); err != nil {
		if errors.Is(err, errNonProductionConfirmation) {
			_, _ = fmt.Fprintln(os.Stderr, "usage: listingkit-phone-onboarding-preflight --non-production")
		}
		os.Exit(1)
	}
}

func run(parent context.Context, getenv func(string) string, read secretReader, newClient clientFactory, input *os.File, stdout, stderr io.Writer, args []string, timeout time.Duration) error {
	if len(args) != 1 || args[0] != "--non-production" {
		return errNonProductionConfirmation
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	phone, err := readSecretValue(ctx, "phone: ", read, input, stdout)
	if err != nil {
		return err
	}

	client, err := newClient(phoneonboardingpreflight.ClientConfig{
		IssuerURL:         getenv("ZITADEL_ISSUER_URL"),
		ProvisioningToken: getenv("ZITADEL_PREFLIGHT_PROVISION_TOKEN"),
		SessionToken:      getenv("ZITADEL_PREFLIGHT_LOGIN_TOKEN"),
	})
	if err != nil {
		return errors.New("phone onboarding preflight setup failed")
	}
	runner, err := phoneonboardingpreflight.NewRunner(client, rand.Reader, time.Now, stdout)
	if err != nil {
		return errors.New("phone onboarding preflight setup failed")
	}
	attempt, err := runner.Start(ctx, phone)
	if err != nil {
		return err
	}

	code, err := readSecretValue(ctx, "verification code: ", read, input, stdout)
	if err != nil {
		return runner.Abandon(attempt)
	}
	if _, err := runner.Verify(ctx, attempt, code); err != nil {
		return err
	}
	return nil
}

func readSecretValue(ctx context.Context, prompt string, read secretReader, input *os.File, stdout io.Writer) (string, error) {
	type result struct {
		value string
		err   error
	}
	resultCh := make(chan result, 1)
	go func() {
		value, err := read(prompt, input, stdout)
		resultCh <- result{value: value, err: err}
	}()
	select {
	case result := <-resultCh:
		if result.err != nil || strings.TrimSpace(result.value) == "" {
			return "", errors.New("secure input failed")
		}
		return result.value, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func readSecret(prompt string, input *os.File, output io.Writer) (string, error) {
	if _, err := fmt.Fprint(output, prompt); err != nil {
		return "", err
	}
	var value []byte
	var one [1]byte
	for {
		n, err := input.Read(one[:])
		if n > 0 {
			if one[0] == '\n' {
				break
			}
			value = append(value, one[0])
		}
		if err != nil {
			if errors.Is(err, io.EOF) && len(value) > 0 {
				break
			}
			return "", errors.New("secure input failed")
		}
	}
	_, _ = fmt.Fprintln(output)
	return strings.TrimSpace(string(value)), nil
}
