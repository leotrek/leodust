package ground

import (
	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

// GroundStationLoaderService is responsible for loading ground station configurations
// from a specified data source and injecting them into the simulation controller.
type GroundStationLoaderService struct {
	controller              types.SimulationController
	groundStationLoader     *GroundStationYmlLoader
	groundStationDataSource string
}

// NewGroundStationLoaderService initializes a new GroundStationLoaderService.
func NewGroundStationLoaderService(
	controller types.SimulationController,
	groundStationLoader *GroundStationYmlLoader,
	dataSourcePath string,
) *GroundStationLoaderService {
	return &GroundStationLoaderService{
		controller:              controller,
		groundStationLoader:     groundStationLoader,
		groundStationDataSource: dataSourcePath,
	}
}

// Start loads ground station configurations from the data source, converts them to Node types,
// and injects them into the simulation controller.
// Returns an error if the loading or injection process fails.
func (s *GroundStationLoaderService) Start() error {
	logging.Infof("Starting ground-station loader service")
	groundStations, err := s.groundStationLoader.Load(s.groundStationDataSource, s.controller.GetSatellites())
	if err != nil {
		return err
	}
	return s.controller.InjectGroundStations(types.AsNodes(groundStations))
}
