// Package safeimagehttp provides the shared SSRF-safe client used for fetching
// externally hosted image references.
package safeimagehttp

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const DefaultMaxBodyBytes int64 = 32 << 20

const maxRedirectHops = 10

// ValidatePublicHTTPSURL accepts only absolute HTTPS URLs whose literal host
// is not localhost or a private/link-local address. Redirects are validated by
// the client returned from NewPublicImageHTTPClient as well.
func ValidatePublicHTTPSURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", &invalidPublicURLError{}
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil || parsed == nil || !parsed.IsAbs() || !strings.EqualFold(parsed.Scheme, "https") || strings.TrimSpace(parsed.Host) == "" {
		return "", &invalidPublicURLError{}
	}
	if isLocalHost(parsed.Hostname()) {
		return "", &invalidPublicURLError{}
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && IsPrivateIP(ip) {
		return "", &invalidPublicURLError{}
	}
	return parsed.String(), nil
}

type invalidPublicURLError struct{}

func (*invalidPublicURLError) Error() string { return "public https url is required" }

var resolvePublicImageHostIPs = func(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

// NewPublicImageHTTPClient returns an HTTP client that validates both the
// initial URL and every redirect, and dials only public IP addresses resolved
// for the requested host.
func NewPublicImageHTTPClient() *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{}
	}
	// Public image downloads must connect directly to the validated target.
	// An environment proxy can terminate CONNECT and resolve a private target
	// itself, bypassing the target-DNS checks in DialContext.
	transport.Proxy = nil
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network string, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := resolvePublicImageHostIPs(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if IsPrivateIP(ip) {
				continue
			}
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
		}
		return nil, &publicAddressUnavailableError{}
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirectHops {
				return fmt.Errorf("stopped after %d redirects", maxRedirectHops)
			}
			_, err := ValidatePublicHTTPSURL(req.URL.String())
			return err
		},
	}
}

// Download fetches a public image URL and rejects responses larger than
// maxBytes. The extra byte read is intentional: it distinguishes an exact
// limit-sized body from a body that was silently truncated at the limit.
func Download(ctx context.Context, client *http.Client, rawURL string, maxBytes int64) ([]byte, error) {
	validatedURL, err := ValidatePublicHTTPSURL(rawURL)
	if err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("image body limit must be positive")
	}
	if client == nil {
		client = NewPublicImageHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validatedURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download image %s: status %d", validatedURL, resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return nil, fmt.Errorf("image body exceeds limit: %d bytes (max %d)", resp.ContentLength, maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("image body exceeds limit: more than %d bytes", maxBytes)
	}
	return data, nil
}

type publicAddressUnavailableError struct{}

func (*publicAddressUnavailableError) Error() string {
	return "image host resolves only to private or unreachable addresses"
}

func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return (ipv4[0] == 100 && ipv4[1] >= 64 && ipv4[1] <= 127) ||
			(ipv4[0] == 198 && ipv4[1] >= 18 && ipv4[1] <= 19)
	}
	return false
}

func isLocalHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
