package simulation

import (
	"sync"
	"time"

	"github.com/leotrek/leodust/configs"
	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

var _ types.SimulationController = (*SimulationService)(nil)

// SimulationService handles simulation lifecycle and state updates
type SimulationService struct {
	BaseSimulationService

	simplugins      []types.SimulationPlugin
	statePluginRepo *types.StatePluginRepository
	running         bool

	simulationStateSerializer *SimulationStateSerializer
}

// NewSimulationService initializes the simulation service
func NewSimulationService(
	config *configs.SimulationConfig,
	simplugins []types.SimulationPlugin,
	statePluginRepo *types.StatePluginRepository,
	simulationStateOutputFile *string,
) *SimulationService {
	simService := &SimulationService{
		simplugins:      simplugins,
		statePluginRepo: statePluginRepo,
	}
	simService.BaseSimulationService = NewBaseSimulationService(config, simService.runSimulationStep)

	if *simulationStateOutputFile != "" {
		simService.simulationStateSerializer = NewSimulationStateSerializer(*simulationStateOutputFile, statePluginRepo.GetAllPlugins())
		logging.Infof("Simulation state will be serialized to %s", *simulationStateOutputFile)
	}

	return simService
}

func (s *SimulationService) GetStatePluginRepository() *types.StatePluginRepository {
	return s.statePluginRepo
}

func (s *SimulationService) Close() {
	if s.simulationStateSerializer != nil {
		s.simulationStateSerializer.Save(s)
	}
}

// runSimulationStep is the core loop to simulate node and orchestrator logic
func (s *SimulationService) runSimulationStep(nextTime func(time.Time) time.Time) {
	if s.running {
		return
	}
	s.lock.Lock()
	if s.running {
		s.lock.Unlock()
		return
	}
	s.running = true
	s.lock.Unlock()

	s.setSimulationTime(nextTime(s.GetSimulationTime()))
	logging.Infof("Simulation time is %s", s.simTime.Format(time.RFC3339))

	// Update positions of all nodes (satellites and ground stations)
	var wg sync.WaitGroup
	for _, n := range s.all {
		wg.Add(1)
		go func(n types.Node) {
			defer wg.Done()
			n.UpdatePosition(s.simTime) // Update each node's position
		}(n)
	}
	wg.Wait()

	// Link updates (ISL and ground links)
	for _, node := range s.all {
		wg.Add(1)
		go func(n types.Node) {
			defer wg.Done()
			node.GetLinkNodeProtocol().UpdateLinks()
		}(node)
	}
	wg.Wait()

	// Routing and computation (if enabled)
	if s.config.UsePreRouteCalc {
		for _, node := range s.all {
			wg.Add(1)
			go func(n types.Node) {
				defer wg.Done()
				n.GetRouter().CalculateRoutingTable()
			}(node)
		}
		wg.Wait()
	}

	// Check if the orchestrator needs to reschedule
	if s.orchestrator != nil {
		logging.Debugf("Checking orchestrator for reschedule")
		// s.orchestrator.CheckReschedule()
	}

	// Execute post-step state plugins
	for _, plugin := range s.statePluginRepo.GetAllPlugins() {
		plugin.PostSimulationStep(s)
	}

	// Execute post-step simulation plugins
	for _, plugin := range s.simplugins {
		if err := plugin.PostSimulationStep(s); err != nil {
			logging.Warnf("Plugin %s PostSimulationStep error: %v", plugin.Name(), err)
		}
	}

	if s.simulationStateSerializer != nil {
		s.simulationStateSerializer.AddState(s)
	}

	time.Sleep(1 * time.Second) // Simulate step duration

	s.running = false
}
