package pod

import (
	"errors"
	"strings"
	"testing"
)

func qualifiedFixture() (DesignIntent, Observation) {
	fabric := `{"version":"5.2.1","objects":[{"type":"image","left":300,"top":300,"scaleX":0.33351862145636,"scaleY":0.33351862145636,"angle":0,"sds":{"originUrl":"https://cdn.sdspod.com/images1000Thumbs/test/art.png?material_id=41&thumbsZoom=1"},"src":"https://cdn.sdspod.com/images1000Thumbs/test/art.png?material_id=41&thumbsZoom=1"}]}`
	i := DesignIntent{OperationID: "12752596-6056-4316-9f2f-380c97df9675", MerchantID: "123", ParentID: "10", VariantID: "11", PrototypeID: "20", GroupID: "30", Materials: []MaterialReference{{ID: "41", FileCode: "art.png"}}, Layers: []DesignLayer{{ID: "50", Width: 567, Height: 850, PrintWidth: 1334, PrintHeight: 2000, FabricJSON: fabric}}, RenderFileIDs: []string{"60", "61"}}
	o := Observation{Finished: FinishedRecord{ID: "70", KeyID: "finished-test", MerchantID: "123", TaskID: "80", DesignTaskID: "80", ParentID: "10", VariantID: "11", PrototypeID: "20", BuildFinish: true, Status: 2, RenderURLs: []string{"https://cdn.sdspod.com/out/123/test/a.jpg", "https://cdn.sdspod.com/out/123/test/b.jpg"}}, Design: SavedDesign{FinishedID: "70", ParentID: "10", VariantID: "11", PrototypeID: "20", GroupID: "30", Layers: append([]DesignLayer(nil), i.Layers...), RenderFileIDs: append([]string(nil), i.RenderFileIDs...)}, Task: TaskRecord{ID: "80", Status: 5, Complete: 1, Success: 1, RenderURLs: []string{"http://cdn.sdspod.com/out/123/test/a.jpg", "http://cdn.sdspod.com/out/123/test/b.jpg"}}}
	return i, o
}

func TestQualifyRequiresActualSavedDesignAssociation(t *testing.T) {
	i, o := qualifiedFixture()
	proof, err := QualifyFinished(i, []Observation{o})
	if err != nil {
		t.Fatal(err)
	}
	ref := proof.Reference()
	if ref.ID != "70" || ref.TaskID != "80" || ref.OperationID != i.OperationID || len(ref.RenderURLs) != 2 || len(ref.EvidenceDigest) != 64 {
		t.Fatalf("unexpected ref: %#v", ref)
	}
	ref.RenderURLs[0] = "changed"
	if proof.Reference().RenderURLs[0] == "changed" {
		t.Fatal("caller mutated qualified proof")
	}
	for _, tc := range []struct {
		name   string
		change func(*DesignIntent, *Observation)
	}{
		{"another account", func(_ *DesignIntent, o *Observation) { o.Finished.MerchantID = "456" }},
		{"another variant", func(_ *DesignIntent, o *Observation) { o.Design.VariantID = "12" }},
		{"another finished design", func(_ *DesignIntent, o *Observation) { o.Design.FinishedID = "71" }},
		{"another group", func(_ *DesignIntent, o *Observation) { o.Design.GroupID = "31" }},
		{"another layer", func(_ *DesignIntent, o *Observation) { o.Design.Layers[0].ID = "51" }},
		{"another material", func(_ *DesignIntent, o *Observation) {
			o.Design.Layers[0].FabricJSON = strings.ReplaceAll(o.Design.Layers[0].FabricJSON, "material_id=41", "material_id=42")
		}},
		{"another transform", func(_ *DesignIntent, o *Observation) {
			o.Design.Layers[0].FabricJSON = strings.Replace(o.Design.Layers[0].FabricJSON, `"left":300`, `"left":301`, 1)
		}},
		{"missing saved fabric", func(_ *DesignIntent, o *Observation) { o.Design.Layers[0].FabricJSON = "" }},
		{"wrong render config", func(_ *DesignIntent, o *Observation) { o.Design.RenderFileIDs[1] = "62" }},
		{"partial rendering", func(_ *DesignIntent, o *Observation) { o.Finished.RenderURLs = o.Finished.RenderURLs[:1] }},
		{"task still running", func(_ *DesignIntent, o *Observation) { o.Task.Status = 3 }},
		{"task has pending work", func(_ *DesignIntent, o *Observation) { o.Task.Wait = 1 }},
		{"task differs", func(_ *DesignIntent, o *Observation) { o.Task.ID = "81" }},
		{"task result differs", func(_ *DesignIntent, o *Observation) {
			o.Task.RenderURLs[1] = "http://cdn.sdspod.com/out/123/test/c.jpg"
		}},
		{"template image", func(_ *DesignIntent, o *Observation) {
			o.Finished.RenderURLs[0] = "https://cdn.sdspod.com/images/template.jpg"
		}},
		{"untrusted image origin", func(_ *DesignIntent, o *Observation) { o.Finished.RenderURLs[0] = "https://example.com/out/a.jpg" }},
		{"not rendered", func(_ *DesignIntent, o *Observation) { o.Finished.BuildFinish = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i, o := qualifiedFixture()
			tc.change(&i, &o)
			if _, err := QualifyFinished(i, []Observation{o}); !errors.Is(err, ErrUnknown) {
				t.Fatalf("must retain UNKNOWN, got %v", err)
			}
		})
	}
}

func TestQualifyDoesNotAdoptLatestOrOneOfMultipleCandidates(t *testing.T) {
	i, o := qualifiedFixture()
	for _, candidates := range [][]Observation{nil, {o, o}} {
		if _, err := QualifyFinished(i, candidates); !errors.Is(err, ErrUnknown) {
			t.Fatalf("ambiguous result adopted: %v", err)
		}
	}
	i.Materials[0].ID = "99"
	if _, err := QualifyFinished(i, []Observation{o}); !errors.Is(err, ErrUnknown) {
		t.Fatalf("frozen fabric not bound to material receipts: %v", err)
	}
}

func TestQualifiedProofCannotBeInvented(t *testing.T) {
	if (QualifiedFinished{}).Reference().ID != "" {
		t.Fatal("zero proof has identity")
	}
}
