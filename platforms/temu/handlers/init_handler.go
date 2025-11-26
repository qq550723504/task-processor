package handlers

import (
	"fmt"
	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"
	"time"

	"github.com/sirupsen/logrus"
)

// InitDataHandler 初始化数据处理器
type InitDataHandler struct {
	logger *logrus.Entry
}

// NewInitDataHandler 创建新的初始化数据处理器
func NewInitDataHandler() *InitDataHandler {
	return &InitDataHandler{
		logger: logrus.WithField("handler", "InitDataHandler"),
	}
}

// Name 返回处理器名称
func (h *InitDataHandler) Name() string {
	return "初始化数据处理器"
}

// Handle 处理任务
func (h *InitDataHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始执行初始化数据处理")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.Task.ProductID == "" {
		return fmt.Errorf("产品ID为空")
	}

	// 初始化TEMU产品结构体（使用types包中定义的结构体）
	temuProduct := &types.Product{
		CanSave: boolPtr(true),
		Extra: types.ExtraInfo{
			CreateEmptyGoods: true,
			Tab:              1,
			CurrentTab:       1,
		},
		GoodsBasic: types.GoodsBasicInfo{
			Lang:                   "en",
			AllowSite:              []int{100}, // 默认美国站点
			GoodsType:              1,
			IsOnSale:               0,
			Source:                 0,
			GoodsCreateTime:        time.Now().UnixMilli(),
			AgreeMaxRetailPrice:    true,
			NeedAccessoryInfo:      true,
			SupportCustomizedGoods: true,
		},
		GoodsSaleInfo: types.GoodsSaleInfo{
			GoodsPattern: 0,
		},
		GoodsServicePromise: types.ServicePromise{
			ShipmentLimitSecond: 2, // 24小时
			FulfillmentType:     1,
		},
		GoodsExtensionInfo: types.ExtensionInfo{
			GoodsDesc:    "",
			BulletPoints: make([]string, 0),
			CertificationInfo: types.CertificationInfo{
				ExtraTemplate: types.ExtraTemplate{
					ExtraTemplateDetailList: []types.ExtraTemplateDetail{
						{
							TemplateID: 1,
							Properties: map[string][]int{
								"1000000001": {1000100066},
							},
							InputText: map[string]interface{}{},
						},
					},
				},
			},
		},
		PlatformExpressBill:    boolPtr(false),
		SupportMaxRetailPrice:  boolPtr(true),
		ReplicateToRelateGoods: boolPtr(false),
		SkcList:                make([]types.Skc, 0),
	}

	// 使用强类型字段设置产品到上下文
	ctx.TemuProduct = temuProduct

	h.logger.Infof("初始化TEMU产品结构完成: ProductID=%s, Platform=%s",
		ctx.Task.ProductID, ctx.Task.Platform)
	return nil
}

// boolPtr 返回bool指针
func boolPtr(b bool) *bool {
	return &b
}
