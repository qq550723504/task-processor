package ptr

// IntPtr 返回int指针
func IntPtr(i int) *int {
	return &i
}

// Int16Ptr 返回int16指针
func Int16Ptr(i int16) *int16 {
	return &i
}

// Int32Ptr 返回int32指针
func Int32Ptr(i int32) *int32 {
	return &i
}

// Int64Ptr 返回int64指针
func Int64Ptr(i int64) *int64 {
	return &i
}

// StringPtr 返回string指针
func StringPtr(s string) *string {
	return &s
}

// Float32Ptr 返回float32指针
func Float32Ptr(f float32) *float32 {
	return &f
}

// Float64Ptr 返回float64指针
func Float64Ptr(f float64) *float64 {
	return &f
}

// BoolPtr 返回bool指针
func BoolPtr(b bool) *bool {
	return &b
}
