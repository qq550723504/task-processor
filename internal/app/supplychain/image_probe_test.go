package supplychainapp

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/marketplace/shein/goods"
	"task-processor/internal/product/asset"
)

type imageTransport func(*http.Request) (*http.Response, error)

func (f imageTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestApprovedImageProbeReadsActualBoundedBytesAndRejectsCorruptOrOversizedFiles(t *testing.T) {
	var content bytes.Buffer
	require.NoError(t, png.Encode(&content, image.NewRGBA(image.Rect(0, 0, 900, 900))))
	raw := content.Bytes()
	reads := 0
	probe := NewPublicImageProbe()
	probe.client = &http.Client{Transport: imageTransport(func(r *http.Request) (*http.Response, error) {
		reads++
		require.Equal(t, "https://images.example.org/source.png", r.URL.String())
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"image/png"}}, Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})}
	approved := asset.ApprovedAsset{ID: "approved-a", URL: "https://images.example.org/source.png", Width: 1, Height: 1}
	observation, err := probe.Probe(context.Background(), approved, 1)
	require.NoError(t, err)
	require.Equal(t, approved.ID, observation.AssetID)
	require.Equal(t, approved.URL, observation.SourceURL)
	require.EqualValues(t, 900, observation.Width)
	require.EqualValues(t, 900, observation.Height)
	require.EqualValues(t, len(raw), observation.Bytes)
	require.True(t, goods.ValidImageObservation(observation))
	require.Empty(t, observation.RemoteURL)
	raw = raw[:40]
	_, err = probe.Probe(context.Background(), approved, 1)
	require.Error(t, err, "header-only or corrupt files are not valid image evidence")
	raw = bytes.Repeat([]byte{0}, int(goods.MaxOfficialImageBytes)+1)
	_, err = probe.Probe(context.Background(), approved, 1)
	require.Error(t, err)
	approved.URL = "https://127.0.0.1/private.png"
	before := reads
	_, err = probe.Probe(context.Background(), approved, 1)
	require.Error(t, err)
	require.Equal(t, before, reads, "private URLs are rejected before transport")
}
