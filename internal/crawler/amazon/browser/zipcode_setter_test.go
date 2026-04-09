package browser

import "testing"

func TestShouldRefreshAfterValidationFailure(t *testing.T) {
	zs := NewZipcodeSetter(nil)

	if !zs.shouldRefreshAfterValidationFailure(assertionErr("temporary getter failure"), "") {
		t.Fatal("验证报错时应触发下一轮刷新")
	}

	if zs.shouldRefreshAfterValidationFailure(nil, "10001") {
		t.Fatal("已经拿到稳定 mismatch 时不应立刻刷新")
	}

	if !zs.shouldRefreshAfterValidationFailure(nil, "") {
		t.Fatal("没有稳定 mismatch 且验证失败时，应允许下一轮刷新")
	}
}

type assertionErr string

func (e assertionErr) Error() string {
	return string(e)
}
