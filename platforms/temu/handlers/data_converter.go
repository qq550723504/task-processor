package handlers

import (
	"task-processor/common/pipeline"
)

// DataConverter 数据转换器
type DataConverter struct{}

// NewDataConverter 创建新的数据转换器
func NewDataConverter() *DataConverter {
	return &DataConverter{}
}

// PreparePropertyMappingData 准备属性映射数据
func (c *DataConverter) PreparePropertyMappingData(ctx *pipeline.TaskContext, templateProps []GoodsProperty) PropertyMappingData {
	data := PropertyMappingData{
		TemuProperties: make([]TemuPropertyOption, 0, len(templateProps)),
	}

	// 组织Amazon产品数据
	if ctx.AmazonProduct != nil {
		data.AmazonProduct = c.convertAmazonProductData(ctx)
	}

	// 组织TEMU属性选项
	for _, templateProp := range templateProps {
		data.TemuProperties = append(data.TemuProperties, c.convertTemuPropertyOption(templateProp))
	}

	return data
}

// convertAmazonProductData 转换Amazon产品数据
func (c *DataConverter) convertAmazonProductData(ctx *pipeline.TaskContext) AmazonProductData {
	amazonProd := ctx.AmazonProduct

	data := AmazonProductData{
		Title:             amazonProd.Title,
		Brand:             amazonProd.Brand,
		Description:       amazonProd.Description,
		Features:          amazonProd.Features,
		ProductDimensions: amazonProd.ProductDimensions,
		ItemWeight:        amazonProd.ItemWeight,
		ModelNumber:       amazonProd.ModelNumber,
		Department:        amazonProd.Department,
		Manufacturer:      amazonProd.Manufacturer,
		Categories:        amazonProd.Categories,
		ProductDetails:    make([]ProductDetailData, 0, len(amazonProd.ProductDetails)),
	}

	// 转换产品详情
	for _, detail := range amazonProd.ProductDetails {
		data.ProductDetails = append(data.ProductDetails, ProductDetailData{
			Type:  detail.Type,
			Value: detail.Value,
		})
	}

	return data
}

// convertTemuPropertyOption 转换TEMU属性选项
func (c *DataConverter) convertTemuPropertyOption(templateProp GoodsProperty) TemuPropertyOption {
	temuProp := TemuPropertyOption{
		PID:         templateProp.PID,
		RefPID:      templateProp.RefPID,
		TemplatePID: templateProp.TemplatePID,
		//TemplateModuleID:  templateProp.TemplateModuleID,
		Name:              templateProp.Name,
		PropertyValueType: templateProp.PropertyValueType,
		Required:          templateProp.Required,
		ChooseMaxNum:      templateProp.ChooseMaxNum,
		ValueUnit:         templateProp.ValueUnit,
		MinValue:          templateProp.MinValue,
		MaxValue:          templateProp.MaxValue,
	}

	// 转换属性值选项
	for _, value := range templateProp.Values {
		temuProp.Values = append(temuProp.Values, PropertyValueOption{
			VID:   value.VID,
			Value: value.Value,
		})
	}

	// 转换ShowCondition
	if len(templateProp.ShowCondition) > 0 {
		temuProp.ShowCondition = make([]ShowCondition, len(templateProp.ShowCondition))
		for i, condition := range templateProp.ShowCondition {
			temuProp.ShowCondition[i] = ShowCondition{
				ParentRefPID: condition.ParentRefPID,
				ParentVIDs:   condition.ParentVIDs,
			}
		}
	}

	return temuProp
}
