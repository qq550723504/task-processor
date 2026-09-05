// Package httpapi contains the unregistered #30 import HTTP contract.
// Production composition is intentionally pending source-account ownership
// cutover. No route/module or source-account implementation is provided here.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	a1688 "task-processor/internal/integration/crawler/a1688"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

const MaxImportBytes = 2 << 20
const ImportTimeout = 30 * time.Second

var ErrAccessDenied = errors.New("source import access denied")
var ErrInvalidImport = errors.New("invalid source import")

type ImportRequest struct {
	URL         string                  `json:"url"`
	Product     *productSnapshotRequest `json:"product"`
	RawSnapshot string                  `json:"raw_snapshot,omitempty"`
	SourceRunID string                  `json:"source_run_id,omitempty"`
	RequestID   string                  `json:"request_id,omitempty"`
	// Required selector: explicit zero is public; positive is account access.
	SourceAccountID *int64 `json:"source_account_id"`
	StoreID         string `json:"store_id"`
}

type ImportCommand struct {
	URL             string
	Product         *a1688.Alibaba1688ProductSnapshot
	RawSnapshot     string
	SourceRunID     string
	RequestID       string
	SourceAccountID int64
	StoreID         string
	OrganizationID  string
	ActorID         string
}

type ImportResult struct {
	Publication    catalog.PublishedSnapshot `json:"publication"`
	SourceIdentity sourcing.SourceIdentity   `json:"source_identity"`
	SourceWarnings []sourcing.SourceWarning  `json:"source_warnings,omitempty"`
}

// Importer must revalidate command identity and Store/source-account access
// before any publication, including on replay. Implementations must cooperate
// with cancellation. This transport is not an authorization substitute.
type Importer interface {
	Import(context.Context, ImportCommand) (ImportResult, error)
}
type Handler struct{ importer Importer }

func NewHandler(importer Importer) *Handler { return &Handler{importer: importer} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, 405, "method_not_allowed")
		return
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(r.Context())
	if !ok || identity.EffectiveOrganizationID == "" {
		writeError(w, 401, "verified_organization_required")
		return
	}
	if h == nil || h.importer == nil {
		writeError(w, 503, "service_unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), ImportTimeout)
	defer cancel()
	if ctx.Err() != nil {
		writeError(w, 504, "import_deadline_exceeded")
		return
	}
	// Real net/http servers bound slow input reads as well as importer work.
	// In-memory transports may not support connection read deadlines.
	controller := http.NewResponseController(w)
	deadline, _ := ctx.Deadline()
	if err := controller.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		writeError(w, 503, "read_deadline_unavailable")
		return
	}
	defer func() { _ = controller.SetReadDeadline(time.Time{}) }()
	r.Body = http.MaxBytesReader(w, r.Body, MaxImportBytes)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request *ImportRequest
	err := decoder.Decode(&request)
	if err == nil {
		var trailing any
		err = decoder.Decode(&trailing)
		if errors.Is(err, io.EOF) {
			err = nil
		} else if err == nil {
			err = ErrInvalidImport
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		var timeout interface{ Timeout() bool }
		switch {
		case errors.As(err, &tooLarge):
			writeError(w, 413, "import_too_large")
		case ctx.Err() != nil || errors.As(err, &timeout) && timeout.Timeout():
			writeError(w, 504, "import_deadline_exceeded")
		default:
			writeError(w, 400, "invalid_request")
		}
		return
	}
	if request == nil || request.SourceAccountID == nil || *request.SourceAccountID < 0 || strings.TrimSpace(request.URL) == "" || request.Product == nil || strings.TrimSpace(request.StoreID) == "" {
		writeError(w, 400, "invalid_request")
		return
	}
	if ctx.Err() != nil {
		writeError(w, 504, "import_deadline_exceeded")
		return
	}
	product := request.Product.toSnapshot()
	result, err := h.importer.Import(ctx, ImportCommand{
		URL: request.URL, Product: &product, RawSnapshot: request.RawSnapshot,
		SourceRunID: request.SourceRunID, RequestID: request.RequestID,
		SourceAccountID: *request.SourceAccountID, StoreID: request.StoreID,
		OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID,
	})
	if err != nil || ctx.Err() != nil {
		switch {
		case ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
			writeError(w, 504, "import_deadline_exceeded")
		case errors.Is(err, ErrAccessDenied):
			writeError(w, 403, "source_access_denied")
		case errors.Is(err, catalog.ErrPublicationConflict):
			writeError(w, 409, "publication_conflict")
		case errors.Is(err, ErrInvalidImport):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(struct {
				Error    string                   `json:"error"`
				Warnings []sourcing.SourceWarning `json:"source_warnings,omitempty"`
			}{Error: "invalid_source", Warnings: result.SourceWarnings})
		default:
			writeError(w, 500, "import_failed")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(importResponse{
		Publication: result.Publication, SourceWarnings: result.SourceWarnings,
		SourceIdentity: sourceIdentityResponse{
			SourceType: result.SourceIdentity.SourceType, SourcePlatform: result.SourceIdentity.SourcePlatform,
			SourceID: result.SourceIdentity.SourceID, SourceURL: result.SourceIdentity.SourceURL,
			SourceVersion: result.SourceIdentity.SourceVersion, SourceFingerprint: result.SourceIdentity.SourceFingerprint,
		},
	})
}

type importResponse struct {
	Publication    catalog.PublishedSnapshot `json:"publication"`
	SourceIdentity sourceIdentityResponse    `json:"source_identity"`
	SourceWarnings []sourcing.SourceWarning  `json:"source_warnings,omitempty"`
}

// Deliberately project only current source facts, without legacy normalization.
type sourceIdentityResponse struct {
	SourceType        string `json:"source_type"`
	SourcePlatform    string `json:"source_platform"`
	SourceID          string `json:"source_id"`
	SourceURL         string `json:"source_url"`
	SourceVersion     string `json:"source_version"`
	SourceFingerprint string `json:"source_fingerprint"`
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
