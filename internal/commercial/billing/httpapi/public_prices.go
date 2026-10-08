package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/httproute"
)

const PublicResourceOfferPath = "/api/v1/commercial/resource-offers"

type publicOfferPrice struct {
	OfferID        string `json:"offer_id"`
	ProductKind    string `json:"product_kind"`
	ResourceType   string `json:"resource_type"`
	Currency       string `json:"currency"`
	UnitPriceMinor string `json:"unit_price_minor"`
	MinQuantity    string `json:"min_quantity"`
	MaxQuantity    string `json:"max_quantity"`
	PricingVersion string `json:"pricing_version"`
}

func publicPriceRoute(h *Handler) httproute.Descriptor {
	return httproute.Descriptor{Method: http.MethodGet, Path: PublicResourceOfferPath, Module: "commercial-billing", AuthPolicy: httproute.AuthPolicyPublic, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone, RequestTimeout: 5 * time.Second, RejectUnreadRequestBody: true, Handler: h.PublicResourceOffers}
}

// ValidatePublicPriceDescriptor keeps the anonymous surface confined to this
// price-only route; enterprise reads and all mutations retain their policies.
func ValidatePublicPriceDescriptor(r httproute.Descriptor) bool {
	return r.Method == http.MethodGet && r.Path == PublicResourceOfferPath && r.Module == "commercial-billing" && r.AuthPolicy == httproute.AuthPolicyPublic && r.OrganizationAccessPolicy == httproute.OrganizationAccessPolicyNone && r.Permission == "" && r.OrganizationTargetResolver == nil && r.RequestTimeout == 5*time.Second && r.RejectUnreadRequestBody && r.Handler != nil
}

func (h *Handler) PublicResourceOffers(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	if c.Request.Method != http.MethodGet || c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if c.Request.Body != nil && c.Request.Body != http.NoBody {
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
		if err != nil || len(body) != 0 {
			writeError(c, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
	}
	if h == nil || h.service == nil {
		writeServiceError(c, billing.ErrFeatureUnavailable)
		return
	}
	offers, err := h.service.ListResourceOffers(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	items := make([]publicOfferPrice, 0, len(offers))
	for _, o := range offers {
		items = append(items, publicOfferPrice{OfferID: o.OfferID, ProductKind: string(o.ProductKind), ResourceType: string(o.ResourceType), Currency: o.Currency, UnitPriceMinor: strconv.FormatInt(o.UnitPriceMinor, 10), MinQuantity: strconv.FormatInt(o.MinQuantity, 10), MaxQuantity: strconv.FormatInt(o.MaxQuantity, 10), PricingVersion: o.PricingVersion})
	}
	body, err := json.Marshal(struct {
		SchemaVersion   string             `json:"schema_version"`
		StorePeriodDays int                `json:"store_period_days"`
		Items           []publicOfferPrice `json:"items"`
	}{"retail-price-catalog-v1", 30, items})
	if err != nil || len(body) > 32*1024 {
		writeServiceError(c, billing.ErrFeatureUnavailable)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}
