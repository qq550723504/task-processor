package promptmgmt

import "task-processor/internal/prompt"

type promptMetadata struct {
	Label       string
	Description string
	Scopes      []TemplateScopeDefinition
	Variables   []TemplateVariableDefinition
}

var tenantOnlyScopes = []TemplateScopeDefinition{
	{
		ID:          "tenant",
		Label:       "租户",
		Description: "租户级提示词覆盖，默认继承全局模板内容。",
	},
}

var promptVariableGlossary = map[string]TemplateVariableDefinition{
	"title":                  {Key: "title", Label: "标题", Description: "商品或 listing 的主标题文本。"},
	"description":            {Key: "description", Label: "描述", Description: "商品详情描述或长文案内容。"},
	"features":               {Key: "features", Label: "卖点", Description: "商品要点、特征或 bullet points。"},
	"categoryInfo":           {Key: "categoryInfo", Label: "类目信息", Description: "候选类目列表或类目上下文信息。"},
	"CategoryPath":           {Key: "CategoryPath", Label: "类目路径", Description: "完整类目路径文本。"},
	"ProductSummary":         {Key: "ProductSummary", Label: "商品摘要", Description: "商品摘要或结构化理解结果。"},
	"Text":                   {Key: "Text", Label: "文本输入", Description: "待分析或待评分的原始文本。"},
	"BaseScore":              {Key: "BaseScore", Label: "基础分", Description: "规则或启发式预估出的基础评分。"},
	"analysis_sections":      {Key: "analysis_sections", Label: "分析片段", Description: "前序理解阶段产出的分析分段内容。"},
	"text":                   {Key: "text", Label: "文本内容", Description: "待抽取尺寸、重量等信息的原始文本。"},
	"product_type":           {Key: "product_type", Label: "商品类型", Description: "商品品类或物体类型。"},
	"scene_intent":           {Key: "scene_intent", Label: "场景意图", Description: "目标场景图想表达的用途或氛围。"},
	"scene_style":            {Key: "scene_style", Label: "场景风格", Description: "场景视觉风格或摄影风格。"},
	"background_tone":        {Key: "background_tone", Label: "背景色调", Description: "背景整体色调或环境基调。"},
	"composition":            {Key: "composition", Label: "构图", Description: "画面构图要求。"},
	"props_level":            {Key: "props_level", Label: "道具程度", Description: "辅助道具使用的丰富程度。"},
	"audience_hint":          {Key: "audience_hint", Label: "受众提示", Description: "目标用户或受众线索。"},
	"custom_scene_hint":      {Key: "custom_scene_hint", Label: "自定义场景提示", Description: "额外的场景补充要求。"},
	"summary_json":           {Key: "summary_json", Label: "摘要 JSON", Description: "待复核的结构化摘要 JSON。"},
	"TransparentHint":        {Key: "TransparentHint", Label: "透明背景提示", Description: "是否需要透明背景的附加说明。"},
	"ReferenceHint":          {Key: "ReferenceHint", Label: "参考图提示", Description: "参考素材或参考图的附加说明。"},
	"PrintableHint":          {Key: "PrintableHint", Label: "可打印提示", Description: "面向印刷或生产的附加约束。"},
	"ThemePrompt":            {Key: "ThemePrompt", Label: "主题提示词", Description: "主设计主题或创意描述。"},
	"ImageIndex":             {Key: "ImageIndex", Label: "图片序号", Description: "当前生成图片在整组中的序号。"},
	"ImageTotal":             {Key: "ImageTotal", Label: "图片总数", Description: "整组需要生成的图片总数。"},
	"ImageRoleLabel":         {Key: "ImageRoleLabel", Label: "图片角色", Description: "当前图片承担的展示角色。"},
	"ImageGoal":              {Key: "ImageGoal", Label: "图片目标", Description: "当前图片需要满足的核心目标。"},
	"ImageComposition":       {Key: "ImageComposition", Label: "图片构图", Description: "当前图片的构图要求。"},
	"ProductNameLine":        {Key: "ProductNameLine", Label: "商品名称行", Description: "商品名称相关的提示文本。"},
	"CategoryLine":           {Key: "CategoryLine", Label: "类目行", Description: "类目信息相关的提示文本。"},
	"StyleLine":              {Key: "StyleLine", Label: "风格行", Description: "风格相关的提示文本。"},
	"UserInstructionLine":    {Key: "UserInstructionLine", Label: "用户指令行", Description: "用户补充要求的提示文本。"},
	"Prompt":                 {Key: "Prompt", Label: "主题提示", Description: "当前图片生成的主提示词。"},
	"SourceAttribute":        {Key: "SourceAttribute", Label: "源属性", Description: "来源平台或原始数据中的属性名。"},
	"SourceValue":            {Key: "SourceValue", Label: "源值", Description: "来源平台或原始数据中的属性值。"},
	"AdditionalContextBlock": {Key: "AdditionalContextBlock", Label: "附加上下文", Description: "额外上下文说明块。"},
	"CandidatesBlock":        {Key: "CandidatesBlock", Label: "候选项", Description: "候选属性、候选值或候选类目列表。"},
	"SourcesBlock":           {Key: "SourcesBlock", Label: "来源列表", Description: "来源属性或来源值的汇总块。"},
	"SourceSegmentsBlock":    {Key: "SourceSegmentsBlock", Label: "来源片段", Description: "来源文本分段或切片内容。"},
	"TemplateAttribute":      {Key: "TemplateAttribute", Label: "模板属性", Description: "目标平台模板中的属性名。"},
	"AttributeTasksBlock":    {Key: "AttributeTasksBlock", Label: "属性任务列表", Description: "待处理属性任务集合。"},
	"AttributeID":            {Key: "AttributeID", Label: "属性 ID", Description: "目标平台属性标识。"},
	"AttributeType":          {Key: "AttributeType", Label: "属性类型", Description: "目标属性的数据类型。"},
	"Required":               {Key: "Required", Label: "是否必填", Description: "属性是否为必填项。"},
	"Important":              {Key: "Important", Label: "是否重要", Description: "属性是否为重要字段。"},
	"SourceAttributesBlock":  {Key: "SourceAttributesBlock", Label: "源属性集合", Description: "原始属性集合摘要。"},
	"AttributesBlock":        {Key: "AttributesBlock", Label: "属性集合", Description: "目标属性集合摘要。"},
	"SourceDimensionsBlock":  {Key: "SourceDimensionsBlock", Label: "来源维度集合", Description: "原始规格维度集合。"},
	"TemplatesBlock":         {Key: "TemplatesBlock", Label: "模板集合", Description: "目标平台模板属性集合。"},
	"FeedbackBlock":          {Key: "FeedbackBlock", Label: "反馈信息", Description: "前轮校验或人工反馈信息。"},
	"SourceDimension":        {Key: "SourceDimension", Label: "来源维度", Description: "原始规格维度名称。"},
	"SourceValuesBlock":      {Key: "SourceValuesBlock", Label: "来源值集合", Description: "来源规格值集合。"},
	"AttributeName":          {Key: "AttributeName", Label: "属性名", Description: "当前目标属性名称。"},
	"ProductTitle":           {Key: "ProductTitle", Label: "商品标题", Description: "当前商品标题。"},
	"Base64Image":            {Key: "Base64Image", Label: "Base64 图片", Description: "待识别图片的 Base64 编码内容。"},
}

var promptCatalogMetadata = map[string]promptMetadata{
	prompt.KSheinAttributeSelectorSystem: {
		Label:       "属性选择 / 系统提示词",
		Description: "SHEIN 属性选择链路的系统提示词。",
		Scopes:      tenantOnlyScopes,
	},
	prompt.KSheinCategorySelectorSelectCategorySystem: {
		Label:       "类目选择 / 系统提示词",
		Description: "SHEIN 类目选择阶段的系统提示词。",
		Scopes:      tenantOnlyScopes,
	},
	prompt.KSheinCategorySelectorSelectCategoryUser: {
		Label:       "类目选择 / 用户提示词",
		Description: "SHEIN 类目选择阶段的用户提示词模板。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("title", "categoryInfo"),
	},
	prompt.KSheinCategorySelectorExtractCoreItemSystem: {
		Label:       "类目选择 / 核心商品提取",
		Description: "SHEIN 类目推断前的核心商品提取提示词。",
		Scopes:      tenantOnlyScopes,
	},
	prompt.KSheinCategorySelectorSemanticValidation: {
		Label:       "类目选择 / 语义校验",
		Description: "SHEIN 类目候选的语义校验提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("CategoryPath", "ProductSummary"),
	},
	prompt.KSheinContentOptimizerOptimizeTitleDescriptionSystem: {
		Label:       "文案优化 / 系统提示词",
		Description: "SHEIN 标题与描述优化的系统提示词。",
		Scopes:      tenantOnlyScopes,
	},
	prompt.KSheinContentOptimizerOptimizeTitleDescriptionUser: {
		Label:       "文案优化 / 用户提示词",
		Description: "SHEIN 标题与描述优化的用户提示词模板。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("title", "description", "features"),
	},
	prompt.KSheinDisplayAttributeFieldSelection: {
		Label:       "展示属性 / 字段选择",
		Description: "SHEIN 展示属性字段选择提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("SourceAttribute", "SourceValue", "AdditionalContextBlock", "CandidatesBlock"),
	},
	prompt.KSheinDisplayAttributeValueMapping: {
		Label:       "展示属性 / 单值映射",
		Description: "SHEIN 展示属性单值映射提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("SourcesBlock", "AdditionalContextBlock", "CandidatesBlock"),
	},
	prompt.KSheinDisplayAttributeValueMappingBatch: {
		Label:       "展示属性 / 批量值映射",
		Description: "SHEIN 展示属性批量值映射提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("SourceAttribute", "SourceValue", "SourceSegmentsBlock", "AdditionalContextBlock", "TemplateAttribute", "CandidatesBlock"),
	},
	prompt.KSheinDisplayAttributeMissingText: {
		Label:       "展示属性 / 缺失文本补全",
		Description: "SHEIN 展示属性缺失文本补全提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("AdditionalContextBlock", "AttributeTasksBlock"),
	},
	prompt.KSheinDisplayAttributeMissingValue: {
		Label:       "展示属性 / 缺失值补全",
		Description: "SHEIN 展示属性缺失值补全提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("TemplateAttribute", "AttributeID", "AttributeType", "Required", "Important", "SourceAttributesBlock"),
	},
	prompt.KSheinDisplayAttributeFieldSelectionBatch: {
		Label:       "展示属性 / 批量字段选择",
		Description: "SHEIN 展示属性批量字段选择提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("TemplateAttribute", "AttributeID", "AttributeType", "Required", "Important", "SourceAttributesBlock", "CandidatesBlock"),
	},
	prompt.KSheinDisplayAttributeBatchInference: {
		Label:       "展示属性 / 批量推断",
		Description: "SHEIN 展示属性批量推断提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("SourceAttributesBlock", "AttributesBlock"),
	},
	prompt.KSheinDisplayAttributeRequiredRepair: {
		Label:       "展示属性 / 必填修复",
		Description: "SHEIN 展示属性必填项修复提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("TemplateAttribute", "AttributeID", "AttributeType", "SourceAttributesBlock", "CandidatesBlock"),
	},
	prompt.KSheinSaleAttributeMapping: {
		Label:       "销售属性 / 属性映射",
		Description: "SHEIN 销售属性映射提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("SourceDimensionsBlock"),
	},
	prompt.KSheinSaleAttributeSourceDimension: {
		Label:       "销售属性 / 来源维度识别",
		Description: "SHEIN 销售属性来源维度识别提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("SourceDimensionsBlock", "TemplatesBlock", "FeedbackBlock"),
	},
	prompt.KSheinSaleAttributeValueBatchMapping: {
		Label:       "销售属性 / 批量值映射",
		Description: "SHEIN 销售属性批量值映射提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("SourceDimension", "TemplateAttribute", "SourceValuesBlock", "CandidatesBlock"),
	},
	prompt.KSheinSaleAttributePromptValueExtraction: {
		Label:       "销售属性 / 值抽取",
		Description: "SHEIN 销售属性值抽取提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("AttributeName", "SourceDimension", "ProductTitle", "SourceValue"),
	},
	prompt.KSheinTranslationBatchOptimizeSystem: {
		Label:       "翻译优化 / 系统提示词",
		Description: "SHEIN 批量翻译优化系统提示词。",
		Scopes:      tenantOnlyScopes,
	},
	prompt.KTemuAttributeMappingSystem: {
		Label:       "属性映射 / 系统提示词",
		Description: "Temu 属性映射链路的系统提示词。",
		Scopes:      tenantOnlyScopes,
	},
	prompt.KTemuContentRewriterSystem: {
		Label:       "文案改写 / 系统提示词",
		Description: "Temu 文案改写链路的系统提示词。",
		Scopes:      tenantOnlyScopes,
	},
	prompt.KTemuSkuVariantMappingSystem: {
		Label:       "变体映射 / 系统提示词",
		Description: "Temu SKU 变体映射链路的系统提示词。",
		Scopes:      tenantOnlyScopes,
	},
	prompt.KTemuVisionDetectorDetect: {
		Label:       "视觉检测 / 系统提示词",
		Description: "Temu 视觉检测链路的系统提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("Base64Image"),
	},
	prompt.KProductEnrichLlmScorerTextScoring: {
		Label:       "文本评分",
		Description: "商品信息生成链路的文本质量评分提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("Text", "BaseScore"),
	},
	prompt.KProductEnrichLlmScorerImageScoring: {
		Label:       "图片评分",
		Description: "商品信息生成链路的图片质量评分提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("BaseScore"),
	},
	prompt.KProductEnrichUnderstandingAnalyzeImage: {
		Label:       "图片理解",
		Description: "商品图片理解提示词。",
		Scopes:      tenantOnlyScopes,
	},
	prompt.KProductEnrichUnderstandingExtractText: {
		Label:       "文本抽取",
		Description: "商品信息文本抽取提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("Text"),
	},
	prompt.KProductEnrichUnderstandingFuseMultimodal: {
		Label:       "多模态融合",
		Description: "商品多模态理解融合提示词。",
		Scopes:      tenantOnlyScopes,
	},
	prompt.KProductEnrichGenerationProductJSON: {
		Label:       "商品 JSON 生成",
		Description: "商品标准 JSON 结构生成提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("analysis_sections"),
	},
	prompt.KProductEnrichGenerationSpecs: {
		Label:       "规格生成",
		Description: "商品规格信息生成提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("analysis_sections"),
	},
	prompt.KProductEnrichGenerationVariants: {
		Label:       "变体生成",
		Description: "商品变体信息生成提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("analysis_sections"),
	},
	prompt.KProductEnrichGenerationExtractDimensions: {
		Label:       "尺寸抽取",
		Description: "商品尺寸信息抽取提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("text"),
	},
	prompt.KProductEnrichGenerationExtractWeight: {
		Label:       "重量抽取",
		Description: "商品重量信息抽取提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("text"),
	},
	prompt.KProductImageSubjectExtract: {
		Label:       "主体抽取",
		Description: "商品图主体抽取提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("product_type"),
	},
	prompt.KProductImageWhiteBackgroundDefault: {
		Label:       "白底图生成",
		Description: "商品白底图生成提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("product_type"),
	},
	prompt.KProductImageSceneDefault: {
		Label:       "场景图生成 / 默认",
		Description: "商品默认场景图生成提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("scene_intent", "product_type", "title", "scene_style", "background_tone", "composition", "props_level", "audience_hint", "custom_scene_hint"),
	},
	prompt.KProductImageSceneShoes: {
		Label:       "场景图生成 / 鞋类",
		Description: "鞋类商品场景图生成提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("scene_intent", "product_type", "title", "scene_style", "background_tone", "composition", "props_level", "audience_hint", "custom_scene_hint"),
	},
	prompt.KProductImageSceneJewelry: {
		Label:       "场景图生成 / 饰品",
		Description: "饰品商品场景图生成提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("scene_intent", "product_type", "title", "scene_style", "background_tone", "composition", "props_level", "audience_hint", "custom_scene_hint"),
	},
	prompt.KProductImageSceneBags: {
		Label:       "场景图生成 / 箱包",
		Description: "箱包商品场景图生成提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("scene_intent", "product_type", "title", "scene_style", "background_tone", "composition", "props_level", "audience_hint", "custom_scene_hint"),
	},
	prompt.KProductImageReviewDefault: {
		Label:       "图片复核",
		Description: "商品图片复核提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("summary_json"),
	},
	prompt.KProductImageStudioGenerationPodDesign: {
		Label:       "Studio 设计稿生成",
		Description: "Studio POD 设计稿生成提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("TransparentHint", "ReferenceHint", "PrintableHint", "ThemePrompt"),
	},
	prompt.KProductImageStudioGenerationAmazonProductImage: {
		Label:       "Studio Amazon 主图生成",
		Description: "Studio Amazon 商品主图生成提示词。",
		Scopes:      tenantOnlyScopes,
		Variables:   promptVars("ImageIndex", "ImageTotal", "ImageRoleLabel", "ImageGoal", "ImageComposition", "ProductNameLine", "CategoryLine", "StyleLine", "UserInstructionLine", "Prompt"),
	},
}

func promptVars(keys ...string) []TemplateVariableDefinition {
	variables := make([]TemplateVariableDefinition, 0, len(keys))
	for _, key := range keys {
		if variable, ok := promptVariableGlossary[key]; ok {
			variables = append(variables, variable)
			continue
		}
		variables = append(variables, TemplateVariableDefinition{Key: key, Label: humanizeToken(key)})
	}
	return variables
}
