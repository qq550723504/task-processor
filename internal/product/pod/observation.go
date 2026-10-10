// Package pod owns retained design associations, never Product or Asset approvals.
package pod

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrInvalid     = errors.New("invalid POD request")
	ErrForbidden   = errors.New("POD permission denied")
	ErrConflict    = errors.New("POD intent conflict")
	ErrUnknown     = errors.New("POD result remains unknown")
	ErrUnavailable = errors.New("POD dependency unavailable")
	ErrNotFound    = errors.New("POD operation not found")
)

// DesignIntent is the frozen protocol portion of an authorized operation. It
// contains references and transforms, without credentials or Asset approvals.
type DesignIntent struct {
	OperationID, MerchantID, ParentID, VariantID, PrototypeID, GroupID string
	Materials                                                          []MaterialReference
	Layers                                                             []DesignLayer
	RenderFileIDs                                                      []string
}
type MaterialReference struct{ ID, FileCode string }
type DesignLayer struct {
	ID                                     string
	Width, Height, PrintWidth, PrintHeight int
	FabricJSON                             string
}
type FinishedRecord struct {
	ID, KeyID, MerchantID, TaskID, DesignTaskID, ParentID, VariantID, PrototypeID string
	BuildFinish                                                                   bool
	Status                                                                        int
	RenderURLs                                                                    []string
}
type SavedDesign struct {
	FinishedID, ParentID, VariantID, PrototypeID, GroupID string
	Layers                                                []DesignLayer
	RenderFileIDs                                         []string
}
type TaskRecord struct {
	ID                                      string
	Status, Complete, Success, Failed, Wait int
	RenderURLs                              []string
}
type Observation struct {
	Finished FinishedRecord
	Design   SavedDesign
	Task     TaskRecord
}

type FinishedReference struct {
	OperationID, MerchantID, ID, KeyID, TaskID, EvidenceDigest string
	RenderURLs                                                 []string
}

// QualifiedFinished can only be created by complete readback. It is evidence,
// not authorization: its consumer must independently recheck the original owner.
type QualifiedFinished struct {
	reference    FinishedReference
	intentDigest string
}

func (q QualifiedFinished) MatchesIntent(intent DesignIntent) bool {
	raw, err := json.Marshal(intent)
	sum := sha256.Sum256(raw)
	return err == nil && q.intentDigest != "" && q.intentDigest == hex.EncodeToString(sum[:])
}

func (q QualifiedFinished) Reference() FinishedReference {
	r := q.reference
	r.RenderURLs = append([]string(nil), r.RenderURLs...)
	return r
}

// QualifyFinished never picks a latest match or treats a mutable name as proof.
// Candidate lookup is followed by the saved-design GET and exact task readback.
func QualifyFinished(intent DesignIntent, candidates []Observation) (QualifiedFinished, error) {
	if !validIntent(intent) || len(candidates) != 1 {
		return QualifiedFinished{}, ErrUnknown
	}
	o := candidates[0]
	f, d, task := o.Finished, o.Design, o.Task
	if !remoteID(f.ID) || f.KeyID == "" || len(f.KeyID) > 128 || f.MerchantID != intent.MerchantID || !remoteID(f.TaskID) || f.TaskID != f.DesignTaskID || f.ParentID != intent.ParentID || f.VariantID != intent.VariantID || f.PrototypeID != intent.PrototypeID || !f.BuildFinish || f.Status != 2 || d.FinishedID != f.ID || d.ParentID != intent.ParentID || d.VariantID != intent.VariantID || d.PrototypeID != intent.PrototypeID || d.GroupID != intent.GroupID || !reflect.DeepEqual(d.Layers, intent.Layers) || !reflect.DeepEqual(d.RenderFileIDs, intent.RenderFileIDs) {
		return QualifiedFinished{}, ErrUnknown
	}
	if task.ID != f.TaskID || task.Status != 5 || task.Complete != 1 || task.Success != 1 || task.Failed != 0 || task.Wait != 0 || len(f.RenderURLs) != len(intent.RenderFileIDs) || len(task.RenderURLs) != len(f.RenderURLs) {
		return QualifiedFinished{}, ErrUnknown
	}
	seen := map[string]bool{}
	for i, raw := range f.RenderURLs {
		canonical, ok := renderURL(raw, false)
		if !ok || seen[canonical] {
			return QualifiedFinished{}, ErrUnknown
		}
		seen[canonical] = true
		result, ok := renderURL(task.RenderURLs[i], true)
		if !ok || result != canonical {
			return QualifiedFinished{}, ErrUnknown
		}
	}
	raw, err := json.Marshal(struct {
		Intent      DesignIntent
		Observation Observation
	}{intent, o})
	if err != nil {
		return QualifiedFinished{}, ErrUnknown
	}
	sum := sha256.Sum256(raw)
	intentRaw, _ := json.Marshal(intent)
	intentSum := sha256.Sum256(intentRaw)
	return QualifiedFinished{reference: FinishedReference{OperationID: intent.OperationID, MerchantID: intent.MerchantID, ID: f.ID, KeyID: f.KeyID, TaskID: f.TaskID, EvidenceDigest: hex.EncodeToString(sum[:]), RenderURLs: append([]string(nil), f.RenderURLs...)}, intentDigest: hex.EncodeToString(intentSum[:])}, nil
}

func validIntent(i DesignIntent) bool {
	id, err := uuid.Parse(i.OperationID)
	if err != nil || id == uuid.Nil || id.String() != i.OperationID || !remoteID(i.MerchantID) || !remoteID(i.ParentID) || !remoteID(i.VariantID) || !remoteID(i.PrototypeID) || !remoteID(i.GroupID) || len(i.Layers) < 1 || len(i.Layers) > 16 || len(i.Materials) < 1 || len(i.Materials) > 16 || len(i.RenderFileIDs) < 1 || len(i.RenderFileIDs) > 32 {
		return false
	}
	materials := map[string]string{}
	for _, m := range i.Materials {
		if !remoteID(m.ID) || m.FileCode == "" || len(m.FileCode) > 128 || strings.ContainsAny(m.FileCode, "/\\?#\x00") || materials[m.ID] != "" {
			return false
		}
		materials[m.ID] = m.FileCode
	}
	seenLayers, seenMaterials, seenFiles := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, layer := range i.Layers {
		if !remoteID(layer.ID) || seenLayers[layer.ID] || layer.Width < 1 || layer.Height < 1 || layer.PrintWidth < 1 || layer.PrintHeight < 1 || layer.Width > 20000 || layer.Height > 20000 || layer.PrintWidth > 20000 || layer.PrintHeight > 20000 || len(layer.FabricJSON) > 2<<20 {
			return false
		}
		seenLayers[layer.ID] = true
		var fabric struct {
			Version string `json:"version"`
			Objects []struct {
				Type string `json:"type"`
				Src  string `json:"src"`
				SDS  struct {
					OriginURL string `json:"originUrl"`
				} `json:"sds"`
			} `json:"objects"`
		}
		if json.Unmarshal([]byte(layer.FabricJSON), &fabric) != nil || fabric.Version != "5.2.1" || len(fabric.Objects) != 1 {
			return false
		}
		object := fabric.Objects[0]
		if object.Type != "image" || object.Src != object.SDS.OriginURL {
			return false
		}
		u, err := url.Parse(object.Src)
		if err != nil || u.Scheme != "https" || u.Host != "cdn.sdspod.com" || u.User != nil || u.Fragment != "" || !strings.HasPrefix(u.Path, "/images1000Thumbs/") {
			return false
		}
		query, err := url.ParseQuery(u.RawQuery)
		material := query.Get("material_id")
		if err != nil || len(query["material_id"]) != 1 || !remoteID(material) || materials[material] == "" || !strings.HasSuffix(u.Path, "/"+materials[material]) {
			return false
		}
		seenMaterials[material] = true
	}
	if len(seenMaterials) != len(materials) {
		return false
	}
	for _, id := range i.RenderFileIDs {
		if !remoteID(id) || seenFiles[id] {
			return false
		}
		seenFiles[id] = true
	}
	return true
}

func remoteID(s string) bool {
	if len(s) < 1 || len(s) > 64 || s[0] == '0' {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func renderURL(raw string, taskMetadata bool) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Host != "cdn.sdspod.com" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || !strings.HasPrefix(u.Path, "/out/") || strings.Contains(u.Path, "..") || u.Scheme != "https" && (!taskMetadata || u.Scheme != "http") {
		return "", false
	}
	// Task children currently report HTTP metadata for the same CDN path. The
	// transport only requests HTTPS; this does not permit an HTTP fetch.
	u.Scheme = "https"
	return u.String(), true
}
