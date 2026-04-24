package design

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
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

	req := &SyncDesignRequest{
		ProductID:                    page.Product.ID,
		PrototypeGroupID:             prototypeGroupID,
		MerchantProductResultGroupID: resultGroupID,
		DesignType:                   designType,
		Prototypes: []SyncDesignPrototype{
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
		},
	}

	return &PrepareSyncDesignResult{
		Page:     page,
		Request:  req,
		Material: material,
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

	if _, err := s.SyncDesign(ctx, *result.Request); err != nil {
		return nil, err
	}
	if err := s.SaveDesign(ctx, buildSaveDesignRequest(result)); err != nil {
		result.RenderedImageURLs = s.fetchRenderedImageURLs(ctx, input, result)
		if len(result.RenderedImageURLs) > 0 {
			return result, nil
		}
		return nil, err
	}

	result.RenderedImageURLs = s.fetchRenderedImageURLs(ctx, input, result)
	return result, nil
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

func selectLayer(layers []DesignLayer, layerID string) (*DesignLayer, error) {
	if len(layers) == 0 {
		return nil, fmt.Errorf("no design layers available")
	}
	if strings.TrimSpace(layerID) == "" {
		return &layers[0], nil
	}
	for i := range layers {
		if layers[i].ID == layerID {
			return &layers[i], nil
		}
	}
	return nil, fmt.Errorf("layer %s not found", layerID)
}

func collectPSDIDs(psds []PSDDocument) []string {
	ids := make([]string, 0, len(psds))
	for _, psd := range psds {
		if strings.TrimSpace(psd.ID) != "" {
			ids = append(ids, psd.ID)
		}
	}
	return ids
}

func buildPreviewImageURLs(psds []PSDDocument, layerName string, material *UploadedMaterial, resizeMode int) []string {
	urls := make([]string, 0, len(psds))
	replaceContent := materialContentPath(material)
	if replaceContent == "" {
		replaceContent = materialImageURL(material)
	}
	imageWidth, imageHeight := materialDimensions(material)
	for _, psd := range psds {
		modelFile := psdModelFile(psd)
		if modelFile == "" {
			continue
		}
		payload := map[string]any{
			"model_file": modelFile,
			"replace_layers_content": []map[string]any{
				{
					"layer_name":      layerName,
					"replace_type":    1,
					"replace_content": replaceContent,
					"image_width":     imageWidth,
					"image_height":    imageHeight,
					"resize_mode":     resizeMode,
					"image_filter":    nil,
				},
			},
			"output_format": "jpg_thumb",
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			continue
		}
		urls = append(urls, "http://e.sdspod.com/builds?content="+url.QueryEscape(string(raw)))
	}
	return urls
}

func buildSaveDesignRequest(result *PrepareSyncDesignResult) SaveDesignRequest {
	if result == nil || result.Page == nil || result.Request == nil {
		return SaveDesignRequest{}
	}

	prototypes := make([]SyncDesignPrototype, 0, len(result.Request.Prototypes))
	for _, prototype := range result.Request.Prototypes {
		layers := make([]SyncDesignLayer, 0, len(prototype.Layers))
		for _, layer := range prototype.Layers {
			layer.MaterialID = ""
			if result.Material != nil && result.Material.Material != nil && result.Material.Material.ID > 0 {
				layer.MaterialID = result.Material.Material.ID
				layer.DesignMaterialID = result.Material.Material.ID
			}
			if content := materialContentPath(result.Material); content != "" {
				layer.Content = content
			}
			layer.ImgWidth = 1
			layer.ImgHeight = 1
			layer.ResizeMode = 0
			layers = append(layers, layer)
		}
		prototype.Layers = layers
		prototype.Images = psdThumbnailURLs(result.Page.PSDs)
		prototypes = append(prototypes, prototype)
	}

	req := SaveDesignRequest{
		ProductID:        result.Request.ProductID,
		PrototypeGroupID: result.Request.PrototypeGroupID,
		DesignType:       result.Request.DesignType,
		Prototypes:       prototypes,
	}
	return req
}

func psdThumbnailURLs(psds []PSDDocument) []string {
	urls := make([]string, 0, len(psds))
	for _, psd := range psds {
		if strings.TrimSpace(psd.ThumbnailURL) != "" {
			urls = append(urls, strings.TrimSpace(psd.ThumbnailURL))
		}
	}
	return urls
}

func psdModelFile(psd PSDDocument) string {
	fileURL := strings.TrimSpace(psd.FileURL)
	if fileURL != "" {
		if parsed, err := url.Parse(fileURL); err == nil {
			path := strings.TrimPrefix(parsed.EscapedPath(), "/")
			path = strings.TrimPrefix(path, "psds/")
			if decoded, err := url.PathUnescape(path); err == nil {
				path = decoded
			}
			if path != "" {
				return path
			}
		}
	}
	return strings.TrimSpace(psd.FileCode)
}

func (s *Service) fetchRenderedImageURLs(ctx context.Context, input PrepareSyncDesignInput, result *PrepareSyncDesignResult) []string {
	parentProductID := input.ParentProductID
	if parentProductID <= 0 && result != nil && result.Page != nil {
		parentProductID = result.Page.Product.ParentID
		if parentProductID <= 0 {
			parentProductID = result.Page.MerchantProductParentID
		}
	}
	if parentProductID <= 0 {
		return nil
	}

	var variantID int64
	if result != nil && result.Page != nil {
		variantID = result.Page.Product.ID
	}
	if variantID <= 0 {
		variantID = input.VariantID
	}

	expectedCount := expectedRenderedImageCount(result)
	var bestURLs []string
	for attempt := 0; attempt < 8; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return bestURLs
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		if urls := s.fetchFinishedProductImageURLs(ctx, input, result, variantID, parentProductID); len(urls) > 0 {
			bestURLs = preferredRenderedImageURLs(bestURLs, urls)
			if renderedImageURLsReady(urls, expectedCount) {
				return urls
			}
		}
		detail, err := s.GetProductDetail(ctx, parentProductID)
		if err != nil {
			continue
		}
		urls := renderedImageURLsFromProduct(detail, variantID)
		if staleRenderedImageURLs(urls, result) {
			continue
		}
		if len(urls) > 0 {
			bestURLs = preferredRenderedImageURLs(bestURLs, urls)
			if renderedImageURLsReady(urls, expectedCount) {
				return urls
			}
		}
	}
	return bestURLs
}

func (s *Service) fetchFinishedProductImageURLs(ctx context.Context, input PrepareSyncDesignInput, result *PrepareSyncDesignResult, variantID int64, parentProductID int64) []string {
	if variantID <= 0 {
		return nil
	}
	list, err := s.ListDesignProducts(ctx, ListDesignProductsRequest{
		ProductID:       variantID,
		ParentProductID: parentProductID,
		DesignType:      input.DesignType,
		Page:            1,
		Size:            10,
	})
	if err != nil || list == nil || len(list.Items) == 0 {
		return nil
	}

	expectedMaterialName := finishedProductMaterialName(result)
	return selectFinishedProductImageURLs(list.Items, variantID, expectedMaterialName)
}

func selectFinishedProductImageURLs(items []DesignProductListItem, variantID int64, expectedMaterialName string) []string {
	candidates := append([]DesignProductListItem(nil), items...)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].FinishTime > candidates[j].FinishTime
	})
	if expectedMaterialName != "" {
		for _, item := range candidates {
			if urls := finishedProductItemImageURLs(item, variantID, expectedMaterialName); len(urls) > 0 {
				return urls
			}
		}
		return nil
	}
	for _, item := range candidates {
		if urls := finishedProductItemImageURLs(item, variantID, ""); len(urls) > 0 {
			return urls
		}
	}
	return nil
}

func finishedProductItemImageURLs(item DesignProductListItem, variantID int64, expectedMaterialName string) []string {
	if item.ProductID != 0 && item.ProductID != variantID {
		return nil
	}
	if !item.BuildFinish || len(item.ImageURLs) == 0 {
		return nil
	}
	if expectedMaterialName != "" && strings.TrimSpace(item.MaterialImageName) != "" && strings.TrimSpace(item.MaterialImageName) != expectedMaterialName {
		return nil
	}
	return renderedImageURLCandidates(item.ImageURLs)
}

func finishedProductMaterialName(result *PrepareSyncDesignResult) string {
	if result == nil || result.Material == nil || result.Material.Material == nil {
		return ""
	}
	name := strings.TrimSpace(result.Material.Material.Name)
	if name == "" {
		return ""
	}
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return strings.TrimSpace(name)
}

func staleRenderedImageURLs(urls []string, result *PrepareSyncDesignResult) bool {
	if len(urls) == 0 || result == nil || result.Page == nil {
		return false
	}
	for _, value := range urls {
		if renderedImageURLUnavailable(value) {
			return true
		}
	}
	initial := uniqueStrings([]string{
		result.Page.Product.ImgURL,
		result.Page.Product.PSDImgURL,
	})
	if len(initial) == 0 {
		return false
	}
	initialSet := make(map[string]struct{}, len(initial))
	for _, value := range initial {
		initialSet[value] = struct{}{}
	}
	if _, ok := initialSet[strings.TrimSpace(urls[0])]; ok {
		return true
	}
	for _, value := range urls {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := initialSet[value]; !ok {
			return false
		}
	}
	return true
}

func renderedImageURLsFromProduct(detail *sdstemplate.ProductDetail, variantID int64) []string {
	if detail == nil {
		return nil
	}
	if detail.Subproducts != nil {
		for i := range detail.Subproducts.Items {
			item := &detail.Subproducts.Items[i]
			if item.ID != variantID {
				continue
			}
			urls := renderedImageURLsFromSummary(item)
			if len(urls) > 0 {
				return urls
			}
		}
	}
	return renderedImageURLsFromSummary(&detail.ProductSummary)
}

func renderedImageURLsFromSummary(product *sdstemplate.ProductSummary) []string {
	if product == nil {
		return nil
	}
	urls := make([]string, 0, 8)
	if product.DesignPrototype != nil {
		groups := append([]sdstemplate.PrototypeResultGroup(nil), product.DesignPrototype.PrototypeResultGroups...)
		sort.SliceStable(groups, func(i, j int) bool {
			return groups[i].Sort < groups[j].Sort
		})
		for _, group := range groups {
			urls = append(urls, group.ResultImage)
		}
	}
	urls = append(urls, product.ImgURL, product.PSDImgURL)
	return renderedImageURLCandidates(urls)
}

func renderedImageURLCandidates(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if renderedImageURLUnavailable(value) {
			continue
		}
		filtered = append(filtered, value)
	}
	return uniqueStrings(filtered)
}

func expectedRenderedImageCount(result *PrepareSyncDesignResult) int {
	if result == nil || result.Page == nil {
		return 0
	}
	count := 0
	for _, psd := range result.Page.PSDs {
		if psdModelFile(psd) != "" {
			count++
		}
	}
	return count
}

func renderedImageURLsReady(urls []string, expectedCount int) bool {
	if len(urls) == 0 {
		return false
	}
	return expectedCount <= 1 || len(urls) >= expectedCount
}

func preferredRenderedImageURLs(current []string, candidate []string) []string {
	if len(candidate) > len(current) {
		return candidate
	}
	return current
}

func renderedImageURLUnavailable(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return true
	}
	return strings.Contains(value, "shengchengzhong") ||
		strings.Contains(value, "/output/generating") ||
		strings.Contains(value, "/output/loading") ||
		strings.Contains(value, "/output/placeholder") ||
		strings.Contains(value, "cdn.sdspod.com/images/")
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func materialImageURL(material *UploadedMaterial) string {
	if material == nil {
		return ""
	}
	if material.Material != nil && strings.TrimSpace(material.Material.ImageURL) != "" {
		return strings.TrimSpace(material.Material.ImageURL)
	}
	if material.Material != nil && strings.TrimSpace(material.Material.ImageURLAlt) != "" {
		return strings.TrimSpace(material.Material.ImageURLAlt)
	}
	if material.Image != nil && strings.TrimSpace(material.Image.ImageURL) != "" {
		return strings.TrimSpace(material.Image.ImageURL)
	}
	return ""
}

func materialDimensions(material *UploadedMaterial) (int, int) {
	if material == nil {
		return 1, 1
	}
	width, height := 0, 0
	if material.Image != nil {
		width = material.Image.Width
		height = material.Image.Height
	}
	if material.Material != nil {
		if width <= 0 {
			width = int(material.Material.Width)
		}
		if height <= 0 {
			height = int(material.Material.Height)
		}
	}
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	return width, height
}

func buildFabricJSON(material *UploadedMaterial, layer *DesignLayer, fitLevel float64) (string, error) {
	imageURL := materialDesignURL(material)
	if strings.TrimSpace(imageURL) == "" {
		return "", fmt.Errorf("material image url is empty")
	}

	sourceWidth := int(material.Material.Width)
	sourceHeight := int(material.Material.Height)
	if sourceWidth <= 0 {
		sourceWidth = material.Image.Width
	}
	if sourceHeight <= 0 {
		sourceHeight = material.Image.Height
	}
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return "", fmt.Errorf("material dimensions are invalid")
	}

	printWidth := float64(layerPrintWidth(*layer))
	printHeight := float64(layerPrintHeight(*layer))
	scale := minFloat(printWidth/float64(sourceWidth), printHeight/float64(sourceHeight))
	scale *= fitLevel

	doc := map[string]any{
		"version":         "5.2.1",
		"centeredScaling": false,
		"objects": []map[string]any{
			{
				"type":                     "image",
				"version":                  "5.2.1",
				"originX":                  "center",
				"originY":                  "center",
				"left":                     300,
				"top":                      300,
				"width":                    sourceWidth,
				"height":                   sourceHeight,
				"fill":                     "rgb(0,0,0)",
				"stroke":                   nil,
				"strokeWidth":              0,
				"strokeDashArray":          nil,
				"strokeLineCap":            "butt",
				"strokeDashOffset":         0,
				"strokeLineJoin":           "miter",
				"strokeUniform":            false,
				"strokeMiterLimit":         4,
				"scaleX":                   scale,
				"scaleY":                   scale,
				"angle":                    0,
				"flipX":                    false,
				"flipY":                    false,
				"opacity":                  1,
				"shadow":                   nil,
				"visible":                  true,
				"backgroundColor":          "",
				"fillRule":                 "nonzero",
				"paintFirst":               "fill",
				"globalCompositeOperation": "source-over",
				"skewX":                    0,
				"skewY":                    0,
				"cropX":                    0,
				"cropY":                    0,
				"sds": map[string]any{
					"originUrl": imageURL,
					"styleKey":  "",
				},
				"selectable":      true,
				"centeredScaling": false,
				"src":             imageURL,
				"crossOrigin":     "anonymous",
				"filters":         []any{},
			},
		},
	}

	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func materialDesignURL(material *UploadedMaterial) string {
	imageURL := materialImageURL(material)
	if imageURL == "" || material == nil || material.Material == nil || material.Material.ID <= 0 {
		return imageURL
	}
	if strings.Contains(imageURL, "material_id=") {
		return imageURL
	}
	separator := "?"
	if strings.Contains(imageURL, "?") {
		separator = "&"
	}
	return imageURL + separator + "material_id=" + strconv.FormatInt(material.Material.ID, 10)
}

func materialContentPath(material *UploadedMaterial) string {
	imageURL := materialImageURL(material)
	if imageURL == "" {
		return ""
	}
	parsed, err := url.Parse(imageURL)
	if err != nil {
		return ""
	}
	path := strings.TrimPrefix(parsed.Path, "/")
	for _, prefix := range []string{"images1000Thumbs/", "imagesThumbs/", "officeImgs1000Thumbs/", "images/"} {
		if strings.HasPrefix(path, prefix) {
			path = strings.TrimPrefix(path, prefix)
			break
		}
	}
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	return strings.TrimSpace(path)
}

func layerPrintWidth(layer DesignLayer) int {
	if layer.PrintWidth > 0 {
		return int(layer.PrintWidth)
	}
	if layer.PrintWidthAlt > 0 {
		return int(layer.PrintWidthAlt)
	}
	return int(layer.Width)
}

func layerPrintHeight(layer DesignLayer) int {
	if layer.PrintHeight > 0 {
		return int(layer.PrintHeight)
	}
	if layer.PrintHeightAlt > 0 {
		return int(layer.PrintHeightAlt)
	}
	return int(layer.Height)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
