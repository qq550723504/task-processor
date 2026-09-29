package httpapi

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"task-processor/internal/knowledge"
	"testing"
)

func TestStrictNamesRejectAmbiguousCommands(t *testing.T) {
	for _, body := range []string{`{"name":"a","name":"b"}`, `{"name":"a","organizationId":"other"}`, `{"name":"a"}{}`, `{"name":1}`, strings.Repeat("x", 8193)} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		if _, ok := readName(ctx); ok {
			t.Fatal("ambiguous command accepted")
		}
	}
}
func TestMultipartPreservesFilenameForDomainValidation(t *testing.T) {
	var data bytes.Buffer
	writer := multipart.NewWriter(&data)
	_ = writer.WriteField("name", "name")
	file, _ := writer.CreateFormFile("file", "../secret.txt")
	_, _ = file.Write([]byte("hello"))
	_ = writer.Close()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/", &data)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	_, filename, body, err := readUpload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = knowledge.DetectDocument(filename, body); err == nil {
		t.Fatal("path-bearing multipart filename normalized past admission")
	}
}
func TestDisabledBaseAndSourceRedactHistoricalNames(t *testing.T) {
	for _, source := range []knowledge.Source{
		{State: knowledge.Disabled, BaseState: knowledge.Active, Name: "private", LatestRevision: &knowledge.Revision{Filename: "private.txt"}},
		{State: knowledge.Active, BaseState: knowledge.Disabled, Name: "private", LatestRevision: &knowledge.Revision{Filename: "private.txt"}},
	} {
		result := publicResult(knowledge.Result{Source: &source, Revision: &knowledge.Revision{Filename: "private.txt"}})
		if result.Source.Name != "" || result.Source.LatestRevision.Filename != "" || result.Revision.Filename != "" {
			t.Fatal("historical labels leaked")
		}
	}
}
func TestMultipartTotalBoundIncludesOversizedEpilogue(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "name")
	file, _ := writer.CreateFormFile("file", "small.txt")
	_, _ = file.Write([]byte("hello"))
	_ = writer.Close()
	body.WriteString(strings.Repeat("x", knowledge.MaxUploadBytes))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/", &body)
	ctx.Request.ContentLength = -1 // streaming callers must obey the same total request bound
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	if _, _, _, err := readUpload(ctx); err == nil {
		t.Fatal("oversized MIME epilogue escaped total request bound")
	}
}
