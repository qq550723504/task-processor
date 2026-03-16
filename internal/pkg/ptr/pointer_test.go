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

func TestStringPtr(t *testing.T) {
	val := "test"
	ptr := StringPtr(val)
	if ptr == nil {
		t.Error("StringPtr returned nil")
	}
	if *ptr != val {
		t.Errorf("StringPtr(%v) = %v, want %v", val, *ptr, val)
	}
}

func TestFloat64Ptr(t *testing.T) {
	val := 3.14
	ptr := Float64Ptr(val)
	if ptr == nil {
		t.Error("Float64Ptr returned nil")
	}
	if *ptr != val {
		t.Errorf("Float64Ptr(%v) = %v, want %v", val, *ptr, val)
	}
}

func TestBoolPtr(t *testing.T) {
	val := true
	ptr := BoolPtr(val)
	if ptr == nil {
		t.Error("BoolPtr returned nil")
	}
	if *ptr != val {
		t.Errorf("BoolPtr(%v) = %v, want %v", val, *ptr, val)
	}
}
