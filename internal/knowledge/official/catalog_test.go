package official

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func article(revision, body string) Article {
	return Article{Summary: Summary{ID: "guide", Revision: revision, Title: "Guide", Summary: "How to use", Category: "AI电商应用指南", UpdatedAt: "2026-10-11", MaintainedBy: "硕米", Digest: Digest(body)}, Body: body, Sources: []Source{{Title: "Source", URL: "https://github.com/qq550723504/task-processor/blob/main/docs/architecture/agent-knowledge-context-v1.md"}}}
}
func TestCatalogExactVersionAndImmutableCopies(t *testing.T) {
	original := article("1", "original")
	next := article("2", "next")
	c, err := NewCatalog([]Article{original, next})
	require.NoError(t, err)
	original.Sources[0].Title = "changed"
	require.Equal(t, "2", c.List()[0].Revision)
	got, err := c.Get("guide", "1")
	require.NoError(t, err)
	require.Equal(t, "original", got.Body)
	require.Equal(t, "Source", got.Sources[0].Title)
	got.Sources[0].Title = "mutated result"
	again, err := c.Get("guide", "1")
	require.NoError(t, err)
	require.Equal(t, "Source", again.Sources[0].Title)
	_, err = c.Get("guide", "3")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = c.Get("guide", "01")
	require.ErrorIs(t, err, ErrInvalid)
}
func TestCatalogRejectsCorruptionAndUnapprovedSources(t *testing.T) {
	for _, modify := range []func(*Article){
		func(a *Article) { a.Digest = strings.Repeat("0", 64) },
		func(a *Article) { a.Body = strings.Repeat("x", MaxBodyBytes+1); a.Digest = Digest(a.Body) },
		func(a *Article) { a.Revision = "01" }, func(a *Article) { a.ID = "../secret" },
		func(a *Article) { a.Sources[0].URL = "javascript:alert(1)" },
		func(a *Article) {
			a.Sources[0].URL = "https://github.com.evil.test/qq550723504/task-processor/blob/main/x"
		},
		func(a *Article) { a.Sources[0].URL = "https://github.com/other/repo/blob/main/x" },
		func(a *Article) { a.Body = ""; a.Digest = Digest(a.Body) },
	} {
		a := article("1", "text")
		modify(&a)
		_, err := NewCatalog([]Article{a})
		require.Error(t, err)
	}
	a := article("1", "text")
	_, err := NewCatalog([]Article{a, a})
	require.Error(t, err)
}
func TestEmbeddedGuideIsRealAndVersionBound(t *testing.T) {
	c, err := NewEmbeddedCatalog()
	require.NoError(t, err)
	require.Len(t, c.List(), 1)
	summary := c.List()[0]
	a, err := c.Get(summary.ID, summary.Revision)
	require.NoError(t, err)
	require.Contains(t, a.Body, "Human Review")
	require.Contains(t, a.Body, "默认")
	require.Contains(t, a.Body, "AI引用尚未开放")
	require.Equal(t, a.Digest, Digest(a.Body))
	require.NotEmpty(t, a.Sources)
}
