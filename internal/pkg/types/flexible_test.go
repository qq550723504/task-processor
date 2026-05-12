package types

import (
	"testing"
	"time"
)

func TestFlexibleTimeScanTime(t *testing.T) {
	want := time.Date(2026, 1, 6, 15, 25, 14, 0, time.FixedZone("CST", 8*3600))
	var got FlexibleTime
	if err := got.Scan(want); err != nil {
		t.Fatalf("Scan(time.Time) error = %v", err)
	}
	if !got.Time.Equal(want) {
		t.Fatalf("Scan(time.Time) = %v, want %v", got.Time, want)
	}
}

func TestFlexibleTimeScanString(t *testing.T) {
	var got FlexibleTime
	if err := got.Scan("2026-01-06 15:25:14"); err != nil {
		t.Fatalf("Scan(string) error = %v", err)
	}
	if got.Time.IsZero() {
		t.Fatal("Scan(string) produced zero time")
	}
}

func TestFlexibleTimeValue(t *testing.T) {
	want := time.Date(2026, 1, 6, 15, 25, 14, 0, time.UTC)
	got, err := (FlexibleTime{Time: want}).Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	gotTime, ok := got.(time.Time)
	if !ok {
		t.Fatalf("Value() type = %T, want time.Time", got)
	}
	if !gotTime.Equal(want) {
		t.Fatalf("Value() = %v, want %v", gotTime, want)
	}
}
