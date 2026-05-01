package design

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/sds/client"
	sdstemplate "task-processor/internal/sds/template"
)

// Service 封装 SDS 设计相关请求。
type Service struct {
	client *client.Client
}

// NewService 创建设计服务。
func NewService(c *client.Client) *Service {
	return &Service{client: c}
}

// Upload 上传设计图文件。
func (s *Service) Upload(ctx context.Context, req UploadRequest, result any) error {
	if req.FileName == "" {
		return fmt.Errorf("fileName is required")
	}
	if len(req.Content) == 0 {
		return fmt.Errorf("content is empty")
	}
	if s.client.Config().Endpoints.DesignUploadPath == "" {
		return fmt.Errorf("design upload endpoint is not configured")
	}

	_, err := s.client.UploadFile(ctx, s.client.Config().Endpoints.DesignUploadPath, req.FormFields, client.MultipartFile{
		FieldName: "file",
		FileName:  req.FileName,
		Content:   req.Content,
	}, result)

	return err
}

// GetUploadSignature 获取 SDS 图片直传签名。
func (s *Service) GetUploadSignature(ctx context.Context) (*UploadSignature, error) {
	path := s.client.Config().Endpoints.UploadSignPath
	if path == "" {
		return nil, fmt.Errorf("upload sign endpoint is not configured")
	}

	result := new(UploadSignature)
	_, err := s.client.Do(ctx, "GET", path, nil, nil, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// UploadToOSS 按 SDS 前端现有逻辑上传图片到 OSS。
func (s *Service) UploadToOSS(ctx context.Context, req UploadRequest) (*UploadedImage, error) {
	if strings.TrimSpace(req.FileName) == "" {
		return nil, fmt.Errorf("fileName is required")
	}
	if len(req.Content) == 0 {
		return nil, fmt.Errorf("content is empty")
	}

	signature, err := s.GetUploadSignature(ctx)
	if err != nil {
		return nil, err
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(req.FileName)), ".")
	if ext == "" {
		return nil, fmt.Errorf("file extension is required")
	}

	sum := md5.Sum(req.Content)
	md5Name := hex.EncodeToString(sum[:]) + "." + ext
	key := signature.Dir + md5Name

	form := map[string]string{
		"key":                   key,
		"policy":                signature.Policy,
		"OSSAccessKeyId":        signature.OSSAccessKeyID,
		"success_action_status": "200",
		"signature":             signature.Signature,
	}

	_, err = s.client.UploadFile(ctx, signature.Host, form, client.MultipartFile{
		FieldName: "file",
		FileName:  req.FileName,
		Content:   req.Content,
	}, nil)
	if err != nil {
		return nil, err
	}

	return &UploadedImage{
		Key:         key,
		MD5Name:     md5Name,
		ImageURL:    strings.TrimRight(signature.Host, "/") + "/" + key,
		Width:       req.Width,
		Height:      req.Height,
		ContentType: req.ContentType,
	}, nil
}

// CreateMaterial 在 SDS 素材库中登记已上传图片。
func (s *Service) CreateMaterial(ctx context.Context, req CreateMaterialRequest) (*Material, error) {
	if strings.TrimSpace(req.FileCode) == "" {
		return nil, fmt.Errorf("fileCode is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Length <= 0 {
		return nil, fmt.Errorf("length must be positive")
	}

	path := s.client.Config().Endpoints.MaterialCreatePath
	if path == "" {
		return nil, fmt.Errorf("material create endpoint is not configured")
	}

	query := map[string]string{
		"t": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	result := new(CreateMaterialResponse)
	_, err := s.client.Do(ctx, "POST", path, query, req, result)
	if err != nil {
		return nil, err
	}

	if result.Ret != 0 {
		return nil, fmt.Errorf("create material failed: %s", result.Msg)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("create material returned empty data")
	}

	return &result.Data[0], nil
}

// FindMaterialsByIDs 按素材 ID 查询素材信息。
func (s *Service) FindMaterialsByIDs(ctx context.Context, req FindMaterialsRequest) ([]Material, error) {
	if len(req.IDs) == 0 {
		return nil, fmt.Errorf("ids cannot be empty")
	}

	path := s.client.Config().Endpoints.MaterialFindByIDs
	if path == "" {
		return nil, fmt.Errorf("material find endpoint is not configured")
	}

	ids := make([]string, 0, len(req.IDs))
	for _, id := range req.IDs {
		ids = append(ids, strconv.FormatInt(id, 10))
	}

	query := map[string]string{
		"ids": strings.Join(ids, ","),
		"t":   strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	if strings.TrimSpace(req.Fields) != "" {
		query["fields"] = req.Fields
	}

	var result []Material
	_, err := s.client.Do(ctx, "GET", path, query, nil, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// UploadAndCreateMaterial 完成 OSS 上传和素材登记。
func (s *Service) UploadAndCreateMaterial(ctx context.Context, req UploadRequest) (*UploadedMaterial, error) {
	image, err := s.UploadToOSS(ctx, req)
	if err != nil {
		return nil, err
	}

	material, err := s.CreateMaterial(ctx, CreateMaterialRequest{
		FileCode:       image.MD5Name,
		Length:         int64(len(req.Content)),
		Name:           req.FileName,
		ContentType:    req.ContentType,
		Width:          req.Width,
		Height:         req.Height,
		ParentFolderID: 0,
		RepeatReturnID: true,
	})
	if err != nil {
		return nil, err
	}
	refreshed, err := s.FindMaterialsByIDs(ctx, FindMaterialsRequest{
		IDs:    []int64{material.ID},
		Fields: "id,name,imgUrl,width,height,file_code,content_type",
	})
	if err == nil && len(refreshed) > 0 {
		material = &refreshed[0]
	}

	return &UploadedMaterial{
		Image:    image,
		Material: material,
	}, nil
}

// GetDesignProduct 获取设计页初始化数据。
func (s *Service) GetDesignProduct(ctx context.Context, variantID int64) (*DesignProductPage, error) {
	if variantID <= 0 {
		return nil, fmt.Errorf("variantID must be positive")
	}

	path := fmt.Sprintf(s.client.Config().Endpoints.DesignProductPath, variantID)
	result := new(DesignProductPage)
	_, err := s.client.Do(ctx, "GET", path, nil, nil, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetProductDetail 获取 SDS 商品详情。syncDesign 的稳定 mockup 图会回写到该详情中。
func (s *Service) GetProductDetail(ctx context.Context, parentProductID int64) (*sdstemplate.ProductDetail, error) {
	if parentProductID <= 0 {
		return nil, fmt.Errorf("parentProductID must be positive")
	}
	pathTemplate := strings.TrimSpace(s.client.Config().Endpoints.TemplateDetailPath)
	if pathTemplate == "" {
		return nil, fmt.Errorf("template detail endpoint is not configured")
	}

	path := fmt.Sprintf(pathTemplate, strconv.FormatInt(parentProductID, 10))
	result := new(sdstemplate.ProductDetail)
	_, err := s.client.Do(ctx, "GET", path, nil, nil, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetPrototypeGroups 获取父商品可用模板组。
func (s *Service) GetPrototypeGroups(ctx context.Context, parentProductID int64) ([]PrototypeGroup, error) {
	if parentProductID <= 0 {
		return nil, fmt.Errorf("parentProductID must be positive")
	}

	path := fmt.Sprintf(s.client.Config().Endpoints.PrototypeGroupPath, parentProductID)
	var result []PrototypeGroup
	_, err := s.client.Do(ctx, "GET", path, nil, nil, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetResultGroups 获取结果分组选项。
func (s *Service) GetResultGroups(ctx context.Context, prototypeGroupID int64) ([]ResultGroupOption, error) {
	if prototypeGroupID <= 0 {
		return nil, fmt.Errorf("prototypeGroupID must be positive")
	}

	query := map[string]string{
		"prototypeGroupId": strconv.FormatInt(prototypeGroupID, 10),
		"t":                strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	var result []ResultGroupOption
	_, err := s.client.Do(ctx, "GET", s.client.Config().Endpoints.ResultGroupPath, query, nil, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetCutFileContent 获取 PSD 切图和智能对象信息。
func (s *Service) GetCutFileContent(ctx context.Context, fileCodes []string) (CutFileContent, error) {
	if len(fileCodes) == 0 {
		return nil, fmt.Errorf("fileCodes cannot be empty")
	}

	codes := make([]string, 0, len(fileCodes))
	for _, code := range fileCodes {
		code = strings.TrimSpace(strings.TrimSuffix(code, ".psd"))
		if code == "" {
			continue
		}
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("valid fileCodes cannot be empty")
	}

	query := map[string]string{
		"ids": strings.Join(codes, ","),
	}

	result := make(CutFileContent)
	_, err := s.client.Do(ctx, "GET", s.client.Config().Endpoints.CutFileContentPath, query, nil, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// SyncDesign 保存 SDS 设计结果。
func (s *Service) SyncDesign(ctx context.Context, req SyncDesignRequest) (*SyncDesignResponse, error) {
	if req.ProductID <= 0 {
		return nil, fmt.Errorf("productID must be positive")
	}
	if req.PrototypeGroupID <= 0 {
		return nil, fmt.Errorf("prototypeGroupID must be positive")
	}
	if strings.TrimSpace(req.DesignType) == "" {
		return nil, fmt.Errorf("designType is required")
	}
	if len(req.Prototypes) == 0 {
		return nil, fmt.Errorf("prototypes cannot be empty")
	}

	path := s.client.Config().Endpoints.SyncDesignPath
	if path == "" {
		return nil, fmt.Errorf("sync design endpoint is not configured")
	}

	_, err := s.client.Do(ctx, "POST", path, nil, req, nil)
	if err != nil {
		return nil, err
	}

	return &SyncDesignResponse{}, nil
}

// SaveDesign calls the same save endpoint as the SDS designer UI. This is the
// action that creates SDS-side rendered product artifacts for the current design.
func (s *Service) SaveDesign(ctx context.Context, req SaveDesignRequest) error {
	if req.ProductID <= 0 {
		return fmt.Errorf("productID must be positive")
	}
	if len(req.Prototypes) == 0 {
		return fmt.Errorf("prototypes cannot be empty")
	}

	path := s.client.Config().Endpoints.AddAndDesignPath
	if path == "" {
		return fmt.Errorf("add and design endpoint is not configured")
	}

	_, err := s.client.Do(ctx, "POST", path, nil, req, nil)
	return err
}

// PrepareSyncDesign 基于设计页初始化数据和上传素材，构造默认 syncDesign 请求。
func (s *Service) PrepareSyncDesign(ctx context.Context, input PrepareSyncDesignInput, material *UploadedMaterial) (*PrepareSyncDesignResult, error) {
	if material == nil || material.Image == nil || material.Material == nil {
		return nil, fmt.Errorf("material is required")
	}
	if input.VariantID <= 0 {
		return nil, fmt.Errorf("variantID must be positive")
	}

	page, err := s.GetDesignProduct(ctx, input.VariantID)
	if err != nil {
		return nil, err
	}

	layer, err := selectLayer(page.Layers, input.LayerID)
	if err != nil {
		return nil, err
	}

	prototypeGroupID := input.PrototypeGroupID
	if prototypeGroupID == 0 {
		prototypeGroupID = page.PrototypeGroup.ID
	}
	resultGroupID := input.MerchantResultID
	if resultGroupID == 0 {
		resultGroupID = page.MerchantProductResultGroupID
	}
	designType := input.DesignType
	if designType == "" {
		designType = "material"
	}
	fitLevel := input.FitLevel
	if fitLevel <= 0 {
		fitLevel = 1
	}

	fabricJSON, err := buildFabricJSON(material, layer, fitLevel)
	if err != nil {
		return nil, err
	}

	prototypes := []SyncDesignPrototype{
		{
			PrototypeID: page.Product.PrototypeID,
			ProductIDs:  []int64{page.Product.ID},
			PSDIDs:      collectPSDIDs(page.PSDs),
			Layers: []SyncDesignLayer{
				{
					MaterialID:         "",
					LayerID:            layer.ID,
					Content:            "",
					ImgWidth:           layerPrintWidth(*layer),
					ImgHeight:          layerPrintHeight(*layer),
					ResizeMode:         input.ResizeMode,
					FitLevel:           fitLevel,
					FabricJSON:         fabricJSON,
					RelatedMaterialIDs: []int64{material.Material.ID},
				},
			},
			Images: buildPreviewImageURLs(page.PSDs, layer.Name, material, input.ResizeMode),
		},
	}
	relatedPages := map[int64]*DesignProductPage{}
	relatedPages[page.Product.ID] = page
	for _, relatedVariantID := range input.RelatedVariantIDs {
		if relatedVariantID <= 0 || relatedVariantID == page.Product.ID {
			continue
		}
		relatedPage, err := s.GetDesignProduct(ctx, relatedVariantID)
		if err != nil {
			return nil, err
		}
		relatedLayer, err := selectLayer(relatedPage.Layers, "")
		if err != nil {
			return nil, err
		}
		relatedFabricJSON, err := buildFabricJSON(material, relatedLayer, fitLevel)
		if err != nil {
			return nil, err
		}
		prototypes = append(prototypes, SyncDesignPrototype{
			PrototypeID: relatedPage.Product.PrototypeID,
			ProductIDs:  []int64{relatedPage.Product.ID},
			PSDIDs:      collectPSDIDs(relatedPage.PSDs),
			Layers: []SyncDesignLayer{
				{
					MaterialID:         "",
					LayerID:            relatedLayer.ID,
					Content:            "",
					ImgWidth:           layerPrintWidth(*relatedLayer),
					ImgHeight:          layerPrintHeight(*relatedLayer),
					ResizeMode:         input.ResizeMode,
					FitLevel:           fitLevel,
					FabricJSON:         relatedFabricJSON,
					RelatedMaterialIDs: []int64{material.Material.ID},
				},
			},
			Images: buildPreviewImageURLs(relatedPage.PSDs, relatedLayer.Name, material, input.ResizeMode),
		})
		relatedPages[relatedPage.Product.ID] = relatedPage
	}

	req := &SyncDesignRequest{
		ProductID:                    page.Product.ID,
		PrototypeGroupID:             prototypeGroupID,
		MerchantProductResultGroupID: resultGroupID,
		DesignType:                   designType,
		Prototypes:                   prototypes,
	}

	return &PrepareSyncDesignResult{
		Page:         page,
		RelatedPages: relatedPages,
		Request:      req,
		Material:     material,
	}, nil
}

// PrepareAndSyncDesign 完成“上传素材并保存设计”。
func (s *Service) PrepareAndSyncDesign(ctx context.Context, input PrepareSyncDesignInput, upload UploadRequest) (*PrepareSyncDesignResult, error) {
	material, err := s.UploadAndCreateMaterial(ctx, upload)
	if err != nil {
		return nil, err
	}

	result, err := s.PrepareSyncDesign(ctx, input, material)
	if err != nil {
		return nil, err
	}

	if err := s.syncDesignWithRetry(ctx, *result.Request); err != nil {
		return nil, err
	}
	if err := s.saveDesignWithRetry(ctx, buildSaveDesignRequest(result)); err != nil {
		result.RenderedImageURLs = s.fetchRenderedImageURLs(ctx, input, result)
		if len(result.RenderedImageURLs) > 0 {
			return result, nil
		}
		return nil, err
	}

	result.RenderedImageURLs = s.fetchRenderedImageURLs(ctx, input, result)
	return result, nil
}

func (s *Service) syncDesignWithRetry(ctx context.Context, req SyncDesignRequest) error {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				if lastErr != nil {
					return lastErr
				}
				return ctx.Err()
			case <-time.After(time.Duration(attempt*5) * time.Second):
			}
		}
		_, err := s.SyncDesign(ctx, req)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isSDSTooFrequentError(err) {
			return err
		}
	}
	return lastErr
}

func (s *Service) saveDesignWithRetry(ctx context.Context, req SaveDesignRequest) error {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				if lastErr != nil {
					return lastErr
				}
				return ctx.Err()
			case <-time.After(time.Duration(attempt*3) * time.Second):
			}
		}
		err := s.SaveDesign(ctx, req)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isSDSTooFrequentError(err) {
			return err
		}
	}
	return lastErr
}

func isSDSTooFrequentError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "太频繁") ||
		strings.Contains(message, "too frequent")
}

// CreatePreview 创建预览图任务。
func (s *Service) CreatePreview(ctx context.Context, req PreviewRequest, result any) error {
	if s.client.Config().Endpoints.PreviewCreatePath == "" {
		return fmt.Errorf("preview create endpoint is not configured")
	}

	_, err := s.client.Do(ctx, "POST", s.client.Config().Endpoints.PreviewCreatePath, nil, req.Body, result)
	return err
}

// SaveProductDraft 创建或更新 SDS 商品草稿。
func (s *Service) SaveProductDraft(ctx context.Context, req ProductDraftRequest, result any) error {
	if s.client.Config().Endpoints.ProductDraftPath == "" {
		return fmt.Errorf("product draft endpoint is not configured")
	}

	_, err := s.client.Do(ctx, "POST", s.client.Config().Endpoints.ProductDraftPath, nil, req.Body, result)
	return err
}

// ListDesignProducts queries SDS 成品库. The frontend uses mapi2 for this API.
func (s *Service) ListDesignProducts(ctx context.Context, req ListDesignProductsRequest) (*DesignProductListResponse, error) {
	path := s.client.Config().Endpoints.DesignProductsPath
	if path == "" {
		return nil, fmt.Errorf("design products endpoint is not configured")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.Size
	if size <= 0 {
		size = 10
	}
	designType := strings.TrimSpace(req.DesignType)
	if designType == "" {
		designType = "material"
	}

	query := map[string]string{
		"1":                "1",
		"designType":       designType,
		"size":             strconv.Itoa(size),
		"page":             strconv.Itoa(page),
		"lifecycleType":    "live",
		"isMicroCustomize": "no",
	}
	if req.ProductID > 0 {
		query["product_id"] = strconv.FormatInt(req.ProductID, 10)
	}
	if req.ParentProductID > 0 {
		query["product_parent_id"] = strconv.FormatInt(req.ParentProductID, 10)
	}

	result := new(DesignProductListResponse)
	_, err := s.client.Do(ctx, "GET", path, query, nil, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListSensitiveWordsByItemIDs queries SDS for sensitive-word hits on rendered
// design product items.
func (s *Service) ListSensitiveWordsByItemIDs(ctx context.Context, merchantID int64, itemIDs []string) map[string][]SensitiveWordHit {
	if merchantID <= 0 || len(itemIDs) == 0 || s == nil || s.client == nil {
		return nil
	}
	trimmedIDs := make([]string, 0, len(itemIDs))
	seen := make(map[string]struct{}, len(itemIDs))
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		if _, ok := seen[itemID]; ok {
			continue
		}
		seen[itemID] = struct{}{}
		trimmedIDs = append(trimmedIDs, itemID)
	}
	if len(trimmedIDs) == 0 {
		return nil
	}

	path := fmt.Sprintf("/merchants/%d/designProducts/sensitiveWordsByIds", merchantID)
	var response []struct {
		ID     string             `json:"id"`
		Result []SensitiveWordHit `json:"result"`
	}
	_, err := s.client.Do(ctx, "POST", path, nil, trimmedIDs, &response)
	if err != nil || len(response) == 0 {
		return nil
	}
	result := make(map[string][]SensitiveWordHit, len(response))
	for _, item := range response {
		itemID := strings.TrimSpace(item.ID)
		if itemID == "" || len(item.Result) == 0 {
			continue
		}
		result[itemID] = append([]SensitiveWordHit(nil), item.Result...)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// UpdateDesignProducts rewrites SDS finished-product export metadata, such as
// export names blocked by sensitive-word checks.
func (s *Service) UpdateDesignProducts(ctx context.Context, updates []UpdateDesignProductRequest) error {
	if len(updates) == 0 {
		return nil
	}
	path := s.client.Config().Endpoints.DesignProductsUpdatePath
	if path == "" {
		return fmt.Errorf("design products update endpoint is not configured")
	}
	_, err := s.client.Do(ctx, "PUT", path, nil, updates, nil)
	return err
}
