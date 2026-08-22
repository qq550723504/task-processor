package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"task-processor/internal/localagent"
	"task-processor/internal/product/sourcing"
)

type Client struct {
	BaseURL     string
	AccessToken string
	HTTPClient  *http.Client
}

const defaultHTTPTimeout = 30 * time.Second

var errRequestEncoding = errors.New("local-agent request could not be encoded")

type productSnapshotPayload struct {
	ID               string                  `json:"id"`
	Title            string                  `json:"title"`
	URL              string                  `json:"url"`
	Images           []string                `json:"images"`
	MainImage        string                  `json:"main_image"`
	Videos           []videoPayload          `json:"videos"`
	MinPrice         float64                 `json:"min_price"`
	MaxPrice         float64                 `json:"max_price"`
	Currency         string                  `json:"currency"`
	MinOrderQuantity int                     `json:"min_order_quantity"`
	Unit             string                  `json:"unit"`
	Supplier         supplierPayload         `json:"supplier"`
	Specifications   []specificationPayload  `json:"specifications"`
	ProductDetails   []productDetailPayload  `json:"product_details"`
	PackInfo         *packInfoPayload        `json:"pack_info"`
	VariationValues  []variationValuePayload `json:"variation_values"`
	Variants         []variantPayload        `json:"variants"`
	SalesVolume      int                     `json:"sales_volume"`
	ReviewCount      int                     `json:"review_count"`
	Rating           float64                 `json:"rating"`
	Shipping         shippingPayload         `json:"shipping"`
	Category         string                  `json:"category"`
	Brand            string                  `json:"brand"`
	Keywords         []string                `json:"keywords"`
	IsCustomized     bool                    `json:"is_customized"`
}
type videoPayload struct {
	VideoURL string `json:"video_url"`
	CoverURL string `json:"cover_url"`
}
type supplierPayload struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	CompanyName     string  `json:"company_name"`
	Location        string  `json:"location"`
	ShopURL         string  `json:"shop_url"`
	CardType        string  `json:"card_type"`
	YearsInBusiness int     `json:"years_in_business"`
	Rating          float64 `json:"rating"`
	ResponseRate    float64 `json:"response_rate"`
	IsGoldSupplier  bool    `json:"is_gold_supplier"`
	IsVerified      bool    `json:"is_verified"`
}
type specificationPayload struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type productDetailPayload struct {
	Content string   `json:"content"`
	Images  []string `json:"images"`
}
type packInfoPayload struct {
	PackageType   string   `json:"package_type"`
	Weight        float64  `json:"weight"`
	PackageImages []string `json:"package_images"`
	Instructions  string   `json:"instructions"`
}
type variationValuePayload struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}
type variantPayload struct {
	Attributes map[string]any `json:"attributes"`
	Name       string         `json:"name"`
	Image      string         `json:"image"`
	Stock      int            `json:"stock"`
	Price      float64        `json:"price"`
}
type shippingPayload struct {
	ShippingFrom   string `json:"shipping_from"`
	ProcessingTime string `json:"processing_time"`
}

func snapshotPayload(snapshot *sourcing.Alibaba1688ProductSnapshot) *productSnapshotPayload {
	if snapshot == nil {
		return nil
	}
	payload := &productSnapshotPayload{
		ID: snapshot.ID, Title: snapshot.Title, URL: snapshot.URL, Images: snapshot.Images, MainImage: snapshot.MainImage,
		MinPrice: snapshot.MinPrice, MaxPrice: snapshot.MaxPrice,
		Currency: snapshot.Currency, MinOrderQuantity: snapshot.MinOrderQuantity, Unit: snapshot.Unit,
		SalesVolume: snapshot.SalesVolume, ReviewCount: snapshot.ReviewCount, Rating: snapshot.Rating,
		Category: snapshot.Category, Brand: snapshot.Brand, Keywords: snapshot.Keywords, IsCustomized: snapshot.IsCustomized,
		Supplier: supplierPayload{ID: snapshot.Supplier.ID, Name: snapshot.Supplier.Name, CompanyName: snapshot.Supplier.CompanyName, Location: snapshot.Supplier.Location, ShopURL: snapshot.Supplier.ShopURL, CardType: snapshot.Supplier.CardType, YearsInBusiness: snapshot.Supplier.YearsInBusiness, Rating: snapshot.Supplier.Rating, ResponseRate: snapshot.Supplier.ResponseRate, IsGoldSupplier: snapshot.Supplier.IsGoldSupplier, IsVerified: snapshot.Supplier.IsVerified},
		Shipping: shippingPayload{ShippingFrom: snapshot.Shipping.ShippingFrom, ProcessingTime: snapshot.Shipping.ProcessingTime},
	}
	for _, video := range snapshot.Videos {
		payload.Videos = append(payload.Videos, videoPayload{VideoURL: video.VideoURL, CoverURL: video.CoverURL})
	}
	for _, spec := range snapshot.Specifications {
		payload.Specifications = append(payload.Specifications, specificationPayload{Name: spec.Name, Value: spec.Value})
	}
	for _, detail := range snapshot.ProductDetails {
		payload.ProductDetails = append(payload.ProductDetails, productDetailPayload{Content: detail.Content, Images: detail.Images})
	}
	for _, variation := range snapshot.VariationValues {
		payload.VariationValues = append(payload.VariationValues, variationValuePayload{Name: variation.Name, Values: variation.Values})
	}
	for _, variant := range snapshot.Variants {
		payload.Variants = append(payload.Variants, variantPayload{Attributes: variant.Attributes, Name: variant.Name, Image: variant.Image, Stock: variant.Stock, Price: variant.Price})
	}
	if snapshot.PackInfo != nil {
		payload.PackInfo = &packInfoPayload{PackageType: snapshot.PackInfo.PackageType, Weight: snapshot.PackInfo.Weight, PackageImages: snapshot.PackInfo.PackageImages, Instructions: snapshot.PackInfo.Instructions}
	}
	return payload
}

func New(baseURL, accessToken string, httpClient *http.Client) (*Client, error) {
	base, err := validateBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(accessToken) == "" {
		return nil, errors.New("access token is required")
	}
	return &Client{BaseURL: strings.TrimRight(base.String(), "/"), AccessToken: strings.TrimSpace(accessToken), HTTPClient: safeHTTPClient(httpClient)}, nil
}

func (c *Client) CreateJob(ctx context.Context, rawURL string) (localagent.Job, error) {
	var response jobResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/local-agent/1688-jobs", map[string]string{"url": rawURL}, http.StatusCreated, &response)
	return response.toJob(), err
}

func (c *Client) Claim(ctx context.Context) (*localagent.Claim, error) {
	var response claimResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/local-agent/1688-jobs/claim", nil, http.StatusOK, &response)
	if errors.Is(err, errNoJob) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &localagent.Claim{Job: response.toJob(), ExecutionToken: response.ExecutionToken}, nil
}

func (c *Client) ClaimJob(ctx context.Context, jobID string) (*localagent.Claim, error) {
	var response claimResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/local-agent/1688-jobs/"+url.PathEscape(jobID)+"/claim", nil, http.StatusOK, &response)
	if errors.Is(err, errNoJob) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &localagent.Claim{Job: response.toJob(), ExecutionToken: response.ExecutionToken}, nil
}

func (c *Client) SubmitSuccess(ctx context.Context, jobID, token string, snapshot *sourcing.Alibaba1688ProductSnapshot) (localagent.Job, error) {
	var response terminalResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/local-agent/1688-jobs/"+url.PathEscape(jobID)+"/result", map[string]any{
		"execution_token":  token,
		"product_snapshot": snapshotPayload(snapshot),
	}, http.StatusOK, &response)
	if errors.Is(err, errRequestEncoding) {
		return response.toJob(), localagent.ErrSnapshotInvalid
	}
	return response.toJob(), err
}

func (c *Client) SubmitFailure(ctx context.Context, jobID, token string, failure localagent.Failure) (localagent.Job, error) {
	var response terminalResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/local-agent/1688-jobs/"+url.PathEscape(jobID)+"/result", map[string]any{
		"execution_token": token,
		"failure":         failure,
	}, http.StatusOK, &response)
	return response.toJob(), err
}

var errNoJob = errors.New("no local-agent job")

type jobResponse struct {
	JobID          string                   `json:"job_id"`
	TenantID       string                   `json:"tenant_id"`
	URL            string                   `json:"url"`
	State          localagent.JobState      `json:"state"`
	ExpiresAt      time.Time                `json:"expires_at"`
	LeaseExpiresAt time.Time                `json:"lease_expires_at"`
	Envelope       *sourcing.SourceEnvelope `json:"envelope"`
	Failure        *localagent.Failure      `json:"failure"`
}

type terminalResponse struct {
	JobID           string                      `json:"job_id"`
	State           localagent.JobState         `json:"state"`
	EnvelopeSummary *localagent.EnvelopeSummary `json:"envelope_summary"`
	Failure         *localagent.Failure         `json:"failure"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type claimResponse struct {
	JobID          string    `json:"job_id"`
	ExecutionToken string    `json:"execution_token"`
	URL            string    `json:"url"`
	ExpiresAt      time.Time `json:"expires_at"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

func (r jobResponse) toJob() localagent.Job {
	return localagent.Job{ID: r.JobID, TenantID: r.TenantID, URL: r.URL, State: r.State, ExpiresAt: r.ExpiresAt, LeaseExpiresAt: r.LeaseExpiresAt, Envelope: r.Envelope, Failure: r.Failure}
}

func (r terminalResponse) toJob() localagent.Job {
	return localagent.Job{ID: r.JobID, State: r.State, EnvelopeSummary: r.EnvelopeSummary, Failure: r.Failure}
}

func (r claimResponse) toJob() localagent.Job {
	return localagent.Job{ID: r.JobID, URL: r.URL, ExpiresAt: r.ExpiresAt, LeaseExpiresAt: r.LeaseExpiresAt, State: localagent.JobClaimed}
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload any, expected int, target any) error {
	if c == nil || strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.AccessToken) == "" {
		return errors.New("local-agent client is not configured")
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("%w: %v", errRequestEncoding, err)
		}
		body = strings.NewReader(string(encoded))
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return errors.New("local-agent request could not be created")
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return errors.New("local-agent request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent && expected == http.StatusOK {
		return errNoJob
	}
	if resp.StatusCode != expected {
		var apiError errorResponse
		if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&apiError); err == nil {
			switch apiError.Error {
			case "snapshot_too_large":
				return localagent.ErrSnapshotTooLarge
			case "snapshot_invalid":
				return localagent.ErrSnapshotInvalid
			}
		}
		return fmt.Errorf("local-agent request returned status %d", resp.StatusCode)
	}
	if target == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(target); err != nil {
		return errors.New("local-agent response was invalid")
	}
	return nil
}

func validateBaseURL(raw string) (*url.URL, error) {
	base, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || base.Scheme == "" || base.Host == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" {
		return nil, errors.New("api base URL must be an absolute HTTPS URI")
	}
	if base.Scheme != "https" && !(base.Scheme == "http" && isLoopback(base.Hostname())) {
		return nil, errors.New("api base URL must use HTTPS unless it is a literal loopback test endpoint")
	}
	return base, nil
}

func safeHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	clone := *client
	if clone.Timeout <= 0 {
		clone.Timeout = defaultHTTPTimeout
	}
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &clone
}

func isLoopback(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
