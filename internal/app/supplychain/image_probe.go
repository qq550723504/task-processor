package supplychainapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"net/http"
	"net/url"
	"time"

	"task-processor/internal/integration/httpimage"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/asset"
)

type ImageProbe interface {
	Probe(context.Context, asset.ApprovedAsset, int) (goods.OfficialImageObservation, error)
}
type PublicImageProbe struct{ client *http.Client }

func NewPublicImageProbe() *PublicImageProbe {
	return &PublicImageProbe{client: httpimage.NewPublicImageHTTPClient()}
}
func (p *PublicImageProbe) Probe(ctx context.Context, approved asset.ApprovedAsset, imageType int) (goods.OfficialImageObservation, error) {
	var observation goods.OfficialImageObservation
	if p == nil || p.client == nil || ctx == nil || approved.ID == "" || imageType != 1 && imageType != 2 && imageType != 5 && imageType != 6 && imageType != 7 {
		return observation, record.ErrNotReady
	}
	validated, err := httpimage.ValidatePublicHTTPSURL(approved.URL)
	parsed, parseErr := url.Parse(approved.URL)
	if err != nil || parseErr != nil || validated != approved.URL || parsed.User != nil || parsed.Fragment != "" || parsed.Port() != "" && parsed.Port() != "443" {
		return observation, record.ErrNotReady
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	content, err := httpimage.Download(ctx, p.client, approved.URL, goods.MaxOfficialImageBytes)
	if err != nil {
		return observation, record.ErrNotReady
	}
	mediaType, width, height, err := httpimage.InspectGeneratedArtifact(content)
	if err != nil || mediaType != "image/jpeg" && mediaType != "image/png" || width > 10000 || height > 10000 || int64(width)*int64(height) > 20_000_000 {
		return observation, record.ErrNotReady
	}
	if _, _, err = image.Decode(bytes.NewReader(content)); err != nil || ctx.Err() != nil {
		return observation, record.ErrNotReady
	}
	hash := sha256.Sum256(content)
	return goods.OfficialImageObservation{AssetID: approved.ID, SourceURL: approved.URL, Width: width, Height: height, Type: imageType, ContentHash: hex.EncodeToString(hash[:]), Bytes: int64(len(content)), MediaType: mediaType}, nil
}

var _ ImageProbe = (*PublicImageProbe)(nil)

func (p *PublicImageProbe) ProbeStockProof(ctx context.Context, skc int, proof model.StockProof) (goods.OfficialStockProofObservation, error) {
	var observation goods.OfficialStockProofObservation
	if p == nil || p.client == nil || ctx == nil || skc < 0 || skc >= 40 || !goods.ValidStockProofInput(proof) {
		return observation, record.ErrNotReady
	}
	validated, err := httpimage.ValidatePublicHTTPSURL(proof.URL)
	if err != nil || validated != proof.URL {
		return observation, record.ErrNotReady
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	content, err := httpimage.Download(ctx, p.client, proof.URL, goods.MaxOfficialImageBytes)
	if err != nil {
		return observation, record.ErrNotReady
	}
	mediaType := "application/pdf"
	if proof.Type == "1" {
		var w, h int
		mediaType, w, h, err = httpimage.InspectGeneratedArtifact(content)
		if err != nil || mediaType != "image/jpeg" && mediaType != "image/png" || w > 10000 || h > 10000 || int64(w)*int64(h) > 20_000_000 {
			return observation, record.ErrNotReady
		}
		if _, _, err = image.Decode(bytes.NewReader(content)); err != nil {
			return observation, record.ErrNotReady
		}
	} else if !goods.IsPDFStockProof(content) {
		return observation, record.ErrNotReady
	}
	if ctx.Err() != nil {
		return observation, record.ErrNotReady
	}
	hash := sha256.Sum256(content)
	return goods.OfficialStockProofObservation{SKC: skc, Filename: proof.Filename, Type: proof.Type, SourceURL: proof.URL, ContentHash: hex.EncodeToString(hash[:]), Bytes: int64(len(content)), MediaType: mediaType}, nil
}
