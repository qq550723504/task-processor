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

func TestFinalizeZipcodeVerificationResult(t *testing.T) {
	if valid, err := finalizeZipcodeVerificationResult(true, assertionErr("transient")); valid || err != nil {
		t.Fatal("只要观测到明确 mismatch，就应返回 false 且不保留临时错误")
	}

	if valid, err := finalizeZipcodeVerificationResult(false, assertionErr("getter failed")); valid || err == nil {
		t.Fatal("只有临时读取错误时，应把错误向上返回")
	}

	if valid, err := finalizeZipcodeVerificationResult(false, nil); valid || err != nil {
		t.Fatal("没有命中且没有错误时，应返回 false,nil")
	}
}

type assertionErr string

func (e assertionErr) Error() string {
	return string(e)
}
