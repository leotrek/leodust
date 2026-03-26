package satellite

import (
	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

// SatelliteLoaderService wires the constellation loader and triggers simulation startup.
type SatelliteLoaderService struct {
	controller            types.SimulationController
	constellationLoader   *SatelliteConstellationLoader
	satelliteDataSource   string
	satelliteSourceFormat string
}

// NewSatelliteLoaderService wires a configured constellation loader to the simulation controller.
func NewSatelliteLoaderService(
	loader *SatelliteConstellationLoader,
	controller types.SimulationController,
	dataSourcePath string,
	sourceFormat string,
) *SatelliteLoaderService {
	return &SatelliteLoaderService{
		controller:            controller,
		constellationLoader:   loader,
		satelliteDataSource:   dataSourcePath,
		satelliteSourceFormat: sourceFormat,
	}
}

// Start loads satellites and injects them into the simulation
func (s *SatelliteLoaderService) Start() error {
	logging.Infof("Starting satellite loader service")
	satellites, err := s.constellationLoader.LoadSatelliteConstellation(s.satelliteDataSource, s.satelliteSourceFormat)
	if err != nil {
		return err
	}
	return s.controller.InjectSatellites(types.AsNodes(satellites))
}
