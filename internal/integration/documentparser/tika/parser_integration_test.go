//go:build integration

package tika

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func testPDF(text string) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	stream := "BT /F1 12 Tf 50 700 Td (" + text + ") Tj ET"
	objects = append(objects, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return out.Bytes()
}
func TestTikaPinnedPrivateParserExtractsRealDocuments(t *testing.T) {
	endpoint := os.Getenv("KNOWLEDGE_TIKA_TEST_URL")
	if endpoint == "" {
		t.Skip("explicit isolated parser endpoint required")
	}
	target, err := url.Parse(endpoint)
	if err != nil || target.Hostname() != "127.0.0.1" {
		t.Fatal("test endpoint must be isolated loopback")
	}
	parser, err := New(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	var docx bytes.Buffer
	writer := zip.NewWriter(&docx)
	for name, content := range map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Knowledge DOCX fixture</w:t></w:r></w:p></w:body></w:document>`,
	} {
		file, e := writer.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = file.Write([]byte(content))
	}
	_ = writer.Close()
	for _, test := range []struct {
		name, mime, want string
		data             []byte
	}{
		{"text PDF", "application/pdf", "Knowledge PDF fixture", testPDF("Knowledge PDF fixture")},
		{"DOCX", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "Knowledge DOCX fixture", docx.Bytes()},
		{"empty PDF", "application/pdf", "", testPDF("")},
		{"corrupt PDF", "application/pdf", "", []byte("%PDF-corrupt")},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			result := parser.Parse(ctx, test.mime, test.data)
			if test.want != "" {
				if result.Failure != "" || !strings.Contains(result.Text, test.want) {
					t.Fatalf("extraction: %+v", result)
				}
			} else if result.Text != "" || result.Failure == "" {
				t.Fatalf("unreadable file advertised as available: %+v", result)
			}
		})
	}
}
