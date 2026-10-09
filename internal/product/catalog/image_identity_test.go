package catalog

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestImageIdentityUsesOriginalPositionAndPreservesExactSource(t *testing.T) {
	images := []Image{{URL: "https://images.example/one.png"}, {URL: "https://images.example/two.png", Role: "style"}}
	identified := IdentifyImages(images)
	require.Equal(t, "catalog-image-1", identified[0].ID)
	require.Equal(t, "catalog-image-2", identified[1].ID)
	require.Equal(t, images[1], identified[1].Image)
	identified[1].URL = "https://attacker.example/replaced.png"
	require.Equal(t, "https://images.example/two.png", images[1].URL)
}
