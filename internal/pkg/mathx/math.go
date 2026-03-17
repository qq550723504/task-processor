package mathx

// Abs 返回浮点数的绝对值
func Abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// AbsInt 返回整数的绝对值
func AbsInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// AbsInt64 返回int64的绝对值
func AbsInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// Min 返回两个整数中的较小值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max 返回两个整数中的较大值
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// MinInt64 返回两个int64中的较小值
func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// MaxInt64 返回两个int64中的较大值
func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
