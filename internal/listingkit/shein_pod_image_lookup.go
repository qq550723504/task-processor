package listingkit

import (
	"context"
	"net/url"
	"path"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

type SheinPODImageLookupRepository interface {
	LookupSheinPODImages(ctx context.Context, query *SheinPODImageLookupQuery) ([]SheinPODImageLookupRecord, int64, error)
}

type SheinPODImageLookupQuery struct {
	StoreID int64  `form:"store_id" json:"store_id,omitempty"`
	Query   string `form:"query" json:"query,omitempty"`
	Limit   int    `form:"limit" json:"limit,omitempty"`
}

type SheinPODImageLookupRecord struct {
	TaskID              string    `json:"task_id"`
	StoreID             int64     `json:"store_id,omitempty"`
	Status              string    `json:"status,omitempty"`
	Prompt              string    `json:"prompt,omitempty"`
	ProductName         string    `json:"product_name,omitempty"`
	SupplierCode        string    `json:"supplier_code,omitempty"`
	SellerSKU           string    `json:"seller_sku,omitempty"`
	SheinSPUName        string    `json:"shein_spu_name,omitempty"`
	SheinVersion        string    `json:"shein_version,omitempty"`
	AIOriginalImageURL  string    `json:"ai_original_image_url,omitempty"`
	AIOriginalImageKey  string    `json:"ai_original_image_key,omitempty"`
	SDSMainImageURL     string    `json:"sds_main_image_url,omitempty"`
	SDSGalleryImageURLs []string  `json:"sds_gallery_image_urls,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func BuildSheinPODImageLookupRecord(task *Task) (SheinPODImageLookupRecord, bool) {
	if task == nil || task.Result == nil || task.Result.Shein == nil {
		return SheinPODImageLookupRecord{}, false
	}
	pkg := task.Result.Shein
	record := SheinPODImageLookupRecord{
		TaskID:    task.ID,
		Status:    string(task.Status),
		CreatedAt: task.CreatedAt,
		UpdatedAt: task.UpdatedAt,
	}
	if task.SheinStoreResolutionSnapshot != nil {
		record.StoreID = task.SheinStoreResolutionSnapshot.StoreID
	}
	if record.StoreID == 0 && task.Request != nil {
		record.StoreID = task.Request.SheinStoreID
	}
	if task.Request != nil {
		record.Prompt = strings.TrimSpace(task.Request.Text)
		if len(task.Request.ImageURLs) > 0 {
			record.AIOriginalImageURL = strings.TrimSpace(task.Request.ImageURLs[0])
		}
	}
	record.AIOriginalImageKey = uploadedImageKeyFromPublicURL(record.AIOriginalImageURL)
	if pkg.Images != nil {
		record.SDSMainImageURL = strings.TrimSpace(pkg.Images.MainImage)
		record.SDSGalleryImageURLs = append([]string(nil), pkg.Images.Gallery...)
		if record.AIOriginalImageURL == "" && len(pkg.Images.SourceImages) > 0 {
			record.AIOriginalImageURL = strings.TrimSpace(pkg.Images.SourceImages[0])
			record.AIOriginalImageKey = uploadedImageKeyFromPublicURL(record.AIOriginalImageURL)
		}
	}
	record.ProductName, record.SupplierCode, record.SellerSKU = resolveSheinPODDraftIdentity(pkg)
	record.SheinSPUName, record.SheinVersion = resolveSheinPODSubmissionIdentity(pkg)
	return record, record.TaskID != "" && (record.AIOriginalImageURL != "" || record.SDSMainImageURL != "" || record.SellerSKU != "" || record.SheinSPUName != "")
}

func SheinPODImageLookupRecordMatches(record SheinPODImageLookupRecord, query string) bool {
	target := normalizeSheinPODImageLookupToken(query)
	if target == "" {
		return true
	}
	for _, value := range []string{
		record.TaskID,
		record.ProductName,
		record.SupplierCode,
		record.SellerSKU,
		record.SheinSPUName,
		record.SheinVersion,
		record.AIOriginalImageURL,
		record.AIOriginalImageKey,
		record.SDSMainImageURL,
	} {
		if strings.Contains(normalizeSheinPODImageLookupToken(value), target) {
			return true
		}
	}
	return false
}

func NormalizeSheinPODImageLookupQueryToken(value string) string {
	return normalizeSheinPODImageLookupToken(value)
}

func resolveSheinPODDraftIdentity(pkg *sheinpub.Package) (productName, supplierCode, sellerSKU string) {
	for _, draft := range []*sheinpub.RequestDraft{pkg.RequestDraft, pkg.DraftPayload} {
		if draft == nil {
			continue
		}
		if productName == "" {
			productName = strings.TrimSpace(draft.SpuName)
		}
		if supplierCode == "" {
			supplierCode = strings.TrimSpace(draft.SupplierCode)
		}
		for _, skc := range draft.SKCList {
			if productName == "" {
				productName = strings.TrimSpace(skc.SkcName)
			}
			if supplierCode == "" {
				supplierCode = strings.TrimSpace(skc.SupplierCode)
			}
			for _, sku := range skc.SKUList {
				if sellerSKU == "" {
					sellerSKU = strings.TrimSpace(sku.SupplierSKU)
				}
				if sellerSKU != "" && supplierCode != "" && productName != "" {
					return productName, supplierCode, sellerSKU
				}
			}
		}
	}
	for _, skc := range pkg.SkcList {
		if productName == "" {
			productName = strings.TrimSpace(skc.SkcName)
		}
		if supplierCode == "" {
			supplierCode = strings.TrimSpace(skc.SupplierCode)
		}
		for _, sku := range skc.SKUs {
			if sellerSKU == "" {
				sellerSKU = strings.TrimSpace(sku.SKU)
			}
		}
	}
	return productName, supplierCode, sellerSKU
}

func resolveSheinPODSubmissionIdentity(pkg *sheinpub.Package) (spuName, version string) {
	for _, report := range []*sheinpub.SubmissionReport{pkg.SubmissionState, pkg.Submission} {
		if report == nil {
			continue
		}
		for _, response := range []*sheinpub.SubmissionResponse{
			submissionRecordResponse(report.Publish),
			submissionRecordResponse(report.SaveDraft),
			report.LastResult,
		} {
			if response == nil {
				continue
			}
			if spuName == "" {
				spuName = strings.TrimSpace(response.SPUName)
			}
			if version == "" {
				version = strings.TrimSpace(response.Version)
			}
			if spuName != "" && version != "" {
				return spuName, version
			}
		}
	}
	for _, event := range pkg.SubmissionEvents {
		if event.Response == nil {
			continue
		}
		if spuName == "" {
			spuName = strings.TrimSpace(event.Response.SPUName)
		}
		if version == "" {
			version = strings.TrimSpace(event.Response.Version)
		}
		if spuName != "" && version != "" {
			return spuName, version
		}
	}
	return spuName, version
}

func submissionRecordResponse(record *sheinpub.SubmissionRecord) *sheinpub.SubmissionResponse {
	if record == nil {
		return nil
	}
	return record.Result
}

func normalizeSheinPODImageLookupToken(value string) string {
	replacer := strings.NewReplacer("-", "", "_", "", " ", "", "\t", "", "\n", "", "\r", "")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(value)))
}

func uploadedImageKeyFromPublicURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	cleanPath := strings.TrimPrefix(path.Clean(parsed.Path), "/")
	const prefix = "listingkit-assets/"
	if strings.HasPrefix(cleanPath, prefix) {
		return strings.TrimPrefix(cleanPath, prefix)
	}
	return cleanPath
}
