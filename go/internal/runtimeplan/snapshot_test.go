package runtimeplan

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/leotrek/leodust/internal/topology"
	"github.com/leotrek/leodust/pkg/types"
)

type runtimeSnapshotTestSimulation struct {
	time       time.Time
	nodes      []types.Node
	satellites []types.Satellite
	grounds    []types.GroundStation
}

func (s *runtimeSnapshotTestSimulation) InjectSatellites([]types.Node) error      { return nil }
func (s *runtimeSnapshotTestSimulation) InjectGroundStations([]types.Node) error  { return nil }
func (s *runtimeSnapshotTestSimulation) StartAutorun() <-chan struct{}            { return nil }
func (s *runtimeSnapshotTestSimulation) StopAutorun()                             {}
func (s *runtimeSnapshotTestSimulation) StepBySeconds(float64)                    {}
func (s *runtimeSnapshotTestSimulation) StepByTime(time.Time)                     {}
func (s *runtimeSnapshotTestSimulation) GetAllNodes() []types.Node                { return s.nodes }
func (s *runtimeSnapshotTestSimulation) GetSatellites() []types.Satellite         { return s.satellites }
func (s *runtimeSnapshotTestSimulation) GetGroundStations() []types.GroundStation { return s.grounds }
func (s *runtimeSnapshotTestSimulation) GetSimulationTime() time.Time             { return s.time }
func (s *runtimeSnapshotTestSimulation) GetStatePluginRepository() *types.StatePluginRepository {
	return nil
}
func (s *runtimeSnapshotTestSimulation) Close() {}

type runtimeSnapshotTestNode struct {
	name      string
	computing types.Computing
	position  types.Vector
	protocol  types.LinkNodeProtocol
}

func (n *runtimeSnapshotTestNode) GetName() string               { return n.name }
func (n *runtimeSnapshotTestNode) GetRouter() types.Router       { return nil }
func (n *runtimeSnapshotTestNode) GetComputing() types.Computing { return n.computing }
func (n *runtimeSnapshotTestNode) GetPosition() types.Vector     { return n.position }
func (n *runtimeSnapshotTestNode) DistanceTo(other types.Node) float64 {
	return other.GetPosition().Subtract(n.position).Magnitude()
}
func (n *runtimeSnapshotTestNode) UpdatePosition(time.Time)                    {}
func (n *runtimeSnapshotTestNode) GetLinkNodeProtocol() types.LinkNodeProtocol { return n.protocol }

type runtimeSnapshotTestSatellite struct {
	*runtimeSnapshotTestNode
}

func (s *runtimeSnapshotTestSatellite) GetISLProtocol() types.InterSatelliteLinkProtocol { return nil }

type runtimeSnapshotTestGround struct {
	*runtimeSnapshotTestNode
}

type runtimeSnapshotTestService struct {
	name   string
	cpu    float64
	memory float64
}

func (s *runtimeSnapshotTestService) GetServiceName() string  { return s.name }
func (s *runtimeSnapshotTestService) GetCpuUsage() float64    { return s.cpu }
func (s *runtimeSnapshotTestService) GetMemoryUsage() float64 { return s.memory }
func (s *runtimeSnapshotTestService) IsDeployed() bool        { return true }
func (s *runtimeSnapshotTestService) Deploy() error           { return nil }
func (s *runtimeSnapshotTestService) Remove() error           { return nil }

type runtimeSnapshotTestComputing struct {
	computingType types.ComputingType
	totalCPU      float64
	totalMemory   float64
	services      []types.DeployableService
}

func (c *runtimeSnapshotTestComputing) Mount(types.Node) error                { return nil }
func (c *runtimeSnapshotTestComputing) GetComputingType() types.ComputingType { return c.computingType }
func (c *runtimeSnapshotTestComputing) TryPlaceDeploymentAsync(types.DeployableService) (bool, error) {
	return false, nil
}
func (c *runtimeSnapshotTestComputing) RemoveDeploymentAsync(types.DeployableService) error {
	return nil
}
func (c *runtimeSnapshotTestComputing) CanPlace(types.DeployableService) bool { return false }
func (c *runtimeSnapshotTestComputing) HostsService(serviceName string) bool {
	for _, service := range c.services {
		if service.GetServiceName() == serviceName {
			return true
		}
	}
	return false
}
func (c *runtimeSnapshotTestComputing) CpuAvailable() float64 {
	used := 0.0
	for _, service := range c.services {
		used += service.GetCpuUsage()
	}
	return c.totalCPU - used
}
func (c *runtimeSnapshotTestComputing) MemoryAvailable() float64 {
	used := 0.0
	for _, service := range c.services {
		used += service.GetMemoryUsage()
	}
	return c.totalMemory - used
}
func (c *runtimeSnapshotTestComputing) Clone() types.Computing {
	return &runtimeSnapshotTestComputing{
		computingType: c.computingType,
		totalCPU:      c.totalCPU,
		totalMemory:   c.totalMemory,
		services:      append([]types.DeployableService{}, c.services...),
	}
}
func (c *runtimeSnapshotTestComputing) GetServices() []types.DeployableService {
	return append([]types.DeployableService{}, c.services...)
}

type runtimeSnapshotTestProtocol struct {
	established []types.Link
}

func (p *runtimeSnapshotTestProtocol) Mount(types.Node)                   {}
func (p *runtimeSnapshotTestProtocol) ConnectLink(types.Link) error       { return nil }
func (p *runtimeSnapshotTestProtocol) DisconnectLink(types.Link) error    { return nil }
func (p *runtimeSnapshotTestProtocol) UpdateLinks() ([]types.Link, error) { return p.established, nil }
func (p *runtimeSnapshotTestProtocol) Established() []types.Link          { return p.established }
func (p *runtimeSnapshotTestProtocol) Links() []types.Link                { return p.established }

type runtimeSnapshotTestLink struct {
	node1     types.Node
	node2     types.Node
	distance  float64
	latency   float64
	bandwidth float64
	reachable bool
}

func (l *runtimeSnapshotTestLink) Distance() float64  { return l.distance }
func (l *runtimeSnapshotTestLink) Latency() float64   { return l.latency }
func (l *runtimeSnapshotTestLink) Bandwidth() float64 { return l.bandwidth }
func (l *runtimeSnapshotTestLink) GetOther(self types.Node) types.Node {
	if self.GetName() == l.node1.GetName() {
		return l.node2
	}
	if self.GetName() == l.node2.GetName() {
		return l.node1
	}
	return nil
}
func (l *runtimeSnapshotTestLink) IsReachable() bool               { return l.reachable }
func (l *runtimeSnapshotTestLink) Nodes() (types.Node, types.Node) { return l.node1, l.node2 }

func TestBuildSnapshotCapturesHostedWorkloadsAndRoutes(t *testing.T) {
	sat1Protocol := &runtimeSnapshotTestProtocol{}
	sat2Protocol := &runtimeSnapshotTestProtocol{}
	groundProtocol := &runtimeSnapshotTestProtocol{}

	serviceA := &runtimeSnapshotTestService{name: "svc-a", cpu: 1.5, memory: 256}
	serviceB := &runtimeSnapshotTestService{name: "svc-b", cpu: 0.5, memory: 128}
	serviceGround := &runtimeSnapshotTestService{name: "svc-ground", cpu: 2, memory: 512}

	sat1 := &runtimeSnapshotTestSatellite{
		runtimeSnapshotTestNode: &runtimeSnapshotTestNode{
			name: "sat-1",
			computing: &runtimeSnapshotTestComputing{
				computingType: types.Edge,
				totalCPU:      4,
				totalMemory:   1024,
				services:      []types.DeployableService{serviceB, serviceA},
			},
			position: types.NewVector(1, 2, 3),
			protocol: sat1Protocol,
		},
	}
	sat2 := &runtimeSnapshotTestSatellite{
		runtimeSnapshotTestNode: &runtimeSnapshotTestNode{
			name: "sat-2",
			computing: &runtimeSnapshotTestComputing{
				computingType: types.Edge,
				totalCPU:      4,
				totalMemory:   1024,
			},
			position: types.NewVector(2, 3, 4),
			protocol: sat2Protocol,
		},
	}
	ground := &runtimeSnapshotTestGround{
		runtimeSnapshotTestNode: &runtimeSnapshotTestNode{
			name: "ground-a",
			computing: &runtimeSnapshotTestComputing{
				computingType: types.Cloud,
				totalCPU:      16,
				totalMemory:   8192,
				services:      []types.DeployableService{serviceGround},
			},
			position: types.NewVector(4, 5, 6),
			protocol: groundProtocol,
		},
	}

	linkSat := &runtimeSnapshotTestLink{
		node1:     sat1,
		node2:     sat2,
		distance:  500,
		latency:   5,
		bandwidth: 20,
		reachable: true,
	}
	linkGround := &runtimeSnapshotTestLink{
		node1:     sat2,
		node2:     ground,
		distance:  750,
		latency:   7,
		bandwidth: 15,
		reachable: true,
	}
	sat1Protocol.established = []types.Link{linkSat}
	sat2Protocol.established = []types.Link{linkSat, linkGround}
	groundProtocol.established = []types.Link{linkGround}

	simulation := &runtimeSnapshotTestSimulation{
		time:       time.Date(2026, time.March, 27, 12, 0, 0, 0, time.UTC),
		nodes:      []types.Node{sat1, sat2, ground},
		satellites: []types.Satellite{sat1, sat2},
		grounds:    []types.GroundStation{ground},
	}

	snapshot := BuildSnapshot(simulation, 4, time.Date(2026, time.March, 27, 12, 0, 1, 0, time.UTC))
	if snapshot.Version != 4 {
		t.Fatalf("snapshot version = %d, want 4", snapshot.Version)
	}
	if len(snapshot.Nodes) != 3 {
		t.Fatalf("snapshot nodes = %d, want 3", len(snapshot.Nodes))
	}
	if len(snapshot.Links) != 2 {
		t.Fatalf("snapshot links = %d, want 2", len(snapshot.Links))
	}
	if len(snapshot.Workloads) != 3 {
		t.Fatalf("snapshot workloads = %d, want 3", len(snapshot.Workloads))
	}
	if len(snapshot.Routes) != 1 {
		t.Fatalf("snapshot routes = %d, want 1", len(snapshot.Routes))
	}

	satNode := snapshot.Nodes[1]
	if satNode.Name != "sat-1" {
		t.Fatalf("sat node name = %s, want sat-1", satNode.Name)
	}
	if satNode.TotalCPU != 4 || satNode.AvailableCPU != 2 {
		t.Fatalf("unexpected CPU values: total=%f available=%f", satNode.TotalCPU, satNode.AvailableCPU)
	}
	if satNode.TotalMemory != 1024 || satNode.AvailableMemory != 640 {
		t.Fatalf("unexpected memory values: total=%f available=%f", satNode.TotalMemory, satNode.AvailableMemory)
	}
	if len(satNode.HostedWorkloads) != 2 || satNode.HostedWorkloads[0] != "svc-a" || satNode.HostedWorkloads[1] != "svc-b" {
		t.Fatalf("unexpected hosted workloads: %+v", satNode.HostedWorkloads)
	}
	if snapshot.Workloads[0].HostNode != "ground-a" || snapshot.Workloads[0].Name != "svc-ground" {
		t.Fatalf("first workload = %+v, want ground-a/svc-ground", snapshot.Workloads[0])
	}
	if snapshot.Routes[0].SourceNode != "ground-a" || snapshot.Routes[0].TargetNode != "sat-1" {
		t.Fatalf("route endpoints = %+v, want ground-a -> sat-1", snapshot.Routes[0])
	}
	if len(snapshot.Routes[0].Hops) != 3 || snapshot.Routes[0].Hops[0] != "ground-a" || snapshot.Routes[0].Hops[1] != "sat-2" || snapshot.Routes[0].Hops[2] != "sat-1" {
		t.Fatalf("unexpected route hops: %+v", snapshot.Routes[0].Hops)
	}
	if snapshot.Routes[0].LatencyMs != 12 {
		t.Fatalf("unexpected route latency: %f", snapshot.Routes[0].LatencyMs)
	}
}

func TestSaveAndLoadSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "live_runtime_plan.json")
	snapshot := Snapshot{
		Version:     7,
		Time:        time.Date(2026, time.March, 27, 12, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, time.March, 27, 12, 0, 1, 0, time.UTC),
		Nodes: []SnapshotNode{
			{Name: "sat-1", Kind: topology.KindSatellite, ComputingType: types.Edge.String()},
		},
		Workloads: []SnapshotWorkload{
			{Name: "svc-a", HostNode: "sat-1", CPU: 1, Memory: 256},
		},
		Routes: []SnapshotRoute{
			{SourceNode: "sat-1", TargetNode: "sat-2", Hops: []string{"sat-1", "sat-2"}, LatencyMs: 5},
		},
	}

	if err := SaveSnapshot(path, snapshot); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	loaded, err := LoadSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if loaded.Version != snapshot.Version || loaded.Workloads[0].Name != snapshot.Workloads[0].Name || loaded.Routes[0].TargetNode != snapshot.Routes[0].TargetNode {
		t.Fatalf("loaded snapshot = %+v, want %+v", loaded, snapshot)
	}
}
