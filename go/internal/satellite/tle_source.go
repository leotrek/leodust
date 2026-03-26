package satellite

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leotrek/leodust/pkg/logging"
)

const staleTLEWarningThreshold = 24 * time.Hour

// TLESummary captures the basic metadata needed to reason about a TLE snapshot.
type TLESummary struct {
	SatelliteCount int
	EarliestEpoch  time.Time
	LatestEpoch    time.Time
}

// Empty reports whether the summary contains any records.
func (s TLESummary) Empty() bool {
	return s.SatelliteCount == 0
}

// DistanceTo returns how far the provided timestamp sits outside the epoch range.
// If the timestamp is inside the range, the distance is zero.
func (s TLESummary) DistanceTo(at time.Time) time.Duration {
	if s.Empty() {
		return 0
	}

	at = at.UTC()
	if at.Before(s.EarliestEpoch) {
		return s.EarliestEpoch.Sub(at)
	}
	if at.After(s.LatestEpoch) {
		return at.Sub(s.LatestEpoch)
	}
	return 0
}

// DownloadResult describes one fetched TLE snapshot.
type DownloadResult struct {
	Group        string
	Path         string
	DownloadedAt time.Time
	Summary      TLESummary
}

// CurrentTLEURL returns the official CelesTrak endpoint for the requested group.
func CurrentTLEURL(group string) string {
	return fmt.Sprintf(
		"https://celestrak.org/NORAD/elements/gp.php?GROUP=%s&FORMAT=tle",
		url.QueryEscape(strings.ToLower(strings.TrimSpace(group))),
	)
}

// CurrentTLEFilename creates a timestamped filename for one downloaded TLE snapshot.
func CurrentTLEFilename(group string, downloadedAt time.Time) string {
	return fmt.Sprintf("%s_%s.tle", sanitizeTLEGroupName(group), downloadedAt.UTC().Format("20060102_150405Z"))
}

// DownloadCurrentTLE fetches the current CelesTrak snapshot for the group and stores it locally.
func DownloadCurrentTLE(client *http.Client, group, outputPath string, downloadedAt time.Time) (DownloadResult, error) {
	return downloadTLEFromURL(client, CurrentTLEURL(group), group, outputPath, downloadedAt)
}

func downloadTLEFromURL(client *http.Client, sourceURL, group, outputPath string, downloadedAt time.Time) (DownloadResult, error) {
	if client == nil {
		client = http.DefaultClient
	}

	response, err := client.Get(sourceURL)
	if err != nil {
		return DownloadResult{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return DownloadResult{}, fmt.Errorf("unexpected TLE download status: %s", response.Status)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return DownloadResult{}, err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(outputPath), "tle-download-*.tmp")
	if err != nil {
		return DownloadResult{}, err
	}
	tempPath := tempFile.Name()
	defer func() {
		tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	if _, err := io.Copy(tempFile, response.Body); err != nil {
		return DownloadResult{}, err
	}
	if err := tempFile.Close(); err != nil {
		return DownloadResult{}, err
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return DownloadResult{}, err
	}

	summary, err := SummarizeTLEFile(outputPath)
	if err != nil {
		return DownloadResult{}, err
	}

	return DownloadResult{
		Group:        group,
		Path:         outputPath,
		DownloadedAt: downloadedAt.UTC(),
		Summary:      summary,
	}, nil
}

// SummarizeTLEFile reads a local TLE file and returns its record count and epoch span.
func SummarizeTLEFile(path string) (TLESummary, error) {
	file, err := os.Open(path)
	if err != nil {
		return TLESummary{}, err
	}
	defer file.Close()

	return SummarizeTLE(file)
}

// SummarizeTLE scans a TLE stream without building satellite objects so the caller
// can reason about the age of the element set before running a simulation.
func SummarizeTLE(r io.Reader) (TLESummary, error) {
	scanner := bufio.NewScanner(r)
	summary := TLESummary{}

	for {
		record, err := readTLERecord(scanner.Scan, scanner.Text, scanner.Err)
		if err != nil {
			if err == io.EOF {
				break
			}
			return TLESummary{}, err
		}

		epoch, err := record.Epoch()
		if err != nil {
			return TLESummary{}, err
		}

		if summary.SatelliteCount == 0 || epoch.Before(summary.EarliestEpoch) {
			summary.EarliestEpoch = epoch
		}
		if summary.SatelliteCount == 0 || epoch.After(summary.LatestEpoch) {
			summary.LatestEpoch = epoch
		}
		summary.SatelliteCount++
	}

	return summary, nil
}

// LogTLESummary emits the epoch range of a TLE snapshot using concrete UTC timestamps.
func LogTLESummary(summary TLESummary, source string) {
	if summary.Empty() {
		logging.Warnf("TLE source %s contained no satellite records", source)
		return
	}

	logging.Infof(
		"TLE source %s contains %d records with epochs from %s to %s",
		source,
		summary.SatelliteCount,
		summary.EarliestEpoch.UTC().Format(time.RFC3339),
		summary.LatestEpoch.UTC().Format(time.RFC3339),
	)
}

// WarnIfSimulationTimeFarFromTLE emits a warning when the configured simulation time
// sits well outside the epoch range covered by the TLE snapshot.
func WarnIfSimulationTimeFarFromTLE(summary TLESummary, simulationTime time.Time, source string) {
	if summary.Empty() {
		return
	}

	distance := summary.DistanceTo(simulationTime)
	if distance <= staleTLEWarningThreshold {
		return
	}

	logging.Warnf(
		"SimulationStartTime %s is %s away from the TLE epoch range in %s (%s to %s). Orbit quality may be poor.",
		simulationTime.UTC().Format(time.RFC3339),
		distance.Round(time.Minute),
		source,
		summary.EarliestEpoch.UTC().Format(time.RFC3339),
		summary.LatestEpoch.UTC().Format(time.RFC3339),
	)
}

func sanitizeTLEGroupName(group string) string {
	group = strings.ToLower(strings.TrimSpace(group))
	if group == "" {
		return "tle"
	}

	var builder strings.Builder
	for _, ch := range group {
		switch {
		case ch >= 'a' && ch <= 'z':
			builder.WriteRune(ch)
		case ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
		default:
			builder.WriteByte('_')
		}
	}
	sanitized := strings.Trim(builder.String(), "_")
	if sanitized == "" {
		return "tle"
	}
	return sanitized
}
