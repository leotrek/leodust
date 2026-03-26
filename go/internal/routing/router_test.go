package routing

import (
	"math"
	"testing"
	"time"

	"github.com/leotrek/leodust/configs"
	computingpkg "github.com/leotrek/leodust/internal/computing"
	"github.com/leotrek/leodust/pkg/types"
)

type routingTestService struct {
	name string
}

func (s routingTestService) GetServiceName() string  { return s.name }
func (s routingTestService) GetCpuUsage() float64    { return 0 }
func (s routingTestService) GetMemoryUsage() float64 { return 0 }
func (s routingTestService) IsDeployed() bool        { return true }
func (s routingTestService) Deploy() error           { return nil }
func (s routingTestService) Remove() error           { return nil }

type routingTestProtocol struct {
	links []types.Link
}

func (p *routingTestProtocol) Mount(types.Node)                   {}
func (p *routingTestProtocol) ConnectLink(types.Link) error       { return nil }
func (p *routingTestProtocol) DisconnectLink(types.Link) error    { return nil }
func (p *routingTestProtocol) UpdateLinks() ([]types.Link, error) { return p.links, nil }
func (p *routingTestProtocol) Established() []types.Link          { return p.links }
func (p *routingTestProtocol) Links() []types.Link                { return p.links }

type routingTestNode struct {
	name      string
	position  types.Vector
	protocol  *routingTestProtocol
	computing *computingpkg.Computing
}

func (n *routingTestNode) GetName() string               { return n.name }
func (n *routingTestNode) GetRouter() types.Router       { return nil }
func (n *routingTestNode) GetComputing() types.Computing { return n.computing }
func (n *routingTestNode) GetPosition() types.Vector     { return n.position }
func (n *routingTestNode) DistanceTo(other types.Node) float64 {
	dx := other.GetPosition().X - n.position.X
	dy := other.GetPosition().Y - n.position.Y
	dz := other.GetPosition().Z - n.position.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}
func (n *routingTestNode) UpdatePosition(time.Time)                    {}
func (n *routingTestNode) GetLinkNodeProtocol() types.LinkNodeProtocol { return n.protocol }

type routingTestLink struct {
	a       types.Node
	b       types.Node
	latency float64
}

func (l *routingTestLink) Distance() float64  { return l.latency * 1000 }
func (l *routingTestLink) Latency() float64   { return l.latency }
func (l *routingTestLink) Bandwidth() float64 { return 1 }
func (l *routingTestLink) GetOther(self types.Node) types.Node {
	if self == l.a {
		return l.b
	}
	if self == l.b {
		return l.a
	}
	return nil
}
func (l *routingTestLink) IsReachable() bool               { return true }
func (l *routingTestLink) Nodes() (types.Node, types.Node) { return l.a, l.b }

func TestRouterBuilderBuild(t *testing.T) {
	builder := NewRouterBuilder(configs.RouterConfig{Protocol: "a-star"})
	router, err := builder.Build()
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if _, ok := router.(*AStarRouter); !ok {
		t.Fatalf("expected *AStarRouter, got %T", router)
	}

	_, err = NewRouterBuilder(configs.RouterConfig{Protocol: "unknown"}).Build()
	if err == nil {
		t.Fatal("expected unknown routing protocol to return an error")
	}
}

func TestAStarRouterFindsLowestLatencyRoute(t *testing.T) {
	a, b, _ := buildRoutingTriangle()

	router := NewAStarRouter()
	if err := router.Mount(a); err != nil {
		t.Fatalf("Mount returned error: %v", err)
	}

	result, err := router.RouteToNode(b, nil)
	if err != nil {
		t.Fatalf("RouteToNode returned error: %v", err)
	}
	if !result.Reachable() {
		t.Fatal("expected target node to be reachable")
	}
	if result.Latency() != 7 {
		t.Fatalf("expected latency 7, got %d", result.Latency())
	}
}

func TestDijkstraRouterCalculatesNodeAndServiceRoutes(t *testing.T) {
	a, b, _ := buildRoutingTriangle()
	b.computing.Services = []types.DeployableService{routingTestService{name: "svc"}}

	router := NewDijkstraRouter()
	if err := router.Mount(a); err != nil {
		t.Fatalf("Mount returned error: %v", err)
	}
	if err := router.CalculateRoutingTable(); err != nil {
		t.Fatalf("CalculateRoutingTable returned error: %v", err)
	}

	nodeRoute, err := router.RouteToNode(b, nil)
	if err != nil {
		t.Fatalf("RouteToNode returned error: %v", err)
	}
	if !nodeRoute.Reachable() || nodeRoute.Latency() != 7 {
		t.Fatalf("unexpected node route result: reachable=%v latency=%d", nodeRoute.Reachable(), nodeRoute.Latency())
	}

	serviceRoute, err := router.RouteToService("svc", nil)
	if err != nil {
		t.Fatalf("RouteToService returned error: %v", err)
	}
	if !serviceRoute.Reachable() || serviceRoute.Latency() != 7 {
		t.Fatalf("unexpected service route result: reachable=%v latency=%d", serviceRoute.Reachable(), serviceRoute.Latency())
	}
}

func buildRoutingTriangle() (*routingTestNode, *routingTestNode, *routingTestNode) {
	a := newRoutingTestNode("A", types.Vector{X: 0, Y: 0, Z: 0})
	b := newRoutingTestNode("B", types.Vector{X: 10, Y: 0, Z: 0})
	c := newRoutingTestNode("C", types.Vector{X: 3, Y: 0, Z: 0})

	ab := &routingTestLink{a: a, b: b, latency: 10}
	ac := &routingTestLink{a: a, b: c, latency: 3}
	cb := &routingTestLink{a: c, b: b, latency: 4}

	a.protocol.links = []types.Link{ab, ac}
	b.protocol.links = []types.Link{ab, cb}
	c.protocol.links = []types.Link{ac, cb}

	return a, b, c
}

func newRoutingTestNode(name string, position types.Vector) *routingTestNode {
	computing := computingpkg.NewComputing(1, 1, types.Edge)
	node := &routingTestNode{
		name:      name,
		position:  position,
		protocol:  &routingTestProtocol{},
		computing: computing,
	}
	_ = computing.Mount(node)
	return node
}
