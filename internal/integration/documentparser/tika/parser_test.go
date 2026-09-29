package tika

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParserBoundsMetadataAndPreservesUsefulPartialText(t *testing.T) {
	for _, test := range []struct {
		name, body, text, failure, warning string
		status                             int
		transient                          bool
	}{
		{"available", `[{"Content-Type":"application/pdf","tk:content":"hello"}]`, "hello", "", "", 200, false},
		{"partial", `[{"Content-Type":"application/pdf","tk:content":"hello","tk:exception:container-exception":"private stack /secret"}]`, "hello", "", "INCOMPLETE_EXTRACTION", 200, false},
		{"scan", `[{"Content-Type":"application/pdf","tk:content":" \n"}]`, "", "NO_EXTRACTABLE_TEXT", "", 200, false},
		{"spoof", `[{"Content-Type":"image/png","tk:content":"hello"}]`, "", "DOCUMENT_TYPE_MISMATCH", "", 200, false},
		{"embedded", `[{"Content-Type":"application/pdf","tk:content":"hello"},{}]`, "", "INVALID_PARSER_RESPONSE", "", 200, false},
		{"encrypted", `[{"Content-Type":"application/pdf","tk:content":"","tk:exception:container-exception":"encrypted"}]`, "", "NO_EXTRACTABLE_TEXT", "", 200, false},
		{"busy", "private parser detail", "", "PARSER_UNAVAILABLE", "", 429, true},
		{"crashed", "private parser detail", "", "PARSER_UNAVAILABLE", "", 503, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "PUT" || r.URL.Path != "/rmeta/text" || r.Header.Get("Content-Type") != "application/pdf" {
					t.Error("wrong parser contract")
				}
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			parser, err := New(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			result := parser.Parse(context.Background(), "application/pdf", []byte("%PDF-test"))
			if result.Text != test.text || result.Failure != test.failure || result.Warning != test.warning || result.Transient != test.transient {
				t.Fatalf("result %+v", result)
			}
			if strings.Contains(result.Failure+result.Warning, "private") {
				t.Fatal("parser detail escaped")
			}
		})
	}
}
