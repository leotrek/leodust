package orbit

import "testing"

func TestValidatePublishedReferenceSuite(t *testing.T) {
	results, err := ValidatePublishedReferenceSuite()
	if err != nil {
		t.Fatalf("ValidatePublishedReferenceSuite returned error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one validation result")
	}

	for _, result := range results {
		if !result.Passed() {
			t.Fatalf("expected reference case %q to pass", result.Case.Name)
		}
		if result.MaxPositionErrorKM <= 0 {
			t.Fatalf("expected non-zero position error for %q due to whole-second truncation", result.Case.Name)
		}
	}
}
