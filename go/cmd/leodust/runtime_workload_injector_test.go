package main

import (
	"strings"
	"testing"
	"time"

	"github.com/leotrek/leodust/internal/computing"
	"github.com/leotrek/leodust/pkg/types"
)

type workloadInjectorTestSimulation struct {
	nodes      []types.Node
	satellites []types.Satellite
	grounds    []types.GroundStation
}

func (s *workloadInjectorTestSimulation) InjectSatellites([]types.Node) error      { return nil }
func (s *workloadInjectorTestSimulation) InjectGroundStations([]types.Node) error  { return nil }
func (s *workloadInjectorTestSimulation) StartAutorun() <-chan struct{}            { return nil }
func (s *workloadInjectorTestSimulation) StopAutorun()                             {}
func (s *workloadInjectorTestSimulation) StepBySeconds(float64)                    {}
func (s *workloadInjectorTestSimulation) StepByTime(time.Time)                     {}
func (s *workloadInjectorTestSimulation) GetAllNodes() []types.Node                { return s.nodes }
func (s *workloadInjectorTestSimulation) GetSatellites() []types.Satellite         { return s.satellites }
func (s *workloadInjectorTestSimulation) GetGroundStations() []types.GroundStation { return s.grounds }
func (s *workloadInjectorTestSimulation) GetSimulationTime() time.Time             { return time.Time{} }
func (s *workloadInjectorTestSimulation) GetStatePluginRepository() *types.StatePluginRepository {
	return nil
}
func (s *workloadInjectorTestSimulation) Close() {}

type workloadInjectorTestNode struct {
	name      string
	computing types.Computing
}

func (n *workloadInjectorTestNode) GetName() string               { return n.name }
func (n *workloadInjectorTestNode) GetRouter() types.Router       { return nil }
func (n *workloadInjectorTestNode) GetComputing() types.Computing { return n.computing }
func (n *workloadInjectorTestNode) GetPosition() types.Vector     { return types.NewVector(0, 0, 0) }
func (n *workloadInjectorTestNode) DistanceTo(types.Node) float64 { return 0 }
func (n *workloadInjectorTestNode) UpdatePosition(time.Time)      {}
func (n *workloadInjectorTestNode) GetLinkNodeProtocol() types.LinkNodeProtocol {
	return workloadInjectorTestProtocol{}
}

type workloadInjectorTestSatellite struct {
	*workloadInjectorTestNode
}

func (s *workloadInjectorTestSatellite) GetISLProtocol() types.InterSatelliteLinkProtocol { return nil }

type workloadInjectorTestGround struct {
	*workloadInjectorTestNode
}

type workloadInjectorTestProtocol struct{}

func (workloadInjectorTestProtocol) Mount(types.Node)                   {}
func (workloadInjectorTestProtocol) ConnectLink(types.Link) error       { return nil }
func (workloadInjectorTestProtocol) DisconnectLink(types.Link) error    { return nil }
func (workloadInjectorTestProtocol) UpdateLinks() ([]types.Link, error) { return nil, nil }
func (workloadInjectorTestProtocol) Established() []types.Link          { return nil }
func (workloadInjectorTestProtocol) Links() []types.Link                { return nil }

func newWorkloadInjectorTestSatellite(name string, cpu, memory float64) *workloadInjectorTestSatellite {
	computingUnit := computing.NewComputing(cpu, memory, types.Edge)
	satellite := &workloadInjectorTestSatellite{
		workloadInjectorTestNode: &workloadInjectorTestNode{
			name:      name,
			computing: computingUnit,
		},
	}
	if err := computingUnit.Mount(satellite); err != nil {
		panic(err)
	}
	return satellite
}

func newWorkloadInjectorTestGround(name string, cpu, memory float64) *workloadInjectorTestGround {
	computingUnit := computing.NewComputing(cpu, memory, types.Cloud)
	ground := &workloadInjectorTestGround{
		workloadInjectorTestNode: &workloadInjectorTestNode{
			name:      name,
			computing: computingUnit,
		},
	}
	if err := computingUnit.Mount(ground); err != nil {
		panic(err)
	}
	return ground
}

func TestInjectSyntheticWorkloadsUsesSortedSatelliteNodes(t *testing.T) {
	satB := newWorkloadInjectorTestSatellite("sat-b", 8, 1024)
	satA := newWorkloadInjectorTestSatellite("sat-a", 8, 1024)
	ground := newWorkloadInjectorTestGround("ground-a", 16, 4096)
	simulation := &workloadInjectorTestSimulation{
		nodes:      []types.Node{satB, satA, ground},
		satellites: []types.Satellite{satB, satA},
		grounds:    []types.GroundStation{ground},
	}

	err := injectSyntheticWorkloads(simulation, workloadInjectionOptions{
		Count:  2,
		CPU:    1,
		Memory: 64,
		Target: "satellites",
		Prefix: "rt",
	})
	if err != nil {
		t.Fatalf("injectSyntheticWorkloads returned error: %v", err)
	}

	servicesA := satA.GetComputing().GetServices()
	servicesB := satB.GetComputing().GetServices()
	if len(servicesA) != 1 || servicesA[0].GetServiceName() != "rt-1" {
		t.Fatalf("sat-a services = %+v, want rt-1", servicesA)
	}
	if len(servicesB) != 1 || servicesB[0].GetServiceName() != "rt-2" {
		t.Fatalf("sat-b services = %+v, want rt-2", servicesB)
	}
	if len(ground.GetComputing().GetServices()) != 0 {
		t.Fatalf("ground should not receive workloads when target=satellites")
	}
}

func TestInjectSyntheticWorkloadsRejectsInsufficientNodes(t *testing.T) {
	satA := newWorkloadInjectorTestSatellite("sat-a", 8, 1024)
	satB := newWorkloadInjectorTestSatellite("sat-b", 0.5, 32)
	simulation := &workloadInjectorTestSimulation{
		nodes:      []types.Node{satA, satB},
		satellites: []types.Satellite{satA, satB},
	}

	err := injectSyntheticWorkloads(simulation, workloadInjectionOptions{
		Count:  2,
		CPU:    1,
		Memory: 64,
		Target: "satellites",
	})
	if err == nil {
		t.Fatal("injectSyntheticWorkloads should fail when too few nodes are eligible")
	}
	if !strings.Contains(err.Error(), "only 1 eligible nodes") {
		t.Fatalf("injectSyntheticWorkloads error = %q, want eligible-node count", err)
	}
}
