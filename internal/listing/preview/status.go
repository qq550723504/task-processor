package preview

// PendingHeader builds the minimal preview header shown before a full preview
// result is available.
func PendingHeader(status string) *Header {
	return &Header{
		StatusMessage: StatusMessage(status),
	}
}

func StatusFromReviewReasons(reviewReasons []string) string {
	if len(reviewReasons) > 0 {
		return "needs_review"
	}
	return "ready"
}

func StatusMessage(status string) string {
	switch status {
	case "pending":
		return "任务已创建，预览结果尚未生成"
	case "processing":
		return "任务处理中，预览结果尚未准备完成"
	case "needs_review":
		return "任务已完成，等待人工审核"
	case "failed":
		return "任务执行失败，暂无预览结果"
	default:
		return ""
	}
}
