package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authidentity"
	"task-processor/internal/localagent"
	"task-processor/internal/product/sourcing"
)

type Handler struct {
	service *localagent.Service
}

const (
	maxCreateBodyBytes = 16 << 10
	maxResultBodyBytes = (1 << 20) + 64*1024
)

func NewHandler(service *localagent.Service) *Handler { return &Handler{service: service} }

type createJobRequest struct {
	URL string `json:"url"`
}

type submitResultRequest struct {
	ExecutionToken  string              `json:"execution_token"`
	ProductSnapshot json.RawMessage     `json:"product_snapshot"`
	Failure         *localagent.Failure `json:"failure"`
}

// productSnapshotRequest is the wire representation of a crawler snapshot.
// The sourcing model intentionally has no JSON tags, so the HTTP boundary
// must explicitly translate its snake_case protocol into the domain model.
type productSnapshotRequest struct {
	ID               string                          `json:"id"`
	Title            string                          `json:"title"`
	URL              string                          `json:"url"`
	Images           []string                        `json:"images"`
	MainImage        string                          `json:"main_image"`
	Videos           []videoSnapshotRequest          `json:"videos"`
	MinPrice         float64                         `json:"min_price"`
	MaxPrice         float64                         `json:"max_price"`
	Currency         string                          `json:"currency"`
	MinOrderQuantity int                             `json:"min_order_quantity"`
	Unit             string                          `json:"unit"`
	Supplier         supplierSnapshotRequest         `json:"supplier"`
	Specifications   []specificationSnapshotRequest  `json:"specifications"`
	ProductDetails   []productDetailSnapshotRequest  `json:"product_details"`
	PackInfo         *packInfoSnapshotRequest        `json:"pack_info"`
	VariationValues  []variationValueSnapshotRequest `json:"variation_values"`
	Variants         []variantSnapshotRequest        `json:"variants"`
	SalesVolume      int                             `json:"sales_volume"`
	ReviewCount      int                             `json:"review_count"`
	Rating           float64                         `json:"rating"`
	Shipping         shippingSnapshotRequest         `json:"shipping"`
	Category         string                          `json:"category"`
	Brand            string                          `json:"brand"`
	Keywords         []string                        `json:"keywords"`
	IsCustomized     bool                            `json:"is_customized"`
}

type videoSnapshotRequest struct {
	VideoURL string `json:"video_url"`
	CoverURL string `json:"cover_url"`
}
type supplierSnapshotRequest struct {
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
type specificationSnapshotRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type productDetailSnapshotRequest struct {
	Content string   `json:"content"`
	Images  []string `json:"images"`
}
type packInfoSnapshotRequest struct {
	PackageType   string   `json:"package_type"`
	Weight        float64  `json:"weight"`
	PackageImages []string `json:"package_images"`
	Instructions  string   `json:"instructions"`
}
type variationValueSnapshotRequest struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}
type variantSnapshotRequest struct {
	Attributes map[string]any `json:"attributes"`
	Name       string         `json:"name"`
	Image      string         `json:"image"`
	Stock      int            `json:"stock"`
	Price      float64        `json:"price"`
}
type shippingSnapshotRequest struct {
	ShippingFrom   string `json:"shipping_from"`
	ProcessingTime string `json:"processing_time"`
}

func (r productSnapshotRequest) toSnapshot() sourcing.Alibaba1688ProductSnapshot {
	snapshot := sourcing.Alibaba1688ProductSnapshot{
		ID: r.ID, Title: r.Title, URL: r.URL, Images: r.Images, MainImage: r.MainImage,
		MinPrice: r.MinPrice, MaxPrice: r.MaxPrice,
		Currency: r.Currency, MinOrderQuantity: r.MinOrderQuantity, Unit: r.Unit,
		SalesVolume: r.SalesVolume, ReviewCount: r.ReviewCount, Rating: r.Rating,
		Category: r.Category, Brand: r.Brand, Keywords: r.Keywords, IsCustomized: r.IsCustomized,
		Shipping: sourcing.Alibaba1688ShippingSnapshot{ShippingFrom: r.Shipping.ShippingFrom, ProcessingTime: r.Shipping.ProcessingTime},
		Supplier: sourcing.Alibaba1688SupplierSnapshot{ID: r.Supplier.ID, Name: r.Supplier.Name, CompanyName: r.Supplier.CompanyName, Location: r.Supplier.Location, ShopURL: r.Supplier.ShopURL, CardType: r.Supplier.CardType, YearsInBusiness: r.Supplier.YearsInBusiness, Rating: r.Supplier.Rating, ResponseRate: r.Supplier.ResponseRate, IsGoldSupplier: r.Supplier.IsGoldSupplier, IsVerified: r.Supplier.IsVerified},
	}
	for _, video := range r.Videos {
		snapshot.Videos = append(snapshot.Videos, sourcing.Alibaba1688VideoSnapshot{VideoURL: video.VideoURL, CoverURL: video.CoverURL})
	}
	for _, spec := range r.Specifications {
		snapshot.Specifications = append(snapshot.Specifications, sourcing.Alibaba1688SpecificationSnapshot{Name: spec.Name, Value: spec.Value})
	}
	for _, detail := range r.ProductDetails {
		snapshot.ProductDetails = append(snapshot.ProductDetails, sourcing.Alibaba1688ProductDetailSnapshot{Content: detail.Content, Images: detail.Images})
	}
	for _, variation := range r.VariationValues {
		snapshot.VariationValues = append(snapshot.VariationValues, sourcing.Alibaba1688VariationValueSnapshot{Name: variation.Name, Values: variation.Values})
	}
	for _, variant := range r.Variants {
		snapshot.Variants = append(snapshot.Variants, sourcing.Alibaba1688VariantSnapshot{Attributes: variant.Attributes, Name: variant.Name, Image: variant.Image, Stock: variant.Stock, Price: variant.Price})
	}
	if r.PackInfo != nil {
		snapshot.PackInfo = &sourcing.Alibaba1688PackInfoSnapshot{PackageType: r.PackInfo.PackageType, Weight: r.PackInfo.Weight, PackageImages: r.PackInfo.PackageImages, Instructions: r.PackInfo.Instructions}
	}
	return snapshot
}

type jobResponse struct {
	JobID          string                   `json:"job_id"`
	TenantID       string                   `json:"tenant_id"`
	URL            string                   `json:"url"`
	State          localagent.JobState      `json:"state"`
	ExpiresAt      time.Time                `json:"expires_at"`
	LeaseExpiresAt time.Time                `json:"lease_expires_at,omitempty"`
	Envelope       *sourcing.SourceEnvelope `json:"envelope,omitempty"`
	Failure        *localagent.Failure      `json:"failure,omitempty"`
}

type claimResponse struct {
	JobID          string    `json:"job_id"`
	ExecutionToken string    `json:"execution_token"`
	URL            string    `json:"url"`
	ExpiresAt      time.Time `json:"expires_at"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

type terminalResponse struct {
	JobID           string                      `json:"job_id"`
	State           localagent.JobState         `json:"state"`
	EnvelopeSummary *localagent.EnvelopeSummary `json:"envelope_summary,omitempty"`
	Failure         *localagent.Failure         `json:"failure,omitempty"`
}

func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.service == nil {
		writeError(c, http.StatusServiceUnavailable, "service_unavailable", "local-agent service is not configured")
		return
	}
	actor, ok := verifiedActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "identity_required", "verified identity is required")
		return
	}
	var req createJobRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCreateBodyBytes)
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	job, err := h.service.Create(actor, req.URL)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, responseFromJob(job))
}

func (h *Handler) Claim(c *gin.Context) {
	if h == nil || h.service == nil {
		writeError(c, http.StatusServiceUnavailable, "service_unavailable", "local-agent service is not configured")
		return
	}
	actor, ok := verifiedActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "identity_required", "verified identity is required")
		return
	}
	claim, err := h.service.Claim(actor)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	if claim == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, claimResponse{JobID: claim.Job.ID, ExecutionToken: claim.ExecutionToken, URL: claim.Job.URL, ExpiresAt: claim.Job.ExpiresAt, LeaseExpiresAt: claim.Job.LeaseExpiresAt})
}

func (h *Handler) ClaimJob(c *gin.Context) {
	if h == nil || h.service == nil {
		writeError(c, http.StatusServiceUnavailable, "service_unavailable", "local-agent service is not configured")
		return
	}
	actor, ok := verifiedActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "identity_required", "verified identity is required")
		return
	}
	claim, err := h.service.ClaimJob(actor, c.Param("job_id"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	if claim == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, claimResponse{JobID: claim.Job.ID, ExecutionToken: claim.ExecutionToken, URL: claim.Job.URL, ExpiresAt: claim.Job.ExpiresAt, LeaseExpiresAt: claim.Job.LeaseExpiresAt})
}

func (h *Handler) SubmitResult(c *gin.Context) {
	if h == nil || h.service == nil {
		writeError(c, http.StatusServiceUnavailable, "service_unavailable", "local-agent service is not configured")
		return
	}
	actor, ok := verifiedActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "identity_required", "verified identity is required")
		return
	}
	var req submitResultRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxResultBodyBytes)
	if err := decodeStrictJSON(c.Request.Body, &req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(c, http.StatusBadRequest, "snapshot_too_large", localagent.ErrSnapshotTooLarge.Error())
			return
		}
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	jobID := c.Param("job_id")
	var (
		job localagent.Job
		err error
	)
	if len(req.ProductSnapshot) > 0 && string(req.ProductSnapshot) != "null" && req.Failure != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "submit exactly one product_snapshot or failure")
		return
	}
	if len(req.ProductSnapshot) > 0 && string(req.ProductSnapshot) != "null" {
		var snapshotReq productSnapshotRequest
		if err := decodeStrictJSON(req.ProductSnapshot, &snapshotReq); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		snapshot := snapshotReq.toSnapshot()
		job, err = h.service.SubmitSuccess(actor, jobID, req.ExecutionToken, &snapshot)
	} else if req.Failure != nil {
		job, err = h.service.SubmitFailure(actor, jobID, req.ExecutionToken, *req.Failure)
	} else {
		writeError(c, http.StatusBadRequest, "invalid_request", "submit exactly one product_snapshot or failure")
		return
	}
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, terminalResponse{JobID: job.ID, State: job.State, EnvelopeSummary: job.EnvelopeSummary, Failure: job.Failure})
}

func responseFromJob(job localagent.Job) jobResponse {
	return jobResponse{JobID: job.ID, TenantID: job.TenantID, URL: job.URL, State: job.State, ExpiresAt: job.ExpiresAt, LeaseExpiresAt: job.LeaseExpiresAt, Envelope: job.Envelope, Failure: job.Failure}
}

func verifiedActor(c *gin.Context) (localagent.Actor, bool) {
	if c == nil || c.Request == nil {
		return localagent.Actor{}, false
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		return localagent.Actor{}, false
	}
	return localagent.Actor{TenantID: identity.TenantID, UserID: identity.UserID}, true
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, localagent.ErrSnapshotTooLarge):
		writeError(c, http.StatusBadRequest, "snapshot_too_large", err.Error())
	case errors.Is(err, localagent.ErrSnapshotInvalid):
		writeError(c, http.StatusBadRequest, "snapshot_invalid", err.Error())
	case errors.Is(err, localagent.ErrIdentityRequired), errors.Is(err, localagent.ErrInvalidURL), errors.Is(err, localagent.ErrFailureInvalid):
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, localagent.ErrClaimExpired), errors.Is(err, localagent.ErrTerminalJob):
		writeError(c, http.StatusConflict, "job_not_active", err.Error())
	case errors.Is(err, localagent.ErrCapacity):
		writeError(c, http.StatusServiceUnavailable, "local_agent_capacity", err.Error())
	case errors.Is(err, localagent.ErrInvalidClaim):
		writeError(c, http.StatusForbidden, "claim_denied", err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "local_agent_error", err.Error())
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": code, "message": strings.TrimSpace(message)})
}

func decodeStrictJSON(source any, target any) error {
	var reader io.Reader
	switch value := source.(type) {
	case io.Reader:
		reader = value
	case json.RawMessage:
		reader = bytes.NewReader(value)
	default:
		return errors.New("unsupported JSON source")
	}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("invalid JSON: trailing data")
		}
		if len(extra) > 0 {
			return errors.New("invalid JSON: trailing data")
		}
		return err
	}
	return nil
}
