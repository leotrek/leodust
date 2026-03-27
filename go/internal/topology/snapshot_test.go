package topology

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/leotrek/leodust/pkg/types"
)

type snapshotTestSimulation struct {
	time       time.Time
	nodes      []types.Node
	satellites []types.Satellite
	grounds    []types.GroundStation
}

func (s *snapshotTestSimulation) InjectSatellites([]types.Node) error                    { return nil }
func (s *snapshotTestSimulation) InjectGroundStations([]types.Node) error                { return nil }
func (s *snapshotTestSimulation) StartAutorun() <-chan struct{}                          { return nil }
func (s *snapshotTestSimulation) StopAutorun()                                           {}
func (s *snapshotTestSimulation) StepBySeconds(float64)                                  {}
func (s *snapshotTestSimulation) StepByTime(time.Time)                                   {}
func (s *snapshotTestSimulation) GetAllNodes() []types.Node                              { return s.nodes }
func (s *snapshotTestSimulation) GetSatellites() []types.Satellite                       { return s.satellites }
func (s *snapshotTestSimulation) GetGroundStations() []types.GroundStation               { return s.grounds }
func (s *snapshotTestSimulation) GetSimulationTime() time.Time                           { return s.time }
func (s *snapshotTestSimulation) GetStatePluginRepository() *types.StatePluginRepository { return nil }
func (s *snapshotTestSimulation) Close()                                                 {}

type snapshotTestNode struct {
	name      string
	computing types.Computing
	position  types.Vector
	protocol  types.LinkNodeProtocol
}

func (n *snapshotTestNode) GetName() string               { return n.name }
func (n *snapshotTestNode) GetRouter() types.Router       { return nil }
func (n *snapshotTestNode) GetComputing() types.Computing { return n.computing }
func (n *snapshotTestNode) GetPosition() types.Vector     { return n.position }
func (n *snapshotTestNode) DistanceTo(other types.Node) float64 {
	return other.GetPosition().Subtract(n.position).Magnitude()
}
func (n *snapshotTestNode) UpdatePosition(time.Time)                    {}
func (n *snapshotTestNode) GetLinkNodeProtocol() types.LinkNodeProtocol { return n.protocol }

type snapshotTestSatellite struct {
	*snapshotTestNode
}

func (s *snapshotTestSatellite) GetISLProtocol() types.InterSatelliteLinkProtocol { return nil }

type snapshotTestGround struct {
	*snapshotTestNode
}

type snapshotTestComputing struct {
	computingType types.ComputingType
}

func (c *snapshotTestComputing) Mount(types.Node) error                { return nil }
func (c *snapshotTestComputing) GetComputingType() types.ComputingType { return c.computingType }
func (c *snapshotTestComputing) TryPlaceDeploymentAsync(types.DeployableService) (bool, error) {
	return false, nil
}
func (c *snapshotTestComputing) RemoveDeploymentAsync(types.DeployableService) error { return nil }
func (c *snapshotTestComputing) CanPlace(types.DeployableService) bool               { return false }
func (c *snapshotTestComputing) HostsService(string) bool                            { return false }
func (c *snapshotTestComputing) CpuAvailable() float64                               { return 0 }
func (c *snapshotTestComputing) MemoryAvailable() float64                            { return 0 }
func (c *snapshotTestComputing) Clone() types.Computing {
	return &snapshotTestComputing{computingType: c.computingType}
}
func (c *snapshotTestComputing) GetServices() []types.DeployableService { return nil }

type snapshotTestProtocol struct {
	established []types.Link
}

func (p *snapshotTestProtocol) Mount(types.Node)                   {}
func (p *snapshotTestProtocol) ConnectLink(types.Link) error       { return nil }
func (p *snapshotTestProtocol) DisconnectLink(types.Link) error    { return nil }
func (p *snapshotTestProtocol) UpdateLinks() ([]types.Link, error) { return p.established, nil }
func (p *snapshotTestProtocol) Established() []types.Link          { return p.established }
func (p *snapshotTestProtocol) Links() []types.Link                { return p.established }

type snapshotTestLink struct {
	node1     types.Node
	node2     types.Node
	distance  float64
	latency   float64
	bandwidth float64
	reachable bool
}

func (l *snapshotTestLink) Distance() float64  { return l.distance }
func (l *snapshotTestLink) Latency() float64   { return l.latency }
func (l *snapshotTestLink) Bandwidth() float64 { return l.bandwidth }
func (l *snapshotTestLink) GetOther(self types.Node) types.Node {
	if self.GetName() == l.node1.GetName() {
		return l.node2
	}
	if self.GetName() == l.node2.GetName() {
		return l.node1
	}
	return nil
}
func (l *snapshotTestLink) IsReachable() bool               { return l.reachable }
func (l *snapshotTestLink) Nodes() (types.Node, types.Node) { return l.node1, l.node2 }

func TestBuildSnapshotDeduplicatesLinksAndCapturesKinds(t *testing.T) {
	sat1Protocol := &snapshotTestProtocol{}
	sat2Protocol := &snapshotTestProtocol{}
	groundProtocol := &snapshotTestProtocol{}

	sat1 := &snapshotTestSatellite{
		snapshotTestNode: &snapshotTestNode{
			name:      "sat-1",
			computing: &snapshotTestComputing{computingType: types.Edge},
			position:  types.NewVector(1, 2, 3),
			protocol:  sat1Protocol,
		},
	}
	sat2 := &snapshotTestSatellite{
		snapshotTestNode: &snapshotTestNode{
			name:      "sat-2",
			computing: &snapshotTestComputing{computingType: types.Edge},
			position:  types.NewVector(4, 5, 6),
			protocol:  sat2Protocol,
		},
	}
	ground := &snapshotTestGround{
		snapshotTestNode: &snapshotTestNode{
			name:      "ground-a",
			computing: &snapshotTestComputing{computingType: types.Cloud},
			position:  types.NewVector(7, 8, 9),
			protocol:  groundProtocol,
		},
	}

	satLink := &snapshotTestLink{
		node1:     sat1,
		node2:     sat2,
		distance:  1000,
		latency:   5,
		bandwidth: 10,
		reachable: true,
	}
	groundLink := &snapshotTestLink{
		node1:     ground,
		node2:     sat1,
		distance:  500,
		latency:   7,
		bandwidth: 20,
		reachable: true,
	}

	sat1Protocol.established = []types.Link{satLink, groundLink}
	sat2Protocol.established = []types.Link{satLink}
	groundProtocol.established = []types.Link{groundLink}

	simulation := &snapshotTestSimulation{
		time:       time.Date(2026, time.March, 26, 14, 10, 0, 0, time.UTC),
		nodes:      []types.Node{ground, sat2, sat1},
		satellites: []types.Satellite{sat1, sat2},
		grounds:    []types.GroundStation{ground},
	}

	snapshot := BuildSnapshot(simulation, 12, time.Date(2026, time.March, 26, 14, 10, 1, 0, time.UTC))
	if snapshot.Version != 12 {
		t.Fatalf("snapshot version = %d, want 12", snapshot.Version)
	}
	if len(snapshot.Nodes) != 3 {
		t.Fatalf("snapshot nodes = %d, want 3", len(snapshot.Nodes))
	}
	if snapshot.Nodes[0].Name != "ground-a" || snapshot.Nodes[0].Kind != KindGround {
		t.Fatalf("first node = %+v, want ground-a/%s", snapshot.Nodes[0], KindGround)
	}
	if snapshot.Nodes[1].Name != "sat-1" || snapshot.Nodes[1].Kind != KindSatellite {
		t.Fatalf("second node = %+v, want sat-1/%s", snapshot.Nodes[1], KindSatellite)
	}
	if len(snapshot.Links) != 2 {
		t.Fatalf("snapshot links = %d, want 2", len(snapshot.Links))
	}
	if snapshot.Links[0].Source != "ground-a" || snapshot.Links[0].Target != "sat-1" {
		t.Fatalf("first link = %+v, want ground-a <-> sat-1", snapshot.Links[0])
	}
	if snapshot.Links[1].Source != "sat-1" || snapshot.Links[1].Target != "sat-2" {
		t.Fatalf("second link = %+v, want sat-1 <-> sat-2", snapshot.Links[1])
	}
}

func TestSaveAndLoadSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "live_topology.json")
	snapshot := Snapshot{
		Version:     3,
		Time:        time.Date(2026, time.March, 26, 14, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, time.March, 26, 14, 0, 1, 0, time.UTC),
		Nodes: []SnapshotNode{
			{Name: "sat-1", Kind: KindSatellite, ComputingType: types.Edge.String()},
		},
	}

	if err := SaveSnapshot(path, snapshot); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	loaded, err := LoadSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if loaded.Version != snapshot.Version || loaded.Nodes[0].Name != snapshot.Nodes[0].Name {
		t.Fatalf("loaded snapshot = %+v, want %+v", loaded, snapshot)
	}
}
