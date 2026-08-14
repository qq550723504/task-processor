package openmeter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	openmeterapi "github.com/openmeterio/openmeter/api/v3/client"
)

// Client is the small OpenMeter boundary used by the shadow-metering proof of concept.
type Client struct {
	sdk *openmeterapi.Client
}

// Config configures an OpenMeter API client.
type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient constructs the adapter around the official OpenMeter v3 SDK.
func NewClient(cfg Config) (*Client, error) {
	opts := clientOptions(cfg)
	if cfg.APIKey != "" {
		opts = append(opts, openmeterapi.WithToken(cfg.APIKey))
	}

	sdk, err := openmeterapi.New(cfg.BaseURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errConfiguration, err)
	}

	return &Client{sdk: sdk}, nil
}

func clientOptions(cfg Config) []openmeterapi.Option {
	if cfg.HTTPClient == nil {
		return nil
	}
	return []openmeterapi.Option{openmeterapi.WithHTTPClient(cfg.HTTPClient)}
}

// Ingest validates and submits one usage event through the official SDK.
func (c *Client) Ingest(ctx context.Context, event openmeterapi.EventInput) error {
	if err := ValidateUsageEvent(event); err != nil {
		return err
	}
	return c.sdk.Events.IngestEvent(ctx, event)
}

// QueryUsage returns the exact numeric usage value for one subject and meter.
func (c *Client) QueryUsage(ctx context.Context, meterID, subject string, from, to time.Time) (string, error) {
	dimensions := map[string]openmeterapi.QueryFilterStringMapItemInput{
		"subject": {Eq: &subject},
	}
	result, err := c.sdk.Meters.Query(ctx, meterID, openmeterapi.MeterQueryRequest{
		From: &from,
		To:   &to,
		Filters: &openmeterapi.MeterQueryFilters{
			Dimensions: &dimensions,
		},
	})
	if err != nil {
		return "", err
	}
	if len(result.Data) == 0 {
		return "0", nil
	}
	if len(result.Data) != 1 {
		return "", fmt.Errorf("openmeter query for meter %q and subject %q returned %d rows: %w", meterID, subject, len(result.Data), errUnexpectedQueryRows)
	}

	return result.Data[0].Value, nil
}

// ListCustomerAccess returns entitlement access for the specified customer.
func (c *Client) ListCustomerAccess(ctx context.Context, customerID string) ([]openmeterapi.EntitlementAccessResult, error) {
	result, err := c.sdk.Entitlements.ListCustomerAccess(ctx, customerID)
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

var errUnexpectedQueryRows = errors.New("openmeter query returned an unexpected number of rows")
