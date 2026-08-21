// Package safeimagehttp provides the shared SSRF-safe client used for fetching
// externally hosted image references.
package safeimagehttp

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
)

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
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network string, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.LookupIP(host)
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
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			_, err := ValidatePublicHTTPSURL(req.URL.String())
			return err
		},
	}
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
