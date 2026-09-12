package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	sigjson "sigs.k8s.io/json"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	a1688 "task-processor/internal/integration/acquisition/a1688"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/product/sourcing"
)

const productAcquisitionBase = "/api/v1/workbench/sourcing/1688/acquisitions"

// NewCurrentApplicationWithAcquisition adds only the explicitly enabled current
// Product module. All three pools remain caller-owned; construction is read-only.
func NewCurrentApplicationWithAcquisition(ctx context.Context, sourceAccountDB, commercialDB, productDB *gorm.DB, cfg *config.Config, logger *logrus.Logger) (*http.Server, error) {
	if ctx == nil || productDB == nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	factories := defaultCurrentApplicationFactories(ctx)
	factories.buildAcquisition = func(authorizer *authz.ListingKitAuthorizer, dependencies routeAuthDependencies) (kernelmodule.Module, error) {
		return buildProductAcquisitionModule(ctx, productDB, dependencies, authorizer, a1688.New())
	}
	return buildCurrentApplication(ctx, sourceAccountDB, commercialDB, cfg, logger, factories)
}

type productAcquisitionService interface {
	Acquire(context.Context, string, string) (sourcing.AcquisitionResult, error)
	Verify(context.Context, string, string) (sourcing.AcquisitionResult, error)
	Read(context.Context, string) (sourcing.AcquisitionResult, error)
}

func productAcquisitionRoutes(service productAcquisitionService, bind func(context.Context, string) (context.Context, error)) []httproute.Descriptor {
	specs := []struct{ method, path, action string }{
		{http.MethodPost, productAcquisitionBase, "acquire"},
		{http.MethodPost, productAcquisitionBase + "/verify", "verify"},
		{http.MethodGet, productAcquisitionBase + "/:operation_id", "read"},
	}
	routes := make([]httproute.Descriptor, 0, len(specs))
	for _, spec := range specs {
		routes = append(routes, httproute.Descriptor{Method: spec.method, Path: spec.path, Module: "product-acquisition", Permission: "product_sourcing.write", AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: sourcing.AcquisitionTimeout, RejectUnreadRequestBody: true, Handler: func(c *gin.Context) {
			if service == nil || bind == nil {
				writeAcquisitionError(c, sourcing.ErrAcquisitionUnavailable)
				return
			}
			ctx, err := bind(c.Request.Context(), c.GetHeader("Authorization"))
			if err != nil {
				writeAcquisitionError(c, sourcing.ErrPublicationForbidden)
				return
			}
			if c.Request.URL.RawQuery != "" {
				writeAcquisitionError(c, sourcing.ErrInvalidAcquisition)
				return
			}
			var result sourcing.AcquisitionResult
			if spec.action == "read" {
				if c.Request.Body != nil {
					raw, e := io.ReadAll(io.LimitReader(c.Request.Body, 1))
					if e != nil || len(raw) > 0 {
						writeAcquisitionError(c, sourcing.ErrInvalidAcquisition)
						return
					}
				}
				id := c.Param("operation_id")
				if !acquisitionHTTPUUID(id) {
					writeAcquisitionError(c, sourcing.ErrInvalidAcquisition)
					return
				}
				result, err = service.Read(ctx, id)
			} else {
				key, source, e := readAcquisitionRequest(c.Request)
				if e != nil {
					writeAcquisitionError(c, e)
					return
				}
				if spec.action == "verify" {
					result, err = service.Verify(ctx, key, source)
				} else {
					result, err = service.Acquire(ctx, key, source)
				}
			}
			if err != nil {
				writeAcquisitionError(c, err)
				return
			}
			writeAcquisitionResult(c, result)
		}})
	}
	return routes
}

func readAcquisitionRequest(request *http.Request) (string, string, error) {
	keys := request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !acquisitionHTTPUUID(keys[0]) {
		return "", "", sourcing.ErrInvalidAcquisition
	}
	media, parameters, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || (parameters["charset"] != "" && !strings.EqualFold(parameters["charset"], "utf-8")) || (request.Header.Get("Content-Encoding") != "" && request.Header.Get("Content-Encoding") != "identity") || request.Body == nil {
		return "", "", sourcing.ErrInvalidAcquisition
	}
	raw, err := io.ReadAll(io.LimitReader(request.Body, 8193))
	if len(raw) > 8192 {
		return "", "", sourcing.ErrSourcePublicationTooLarge
	}
	if err != nil || !utf8.Valid(raw) {
		return "", "", sourcing.ErrInvalidAcquisition
	}
	var body struct {
		Source string `json:"source"`
	}
	strict, err := sigjson.UnmarshalStrict(raw, &body, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if err != nil || len(strict) > 0 {
		return "", "", sourcing.ErrInvalidAcquisition
	}
	if _, err := sourcing.Canonical1688Source(body.Source); err != nil {
		return "", "", err
	}
	return keys[0], body.Source, nil
}

func acquisitionHTTPUUID(raw string) bool {
	value, err := uuid.Parse(raw)
	return err == nil && value != uuid.Nil && value.String() == raw && value.Variant() == uuid.RFC4122
}

type acquisitionWarningDTO struct {
	Code  string `json:"code"`
	Field string `json:"field"`
}
type acquisitionMissingDTO struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
type acquisitionResultDTO struct {
	SchemaVersion  int                     `json:"schemaVersion"`
	OperationID    string                  `json:"operationId"`
	Outcome        string                  `json:"outcome"`
	Replayed       bool                    `json:"replayed"`
	ProductKey     string                  `json:"productKey,omitempty"`
	PublicationID  string                  `json:"publicationId,omitempty"`
	CatalogVersion string                  `json:"catalogVersion,omitempty"`
	Warnings       []acquisitionWarningDTO `json:"warnings"`
	MissingFacts   []acquisitionMissingDTO `json:"missingFacts"`
}

func writeAcquisitionResult(c *gin.Context, result sourcing.AcquisitionResult) {
	dto := acquisitionResultDTO{SchemaVersion: 1, OperationID: result.Operation.ID, Outcome: result.Operation.State, Replayed: result.Replayed, Warnings: []acquisitionWarningDTO{}, MissingFacts: []acquisitionMissingDTO{}}
	if !acquisitionHTTPUUID(dto.OperationID) {
		writeAcquisitionError(c, sourcing.ErrAcquisitionUnavailable)
		return
	}
	switch dto.Outcome {
	case sourcing.AcquisitionPublishing:
		dto.Outcome = "outcome_unknown"
	case sourcing.AcquisitionPublished:
		if result.Publication == nil || result.Publication.Receipt.CatalogVersion == 0 {
			writeAcquisitionError(c, sourcing.ErrAcquisitionUnavailable)
			return
		}
		r := result.Publication.Receipt
		dto.ProductKey = r.ProductKey
		dto.PublicationID = r.PublicationID
		dto.CatalogVersion = strconv.FormatUint(r.CatalogVersion, 10)
		for _, warning := range result.Publication.Envelope.Warnings {
			dto.Warnings = append(dto.Warnings, acquisitionWarningDTO{Code: warning.Code, Field: warning.Field})
		}
		for _, missing := range result.Publication.Envelope.MissingFacts {
			dto.MissingFacts = append(dto.MissingFacts, acquisitionMissingDTO{Field: missing.Field, Reason: missing.Reason})
		}
	case sourcing.AcquisitionAcquiring, sourcing.AcquisitionPrepared, sourcing.AcquisitionFailed:
	default:
		writeAcquisitionError(c, sourcing.ErrAcquisitionUnavailable)
		return
	}
	raw, err := json.Marshal(dto)
	if err != nil || len(raw) > sourcing.MaxAcquisitionCommandBytes {
		writeAcquisitionError(c, sourcing.ErrAcquisitionUnavailable)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

func writeAcquisitionError(c *gin.Context, err error) {
	status, code := http.StatusServiceUnavailable, "ACQUISITION_UNAVAILABLE"
	switch {
	case errors.Is(err, sourcing.ErrPublicationForbidden):
		status, code = http.StatusForbidden, "FORBIDDEN"
	case errors.Is(err, sourcing.ErrInvalidAcquisition):
		status, code = http.StatusBadRequest, "INVALID_ACQUISITION"
	case errors.Is(err, sourcing.ErrSourcePublicationTooLarge):
		status, code = http.StatusRequestEntityTooLarge, "SOURCE_TOO_LARGE"
	case errors.Is(err, sourcing.ErrAcquisitionCapacity):
		status, code = http.StatusTooManyRequests, "ACQUISITION_CAPACITY"
	case errors.Is(err, sourcing.ErrAcquisitionConflict):
		status, code = http.StatusConflict, "IDEMPOTENCY_CONFLICT"
	case errors.Is(err, sourcing.ErrAcquisitionNotFound):
		status, code = http.StatusNotFound, "ACQUISITION_NOT_FOUND"
	case errors.Is(err, sourcing.ErrAcquisitionUnknown), errors.Is(err, sourcing.ErrSourcePublicationOutcomeUnknown):
		code = "OUTCOME_UNKNOWN"
	case errors.Is(err, context.DeadlineExceeded):
		status, code = http.StatusGatewayTimeout, "DEADLINE_EXCEEDED"
	case errors.Is(err, sourcing.ErrAcquisitionFailed):
		status, code = http.StatusBadGateway, "SOURCE_UNAVAILABLE"
	}
	c.AbortWithStatusJSON(status, gin.H{"schemaVersion": 1, "error": gin.H{"code": code}})
}

type productAcquisitionModule struct{ routes []httproute.Descriptor }

func (productAcquisitionModule) Name() string { return "product-acquisition" }
func (productAcquisitionModule) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Workbench.Enabled
}
func (m productAcquisitionModule) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(m.routes...)
	return nil
}

func buildProductAcquisitionModule(ctx context.Context, db *gorm.DB, dependencies routeAuthDependencies, authorizer *authz.ListingKitAuthorizer, provider sourcing.PublicAcquirer) (kernelmodule.Module, error) {
	if dependencies.organizationResolver == nil || authorizer == nil || provider == nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	if err := acquisitionstore.VerifyRuntimePermissions(ctx, db); err != nil {
		return nil, err
	}
	// Reuse the existing request-local live organization capability, not Review
	// domain behavior and not cached request roles or another IAM implementation.
	live := &productReviewLiveOrganizationAccess{resolver: dependencies.organizationResolver, now: time.Now}
	service, err := productsourcing.NewPublicAcquisition(ctx, db, live, authorizer, provider)
	if err != nil {
		return nil, err
	}
	binder := productReviewCapabilityBinder{now: time.Now}
	return productAcquisitionModule{routes: productAcquisitionRoutes(service, binder.Bind)}, nil
}
