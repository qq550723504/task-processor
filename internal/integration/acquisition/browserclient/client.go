// Package browserclient is the current-application side of the internal
// collector RPC (design D13). It presents the collector to the sourcing owner as
// an ordinary sourcing.PublicAcquirer, so the current-application keeps every
// authorization, idempotency, operation-row, and publication responsibility and
// the collector remains a pure, credential-free evidence producer.
package browserclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"task-processor/internal/integration/acquisition/browsercollector"
	"task-processor/internal/product/sourcing"
)

// Client calls the anonymous public browser collector over the internal RPC.
type Client struct {
	endpoint   string
	admission  func(*http.Request) error
	httpClient *http.Client
}

// Options configures the collector client.
type Options struct {
	// Endpoint is the collector's base URL. Required.
	Endpoint string
	// Admission attaches the caller credential the collector requires. It is
	// the client side of the design 12-A1 decision; until that decision lands
	// this may be nil only in isolated tests.
	Admission func(*http.Request) error
	// Timeout bounds one RPC call. Zero means the provider budget default.
	Timeout time.Duration
	// Transport allows an injected transport in tests. Nil uses the default.
	Transport http.RoundTripper
}

var _ sourcing.PublicAcquirer = (*Client)(nil)

// New builds a collector client.
func New(opts Options) (*Client, error) {
	if opts.Endpoint == "" {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	transport := opts.Transport
	if transport == nil {
		// A dedicated transport that never uses an ambient proxy. With
		// HTTP_PROXY set and a non-loopback collector hostname, the shared default
		// transport would send this internal RPC through the proxy, handing it the
		// X-Collector-Credential header and letting it impersonate the application.
		transport = &http.Transport{
			Proxy:                 nil,
			DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			MaxIdleConns:          8,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		}
	}
	return &Client{
		endpoint:  opts.Endpoint + browsercollector.AcquirePath,
		admission: opts.Admission,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			// Never follow a redirect. The service credential travels as a custom
			// header and Go copies headers onto the redirect request, which would
			// hand the secret to whatever the collector or an intermediate proxy
			// points at.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// Acquire fetches untrusted evidence from the collector. The collector returns
// only untrusted evidence, so this client performs no mapping and no
// authorization: the current-application owner does that after the response
// arrives.
func (c *Client) Acquire(ctx context.Context, source sourcing.AcquisitionSource) (sourcing.AcquisitionEvidence, error) {
	if c == nil || c.httpClient == nil {
		return sourcing.AcquisitionEvidence{}, sourcing.ErrAcquisitionUnavailable
	}
	// The collector only accepts the canonical public source; guard here so a
	// malformed source never leaves this process.
	if canonical, err := sourcing.Canonical1688Source(source.URL); err != nil || canonical != source {
		return sourcing.AcquisitionEvidence{}, sourcing.ErrInvalidAcquisition
	}

	body, err := json.Marshal(browsercollector.AcquireRequest{SourceURL: source.URL})
	if err != nil {
		return sourcing.AcquisitionEvidence{}, sourcing.ErrAcquisitionUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return sourcing.AcquisitionEvidence{}, sourcing.ErrAcquisitionUnavailable
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	if c.admission != nil {
		// Admission attaches the caller credential the collector requires. A
		// failure to attach must fail closed: sending the request without it
		// would only waste a browser slot and leak the attempt.
		if err := c.admission(req); err != nil {
			return sourcing.AcquisitionEvidence{}, errors.Join(sourcing.ErrAcquisitionUnavailable, err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Preserve the cause: a transport timeout must surface as a deadline so
		// the caller renders 504 rather than a 502 source failure. Formatting the
		// error with %v would discard context.DeadlineExceeded.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return sourcing.AcquisitionEvidence{}, ctxErr
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: collector transport timeout: %v", context.DeadlineExceeded, err)
		}
		// A transport failure is an unknown source outcome, not a caller error.
		return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: collector transport: %w", sourcing.ErrAcquisitionUnavailable, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, browsercollector.MaxResponseBytes))
		_ = resp.Body.Close()
	}()

	// Bound the body before decoding, independently of Content-Length.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, browsercollector.MaxResponseBytes+1))
	if err != nil {
		return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: collector read: %v", sourcing.ErrAcquisitionUnavailable, err)
	}
	if len(raw) > browsercollector.MaxResponseBytes {
		return sourcing.AcquisitionEvidence{}, sourcing.ErrAcquisitionUnavailable
	}

	if resp.StatusCode != http.StatusOK {
		return sourcing.AcquisitionEvidence{}, decodeErrorStatus(resp.StatusCode, raw)
	}
	var evidence sourcing.AcquisitionEvidence
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: collector decode: %v", sourcing.ErrAcquisitionFailed, err)
	}
	return evidence, nil
}

// decodeErrorStatus maps a bounded collector error body back to a domain
// sentinel. A body that cannot be parsed is treated as a source failure, never
// as success and never as a caller error.
func decodeErrorStatus(status int, raw []byte) error {
	if status == http.StatusBadRequest {
		return sourcing.ErrInvalidAcquisition
	}
	if status == http.StatusForbidden {
		// The collector refused this caller. That is an admission/availability
		// problem on the service boundary, not a bad public source.
		return sourcing.ErrAcquisitionUnavailable
	}
	var body browsercollector.ErrorBody
	if err := json.Unmarshal(raw, &body); err == nil && body.Code != "" {
		return browsercollector.SentinelFor(body.Code)
	}
	if status == http.StatusGatewayTimeout {
		return context.DeadlineExceeded
	}
	return sourcing.ErrAcquisitionFailed
}
