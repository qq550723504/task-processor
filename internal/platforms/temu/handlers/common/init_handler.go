package common

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	"task-processor/internal/platforms/temu/api/models"
	temucontext "task-processor/internal/platforms/temu/context"
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
		logger: logger.GetGlobalLogger("temu.handlers.init").WithField("handler", "InitDataHandler"),
	}
}

// Name 返回处理器名称
func (h *InitDataHandler) Name() string {
	return "初始化数据处理器"
}

func (h *InitDataHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始执行初始化数据处理")

	// 直接使用强类型上下文，无需类型断言！
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if task.ProductID == "" {
		return fmt.Errorf("产品ID为空")
	}

	// 初始化TEMU产品结构体
	temuProduct := &models.Product{
		CanSave: boolPtr(true),
		Extra: models.ExtraInfo{
			CreateEmptyGoods: true,
			Tab:              1,
			CurrentTab:       1,
		},
		GoodsBasic: models.GoodsBasicInfo{
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
		GoodsSaleInfo: models.GoodsSaleInfo{
			GoodsPattern: 0,
		},
		GoodsServicePromise: models.ServicePromise{
			ShipmentLimitSecond: 2, // 24小时
			FulfillmentType:     1,
		},
		GoodsExtensionInfo: models.ExtensionInfo{
			GoodsDesc:    "",
			BulletPoints: make([]string, 0),
			CertificationInfo: models.CertificationInfo{
				ExtraTemplate: models.ExtraTemplate{
					ExtraTemplateDetailList: []models.ExtraTemplateDetail{
						{
							TemplateID: 1,
							Properties: map[string][]int{
								"1000000001": {1000100066},
							},
							InputText: map[string]any{},
						},
					},
				},
			},
		},
		PlatformExpressBill:    boolPtr(false),
		SupportMaxRetailPrice:  boolPtr(true),
		ReplicateToRelateGoods: boolPtr(false),
		SkcList:                make([]models.Skc, 0),
	}

	// 直接赋值到强类型字段（这就是你想要的方式！）
	temuCtx.TemuProduct = temuProduct

	// 也可以同时保持兼容性，设置到通用数据存储（可选）
	temuCtx.SetData("temu_product", temuProduct)

	h.logger.WithFields(map[string]interface{}{
		logger.FieldProductID: task.ProductID,
		logger.FieldPlatform:  task.Platform,
	}).Info("初始化TEMU产品结构完成")
	return nil
}

// Handle 兼容原有的Handler接口（用于pipeline.AddHandler）
func (h *InitDataHandler) Handle(ctx pipeline.TaskContext) error {
	// 尝试类型断言为TemuTaskContext
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return h.HandleTemu(temuCtx)
	}
	// 如果不是TemuTaskContext，返回错误
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}

// boolPtr 返回bool指针
func boolPtr(b bool) *bool {
	return &b
}
