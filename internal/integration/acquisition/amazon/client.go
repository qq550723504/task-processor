package amazon

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"
	"task-processor/internal/product/dataacquisition"
)

var ErrRejected = errors.New("Amazon request rejected by public egress policy")

type Options struct {
	ExecutablePath, DriverDirectory string
	EnabledSites                    []string
}
type Client struct {
	options Options
	slots   chan struct{}
	http    *http.Client
}

func New(options Options) (*Client, error) {
	if options.ExecutablePath == "" || options.DriverDirectory == "" || len(options.EnabledSites) == 0 {
		return nil, dataacquisition.ErrUnavailable
	}
	seen := map[string]bool{}
	for _, code := range options.EnabledSites {
		if _, err := dataacquisition.ResolveSite(code); err != nil || seen[code] {
			return nil, dataacquisition.ErrInvalid
		}
		seen[code] = true
	}
	options.EnabledSites = append([]string(nil), options.EnabledSites...)
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, MaxResponseHeaderBytes: 64 << 10, ResponseHeaderTimeout: 15 * time.Second, TLSHandshakeTimeout: 10 * time.Second, DialContext: publicDial}
	return &Client{options: options, slots: make(chan struct{}, 2), http: &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Sites() []dataacquisition.Site {
	var sites []dataacquisition.Site
	for _, code := range c.options.EnabledSites {
		s, _ := dataacquisition.ResolveSite(code)
		sites = append(sites, s)
	}
	return sites
}
func (c *Client) enabled(code string) bool {
	for _, s := range c.options.EnabledSites {
		if s == code {
			return true
		}
	}
	return false
}

var deniedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("224.0.0.0/4"), netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2001::/32"), netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("64:ff9b::/96"),
}

func PublicAddress(ip netip.Addr) bool {
	if !ip.IsValid() || ip.Is4In6() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range deniedPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// DNS answers are checked and the socket is dialed to the checked literal IP.
// TLS still verifies the original hostname. No proxy/env fallback can re-resolve.
func publicDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" {
		return nil, ErrRejected
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, ErrRejected
	}
	for _, ip := range addresses {
		if !PublicAddress(ip) {
			return nil, ErrRejected
		}
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	var failures []error
	for _, ip := range addresses {
		connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return connection, nil
		}
		failures = append(failures, err)
	}
	return nil, errors.Join(failures...)
}
func allowedNavigation(site dataacquisition.Site, address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Host != site.Domain || u.User != nil || u.Fragment != "" {
		return false
	}
	if u.Path == "/s" {
		for key := range u.Query() {
			if key != "k" && key != "node" && key != "page" {
				return false
			}
		}
		return true
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "dp" || u.RawQuery != "" {
		return false
	}
	asin, err := dataacquisition.NormalizeASIN(site, parts[1])
	return err == nil && asin == parts[1]
}
func (c *Client) Discover(ctx context.Context, q dataacquisition.Query) ([]string, error) {
	q, err := dataacquisition.NormalizeQuery(q)
	if err != nil || !c.enabled(q.Site) {
		return nil, dataacquisition.ErrInvalid
	}
	if q.Mode == "asin" {
		ids := append([]string(nil), q.ASINs...)
		if len(ids) > q.Limit {
			ids = ids[:q.Limit]
		}
		return ids, nil
	}
	site, _ := dataacquisition.ResolveSite(q.Site)
	ids := []string{}
	seen := map[string]bool{}
	for page := 1; page <= 10 && len(ids) < q.Limit; page++ {
		values := url.Values{"page": {strconv.Itoa(page)}}
		if q.Keyword != "" {
			values.Set("k", q.Keyword)
		}
		if q.CategoryNode != "" {
			values.Set("node", q.CategoryNode)
		}
		raw, err := c.page(ctx, site, "https://"+site.Domain+"/s?"+values.Encode())
		if err != nil {
			return nil, err
		}
		found, err := ParseDiscovery(raw, 200)
		if err != nil {
			return nil, err
		}
		added := 0
		for _, asin := range found {
			if !seen[asin] {
				seen[asin] = true
				ids = append(ids, asin)
				added++
				if len(ids) == q.Limit {
					break
				}
			}
		}
		if added == 0 {
			break
		}
	}
	return ids, nil
}
func (c *Client) Fetch(ctx context.Context, code, asin string) (dataacquisition.Evidence, error) {
	site, err := dataacquisition.ResolveSite(code)
	if err != nil || !c.enabled(code) {
		return dataacquisition.Evidence{}, dataacquisition.ErrInvalid
	}
	normalized, err := dataacquisition.NormalizeASIN(site, asin)
	if err != nil || normalized != asin {
		return dataacquisition.Evidence{}, dataacquisition.ErrInvalid
	}
	raw, err := c.page(ctx, site, "https://"+site.Domain+"/dp/"+asin)
	if err != nil {
		return dataacquisition.Evidence{}, err
	}
	return ParseProduct(raw, code, asin, time.Now())
}
func (c *Client) page(parent context.Context, site dataacquisition.Site, address string) (string, error) {
	if !allowedNavigation(site, address) {
		return "", ErrRejected
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	// Only preinstalled driver/runtime paths are used; serving never downloads.
	pw, err := playwright.Run(&playwright.RunOptions{DriverDirectory: c.options.DriverDirectory})
	if err != nil {
		return "", dataacquisition.ErrUnavailable
	}
	defer pw.Stop()
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{ExecutablePath: playwright.String(c.options.ExecutablePath), Headless: playwright.Bool(true), Timeout: playwright.Float(10000)})
	if err != nil {
		return "", dataacquisition.ErrUnavailable
	}
	defer browser.Close()
	bc, err := browser.NewContext(playwright.BrowserNewContextOptions{ServiceWorkers: playwright.ServiceWorkerPolicyBlock, AcceptDownloads: playwright.Bool(false), JavaScriptEnabled: playwright.Bool(false), IgnoreHttpsErrors: playwright.Bool(false)})
	if err != nil {
		return "", err
	}
	defer bc.Close()
	var mutex sync.Mutex
	var routeError error
	setError := func(err error) {
		mutex.Lock()
		if routeError == nil {
			routeError = err
		}
		mutex.Unlock()
	}
	// Every document is fetched through the checked, bounded Go connection.
	// All subresources are aborted; evidence references image URLs, not bytes.
	if err = bc.Route("**/*", func(route playwright.Route) {
		request := route.Request()
		if request.ResourceType() != "document" || request.Method() != "GET" || !allowedNavigation(site, request.URL()) {
			_ = route.Abort()
			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, request.URL(), nil)
		if err != nil {
			setError(ErrRejected)
			_ = route.Abort()
			return
		}
		req.Header.Set("Accept", "text/html")
		req.Header.Set("User-Agent", "Mozilla/5.0 AmazonData/1.0")
		resp, err := c.http.Do(req)
		if err != nil {
			setError(err)
			_ = route.Abort()
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK || resp.ContentLength > maxHTMLBytes || !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
			setError(ErrRejected)
			_ = route.Abort()
			return
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTMLBytes+1))
		if err != nil || len(body) > maxHTMLBytes {
			setError(ErrRejected)
			_ = route.Abort()
			return
		}
		if err := route.Fulfill(playwright.RouteFulfillOptions{Status: playwright.Int(200), Body: body, ContentType: playwright.String("text/html; charset=utf-8")}); err != nil {
			setError(err)
		}
	}); err != nil {
		return "", err
	}
	if err = bc.RouteWebSocket("**/*", func(ws playwright.WebSocketRoute) { ws.Close() }); err != nil {
		return "", err
	}
	page, err := bc.NewPage()
	if err != nil {
		return "", err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = bc.Close()
		case <-done:
		}
	}()
	_, err = page.Goto(address, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded, Timeout: playwright.Float(30000)})
	if err != nil {
		mutex.Lock()
		specific := routeError
		mutex.Unlock()
		if specific != nil {
			return "", specific
		}
		return "", err
	}
	raw, err := page.Content()
	if err != nil || len(raw) > maxHTMLBytes {
		return "", ErrUnsupported
	}
	return raw, nil
}

var _ dataacquisition.Provider = (*Client)(nil)
