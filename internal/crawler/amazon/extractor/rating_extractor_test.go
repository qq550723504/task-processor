package extractor

import "testing"

func TestRatingExtractorParseReviewsCountJapanese(t *testing.T) {
	extractor := &RatingExtractor{}

	cases := []struct {
		text string
		want int
	}{
		{text: "123件の評価", want: 123},
		{text: "1,234件のグローバル評価", want: 1234},
		{text: "56件のレビュー", want: 56},
	}

	for _, tc := range cases {
		if got := extractor.parseReviewsCount(tc.text); got != tc.want {
			t.Fatalf("parseReviewsCount(%q)=%d want %d", tc.text, got, tc.want)
		}
	}
}

func TestRatingExtractorLooksLikeReviewSectionLabel(t *testing.T) {
	extractor := &RatingExtractor{}

	cases := []struct {
		text string
		want bool
	}{
		{text: "评论", want: true},
		{text: "カスタマーレビュー", want: true},
		{text: "123件の評価", want: false},
		{text: "1,234 global ratings", want: false},
		{text: "", want: false},
	}

	for _, tc := range cases {
		if got := extractor.looksLikeReviewSectionLabel(tc.text); got != tc.want {
			t.Fatalf("looksLikeReviewSectionLabel(%q)=%v want %v", tc.text, got, tc.want)
		}
	}
}
