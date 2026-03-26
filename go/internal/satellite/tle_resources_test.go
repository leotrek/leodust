package satellite

import (
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

var countedTLEPattern = regexp.MustCompile(`^starlink_(\d+)\.tle$`)

func TestBundledTLEFilesSummarizeAndMatchExpectedCounts(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("..", "..", "resources", "tle", "*.tle"))
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected bundled TLE files to exist")
	}

	for _, path := range matches {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			summary, err := SummarizeTLEFile(path)
			if err != nil {
				t.Fatalf("SummarizeTLEFile returned error: %v", err)
			}
			if summary.Empty() {
				t.Fatal("expected bundled TLE file to contain at least one record")
			}
			if summary.EarliestEpoch.After(summary.LatestEpoch) {
				t.Fatalf("invalid epoch range: %s > %s", summary.EarliestEpoch, summary.LatestEpoch)
			}

			match := countedTLEPattern.FindStringSubmatch(filepath.Base(path))
			if match == nil {
				return
			}

			wantCount, err := strconv.Atoi(match[1])
			if err != nil {
				t.Fatalf("Atoi returned error: %v", err)
			}
			if summary.SatelliteCount != wantCount {
				t.Fatalf("satellite count = %d, want %d", summary.SatelliteCount, wantCount)
			}
		})
	}
}
