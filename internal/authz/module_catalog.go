package authz

import "slices"

type MenuModule struct {
	ID          string   `json:"id"`
	Group       string   `json:"group"`
	Label       string   `json:"label"`
	Available   bool     `json:"available"`
	Permissions []string `json:"permissions"`
}

var enterpriseModules = []MenuModule{
	{"goals", "运营驾驶舱", "目标管理", false, nil},
	{"overview-stores", "运营驾驶舱", "店铺矩阵", false, nil},
	{"alerts", "运营驾驶舱", "经营预警", false, nil},
	{"advice", "运营驾驶舱", "经营建议", false, nil},
	{"chat", "AI工作台", "硕米Chat", true, []string{PermissionWorkbenchChatRead, PermissionWorkbenchChatUse, PermissionWorkbenchTaskRead}},
	{"tasks", "AI工作台", "任务中心", true, []string{PermissionWorkbenchTaskRead}},
	{"projects", "AI工作台", "项目中心", false, nil},
	{"knowledge", "AI工作台", "知识库", true, []string{PermissionWorkbenchKnowledgeRead, PermissionWorkbenchKnowledgeManage}},
	{"reports", "AI工作台", "我的报告", false, nil},
	{"acquisition", "供应市场", "1688采集", true, []string{PermissionProductSourcingWrite, PermissionLocalAgentWrite, PermissionWorkbenchAgentRead, PermissionWorkbenchAgentUse, PermissionWorkbenchTaskRead}},
	{"supply-official", "供应市场", "硕米自营", false, nil},
	{"supply-selected", "供应市场", "硕米优选", false, nil},
	{"supply-catalogs", "供应市场", "货盘集成", false, nil},
	{"supply-mine", "供应市场", "我的供应链", false, nil},
	{"agent-market", "智能市场", "智能体市场", true, []string{PermissionWorkbenchAgentRead}},
	{"agents", "智能市场", "我的智能体", true, []string{PermissionLocalAgentWrite, PermissionWorkbenchAgentRead, PermissionWorkbenchAgentUse, PermissionWorkbenchTaskRead}},
	{"agent-custom", "智能市场", "智能体定制", false, nil},
	{"images", "工具市场", "商品图片", true, []string{PermissionImageAgentRead, PermissionImageAgentWrite}},
	{"tools", "工具市场", "我的工具", false, nil},
	{"tools-custom", "工具市场", "工具定制", false, nil},
	{"services", "生态服务", "服务市场", true, []string{PermissionWorkbenchEcoservicesRead}},
	{"services-mine", "生态服务", "我的服务", true, []string{PermissionWorkbenchEcoservicesRead, PermissionWorkbenchEcoservicesPurchase, PermissionWorkbenchEcoservicesManage}},
	{"services-join", "生态服务", "申请加入", true, []string{PermissionWorkbenchEcoservicesJoin, PermissionWorkbenchEcoservicesManage}},
	{"data-market", "数据服务", "数据市场", false, nil},
	{"data-api", "数据服务", "API管理", false, nil},
	{"data-mine", "数据服务", "我的数据", false, nil},
	{"stores", "店铺中心", "我的店铺", true, []string{PermissionWorkbenchStoreRead, PermissionWorkbenchStoreCreate, PermissionWorkbenchStoreUpdate, PermissionWorkbenchStoreLifecycle}},
	{"store-products", "店铺中心", "店铺商品", false, nil},
	{"store-orders", "店铺中心", "订单履约", false, nil},
	{"plans", "套餐与权益", "套餐与权益（查看）", true, []string{PermissionWorkbenchCommercialRead}},
	{"members", "我的账户", "成员与权限（查看）", true, []string{PermissionWorkbenchOrganizationMemberRead}},
	{"source-accounts", "我的账户", "源账号", true, []string{PermissionWorkbenchSourceAccountRead, PermissionWorkbenchSourceAccountManage}},
}

func MenuModules() []MenuModule {
	result := make([]MenuModule, len(enterpriseModules))
	for i, module := range enterpriseModules {
		result[i] = module
		result[i].Permissions = append([]string{}, module.Permissions...)
	}
	return result
}

func ValidModuleIDs(ids []string) bool {
	if len(ids) > 32 {
		return false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		found := false
		for _, module := range enterpriseModules {
			if module.ID == id && module.Available {
				found = true
				break
			}
		}
		if !found || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func ModulePermissions(ids []string) []string {
	result := []string{}
	for _, module := range enterpriseModules {
		if module.Available && slices.Contains(ids, module.ID) {
			for _, p := range module.Permissions {
				if !slices.Contains(result, p) {
					result = append(result, p)
				}
			}
		}
	}
	return result
}
