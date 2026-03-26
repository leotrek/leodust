package simplugin

import (
	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

var _ types.SimulationPlugin = (*DummyPlugin)(nil)

type DummyPlugin struct {
}

func (p *DummyPlugin) Name() string {
	return "DummyPlugin"
}

func (p *DummyPlugin) PostSimulationStep(simulation types.SimulationController) error {
	logging.Debugf("DummyPlugin: PostSimulationStep called")
	logging.Debugf("Current Simulation Time: %s", simulation.GetSimulationTime())
	logging.Debugf("Number of Nodes: %d", len(simulation.GetAllNodes()))
	logging.Debugf("Number of Satellites: %d", len(simulation.GetSatellites()))
	logging.Debugf("Number of Ground Stations: %d", len(simulation.GetGroundStations()))
	return nil
}
