// Package strx provides unit tests for string utilities
package strx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestContainsIgnoreCase tests case-insensitive contains
func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"match case", "Hello World", "hello", true},
		{"no match", "Hello", "xyz", false},
		{"empty substr", "Hello", "", true},
		{"empty main", "", "abc", false},
		{"both empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsIgnoreCase(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFindSubstring tests finding substring
func TestFindSubstring(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"found", "hello world", "world", true},
		{"not found", "hello", "xyz", false},
		{"empty substr", "hello", "", true},
		{"empty main", "", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindSubstring(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTruncateString tests string truncation
func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"truncate", "hello world", 5, "hello"},
		{"no truncate", "hi", 10, "hi"},
		{"empty", "", 5, ""},
		{"exact", "hello", 5, "hello"},
		{"unicode", "你好世界", 4, "你好世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCleanWhitespace tests whitespace cleaning
func TestCleanWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"multiple spaces", "hello   world", "hello world"},
		{"tabs", "a\tb", "a b"},
		{"newlines", "a\nb", "a b"},
		{"mixed", "  a   b  c  ", "a b c"},
		{"no change", "abc", "abc"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanWhitespace(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestContains tests string contains
func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"contains", "hello world", "world", true},
		{"not contains", "hello", "xyz", false},
		{"empty substr", "hello", "", true},
		{"empty main", "", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestToLower tests to lower
func TestToLower(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lower", "HELLO", "hello"},
		{"mixed", "HeLLo", "hello"},
		{"already lower", "hello", "hello"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToLower(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestToUpper tests to upper
func TestToUpper(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"upper", "hello", "HELLO"},
		{"mixed", "HeLLo", "HELLO"},
		{"already upper", "HELLO", "HELLO"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToUpper(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
