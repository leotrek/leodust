package logging

import "testing"

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Level
		wantErr bool
	}{
		{name: "default", input: "", want: InfoLevel},
		{name: "info", input: "info", want: InfoLevel},
		{name: "warn", input: "warn", want: WarnLevel},
		{name: "warning", input: "warning", want: WarnLevel},
		{name: "error", input: "error", want: ErrorLevel},
		{name: "debug", input: "debug", want: DebugLevel},
		{name: "invalid", input: "trace", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLevel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected ParseLevel to return an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseLevel returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestEnabled(t *testing.T) {
	if err := Configure("warn"); err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := Configure("info"); err != nil {
			t.Fatalf("cleanup Configure returned error: %v", err)
		}
	})

	if Enabled(InfoLevel) {
		t.Fatal("info logs should be disabled at warn level")
	}
	if !Enabled(ErrorLevel) || !Enabled(WarnLevel) {
		t.Fatal("error and warn logs should be enabled at warn level")
	}
}
