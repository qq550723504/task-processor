package ptr

import "testing"

func TestIntPtr(t *testing.T) {
	val := 42
	ptr := IntPtr(val)
	if ptr == nil {
		t.Error("IntPtr returned nil")
	}
	if *ptr != val {
		t.Errorf("IntPtr(%v) = %v, want %v", val, *ptr, val)
	}
}
