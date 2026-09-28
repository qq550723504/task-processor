package knowledge

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestDocumentAdmissionUsesContentAndBounds(t *testing.T) {
	var docx bytes.Buffer
	zw := zip.NewWriter(&docx)
	for name, body := range map[string]string{"[Content_Types].xml": `<Types><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`, "word/document.xml": `<document><body>guideline</body></document>`} {
		f, _ := zw.Create(name)
		_, _ = f.Write([]byte(body))
	}
	_ = zw.Close()
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"brand.txt", []byte("hello 品牌"), "text/plain"},
		{"brand.md", []byte("# 品牌"), "text/markdown"},
		{"brand.pdf", []byte("%PDF-1.7\nfixture"), "application/pdf"},
		{"brand.docx", docx.Bytes(), "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"forged.pdf", []byte("ordinary text"), ""},
		{"payload.txt", []byte("%PDF-1.7\n"), ""},
		{"binary.txt", []byte{0, 1, 2}, ""},
		{"invalid.txt", []byte{0xff}, ""},
		{"empty.md", nil, ""},
		{"a.xlsx", docx.Bytes(), ""},
		{strings.Repeat("a", 256) + ".txt", []byte("a"), ""},
		{"../a.txt", []byte("a"), ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectDocument(tt.name, tt.data)
			if tt.want == "" {
				if err == nil {
					t.Fatalf("unsafe admission: %q", got)
				}
			} else if err != nil || got != tt.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}
func TestNameNormalizationRejectsControlsAndExcess(t *testing.T) {
	for _, bad := range []string{"", "  ", "a\n", strings.Repeat("品", 121), string([]byte{0xff})} {
		if _, err := NormalizeName(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if got, err := NormalizeName("  品牌  "); err != nil || got != "品牌" {
		t.Fatalf("%q %v", got, err)
	}
}
