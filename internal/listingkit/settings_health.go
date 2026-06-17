package listingkit

import "strings"

type SettingsHealthInputs struct {
	DefaultAI             *AIClientSettings
	ImageAI               *AIClientSettings
	Shein                 *SheinSettings
	SDSProbeConfigured    bool
	ObjectStoreConfigured bool
	Probes                SettingsHealthProbes
}

type SettingsHealthProbes struct {
	SheinIntegration SettingsHealthProbe
	SDSLogin         SettingsHealthProbe
	ObjectStorage    SettingsHealthProbe
}

type SettingsHealthProbe struct {
	Configured bool
	Missing    []string
}

type SettingsHealthPage struct {
	Status string               `json:"status"`
	Items  []SettingsHealthItem `json:"items"`
}

type SettingsHealthItem struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Impact  []string `json:"impact,omitempty"`
	Action  string   `json:"action,omitempty"`
}

func BuildSettingsHealth(inputs SettingsHealthInputs) SettingsHealthPage {
	items := []SettingsHealthItem{
		aiHealthItem("ai.default", "AI 文案模型", inputs.DefaultAI, []string{"生成 ListingKit 草稿", "标题/卖点/属性推理"}),
		aiHealthItem("ai.image", "AI 图片模型", inputs.ImageAI, []string{"图片生成与重绘", "SHEIN 图片补齐"}),
		sheinAccountHealthItem(inputs.Shein),
		runtimeProbeHealthItem("shein.integration", "SHEIN Token / 权限 / 类目接口", inputs.Probes.SheinIntegration, []string{"保存草稿和发布", "SHEIN 类目接口校验"}, "补齐 SHEIN loginService、Cookie Redis 或店铺 API 客户端配置，并确认当前店铺具备类目与提交流程权限。"),
		sheinPricingHealthItem(inputs.Shein),
		runtimeProbeHealthItem("sds.session", "SDS 登录态", mergeLegacyProbe(inputs.Probes.SDSLogin, inputs.SDSProbeConfigured), []string{"SDS 属性补全", "SDS 商品库和 Studio"}, "接入 SDS 登录态探针或在环境配置中补齐 SDS loginService。"),
		runtimeProbeHealthItem("storage.object", "对象存储", mergeLegacyProbe(inputs.Probes.ObjectStorage, inputs.ObjectStoreConfigured), []string{"图片上传", "任务素材预览"}, "接入对象存储探针或在环境配置中补齐 bucket/endpoint/credentials。"),
	}
	return SettingsHealthPage{
		Status: overallSettingsHealthStatus(items),
		Items:  items,
	}
}

func aiHealthItem(key string, label string, settings *AIClientSettings, impact []string) SettingsHealthItem {
	missing := make([]string, 0, 4)
	if settings == nil {
		missing = append(missing, "配置不存在")
	} else {
		if !settings.Enabled {
			missing = append(missing, "未启用")
		}
		if strings.TrimSpace(settings.BaseURL) == "" {
			missing = append(missing, "endpoint 缺失")
		}
		if strings.TrimSpace(settings.Model) == "" {
			missing = append(missing, "model 缺失")
		}
		if !settings.APIKeySet && strings.TrimSpace(settings.APIKey) == "" {
			missing = append(missing, "api key 缺失")
		}
	}
	if len(missing) == 0 {
		return SettingsHealthItem{
			Key:     key,
			Label:   label,
			Status:  "ready",
			Message: "配置完整，可供任务运行时读取。",
			Impact:  impact,
		}
	}
	return SettingsHealthItem{
		Key:     key,
		Label:   label,
		Status:  "blocked",
		Message: strings.Join(missing, "、"),
		Impact:  impact,
		Action:  "在 ListingKit 设置页补齐 endpoint、model、api key 并启用该客户端。",
	}
}

func sheinAccountHealthItem(settings *SheinSettings) SettingsHealthItem {
	missing := make([]string, 0, 4)
	if settings == nil {
		missing = append(missing, "配置不存在")
	} else {
		if settings.DefaultStoreID <= 0 {
			missing = append(missing, "默认店铺缺失")
		}
		if strings.TrimSpace(settings.Site) == "" {
			missing = append(missing, "站点缺失")
		}
		if settings.DefaultStock <= 0 {
			missing = append(missing, "默认库存缺失")
		}
		mode := strings.ToLower(strings.TrimSpace(settings.DefaultSubmitMode))
		if mode != "publish" && mode != "save_draft" {
			missing = append(missing, "提交方式无效")
		}
	}
	if len(missing) == 0 {
		return SettingsHealthItem{
			Key:     "shein.account",
			Label:   "SHEIN 店铺与提交配置",
			Status:  "ready",
			Message: "默认店铺、站点、库存和提交方式已配置。",
			Impact:  []string{"SHEIN 提交", "新任务预检"},
		}
	}
	return SettingsHealthItem{
		Key:     "shein.account",
		Label:   "SHEIN 店铺与提交配置",
		Status:  "blocked",
		Message: strings.Join(missing, "、"),
		Impact:  []string{"SHEIN 提交", "新任务预检"},
		Action:  "在 SHEIN 配置中选择默认店铺、站点、库存和提交方式。",
	}
}

func sheinPricingHealthItem(settings *SheinSettings) SettingsHealthItem {
	missing := make([]string, 0, 3)
	if settings == nil {
		missing = append(missing, "配置不存在")
	} else {
		rule := settings.Pricing
		if strings.TrimSpace(rule.TargetCurrency) == "" {
			missing = append(missing, "目标币种缺失")
		}
		if rule.ExchangeRate <= 0 {
			missing = append(missing, "汇率无效")
		}
		if rule.MarkupMultiplier <= 0 {
			missing = append(missing, "加价倍率无效")
		}
	}
	if len(missing) == 0 {
		return SettingsHealthItem{
			Key:     "shein.pricing",
			Label:   "SHEIN 价格规则",
			Status:  "ready",
			Message: "价格规则已配置，可生成提交前价格预览。",
			Impact:  []string{"价格预览", "SHEIN 提交"},
		}
	}
	return SettingsHealthItem{
		Key:     "shein.pricing",
		Label:   "SHEIN 价格规则",
		Status:  "blocked",
		Message: strings.Join(missing, "、"),
		Impact:  []string{"价格预览", "SHEIN 提交"},
		Action:  "在 SHEIN 配置中补齐目标币种、汇率和加价倍率。",
	}
}

func mergeLegacyProbe(probe SettingsHealthProbe, configured bool) SettingsHealthProbe {
	if probe.Configured || len(probe.Missing) > 0 || !configured {
		return probe
	}
	probe.Configured = true
	return probe
}

func runtimeProbeHealthItem(key string, label string, probe SettingsHealthProbe, impact []string, action string) SettingsHealthItem {
	if len(probe.Missing) > 0 {
		return SettingsHealthItem{
			Key:     key,
			Label:   label,
			Status:  "blocked",
			Message: strings.Join(probe.Missing, "、"),
			Impact:  impact,
			Action:  action,
		}
	}
	if probe.Configured {
		return SettingsHealthItem{
			Key:     key,
			Label:   label,
			Status:  "ready",
			Message: "运行时探针已接入。",
			Impact:  impact,
		}
	}
	return SettingsHealthItem{
		Key:     key,
		Label:   label,
		Status:  "unknown",
		Message: "当前设置服务尚未接入该运行时探针，无法确认配置是否可用。",
		Impact:  impact,
		Action:  action,
	}
}

func overallSettingsHealthStatus(items []SettingsHealthItem) string {
	status := "ready"
	for _, item := range items {
		switch item.Status {
		case "blocked":
			return "blocked"
		case "warning", "unknown":
			status = "warning"
		}
	}
	return status
}
