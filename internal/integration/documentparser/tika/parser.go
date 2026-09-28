package tika

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"task-processor/internal/knowledge"
)

type Parser struct {
	endpoint string
	client   *http.Client
}

func New(endpoint string) (*Parser, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return nil, knowledge.ErrInvalid
	}
	// Endpoint is a private operator configuration, never a document URL.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.MaxConnsPerHost = 2
	return &Parser{endpoint: strings.TrimRight(endpoint, "/"), client: &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (p *Parser) Parse(ctx context.Context, contentType string, data []byte) knowledge.ParseResult {
	if contentType != "application/pdf" && contentType != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" || len(data) == 0 || len(data) > knowledge.MaxUploadBytes {
		return knowledge.ParseResult{Failure: "UNSUPPORTED_DOCUMENT"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, p.endpoint+"/rmeta/text", bytes.NewReader(data))
	if err != nil {
		return knowledge.ParseResult{Failure: "PARSER_UNAVAILABLE", Transient: true}
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return knowledge.ParseResult{Failure: "PARSER_UNAVAILABLE", Transient: true}
	}
	defer response.Body.Close()
	if response.StatusCode == 429 || response.StatusCode >= 500 {
		return knowledge.ParseResult{Failure: "PARSER_UNAVAILABLE", Transient: true}
	}
	if response.StatusCode != 200 {
		return knowledge.ParseResult{Failure: "CORRUPT_OR_UNSUPPORTED_DOCUMENT"}
	}
	// Metadata is untrusted and can contain document text/stack traces. Keep it
	// private; only fixed product-safe categories leave this adapter.
	body, err := io.ReadAll(io.LimitReader(response.Body, 4*knowledge.MaxTextBytes+1))
	if err != nil {
		return knowledge.ParseResult{Failure: "PARSER_UNAVAILABLE", Transient: true}
	}
	if len(body) > 4*knowledge.MaxTextBytes {
		return knowledge.ParseResult{Failure: "PARSER_OUTPUT_TOO_LARGE"}
	}
	var entries []map[string]json.RawMessage
	if json.Unmarshal(body, &entries) != nil || len(entries) != 1 {
		return knowledge.ParseResult{Failure: "INVALID_PARSER_RESPONSE"}
	}
	metadata := entries[0]
	var detected, text string
	if json.Unmarshal(metadata["Content-Type"], &detected) != nil || detected != contentType || json.Unmarshal(metadata["tk:content"], &text) != nil {
		return knowledge.ParseResult{Failure: "DOCUMENT_TYPE_MISMATCH"}
	}
	warning := ""
	for key, value := range metadata {
		if (strings.HasPrefix(key, "tk:exception:") || strings.HasPrefix(key, "tk:warn:") || key == "tk:task-deadline-reached") && string(value) != "false" && string(value) != "null" && string(value) != "\"\"" {
			warning = "INCOMPLETE_EXTRACTION"
		}
	}
	return knowledge.NormalizeText(text, warning)
}
