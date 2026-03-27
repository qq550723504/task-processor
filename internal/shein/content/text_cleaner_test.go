package content_test

import (
	"testing"

	"task-processor/internal/shein/content"
)

func newCleaner() *content.TextCleaner {
	return content.NewTextCleaner()
}

func TestTextCleaner_RemoveBrandFromText(t *testing.T) {
	c := newCleaner()

	tests := []struct {
		name  string
		text  string
		brand string
		want  string
	}{
		{"empty_text", "", "Nike", ""},
		{"empty_brand", "Nike Shoes", "", "Nike Shoes"},
		{"brand_at_start", "Nike Running Shoes", "Nike", "Running Shoes"},
		{"brand_at_end", "Running Shoes Nike", "Nike", "Running Shoes"},
		{"brand_case_insensitive", "NIKE shoes", "nike", "shoes"},
		{"brand_not_present", "Adidas Shoes", "Nike", "Adidas Shoes"},
		{"only_brand_returns_original", "Nike", "Nike", "Nike"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.RemoveBrandFromText(tc.text, tc.brand)
			if got != tc.want {
				t.Errorf("RemoveBrandFromText(%q, %q) = %q, want %q", tc.text, tc.brand, got, tc.want)
			}
		})
	}
}

func TestTextCleaner_RemoveSpecialCharacters(t *testing.T) {
	c := newCleaner()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no_special_chars", "Hello World", "Hello World"},
		{"with_hashtag", "Hello #World", "Hello World"},
		{"with_at_sign", "Hello @World", "Hello World"},
		{"keeps_basic_punctuation", "Hello, World! How are you?", "Hello, World! How are you?"},
		{"keeps_parens_and_dash", "Size (M-L)", "Size (M-L)"},
		{"empty_string", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.RemoveSpecialCharacters(tc.input)
			if got != tc.want {
				t.Errorf("RemoveSpecialCharacters(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestTextCleaner_CleanWhitespace(t *testing.T) {
	c := newCleaner()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single_spaces", "Hello World", "Hello World"},
		{"multiple_spaces", "Hello   World", "Hello World"},
		{"leading_trailing", "  Hello World  ", "Hello World"},
		{"tabs_and_newlines", "Hello\t\nWorld", "Hello World"},
		{"empty_string", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.CleanWhitespace(tc.input)
			if got != tc.want {
				t.Errorf("CleanWhitespace(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestTextCleaner_RemoveHTMLTags(t *testing.T) {
	c := newCleaner()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no_tags", "Hello World", "Hello World"},
		{"simple_tag", "<b>Hello</b>", "Hello"},
		{"nested_tags", "<div><p>Hello</p></div>", "Hello"},
		{"tag_with_attrs", `<a href="url">Link</a>`, "Link"},
		{"empty_string", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.RemoveHTMLTags(tc.input)
			if got != tc.want {
				t.Errorf("RemoveHTMLTags(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestTextCleaner_ValidateTextLength(t *testing.T) {
	c := newCleaner()

	tests := []struct {
		name      string
		text      string
		minLength int
		maxLength int
		wantErr   bool
	}{
		{"within_range", "Hello", 1, 10, false},
		{"at_min", "Hi", 2, 10, false},
		{"at_max", "Hello", 1, 5, false},
		{"below_min", "Hi", 5, 10, true},
		{"above_max", "Hello World", 1, 5, true},
		{"empty_with_min_zero", "", 0, 10, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := c.ValidateTextLength(tc.text, tc.minLength, tc.maxLength)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateTextLength(%q, %d, %d) error = %v, wantErr %v",
					tc.text, tc.minLength, tc.maxLength, err, tc.wantErr)
			}
		})
	}
}

func TestTextCleaner_TruncateAtWordBoundary(t *testing.T) {
	c := newCleaner()

	tests := []struct {
		name      string
		text      string
		maxLength int
		wantLen   int // 期望结果长度 <= maxLength
	}{
		{"short_text_unchanged", "Hello", 10, 5},
		{"exact_length_unchanged", "Hello", 5, 5},
		{"truncate_at_word", "Hello World Foo", 11, 5}, // "Hello World" 是11字符，但最后空格在10，截到"Hello World"=11，再找空格在5
		{"empty_string", "", 10, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.TruncateAtWordBoundary(tc.text, tc.maxLength)
			if len(got) > tc.maxLength {
				t.Errorf("TruncateAtWordBoundary(%q, %d) = %q (len=%d), exceeds maxLength",
					tc.text, tc.maxLength, got, len(got))
			}
		})
	}
}

func TestTextCleaner_TruncateAtSentenceBoundary(t *testing.T) {
	c := newCleaner()

	tests := []struct {
		name      string
		text      string
		maxLength int
	}{
		{"short_text_unchanged", "Hello.", 20},
		{"truncate_at_period", "Hello. World is great. More text here.", 25},
		{"no_sentence_boundary", "Hello World no period here", 10},
		{"empty_string", "", 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.TruncateAtSentenceBoundary(tc.text, tc.maxLength)
			if len(got) > tc.maxLength {
				t.Errorf("TruncateAtSentenceBoundary(%q, %d) = %q (len=%d), exceeds maxLength",
					tc.text, tc.maxLength, got, len(got))
			}
		})
	}
}

func TestTextCleaner_RemoveForbiddenWords(t *testing.T) {
	c := newCleaner()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"removes_best", "This is the best product", "This is the product"},
		{"removes_guarantee", "We guarantee quality", "We quality"},
		{"removes_medical", "Has medical benefits", "Has benefits"},
		{"no_forbidden_words", "Quality product for you", "Quality product for you"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.RemoveForbiddenWords(tc.input)
			if got != tc.want {
				t.Errorf("RemoveForbiddenWords(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
