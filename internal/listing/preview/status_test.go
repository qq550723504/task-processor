package preview

import "testing"

func TestPendingHeader(t *testing.T) {
	t.Parallel()

	header := PendingHeader("processing")
	if header == nil {
		t.Fatal("expected header")
	}
	if header.StatusMessage != "任务处理中，预览结果尚未准备完成" {
		t.Fatalf("status message = %q", header.StatusMessage)
	}
}

func TestStatusFromReviewReasons(t *testing.T) {
	t.Parallel()

	if got := StatusFromReviewReasons([]string{"missing image"}); got != "needs_review" {
		t.Fatalf("StatusFromReviewReasons(non-empty) = %q", got)
	}
	if got := StatusFromReviewReasons(nil); got != "ready" {
		t.Fatalf("StatusFromReviewReasons(nil) = %q", got)
	}
}

func TestStatusMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status string
		want   string
	}{
		{status: "pending", want: "任务已创建，预览结果尚未生成"},
		{status: "processing", want: "任务处理中，预览结果尚未准备完成"},
		{status: "needs_review", want: "任务已完成，等待人工审核"},
		{status: "failed", want: "任务执行失败，暂无预览结果"},
		{status: "unknown", want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.status, func(t *testing.T) {
			t.Parallel()
			if got := StatusMessage(tt.status); got != tt.want {
				t.Fatalf("StatusMessage(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
