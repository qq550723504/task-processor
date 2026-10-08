package submission

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
)

type OfficialIntent struct {
	Key                string                             `json:"key"`
	Owner              collection.Scope                   `json:"owner"`
	Kind               string                             `json:"kind"`
	RecordID           string                             `json:"record_id"`
	RecordHash         string                             `json:"record_hash"`
	Source             preparation.SourceItem             `json:"source"`
	Target             ExecutionTarget                    `json:"target"`
	Binding            storecenter.ProductMerchantBinding `json:"binding"`
	PayloadFingerprint string                             `json:"payload_fingerprint"`
	InputHash          string                             `json:"input_hash"`
	Image              *goods.OfficialImageObservation    `json:"image,omitempty"`
	CreatedAt          time.Time                          `json:"created_at"`
}
type OfficialIntentCommit struct {
	source preparation.AuthorizedSource
	intent OfficialIntent
	hash   string
}

func (p OfficialIntentCommit) Read(ctx context.Context) (OfficialIntent, error) {
	scope, source, _, err := p.source.Read(ctx)
	if err != nil || scope != p.intent.Owner || source != p.intent.Source || ValidateOfficialIntent(p.intent) != nil || collection.Digest(p.intent) != p.hash {
		return OfficialIntent{}, ErrExecutionEvidenceRequired
	}
	return cloneOfficialIntent(p.intent)
}
func cloneOfficialIntent(value OfficialIntent) (OfficialIntent, error) {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > MaxExecutionPayloadBytes {
		return OfficialIntent{}, ErrExecutionEvidenceRequired
	}
	var copy OfficialIntent
	if json.Unmarshal(raw, &copy) != nil {
		return OfficialIntent{}, ErrExecutionEvidenceRequired
	}
	return copy, nil
}

// PrepareOfficialIntent binds execution to the exact private source and saved
// record. It retains references and fingerprints, never a second product body.
func PrepareOfficialIntent(ctx context.Context, sourceProof preparation.AuthorizedSource, saved record.TargetRecord, key string, payload []byte, image *goods.OfficialImageObservation) (OfficialIntentCommit, error) {
	scope, source, _, err := sourceProof.Read(ctx)
	if err != nil || saved.Source != source || saved.Merchant.OrganizationID != scope.OrganizationID || !saved.Result.ReadyForUpload || !collection.ValidID(saved.ID) || !authidentity.IsBoundedIdentifier(key) || len(payload) < 1 || len(payload) > MaxExecutionPayloadBytes {
		return OfficialIntentCommit{}, ErrExecutionEvidenceRequired
	}
	body := OfficialIntent{Key: key, Owner: scope, Kind: "publish", RecordID: saved.ID, RecordHash: collection.Digest(saved), Source: source, Binding: saved.Merchant, Target: ExecutionTarget{Platform: "shein", StoreID: saved.Merchant.StoreID}, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	action := OfficialPublishAction
	if image == nil {
		body.Target.SubjectID, err = ProductSubjectID(source.Source.ProductKey, saved.Merchant.Site)
		if !MatchesPublicationRecord(saved, payload) {
			return OfficialIntentCommit{}, ErrExecutionEvidenceRequired
		}
	} else {
		if !goods.ValidImageObservation(*image) || image.RemoteURL != "" || image.ResponseHash != "" || !matchesSelectedImage(saved, *image) {
			return OfficialIntentCommit{}, ErrExecutionEvidenceRequired
		}
		var input model.TransformImage
		if json.Unmarshal(payload, &input) != nil || input.OriginalURL != image.SourceURL || input.Type != image.Type {
			return OfficialIntentCommit{}, ErrExecutionEvidenceRequired
		}
		canonical, _ := json.Marshal(input)
		if !bytes.Equal(canonical, payload) {
			return OfficialIntentCommit{}, ErrExecutionEvidenceRequired
		}
		body.Kind, body.Image, action = "image", image, OfficialImageAction
		body.Target.SubjectID, err = ImageSubjectID(image.ContentHash, saved.Merchant.Site, image.Type)
	}
	if err != nil {
		return OfficialIntentCommit{}, err
	}
	body.PayloadFingerprint = digestExecutionParts(body.Target.Platform, body.Target.StoreID, body.Target.SubjectID, action, string(payload))
	body.InputHash = OfficialIntentInputHash(body)
	body, err = cloneOfficialIntent(body)
	if err != nil || ValidateOfficialIntent(body) != nil {
		return OfficialIntentCommit{}, ErrExecutionEvidenceRequired
	}
	return OfficialIntentCommit{source: sourceProof, intent: body, hash: collection.Digest(body)}, nil
}
func OfficialIntentInputHash(value OfficialIntent) string {
	// Canonical image effects can be reused by another authorized source with
	// identical bytes/input and the same merchant. First-origin audit is retained.
	if value.Kind == "image" {
		return collection.Digest([]string{value.Kind, value.PayloadFingerprint, value.Binding.SupplierIdentityHash, value.Binding.ApplicationRevision})
	}
	return collection.Digest([]any{value.Kind, value.Owner, value.RecordID, value.RecordHash, value.Source, value.Target, value.Binding, value.PayloadFingerprint})
}
func ValidateOfficialIntent(value OfficialIntent) error {
	if !authidentity.IsBoundedIdentifier(value.Key) || value.Owner.Validate() != nil || !collection.ValidID(value.RecordID) || !isExecutionDigest(value.RecordHash) || !collection.ValidID(value.Source.ID) ||
		value.Target.Platform != "shein" || !collection.ValidID(value.Target.StoreID) || value.Binding.OrganizationID != value.Owner.OrganizationID || value.Binding.StoreID != value.Target.StoreID || value.Binding.Site != "shein-us" ||
		value.Binding.StoreVersion < 1 || value.Binding.ConnectionRevision < 1 || !authidentity.IsBoundedIdentifier(value.Binding.ApplicationRevision) || !isExecutionDigest(value.Binding.SupplierIdentityHash) || value.Binding.ServiceExpiresAt.IsZero() ||
		!isExecutionDigest(value.PayloadFingerprint) || value.CreatedAt.IsZero() || OfficialIntentInputHash(value) != value.InputHash {
		return ErrExecutionEvidenceRequired
	}
	var subject string
	var err error
	if value.Kind == "publish" && value.Image == nil {
		subject, err = ProductSubjectID(value.Source.Source.ProductKey, value.Binding.Site)
	} else if value.Kind == "image" && value.Image != nil && goods.ValidImageObservation(*value.Image) && value.Image.RemoteURL == "" && value.Image.ResponseHash == "" {
		subject, err = ImageSubjectID(value.Image.ContentHash, value.Binding.Site, value.Image.Type)
	} else {
		return ErrExecutionEvidenceRequired
	}
	if err != nil || value.Target.SubjectID != subject {
		return ErrExecutionEvidenceRequired
	}
	return nil
}
func MatchesPublicationRecord(saved record.TargetRecord, raw []byte) bool {
	if !saved.Result.ReadyForUpload || len(raw) > MaxExecutionPayloadBytes {
		return false
	}
	var sent model.PublishProduct
	if json.Unmarshal(raw, &sent) != nil {
		return false
	}
	canonical, err := json.Marshal(sent)
	if err != nil || !bytes.Equal(canonical, raw) {
		return false
	}
	localBytes, err := json.Marshal(saved.Result.Product)
	if err != nil {
		return false
	}
	var local model.PublishProduct
	if json.Unmarshal(localBytes, &local) != nil {
		return false
	}
	clearURLs := func(p *model.PublishProduct, requireOfficial bool) bool {
		valid := true
		clear := func(info *model.ImageInfo) {
			if info == nil {
				return
			}
			for i := range info.Images {
				if requireOfficial && !goods.IsOfficialImageReference(info.Images[i].URL) {
					valid = false
				}
				info.Images[i].URL = ""
			}
		}
		clear(p.ImageInfo)
		for i := range p.SKCs {
			clear(&p.SKCs[i].ImageInfo)
			for j := range p.SKCs[i].SiteDetailImages {
				for k := range p.SKCs[i].SiteDetailImages[j].Images {
					if requireOfficial && !goods.IsOfficialImageReference(p.SKCs[i].SiteDetailImages[j].Images[k].URL) {
						valid = false
					}
					p.SKCs[i].SiteDetailImages[j].Images[k].URL = ""
				}
			}
			for j := range p.SKCs[i].SKUs {
				clear(p.SKCs[i].SKUs[j].ImageInfo)
			}
		}
		return valid
	}
	if !clearURLs(&sent, true) || !clearURLs(&local, false) {
		return false
	}
	return collection.Digest(sent) == collection.Digest(local)
}
func matchesSelectedImage(saved record.TargetRecord, observation goods.OfficialImageObservation) bool {
	for _, slot := range saved.Input.Draft.Images {
		if slot.AssetID != observation.AssetID || slot.Type != observation.Type {
			continue
		}
		var images []model.ProductImage
		switch slot.Group {
		case "spu":
			if saved.Result.Product.ImageInfo != nil {
				images = saved.Result.Product.ImageInfo.Images
			}
		case "skc":
			if slot.SKC >= 0 && slot.SKC < len(saved.Result.Product.SKCs) {
				images = saved.Result.Product.SKCs[slot.SKC].ImageInfo.Images
			}
		case "sku":
			if slot.SKC >= 0 && slot.SKC < len(saved.Result.Product.SKCs) && slot.SKU >= 0 && slot.SKU < len(saved.Result.Product.SKCs[slot.SKC].SKUs) && saved.Result.Product.SKCs[slot.SKC].SKUs[slot.SKU].ImageInfo != nil {
				images = saved.Result.Product.SKCs[slot.SKC].SKUs[slot.SKU].ImageInfo.Images
			}
		case "detail":
			if slot.SKC >= 0 && slot.SKC < len(saved.Result.Product.SKCs) {
				for _, detail := range saved.Result.Product.SKCs[slot.SKC].SiteDetailImages {
					for _, image := range detail.Images {
						images = append(images, model.ProductImage{Type: 7, Sort: image.Sort, URL: image.URL})
					}
				}
			}
		}
		for _, image := range images {
			if image.Type == slot.Type && image.Sort == slot.Sort && image.URL == observation.SourceURL {
				return true
			}
		}
	}
	return false
}

type OfficialIntentRepository interface {
	PrepareOfficial(context.Context, OfficialIntentCommit) (OfficialIntent, error)
	ReadOfficialIntent(context.Context, collection.Scope, string) (OfficialIntent, error)
}
