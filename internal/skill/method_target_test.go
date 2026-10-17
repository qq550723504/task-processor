package skill

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMethodMarkerProjectsOnlyExplicitTargetWithoutReservingExistingMetadata(t *testing.T) {
	for _, tc := range []struct{ name, metadata, target, display string }{
		{"original metadata", "  target: arbitrary-current-title-metadata\n", "", ""},
		{"unknown marker", "  listingkit-method-v1: other-future-value\n", "", ""},
		{"image", "  listingkit-method-v1: product-image\n  listingkit-display-name: 标准商品套图\n", "product-image", "标准商品套图"},
		{"title", "  listingkit-method-v1: product-title\n", "product-title", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := []File{{Path: "SKILL.md", Text: "---\nname: valid-current-method\ndescription: A bounded current method.\nmetadata:\n" + tc.metadata + "---\nUse only saved facts.\n"}}
			p, err := ValidatePackage(files)
			require.NoError(t, err)
			require.Equal(t, files, p.Files)
			raw, err := json.Marshal(p)
			require.NoError(t, err)
			var projected map[string]any
			require.NoError(t, json.Unmarshal(raw, &projected))
			if tc.target == "" {
				require.NotContains(t, projected, "target")
			} else {
				require.Equal(t, tc.target, projected["target"])
			}
			if tc.display == "" {
				require.NotContains(t, projected, "displayName")
			} else {
				require.Equal(t, tc.display, projected["displayName"])
			}
		})
	}
}
