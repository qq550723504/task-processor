package ptr

// IntPtr 返回int指针
func IntPtr(i int) *int {
	return &i
}

// Float32Ptr 返回float32指针
func Float32Ptr(f float32) *float32 {
	return &f
}
