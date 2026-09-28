package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"task-processor/internal/authidentity"
	"task-processor/internal/knowledge"
)

type Handler struct{ service *knowledge.Service }

func NewHandler(service *knowledge.Service) (*Handler, error) {
	if service == nil {
		return nil, knowledge.ErrUnavailable
	}
	return &Handler{service: service}, nil
}
func failure(c *gin.Context, err error) {
	status := http.StatusServiceUnavailable
	code := knowledge.ErrUnavailable.Error()
	for _, known := range []error{knowledge.ErrInvalid, knowledge.ErrNotFound, knowledge.ErrConflict, knowledge.ErrInactive, knowledge.ErrSourceLimit, knowledge.ErrRevisionBusy, knowledge.ErrNotReadable, knowledge.ErrIntegrity} {
		if errors.Is(err, known) {
			code = known.Error()
			switch known {
			case knowledge.ErrInvalid:
				status = 400
			case knowledge.ErrNotFound:
				status = 404
			default:
				status = 409
			}
			break
		}
	}
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": "知识库请求未完成", "requestId": c.GetHeader("X-Request-ID"), "fieldErrors": []any{}})
}
func scope(c *gin.Context) (knowledge.Scope, bool) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	id, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || id.EffectiveOrganizationID == "" {
		failure(c, knowledge.ErrInvalid)
		return knowledge.Scope{}, false
	}
	return knowledge.Scope{OrganizationID: id.EffectiveOrganizationID, ActorID: id.UserID}, true
}
func noQuery(c *gin.Context) bool {
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		failure(c, knowledge.ErrInvalid)
		return false
	}
	return true
}
func query(c *gin.Context) (int, int, bool) {
	if len(c.Request.URL.RawQuery) > 256 || c.Request.URL.ForceQuery {
		return 0, 0, false
	}
	q, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return 0, 0, false
	}
	page, size := 1, 20
	for key, values := range q {
		if len(values) != 1 || key != "page" && key != "pageSize" {
			return 0, 0, false
		}
		n, e := strconv.Atoi(values[0])
		if e != nil || strconv.Itoa(n) != values[0] || n < 1 {
			return 0, 0, false
		}
		if key == "page" {
			page = n
		} else {
			size = n
		}
	}
	return page, size, page <= 100000 && size <= 100
}
func command(c *gin.Context, s knowledge.Scope, kind string) (knowledge.Command, bool) {
	cmd := knowledge.Command{Scope: s, Kind: kind, BaseID: c.Param("knowledge_base_id"), SourceID: c.Param("source_id")}
	keys := c.Request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !knowledge.ValidID(keys[0]) || c.GetHeader("Content-Encoding") != "" {
		failure(c, knowledge.ErrInvalid)
		return cmd, false
	}
	cmd.Key = keys[0]
	if kind != "base_create" && kind != "source_create" {
		values := c.Request.Header.Values("If-Match")
		if len(values) != 1 {
			return cmd, invalid(c)
		}
		value := values[0]
		if len(value) < 3 || !strings.HasPrefix(value, "\"") || !strings.HasSuffix(value, "\"") {
			return cmd, invalid(c)
		}
		version, err := strconv.ParseInt(value[1:len(value)-1], 10, 64)
		if err != nil || version < 1 || version > 9007199254740991 || strconv.FormatInt(version, 10) != value[1:len(value)-1] {
			return cmd, invalid(c)
		}
		cmd.Version = version
	}
	return cmd, true
}
func invalid(c *gin.Context) bool { failure(c, knowledge.ErrInvalid); return false }
func readName(c *gin.Context) (string, bool) {
	contentType, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || contentType != "application/json" || len(params) > 1 || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") {
		return "", invalid(c)
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10))
	if err != nil || !utf8.Valid(body) {
		return "", invalid(c)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, e := decoder.Token()
	if e != nil || token != json.Delim('{') {
		return "", invalid(c)
	}
	name := ""
	seen := false
	for decoder.More() {
		key, e := decoder.Token()
		if e != nil || key != "name" || seen {
			return "", invalid(c)
		}
		seen = true
		if decoder.Decode(&name) != nil {
			return "", invalid(c)
		}
	}
	if token, e = decoder.Token(); e != nil || token != json.Delim('}') || !seen {
		return "", invalid(c)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", invalid(c)
	}
	return name, true
}
func emptyBody(c *gin.Context) bool {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1))
	return err == nil && len(body) == 0
}
func (h *Handler) ListBases(c *gin.Context) {
	s, ok := scope(c)
	if !ok {
		return
	}
	page, size, ok := query(c)
	if !ok {
		invalid(c)
		return
	}
	items, total, err := h.service.ListBases(c.Request.Context(), s, page, size)
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items, "pagination": gin.H{"page": page, "pageSize": size, "total": total}})
}
func (h *Handler) GetBase(c *gin.Context) {
	s, ok := scope(c)
	if !ok || !noQuery(c) {
		return
	}
	base, err := h.service.GetBase(c.Request.Context(), s, c.Param("knowledge_base_id"))
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(200, base)
}
func (h *Handler) mutate(c *gin.Context, kind string) {
	s, ok := scope(c)
	if !ok || !noQuery(c) {
		return
	}
	cmd, ok := command(c, s, kind)
	if !ok {
		return
	}
	if kind == "base_create" || kind == "base_update" {
		name, valid := readName(c)
		if !valid {
			return
		}
		cmd.Name = name
	} else if !emptyBody(c) {
		invalid(c)
		return
	}
	result, err := h.service.Mutate(c.Request.Context(), cmd)
	if err != nil {
		failure(c, err)
		return
	}
	status := 200
	if kind == "base_create" {
		status = 201
	}
	c.JSON(status, publicResult(result))
}
func (h *Handler) CreateBase(c *gin.Context)    { h.mutate(c, "base_create") }
func (h *Handler) UpdateBase(c *gin.Context)    { h.mutate(c, "base_update") }
func (h *Handler) DisableBase(c *gin.Context)   { h.mutate(c, "base_disable") }
func (h *Handler) DisableSource(c *gin.Context) { h.mutate(c, "source_disable") }
func (h *Handler) ListSources(c *gin.Context) {
	s, ok := scope(c)
	if !ok || !noQuery(c) {
		return
	}
	items, err := h.service.ListSources(c.Request.Context(), s, c.Param("knowledge_base_id"))
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(200, gin.H{"items": publicSources(items)})
}
func publicSources(sources []knowledge.Source) []knowledge.Source {
	for i := range sources {
		if sources[i].State != knowledge.Active || sources[i].BaseState != knowledge.Active {
			sources[i].Name = ""
			if sources[i].LatestRevision != nil {
				sources[i].LatestRevision.Filename = ""
			}
			if sources[i].CurrentReadableRevision != nil {
				sources[i].CurrentReadableRevision.Filename = ""
			}
		}
	}
	return sources
}
func publicResult(result knowledge.Result) knowledge.Result {
	if result.Source != nil {
		source := publicSources([]knowledge.Source{*result.Source})[0]
		result.Source = &source
		if (source.State != knowledge.Active || source.BaseState != knowledge.Active) && result.Revision != nil {
			revision := *result.Revision
			revision.Filename = ""
			result.Revision = &revision
		}
	}
	return result
}
func (h *Handler) GetSource(c *gin.Context) {
	s, ok := scope(c)
	if !ok || !noQuery(c) {
		return
	}
	source, err := h.service.GetSource(c.Request.Context(), s, c.Param("source_id"))
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(200, publicSources([]knowledge.Source{source})[0])
}
func (h *Handler) Preview(c *gin.Context) {
	s, ok := scope(c)
	if !ok || !noQuery(c) {
		return
	}
	preview, err := h.service.Preview(c.Request.Context(), s, c.Param("source_id"), c.Param("revision_id"))
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(200, preview)
}
func readUpload(c *gin.Context) (name, filename string, data []byte, err error) {
	if c.Request.ContentLength > knowledge.MaxUploadBytes {
		return "", "", nil, knowledge.ErrInvalid
	}
	contentType, params, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if e != nil || contentType != "multipart/form-data" || params["boundary"] == "" || len(params) != 1 {
		return "", "", nil, knowledge.ErrInvalid
	}
	boundedBody := http.MaxBytesReader(c.Writer, c.Request.Body, knowledge.MaxUploadBytes)
	reader := multipart.NewReader(boundedBody, params["boundary"])
	seen := map[string]bool{}
	for {
		part, e := reader.NextPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", "", nil, knowledge.ErrInvalid
		}
		field := part.FormName()
		if seen[field] || field != "name" && field != "file" || part.Header.Get("Content-Transfer-Encoding") != "" {
			_ = part.Close()
			return "", "", nil, knowledge.ErrInvalid
		}
		seen[field] = true
		if field == "file" {
			_, disposition, parseErr := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
			if parseErr != nil {
				return "", "", nil, knowledge.ErrInvalid
			}
			filename = disposition["filename"]
			data, e = io.ReadAll(io.LimitReader(part, knowledge.MaxUploadBytes+1))
		} else {
			var raw []byte
			raw, e = io.ReadAll(io.LimitReader(part, 1025))
			if len(raw) > 1024 || part.FileName() != "" {
				e = knowledge.ErrInvalid
			}
			name = string(raw)
		}
		_ = part.Close()
		if e != nil {
			return "", "", nil, knowledge.ErrInvalid
		}
	}
	if !seen["name"] || !seen["file"] {
		return "", "", nil, knowledge.ErrInvalid
	}
	// MIME permits an epilogue. Bound the whole request before durable admission.
	if _, err = io.Copy(io.Discard, boundedBody); err != nil {
		return "", "", nil, knowledge.ErrInvalid
	}
	return
}
func (h *Handler) upload(c *gin.Context, kind string) {
	s, ok := scope(c)
	if !ok || !noQuery(c) {
		return
	}
	cmd, ok := command(c, s, kind)
	if !ok {
		return
	}
	name, filename, data, err := readUpload(c)
	if err != nil {
		failure(c, err)
		return
	}
	cmd.Name = name
	result, err := h.service.Upload(c.Request.Context(), cmd, filename, data)
	if err != nil {
		failure(c, err)
		return
	}
	c.JSON(202, publicResult(result))
}
func (h *Handler) CreateSource(c *gin.Context)   { h.upload(c, "source_create") }
func (h *Handler) CreateRevision(c *gin.Context) { h.upload(c, "revision_create") }
