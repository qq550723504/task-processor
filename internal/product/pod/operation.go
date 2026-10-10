package pod

import (
	"encoding/json"
	"math"
	"net/url"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/collection"
	"time"
	"unicode/utf8"
)

const ProtocolRevision = "sds-free-fabric-5.2.1-v1"
const MaxArtworkBytes = 3 << 20

type AccountBinding struct{ ID, Revision, MerchantID, ProtocolRevision string }
type EditableLayer struct {
	ID          string `json:"id"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	PrintWidth  int    `json:"printWidth"`
	PrintHeight int    `json:"printHeight"`
}
type RenderFile struct {
	ID        string `json:"id"`
	Thumbnail string `json:"thumbnail"`
}
type TemplateManifest struct {
	ParentID    string          `json:"parentId"`
	VariantID   string          `json:"variantId"`
	PrototypeID string          `json:"prototypeId"`
	GroupID     string          `json:"groupId"`
	Type        string          `json:"type"`
	Layers      []EditableLayer `json:"layers"`
	RenderFiles []RenderFile    `json:"renderFiles"`
}
type InputReference struct {
	ItemID   string            `json:"itemId"`
	Revision int64             `json:"revision"`
	Source   collection.Source `json:"source"`
}
type ArtworkReference struct {
	Input                                              InputReference
	Version                                            uint64
	ApplyReceiptID, ActionID, AssetID, Hash, MediaType string
	Bytes                                              int64
	Width, Height                                      int
}
type Transform struct {
	LayerID string  `json:"layerId"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Scale   float64 `json:"scale"`
	Angle   float64 `json:"angle"`
}
type Plan struct {
	OperationID  string
	Scope        collection.Scope
	Binding      AccountBinding
	Template     TemplateManifest
	TemplateItem InputReference
	Artwork      ArtworkReference
	Transforms   []Transform
	Name         string
}
type MaterialReceipt struct {
	ID, FileCode, URL, Hash, Name string
	Width, Height                 int
}
type ObjectReceipt struct{ FileCode, Hash string }
type Operation struct {
	Plan              Plan
	Object            *ObjectReceipt
	Material          *MaterialReceipt
	Intent            *DesignIntent
	Payload           []byte
	Finished          *FinishedReference
	CreatedAt         time.Time
	NextObservationAt time.Time
	Imported          *collection.Receipt
}

func MaterialName(operation string) string { return "sds-pod-" + operation }

// Binding and organization revisions cannot circumvent the same physical
// merchant/template fence. Every enterprise competes for this one identity.
func (p Plan) FenceKey() string {
	return collection.Digest([]string{"sds", p.Binding.MerchantID, p.Template.ParentID, p.Template.GroupID})
}
func (p Plan) Validate() error {
	if p.Binding.ProtocolRevision != ProtocolRevision || p.Template.Type != "FREE" {
		return ErrUnavailable
	}
	if !collection.ValidID(p.OperationID) || p.Scope.Validate() != nil || !authidentity.IsBoundedIdentifier(p.Binding.ID) || !authidentity.IsBoundedIdentifier(p.Binding.Revision) || !remoteID(p.Binding.MerchantID) || p.Template.Validate() != nil || !validInput(p.TemplateItem) || p.TemplateItem.Source.Kind != "sds_template" || !validInput(p.Artwork.Input) || p.Artwork.Input.Source.Kind == "sds_template" || p.Artwork.Version < p.Artwork.Input.Source.Version || !authidentity.IsBoundedIdentifier(p.Artwork.ActionID) || !authidentity.IsBoundedIdentifier(p.Artwork.AssetID) || !hexDigest(p.Artwork.Hash) || p.Artwork.Bytes < 1 || p.Artwork.Bytes > MaxArtworkBytes || p.Artwork.Width < 1 || p.Artwork.Height < 1 || p.Artwork.Width > 10000 || p.Artwork.Height > 10000 || int64(p.Artwork.Width)*int64(p.Artwork.Height) > 40000000 || p.Artwork.MediaType != "image/jpeg" && p.Artwork.MediaType != "image/png" || p.Name == "" || strings.TrimSpace(p.Name) != p.Name || !utf8.ValidString(p.Name) || len(p.Name) > 200 || strings.ContainsAny(p.Name, "\r\n\x00") || len(p.Transforms) != len(p.Template.Layers) {
		return ErrInvalid
	}
	if p.Artwork.Version != p.Artwork.Input.Source.Version && !collection.ValidID(p.Artwork.ApplyReceiptID) {
		return ErrInvalid
	}
	layers := map[string]bool{}
	for _, l := range p.Template.Layers {
		layers[l.ID] = true
	}
	for _, t := range p.Transforms {
		if !layers[t.LayerID] || !finite(t.X) || !finite(t.Y) || !finite(t.Scale) || !finite(t.Angle) || t.X < 0 || t.X > 1 || t.Y < 0 || t.Y > 1 || t.Scale < .05 || t.Scale > 4 || t.Angle < -180 || t.Angle > 180 {
			return ErrInvalid
		}
		delete(layers, t.LayerID)
	}
	raw, err := json.Marshal(p)
	if err != nil || len(raw) > 65536 {
		return ErrInvalid
	}
	return nil
}
func validInput(r InputReference) bool {
	return collection.ValidID(r.ItemID) && r.Revision > 0 && authidentity.IsBoundedIdentifier(r.Source.ProductKey) && authidentity.IsBoundedIdentifier(r.Source.PublicationID) && r.Source.Version > 0 && collection.ValidSourceKind(r.Source.Kind)
}
func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func hexDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
func (m TemplateManifest) Validate() error {
	if m.Type != "FREE" {
		return ErrUnavailable
	}
	if !remoteID(m.ParentID) || !remoteID(m.VariantID) || !remoteID(m.PrototypeID) || !remoteID(m.GroupID) || len(m.Layers) < 1 || len(m.Layers) > 16 || len(m.RenderFiles) < 1 || len(m.RenderFiles) > 32 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, l := range m.Layers {
		if !remoteID(l.ID) || seen[l.ID] || l.Width < 1 || l.Height < 1 || l.PrintWidth < 1 || l.PrintHeight < 1 || l.Width > 20000 || l.Height > 20000 || l.PrintWidth > 20000 || l.PrintHeight > 20000 {
			return ErrInvalid
		}
		seen[l.ID] = true
	}
	seen = map[string]bool{}
	for _, f := range m.RenderFiles {
		u, e := url.Parse(f.Thumbnail)
		if !remoteID(f.ID) || seen[f.ID] || e != nil || u.User != nil || u.Fragment != "" || len(f.Thumbnail) > 8192 || !(u.Host == "e.sdspod.com" && u.Path == "/builds" && (u.Scheme == "http" || u.Scheme == "https") || u.Host == "cdn.sdspod.com" && u.Scheme == "https") {
			return ErrInvalid
		}
		seen[f.ID] = true
	}
	return nil
}

// Geometry uses the qualified provider's 600px design plane. The editable
// rectangle is centered and aspect-preserving. X/Y are fractions of that plane;
// Scale is relative to image contain-fit, shared by browser preview and payload.
func BuildSync(p Plan, m MaterialReceipt) (DesignIntent, []byte, error) {
	if err := p.Validate(); err != nil {
		return DesignIntent{}, nil, err
	}
	if !remoteID(m.ID) || m.Name != MaterialName(p.OperationID) || m.Hash != p.Artwork.Hash || m.Width < 1 || m.Height < 1 || m.Width > 10000 || m.Height > 10000 || m.FileCode == "" || strings.ContainsAny(m.FileCode, "/\\?#\x00") {
		return DesignIntent{}, nil, ErrUnknown
	}
	u, e := url.Parse(m.URL)
	if e != nil || u.Scheme != "https" || u.Host != "cdn.sdspod.com" || u.User != nil || u.Fragment != "" || !strings.HasPrefix(u.Path, "/images1000Thumbs/") || !strings.HasSuffix(u.Path, "/"+m.FileCode) || u.Query().Get("material_id") != m.ID {
		return DesignIntent{}, nil, ErrUnknown
	}
	intent := DesignIntent{OperationID: p.OperationID, MerchantID: p.Binding.MerchantID, ParentID: p.Template.ParentID, VariantID: p.Template.VariantID, PrototypeID: p.Template.PrototypeID, GroupID: p.Template.GroupID, Materials: []MaterialReference{{m.ID, m.FileCode}}}
	layers := []map[string]any{}
	images := []string{}
	psds := []string{}
	for _, l := range p.Template.Layers {
		var t Transform
		for _, candidate := range p.Transforms {
			if candidate.LayerID == l.ID {
				t = candidate
			}
		}
		factor := 600 / math.Max(float64(l.Width), float64(l.Height))
		scale := math.Min(float64(l.Width)*factor/float64(m.Width), float64(l.Height)*factor/float64(m.Height)) * t.Scale
		object := map[string]any{"type": "image", "version": "5.2.1", "originX": "center", "originY": "center", "left": 600 * t.X, "top": 600 * t.Y, "width": m.Width, "height": m.Height, "fill": "rgb(0,0,0)", "stroke": nil, "strokeWidth": 0, "strokeDashArray": nil, "strokeLineCap": "butt", "strokeDashOffset": 0, "strokeLineJoin": "miter", "strokeUniform": false, "strokeMiterLimit": 4, "scaleX": scale, "scaleY": scale, "angle": t.Angle, "flipX": false, "flipY": false, "opacity": 1, "shadow": nil, "visible": true, "backgroundColor": "", "fillRule": "nonzero", "paintFirst": "fill", "globalCompositeOperation": "source-over", "skewX": 0, "skewY": 0, "cropX": 0, "cropY": 0, "sds": map[string]any{"originUrl": m.URL, "styleKey": ""}, "selectable": true, "centeredScaling": false, "src": m.URL, "crossOrigin": "anonymous", "filters": []any{}}
		fabric, _ := json.Marshal(map[string]any{"version": "5.2.1", "objects": []any{object}, "centeredScaling": false})
		intent.Layers = append(intent.Layers, DesignLayer{ID: l.ID, Width: l.Width, Height: l.Height, PrintWidth: l.PrintWidth, PrintHeight: l.PrintHeight, FabricJSON: string(fabric)})
		layers = append(layers, map[string]any{"material_id": "", "layer_id": l.ID, "content": "", "img_width": l.Width, "img_height": l.Height, "resize_mode": 0, "fit_level": 1, "fabric_json": string(fabric), "related_material_ids": []json.Number{json.Number(m.ID)}})
	}
	for _, f := range p.Template.RenderFiles {
		psds = append(psds, f.ID)
		images = append(images, f.Thumbnail)
	}
	intent.RenderFileIDs = psds
	body := map[string]any{"product_id": json.Number(p.Template.VariantID), "prototypeGroupId": json.Number(p.Template.GroupID), "merchantProductResultGroupId": 0, "designType": "material", "prototypes": []any{map[string]any{"prototype_id": p.Template.PrototypeID, "product_ids": []json.Number{json.Number(p.Template.VariantID)}, "psd_ids": psds, "layers": layers, "images": images}}}
	raw, err := json.Marshal(body)
	if err != nil || len(raw) > 2<<20 || !validIntent(intent) {
		return DesignIntent{}, nil, ErrInvalid
	}
	return intent, raw, nil
}
