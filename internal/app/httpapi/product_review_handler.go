package httpapi

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/review"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	sigjson "sigs.k8s.io/json"
)

func productReviewRoutes(s *review.Service) []httproute.Descriptor {
	base := "/api/product/text-proposals"
	specs := []struct{ method, path, kind string }{{"POST", base, "create"}, {"GET", base, "list"}, {"GET", base + "/:proposal_id", "get"}, {"POST", base + "/:proposal_id/decisions", "decision"}, {"POST", base + "/:proposal_id/apply", "apply"}}
	routes := make([]httproute.Descriptor, 0, len(specs))
	for _, spec := range specs {
		policy, permission := httproute.OrganizationAccessPolicyLiveWrite, authz.PermissionListingKitAdminWrite
		if spec.kind == "get" || spec.kind == "list" {
			policy = httproute.OrganizationAccessPolicyCachedRead
			permission = authz.PermissionListingKitAdminRead
		}
		routes = append(routes, httproute.Descriptor{Method: spec.method, Path: spec.path, Module: "product-review", Permission: permission, AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: policy, Handler: func(c *gin.Context) {
			if spec.kind == "list" {
				listProductReviews(c, s)
				return
			}
			productReviewRequest(c, s, spec.kind)
		}})
	}
	return routes
}

func readProductReviewGETBody(c *gin.Context) error {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
	var transport net.Error
	if c.Request.Context().Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.As(err, &transport) && transport.Timeout() {
		return context.DeadlineExceeded
	}
	if err != nil {
		return review.ErrUnavailable
	}
	if len(body) != 0 {
		return review.ErrInvalid
	}
	return nil
}

func listProductReviews(c *gin.Context, service *review.Service) {
	if err := readProductReviewGETBody(c); err != nil {
		writeProductReviewError(c, err)
		return
	}
	rawQuery := c.Request.URL.RawQuery
	if len(rawQuery) > maxProductReviewQueryBytes {
		writeProductReviewError(c, review.ErrInvalid)
		return
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		writeProductReviewError(c, review.ErrInvalid)
		return
	}
	request, err := parseProductReviewCollectionQuery(query)
	if err != nil {
		writeProductReviewError(c, err)
		return
	}
	page, err := service.List(c.Request.Context(), request)
	if err != nil {
		writeProductReviewError(c, err)
		return
	}
	wire, err := marshalProductReviewPage(page)
	if c.Request.Context().Err() != nil {
		writeProductReviewError(c, c.Request.Context().Err())
		return
	}
	if err != nil || len(wire) > maxProductReviewResponseBytes {
		writeProductReviewError(c, review.ErrUnavailable)
		return
	}
	setProductReviewHeaders(c)
	c.Data(http.StatusOK, "application/json; charset=utf-8", wire)
}

func productReviewRequest(c *gin.Context, s *review.Service, kind string) {
	ctx := c.Request.Context()
	if c.Request.URL.RawQuery != "" {
		reviewResponse(c, review.View{}, review.ErrInvalid)
		return
	}
	var v review.View
	var err error
	if kind == "get" {
		if err = readProductReviewGETBody(c); err != nil {
			reviewResponse(c, v, err)
			return
		}
		v, err = s.Get(ctx, c.Param("proposal_id"))
		reviewResponse(c, v, err)
		return
	}
	var create review.CreateInput
	var decision review.DecisionInput
	var apply review.ApplyInput
	var input any = &create
	if kind == "decision" {
		input = &decision
	} else if kind == "apply" {
		input = &apply
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	defer c.Request.Body.Close()
	raw, err := io.ReadAll(c.Request.Body)
	var ne net.Error
	if ctx.Err() != nil || errors.As(err, &ne) && ne.Timeout() {
		reviewResponse(c, v, context.DeadlineExceeded)
		return
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		reviewResponse(c, v, review.ErrTooLarge)
		return
	}
	if err == nil {
		if !validReviewJSONUnicode(raw) {
			reviewResponse(c, v, review.ErrInvalid)
			return
		}
		var violations []error
		violations, err = sigjson.UnmarshalStrict(raw, input)
		if len(violations) > 0 {
			err = review.ErrInvalid
		}
	}
	keys := c.Request.Header.Values("Idempotency-Key")
	if err != nil || len(keys) != 1 {
		reviewResponse(c, v, review.ErrInvalid)
		return
	}
	switch kind {
	case "create":
		v, err = s.Create(ctx, keys[0], create)
	case "decision":
		v, err = s.Decide(ctx, keys[0], c.Param("proposal_id"), decision)
	case "apply":
		v, err = s.Apply(ctx, keys[0], c.Param("proposal_id"), apply)
	}
	reviewResponse(c, v, err)
}

// encoding/json replaces malformed string encodings. Reject those encodings
// before the established strict decoder, so an edit never silently changes text.
func validReviewJSONUnicode(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return false
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return false
		}
		code, e := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if e != nil {
			return false
		}
		i += 4
		if code >= 0xDC00 && code <= 0xDFFF {
			return false
		}
		if code >= 0xD800 && code <= 0xDBFF {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, e := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if e != nil || low < 0xDC00 || low > 0xDFFF {
				return false
			}
			i += 6
		}
	}
	return true
}
func reviewResponse(c *gin.Context, v review.View, err error) {
	if err == nil {
		wire, marshalErr := marshalProductReviewView(v)
		if marshalErr != nil || len(wire) > maxProductReviewResponseBytes {
			writeProductReviewError(c, review.ErrUnavailable)
			return
		}
		setProductReviewHeaders(c)
		c.Data(http.StatusOK, "application/json; charset=utf-8", wire)
		return
	}
	writeProductReviewError(c, err)
}

func setProductReviewHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
}

func writeProductReviewError(c *gin.Context, err error) {
	status, code := 503, "unavailable"
	switch {
	case errors.Is(err, review.ErrInvalid):
		status, code = 400, "invalid_request"
	case errors.Is(err, review.ErrForbidden):
		status, code = 403, "permission_denied"
	case errors.Is(err, review.ErrNotFound), errors.Is(err, catalog.ErrSnapshotNotReady):
		status, code = 404, "not_found"
	case errors.Is(err, catalog.ErrStaleSnapshot):
		status, code = 409, "stale_product_version"
	case errors.Is(err, review.ErrConflict), errors.Is(err, catalog.ErrPublicationConflict):
		status, code = 409, "operation_conflict"
	case errors.Is(err, review.ErrTooLarge):
		status, code = 413, "input_too_large"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status, code = 504, "deadline_exceeded"
	}
	setProductReviewHeaders(c)
	c.JSON(status, gin.H{"error": code})
}
