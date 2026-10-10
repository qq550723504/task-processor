package pod

import (
	"encoding/json"
	"math"
	"task-processor/internal/product/collection"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func testPlan() Plan {
	return Plan{OperationID: uuid.NewString(), Scope: collection.Scope{"org-a", "actor-a", "member-a"}, Binding: AccountBinding{"binding-a", "revision-a", "36811", ProtocolRevision}, Template: TemplateManifest{ParentID: "95146", VariantID: "95147", PrototypeID: "730897612975054849", GroupID: "15261", Type: "FREE", Layers: []EditableLayer{{ID: "730897620537384960", Width: 567, Height: 850, PrintWidth: 1334, PrintHeight: 2000}}, RenderFiles: []RenderFile{{ID: "753068348962332672", Thumbnail: "http://e.sdspod.com/builds?content=test"}}}, TemplateItem: InputReference{ItemID: uuid.NewString(), Revision: 1, Source: collection.Source{ProductKey: "template-a", PublicationID: uuid.NewString(), Version: 1, Kind: "sds_template"}}, Artwork: ArtworkReference{Input: InputReference{ItemID: uuid.NewString(), Revision: 1, Source: collection.Source{ProductKey: "artwork-a", PublicationID: uuid.NewString(), Version: 1, Kind: "own"}}, Version: 1, ActionID: uuid.NewString(), AssetID: "approved-a", Hash: collection.Digest("artwork"), Bytes: 1200, Width: 100, Height: 150, MediaType: "image/png"}, Transforms: []Transform{{LayerID: "730897620537384960", X: 0.5, Y: 0.5, Scale: 1}}, Name: "测试成品"}
}
func TestFrozenPlanRejectsMissingRegionAndUnsupportedProtocol(t *testing.T) {
	p := testPlan()
	require.NoError(t, p.Validate())
	bad := p
	bad.Transforms = nil
	require.ErrorIs(t, bad.Validate(), ErrInvalid)
	bad = p
	bad.Transforms = []Transform{{LayerID: p.Transforms[0].LayerID, X: math.NaN(), Y: .5, Scale: 1}}
	require.ErrorIs(t, bad.Validate(), ErrInvalid)
	bad = p
	bad.Binding.ProtocolRevision = "legacy"
	require.ErrorIs(t, bad.Validate(), ErrUnavailable)
	bad = p
	bad.Template.Type = "TEXT"
	require.ErrorIs(t, bad.Validate(), ErrUnavailable)
	bad = p
	bad.Artwork.Version = 0
	require.ErrorIs(t, bad.Validate(), ErrInvalid)
	require.Equal(t, p.FenceKey(), func() string {
		p.Binding.ID = "other"
		p.Binding.Revision = "rotated"
		p.Scope.OrganizationID = "org-b"
		return p.FenceKey()
	}())
}
func TestSyncPayloadBindsActualMaterialExactManifestAndTransform(t *testing.T) {
	p := testPlan()
	m := MaterialReceipt{ID: "480953643", FileCode: "pattern.png", URL: "https://cdn.sdspod.com/images1000Thumbs/account/pattern.png?material_id=480953643", Width: 1200, Height: 1799, Hash: p.Artwork.Hash, Name: MaterialName(p.OperationID)}
	i, raw, err := BuildSync(p, m)
	require.NoError(t, err)
	require.True(t, validIntent(i))
	require.Equal(t, p.Template.Layers[0].ID, i.Layers[0].ID)
	var body struct {
		ProductID  json.Number `json:"product_id"`
		Prototypes []struct {
			Layers []struct {
				Related []json.Number `json:"related_material_ids"`
				Fabric  string        `json:"fabric_json"`
			} `json:"layers"`
		} `json:"prototypes"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, p.Template.VariantID, string(body.ProductID))
	require.Equal(t, m.ID, string(body.Prototypes[0].Layers[0].Related[0]))
	var fabric struct {
		Objects []struct{ Left, Top, ScaleX, ScaleY float64 }
	}
	require.NoError(t, json.Unmarshal([]byte(body.Prototypes[0].Layers[0].Fabric), &fabric))
	require.Equal(t, 300.0, fabric.Objects[0].Left)
	require.InDelta(t, 600.0/1799, fabric.Objects[0].ScaleX, 1e-12)
	m.Name = "foreign-operation"
	_, _, err = BuildSync(p, m)
	require.ErrorIs(t, err, ErrUnknown)
}
