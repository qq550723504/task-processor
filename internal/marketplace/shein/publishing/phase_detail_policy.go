package publishing

import listingsubmission "task-processor/internal/listing/submission"

func SubmissionPhaseDetail(action, phase string) string {
	return listingsubmission.PhaseDetail(action, phase, submissionPhaseDetailLabels)
}

var submissionPhaseDetailLabels = listingsubmission.PhaseDetailLabels{
	Validate:        "检查 SHEIN 提交前状态",
	PrepareProduct:  "准备 SHEIN 商品载荷",
	UploadImages:    "上传 SHEIN 商品图片",
	PreValidate:     "执行 SHEIN 提交前校验",
	SubmitRemote:    "提交 SHEIN 发布请求",
	SaveDraftRemote: "提交 SHEIN 草稿",
	PersistResult:   "保存本地提交结果",
	ConfirmRemote:   "刷新 SHEIN 远端诊断状态",
}
