package main

import "testing"

func TestEncodeSandboxSpecs(t *testing.T) {
	encoded, err := encodeSandboxSpecs([]sandboxSpec{
		{SatelliteID: "STARLINK-1036", Role: "endpoint"},
		{SatelliteID: "STARLINK-11414 [DTC]", Role: "relay"},
	})
	if err != nil {
		t.Fatalf("encodeSandboxSpecs returned error: %v", err)
	}

	want := "STARLINK-1036|endpoint;STARLINK-11414 [DTC]|relay"
	if encoded != want {
		t.Fatalf("encodeSandboxSpecs returned %q, want %q", encoded, want)
	}
}

func TestEncodeSandboxSpecsRejectsReservedSeparators(t *testing.T) {
	_, err := encodeSandboxSpecs([]sandboxSpec{{SatelliteID: "bad|id", Role: "endpoint"}})
	if err == nil {
		t.Fatal("encodeSandboxSpecs should reject reserved separators")
	}
}
