package satellite

import (
	"io"

	"bufio"
	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

const (
	errCannotParse = "cannot parse tle data source"
)

// TleLoader reads and parses satellites from a TLE (Two-Line Element) data source.
type TleLoader struct {
	satelliteBuilder *SatelliteBuilder
}

// NewTleLoader creates a new TleLoader instance.
func NewTleLoader(builder *SatelliteBuilder) *TleLoader {
	return &TleLoader{
		satelliteBuilder: builder,
	}
}

// Load parses the TLE stream into Satellite instances.
func (l *TleLoader) Load(r io.Reader) ([]types.Satellite, error) {
	scanner := bufio.NewScanner(r)
	var satellites []types.Satellite

	for {
		record, err := readTLERecord(scanner.Scan, scanner.Text, scanner.Err)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		sat, err := l.satelliteBuilder.Build(record)
		if err != nil {
			return nil, err
		}
		satellites = append(satellites, sat)
	}

	logging.Infof("Parsed %d satellites from TLE", len(satellites))
	return satellites, nil
}

// Ensure TleLoader implements SatelliteDataSourceLoader interface
var _ SatelliteDataSourceLoader = (*TleLoader)(nil)
