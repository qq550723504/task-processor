package goods

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	model "task-processor/internal/marketplace/shein/model"
)

type OfficialStockProofObservation struct {
	SKC         int    `json:"skc"`
	Filename    string `json:"file_name"`
	Type        string `json:"type"`
	SourceURL   string `json:"source_url"`
	ContentHash string `json:"content_hash"`
	Bytes       int64  `json:"bytes"`
	MediaType   string `json:"media_type"`
}

func ValidStockProofInput(p model.StockProof) bool {
	u, e := url.Parse(p.URL)
	return officialText(p.Filename, 1, 256) && (p.Type == "1" || p.Type == "2") && e == nil && len(p.URL) <= 2048 && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == "" && (u.Port() == "" || u.Port() == "443")
}
func ValidStockProofObservation(v OfficialStockProofObservation) bool {
	return v.SKC >= 0 && v.SKC < 40 && ValidStockProofInput(model.StockProof{Filename: v.Filename, Type: v.Type, URL: v.SourceURL}) && v.Bytes > 0 && v.Bytes <= MaxOfficialImageBytes && regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(v.ContentHash) && (v.Type == "1" && (v.MediaType == "image/jpeg" || v.MediaType == "image/png") || v.Type == "2" && v.MediaType == "application/pdf")
}
func (b *officialBuild) stockProofs(p *model.PublishProduct, observed []OfficialStockProofObservation) {
	seen := make(map[int]bool)
	for k, skc := range p.SKCs {
		field := fmt.Sprintf("skc_list.%d.proof_of_stock_list", k)
		if len(skc.StockProofs) > 1 {
			b.issue(field, "invalid", "每个 SKC 最多一个库存证明文件")
			continue
		}
		for _, proof := range skc.StockProofs {
			matched := false
			for _, v := range observed {
				if v.SKC == k && v.Filename == proof.Filename && v.Type == proof.Type && v.SourceURL == proof.URL && ValidStockProofObservation(v) && !seen[k] {
					matched = true
					seen[k] = true
					break
				}
			}
			if !ValidStockProofInput(proof) || !matched {
				b.issue(field, "missing", "填写实际 JPEG/PNG 或 PDF 库存证明的公开 HTTPS 链接，文件最多 3MiB")
			}
		}
	}
	for _, v := range observed {
		if !seen[v.SKC] {
			b.issue("proof_of_stock", "invalid", "库存证明与当前商品不一致")
			break
		}
	}
}

// PDF is sent as bounded file data, never executed or rendered. MIME is based
// on bytes, not an untrusted HTTP Content-Type or filename.
func IsPDFStockProof(content []byte) bool {
	return len(content) >= 16 && (strings.HasPrefix(string(content[:8]), "%PDF-1.") || strings.HasPrefix(string(content[:8]), "%PDF-2.")) && strings.Contains(string(content[max(0, len(content)-1024):]), "%%EOF")
}
