package shein

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	sheinmodel "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/storecenter"
)

func (c *OfficialClient) QueryProductWarehouses(ctx context.Context, credential storecenter.OfficialMerchantCredential) ([]sheinmodel.Warehouse, error) {
	raw, err := c.goodsRequest(ctx, http.MethodGet, "/open-api/msc/warehouse/list", "", credential, nil)
	if err != nil {
		return nil, ErrGoodsUnavailable
	}
	var response struct {
		Code string `json:"code"`
		Info *struct {
			List []struct {
				Code          string          `json:"warehouseCode"`
				Name          string          `json:"warehouseName"`
				SaleCountries []string        `json:"saleCountryList"`
				Type          json.RawMessage `json:"warehouseType"`
			} `json:"list"`
		} `json:"info"`
	}
	if decodeGoods(raw, &response) != nil || response.Code != "0" || response.Info == nil || response.Info.List == nil || len(response.Info.List) > 20 {
		return nil, ErrGoodsUnavailable
	}
	output := make([]sheinmodel.Warehouse, 0, len(response.Info.List))
	seen := map[string]bool{}
	for _, row := range response.Info.List {
		var text string
		if json.Unmarshal(row.Type, &text) != nil {
			text = string(row.Type)
		}
		kind, err := strconv.Atoi(text)
		if err != nil || kind < 1 || kind > 2 || !boundedParameter(row.Code, 128) || seen[row.Code] || !boundedParameter(row.Name, 256) || row.SaleCountries == nil || len(row.SaleCountries) > 64 {
			return nil, ErrGoodsUnavailable
		}
		for _, country := range row.SaleCountries {
			if len(country) != 2 || country[0] < 'A' || country[0] > 'Z' || country[1] < 'A' || country[1] > 'Z' {
				return nil, ErrGoodsUnavailable
			}
		}
		seen[row.Code] = true
		output = append(output, sheinmodel.Warehouse{Code: row.Code, Name: row.Name, SaleCountries: row.SaleCountries, Type: kind})
	}
	return output, nil
}

func queryGoodsData[T any](c *OfficialClient, ctx context.Context, credential storecenter.OfficialMerchantCredential, path string, payload []byte) ([]T, error) {
	raw, err := c.goodsPost(ctx, path, credential, payload)
	if err != nil {
		return nil, ErrGoodsUnavailable
	}
	var response struct {
		Code string `json:"code"`
		Info *struct {
			Data []T `json:"data"`
		} `json:"info"`
	}
	if decodeGoods(raw, &response) != nil || response.Code != "0" || response.Info == nil || response.Info.Data == nil {
		return nil, ErrGoodsUnavailable
	}
	return response.Info.Data, nil
}
func (c *OfficialClient) QueryProductCategories(ctx context.Context, credential storecenter.OfficialMerchantCredential) ([]sheinmodel.Category, error) {
	data, err := queryGoodsData[sheinmodel.Category](c, ctx, credential, "/open-api/goods/query-category-tree", nil)
	if err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	var visit func([]sheinmodel.Category, int) bool
	visit = func(nodes []sheinmodel.Category, depth int) bool {
		if depth > 32 {
			return false
		}
		for _, node := range nodes {
			if node.ID <= 0 || node.Leaf == nil || seen[node.ID] || len(seen) >= 20000 || *node.Leaf && (node.ProductTypeID <= 0 || len(node.Children) > 0) {
				return false
			}
			seen[node.ID] = true
			if !visit(node.Children, depth+1) {
				return false
			}
		}
		return true
	}
	if !visit(data, 1) {
		return nil, ErrGoodsUnavailable
	}
	return data, nil
}
func (c *OfficialClient) QueryProductAttributes(ctx context.Context, credential storecenter.OfficialMerchantCredential, productTypeID int64) (sheinmodel.AttributeTemplate, error) {
	if productTypeID <= 0 {
		return sheinmodel.AttributeTemplate{}, ErrGoodsInvalid
	}
	payload, _ := json.Marshal(struct {
		IDs []int64 `json:"product_type_id_list"`
	}{[]int64{productTypeID}})
	data, err := queryGoodsData[sheinmodel.AttributeTemplate](c, ctx, credential, "/open-api/goods/query-attribute-template", payload)
	if err != nil || len(data) != 1 || data[0].ProductTypeID != productTypeID || data[0].MainAttributeStatus == nil || *data[0].MainAttributeStatus < 1 || *data[0].MainAttributeStatus > 3 || data[0].Attributes == nil || len(data[0].Attributes) > 1000 {
		return sheinmodel.AttributeTemplate{}, ErrGoodsUnavailable
	}
	seen := map[int64]bool{}
	for _, attribute := range data[0].Attributes {
		if attribute.ID <= 0 || seen[attribute.ID] || attribute.Type == nil || *attribute.Type < 1 || *attribute.Type > 4 || attribute.Status == nil || *attribute.Status < 1 || *attribute.Status > 3 || attribute.Mode == nil || *attribute.Mode < 0 || *attribute.Mode > 4 || len(attribute.Options) > 10000 || len(attribute.Rules) > 64 {
			return sheinmodel.AttributeTemplate{}, ErrGoodsUnavailable
		}
		seen[attribute.ID] = true
	}
	return data[0], nil
}
func (c *OfficialClient) QueryProductLinkedRules(ctx context.Context, credential storecenter.OfficialMerchantCredential, input sheinmodel.LinkedRulesRequest) ([]sheinmodel.LinkedRules, error) {
	if len(input.Groups) < 1 || len(input.Groups) > 10 {
		return nil, ErrGoodsInvalid
	}
	expected := map[string]bool{}
	for _, group := range input.Groups {
		if !boundedParameter(group.ID, 128) || expected[group.ID] || group.CategoryID <= 0 || group.ProductTypeID <= 0 || group.Attributes == nil || len(group.Attributes) > 1000 {
			return nil, ErrGoodsInvalid
		}
		expected[group.ID] = true
		for _, attribute := range group.Attributes {
			if attribute.AttributeID <= 0 || attribute.AttributeValueID != nil && *attribute.AttributeValueID <= 0 || attribute.CustomValue != "" || attribute.ExtraValue != "" || attribute.Language != "" {
				return nil, ErrGoodsInvalid
			}
		}
	}
	payload, _ := json.Marshal(input)
	data, err := queryGoodsData[sheinmodel.LinkedRules](c, ctx, credential, "/open-api/goods/get-associated-attribute-rules", payload)
	if err != nil || len(data) != len(expected) {
		return nil, ErrGoodsUnavailable
	}
	seen := map[string]bool{}
	for _, group := range data {
		if !expected[group.GroupID] || seen[group.GroupID] || len(group.Attributes) > 1000 {
			return nil, ErrGoodsUnavailable
		}
		seen[group.GroupID] = true
		attributes := map[int64]bool{}
		for _, rule := range group.Attributes {
			if rule.AttributeID <= 0 || attributes[rule.AttributeID] || len(rule.Values) > 10000 || len(rule.PrefilledValues) > 10000 {
				return nil, ErrGoodsUnavailable
			}
			attributes[rule.AttributeID] = true
			for _, values := range [][]int64{rule.Values, rule.PrefilledValues} {
				for _, id := range values {
					if id <= 0 {
						return nil, ErrGoodsUnavailable
					}
				}
			}
		}
	}
	return data, nil
}
func (c *OfficialClient) QueryProductBrands(ctx context.Context, credential storecenter.OfficialMerchantCredential) ([]sheinmodel.Brand, error) {
	data, err := queryGoodsData[sheinmodel.Brand](c, ctx, credential, "/open-api/goods/query-brand-list", nil)
	if err != nil || len(data) > 10000 {
		return nil, ErrGoodsUnavailable
	}
	seen := map[string]bool{}
	for _, brand := range data {
		if !boundedParameter(brand.Code, 128) || seen[brand.Code] {
			return nil, ErrGoodsUnavailable
		}
		seen[brand.Code] = true
	}
	return data, nil
}
func (c *OfficialClient) QueryProductPublishPermission(ctx context.Context, credential storecenter.OfficialMerchantCredential, brandCode string) (sheinmodel.PublishPermission, error) {
	if brandCode != "" && !boundedParameter(brandCode, 128) {
		return sheinmodel.PublishPermission{}, ErrGoodsInvalid
	}
	query := ""
	if brandCode != "" {
		query = url.Values{"brandCode": []string{brandCode}}.Encode()
	}
	raw, err := c.goodsRequest(ctx, http.MethodGet, "/open-api/goods/product/check-publish-permission", query, credential, nil)
	if err != nil {
		return sheinmodel.PublishPermission{}, ErrGoodsUnavailable
	}
	var response struct {
		Code string `json:"code"`
		Info *struct {
			CanPublish *bool  `json:"canPublishProduct"`
			Reason     string `json:"reason"`
		} `json:"info"`
	}
	if decodeGoods(raw, &response) != nil || response.Code != "0" || response.Info == nil || response.Info.CanPublish == nil || len(response.Info.Reason) > 4096 {
		return sheinmodel.PublishPermission{}, ErrGoodsUnavailable
	}
	return sheinmodel.PublishPermission{Allowed: *response.Info.CanPublish, Reason: response.Info.Reason}, nil
}
