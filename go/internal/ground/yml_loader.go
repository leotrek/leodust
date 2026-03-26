package ground

import (
	"os"

	"github.com/leotrek/leodust/pkg/types"
	"gopkg.in/yaml.v3"
)

type rawGroundStation struct {
	Name          string  `yaml:"Name"`
	Lat           float64 `yaml:"Lat"`
	Lon           float64 `yaml:"Lon"`
	Alt           float64 `yaml:"Alt"`
	Protocol      string  `yaml:"Protocol"`
	ComputingType string  `yaml:"ComputingType"`
}

// GroundStationYmlLoader is responsible for loading ground station configurations from a YAML file.
type GroundStationYmlLoader struct {
	groundStationBuilder *GroundStationBuilder
}

// NewGroundStationYmlLoader initializes a new GroundStationYmlLoader.
func NewGroundStationYmlLoader(builder *GroundStationBuilder) *GroundStationYmlLoader {
	return &GroundStationYmlLoader{
		groundStationBuilder: builder,
	}
}

// Load reads a YAML file from the specified path and parses its content to build the ground stations slice.
func (l *GroundStationYmlLoader) Load(path string, satellites []types.Satellite) ([]types.GroundStation, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var groundStations []rawGroundStation
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&groundStations); err != nil {
		return nil, err
	}

	var result []types.GroundStation
	for _, gs := range groundStations {
		station, err := l.groundStationBuilder.Build(GroundStationSpec{
			Name:          gs.Name,
			Latitude:      gs.Lat,
			Longitude:     gs.Lon,
			Altitude:      gs.Alt,
			Protocol:      gs.Protocol,
			ComputingType: gs.ComputingType,
		}, satellites)
		if err != nil {
			return nil, err
		}
		result = append(result, station)
	}

	return result, nil
}
