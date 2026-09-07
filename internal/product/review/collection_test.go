package review

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPageRequestValidation(t *testing.T) {
	validID := "12345678-1234-4234-8234-123456789abc"
	for _, request := range []PageRequest{{Limit: 1}, {Limit: MaxPageSize}, {Limit: 20, Cursor: &PageCursor{ID: validID}}} {
		require.NoError(t, request.Validate())
	}
	for _, request := range []PageRequest{{}, {Limit: -1}, {Limit: MaxPageSize + 1}, {Limit: 20, Cursor: &PageCursor{}}, {Limit: 20, Cursor: &PageCursor{ID: "invalid"}}} {
		require.ErrorIs(t, request.Validate(), ErrInvalid)
	}
}
