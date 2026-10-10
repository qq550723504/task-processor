package supplychainapp

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/asset"
)

type imageTransport func(*http.Request) (*http.Response, error)

func TestOfficialImageProbeStillRejectsGenericWebP(t *testing.T) {
	content, err := os.ReadFile("../imageagent/testdata/source.webp")
	require.NoError(t, err)
	probe := NewPublicImageProbe()
	probe.client = &http.Client{Transport: imageTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"image/png"}}, Body: io.NopCloser(bytes.NewReader(content))}, nil
	})}
	_, err = probe.Probe(context.Background(), asset.ApprovedAsset{ID: "source", URL: "https://images.example.org/source.webp"}, 1)
	require.ErrorIs(t, err, record.ErrNotReady)
}

func TestStockProofProbeUsesActualBytesAndRejectsMIMEForgeryAndPrivateURLs(t *testing.T) {
	content := []byte("%PDF-1.7\n1 0 obj <<>> endobj\n%%EOF\n")
	reads := 0
	p := NewPublicImageProbe()
	p.client = &http.Client{Transport: imageTransport(func(r *http.Request) (*http.Response, error) {
		reads++
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"image/png"}}, Body: io.NopCloser(bytes.NewReader(content))}, nil
	})}
	proof := model.StockProof{Filename: "inventory.pdf", Type: "2", URL: "https://files.example.org/stock.pdf"}
	v, e := p.ProbeStockProof(context.Background(), 0, proof)
	require.NoError(t, e)
	require.Equal(t, "application/pdf", v.MediaType)
	require.EqualValues(t, len(content), v.Bytes)
	require.Len(t, v.ContentHash, 64)
	proof.Type = "1"
	_, e = p.ProbeStockProof(context.Background(), 0, proof)
	require.Error(t, e)
	proof.Type = "2"
	content = []byte("not an actual PDF file")
	_, e = p.ProbeStockProof(context.Background(), 0, proof)
	require.Error(t, e)
	proof.URL = "https://127.0.0.1/stock.pdf"
	_, e = p.ProbeStockProof(context.Background(), 0, proof)
	require.Error(t, e)
	require.Equal(t, 3, reads)
	proof.URL = "https://files.example.org/stock.pdf"
	content = bytes.Repeat([]byte("x"), int(goods.MaxOfficialImageBytes)+1)
	_, e = p.ProbeStockProof(context.Background(), 0, proof)
	require.Error(t, e)
}

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
