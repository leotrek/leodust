package satellite

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTLEEpoch(t *testing.T) {
	got, err := ParseTLEEpoch(validLine1)
	if err != nil {
		t.Fatalf("ParseTLEEpoch returned error: %v", err)
	}

	want := time.Date(2026, time.March, 11, 12, 34, 59, 699712000, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("ParseTLEEpoch returned %s, want %s", got.Format(time.RFC3339Nano), want.Format(time.RFC3339Nano))
	}
}

func TestSummarizeTLE(t *testing.T) {
	summary, err := SummarizeTLE(strings.NewReader(strings.Join([]string{
		"STARLINK-3566",
		validLine1,
		validLine2,
		"STARLINK-2141",
		"1 47731U 21017K   26070.50003471  .00161105  00000+0  29333-2 0  9993",
		"2 47731  53.1598 195.2575 0000658  88.3638 320.8790 15.48439599  5847",
	}, "\n")))
	if err != nil {
		t.Fatalf("SummarizeTLE returned error: %v", err)
	}
	if summary.SatelliteCount != 2 {
		t.Fatalf("expected 2 records, got %d", summary.SatelliteCount)
	}

	wantEarliest := time.Date(2026, time.March, 11, 12, 0, 3, 0, time.UTC)
	wantLatest := time.Date(2026, time.March, 11, 12, 34, 59, 699712000, time.UTC)
	if diff := summary.EarliestEpoch.Sub(wantEarliest); diff < -2*time.Millisecond || diff > 2*time.Millisecond {
		t.Fatalf("earliest epoch = %s, want %s", summary.EarliestEpoch.Format(time.RFC3339Nano), wantEarliest.Format(time.RFC3339Nano))
	}
	if !summary.LatestEpoch.Equal(wantLatest) {
		t.Fatalf("latest epoch = %s, want %s", summary.LatestEpoch.Format(time.RFC3339Nano), wantLatest.Format(time.RFC3339Nano))
	}
}

func TestCurrentTLEFilename(t *testing.T) {
	got := CurrentTLEFilename("Starlink", time.Date(2026, time.March, 26, 16, 20, 4, 0, time.UTC))
	want := "starlink_20260326_162004Z.tle"
	if got != want {
		t.Fatalf("CurrentTLEFilename returned %q, want %q", got, want)
	}
}

func TestDownloadCurrentTLE(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("STARLINK-3566\n" + validLine1 + "\n" + validLine2 + "\n")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	downloadedAt := time.Date(2026, time.March, 26, 16, 20, 4, 0, time.UTC)
	outputPath := filepath.Join(t.TempDir(), "starlink_today.tle")

	result, err := downloadTLEFromURL(client, "https://example.test/starlink.tle", "starlink", outputPath, downloadedAt)
	if err != nil {
		t.Fatalf("downloadTLEFromURL returned error: %v", err)
	}
	if result.Path != outputPath {
		t.Fatalf("download path = %q, want %q", result.Path, outputPath)
	}
	if result.Group != "starlink" {
		t.Fatalf("download group = %q, want starlink", result.Group)
	}
	if result.Summary.SatelliteCount != 1 {
		t.Fatalf("expected 1 satellite in summary, got %d", result.Summary.SatelliteCount)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
