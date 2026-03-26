package main

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestParseListFlag(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty",
			input: "",
			want:  []string{},
		},
		{
			name:  "comma separated",
			input: "alpha,beta,gamma",
			want:  []string{"alpha", "beta", "gamma"},
		},
		{
			name:  "trims whitespace and drops blanks",
			input: " alpha, beta ,, gamma ",
			want:  []string{"alpha", "beta", "gamma"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseListFlag(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseListFlag(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestResourcePath(t *testing.T) {
	got := resourcePath("tle", "starlink_500.tle")
	want := "./resources/tle/starlink_500.tle"
	if got != want {
		t.Fatalf("resourcePath returned %q, want %q", got, want)
	}
}

func TestResolveDataSourcePath(t *testing.T) {
	tests := []struct {
		name  string
		kind  string
		value string
		want  string
	}{
		{
			name:  "resource name",
			kind:  "tle",
			value: "starlink_500.tle",
			want:  "./resources/tle/starlink_500.tle",
		},
		{
			name:  "relative path",
			kind:  "tle",
			value: "./resources/tle/starlink_today.tle",
			want:  "./resources/tle/starlink_today.tle",
		},
		{
			name:  "absolute path",
			kind:  "tle",
			value: filepath.Join(string(filepath.Separator), "tmp", "starlink_today.tle"),
			want:  filepath.Join(string(filepath.Separator), "tmp", "starlink_today.tle"),
		},
		{
			name:  "remote url",
			kind:  "tle",
			value: "https://example.com/starlink.tle",
			want:  "https://example.com/starlink.tle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveDataSourcePath(tt.kind, tt.value)
			if got != tt.want {
				t.Fatalf("resolveDataSourcePath(%q, %q) = %q, want %q", tt.kind, tt.value, got, tt.want)
			}
		})
	}
}

func TestDefaultDownloadedTLEPath(t *testing.T) {
	got := defaultDownloadedTLEPath("starlink", time.Date(2026, time.March, 26, 16, 20, 4, 0, time.UTC))
	want := filepath.Join(".", "resources", "tle", "starlink_20260326_162004Z.tle")
	if got != want {
		t.Fatalf("defaultDownloadedTLEPath returned %q, want %q", got, want)
	}
}
