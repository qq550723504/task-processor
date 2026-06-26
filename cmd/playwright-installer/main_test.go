package main

import (
	"reflect"
	"testing"
)

func TestBrowsersFromEnvDefaultsToChromium(t *testing.T) {
	got := browsersFromEnv(func(string) string { return "" })
	want := []string{"chromium"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("browsersFromEnv() = %#v, want %#v", got, want)
	}
}

func TestBrowsersFromEnvSplitsCommonSeparators(t *testing.T) {
	got := browsersFromEnv(func(string) string { return "chromium, firefox;webkit" })
	want := []string{"chromium", "firefox", "webkit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("browsersFromEnv() = %#v, want %#v", got, want)
	}
}
