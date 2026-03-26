package links

import (
	"math"
	"testing"
	"time"

	"github.com/leotrek/leodust/internal/orbit"
	"github.com/leotrek/leodust/pkg/types"
)

type testNode struct {
	name     string
	position types.Vector
	protocol types.LinkNodeProtocol
}

func (n *testNode) GetName() string {
	return n.name
}

func (n *testNode) GetRouter() types.Router {
	return nil
}

func (n *testNode) GetComputing() types.Computing {
	return nil
}

func (n *testNode) GetPosition() types.Vector {
	return n.position
}

func (n *testNode) DistanceTo(other types.Node) float64 {
	dx := other.GetPosition().X - n.position.X
	dy := other.GetPosition().Y - n.position.Y
	dz := other.GetPosition().Z - n.position.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func (n *testNode) UpdatePosition(time.Time) {}

func (n *testNode) GetLinkNodeProtocol() types.LinkNodeProtocol {
	return n.protocol
}

type recordingISLProtocol struct {
	connected    []types.Link
	disconnected []types.Link
}

func (p *recordingISLProtocol) Mount(types.Node) {}

func (p *recordingISLProtocol) AddLink(types.Link) {}

func (p *recordingISLProtocol) ConnectLink(link types.Link) error {
	p.connected = append(p.connected, link)
	return nil
}

func (p *recordingISLProtocol) DisconnectLink(link types.Link) error {
	p.disconnected = append(p.disconnected, link)
	return nil
}

func (p *recordingISLProtocol) UpdateLinks() ([]types.Link, error) {
	return nil, nil
}

func (p *recordingISLProtocol) Established() []types.Link {
	return p.connected
}

func (p *recordingISLProtocol) Links() []types.Link {
	return p.connected
}

type testSatellite struct {
	*testNode
	isl *recordingISLProtocol
}

func newTestSatellite(name string, position types.Vector) *testSatellite {
	isl := &recordingISLProtocol{}
	return &testSatellite{
		testNode: &testNode{
			name:     name,
			position: position,
			protocol: isl,
		},
		isl: isl,
	}
}

func (s *testSatellite) GetISLProtocol() types.InterSatelliteLinkProtocol {
	return s.isl
}

func TestGroundNearestProtocolChoosesNearestVisibleSatellite(t *testing.T) {
	groundPosition := orbit.GeodeticToECEF(0, 0, 0)
	reachable := newTestSatellite("reachable", types.Vector{
		X: groundPosition.X + 4_000_000,
		Y: 0,
		Z: 0,
	})

	orbitalRadius := groundPosition.Magnitude() + 550_000
	hidden := newTestSatellite("hidden", types.Vector{
		X: orbitalRadius * math.Cos(25*math.Pi/180),
		Y: orbitalRadius * math.Sin(25*math.Pi/180),
		Z: 0,
	})

	if orbit.GroundSatelliteVisible(groundPosition, hidden.position) {
		t.Fatal("test setup invalid: hidden satellite should be below the horizon")
	}
	if !orbit.GroundSatelliteVisible(groundPosition, reachable.position) {
		t.Fatal("test setup invalid: reachable satellite should be above the horizon")
	}
	if hidden.DistanceTo(&testNode{position: groundPosition}) >= reachable.DistanceTo(&testNode{position: groundPosition}) {
		t.Fatal("test setup invalid: hidden satellite should be the closer option")
	}

	protocol := NewGroundSatelliteNearestProtocol([]types.Satellite{hidden, reachable}).(*GroundSatelliteNearestProtocol)
	ground := &testNode{name: "ground", position: groundPosition, protocol: protocol}
	protocol.Mount(ground)

	links, err := protocol.UpdateLinks()
	if err != nil {
		t.Fatalf("UpdateLinks returned error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected one established link, got %d", len(links))
	}
	if other := links[0].GetOther(ground); other.GetName() != reachable.GetName() {
		t.Fatalf("expected link to %s, got %s", reachable.GetName(), other.GetName())
	}
	if len(hidden.isl.connected) != 0 {
		t.Fatalf("hidden satellite should not have received a ground link, got %d", len(hidden.isl.connected))
	}
	if len(reachable.isl.connected) != 1 {
		t.Fatalf("reachable satellite should have exactly one ground link, got %d", len(reachable.isl.connected))
	}
}

func TestGroundNearestProtocolDropsLinkWhenVisibilityIsLost(t *testing.T) {
	groundPosition := orbit.GeodeticToECEF(0, 0, 0)
	reachable := newTestSatellite("reachable", types.Vector{
		X: groundPosition.X + 4_000_000,
		Y: 0,
		Z: 0,
	})

	protocol := NewGroundSatelliteNearestProtocol([]types.Satellite{reachable}).(*GroundSatelliteNearestProtocol)
	ground := &testNode{name: "ground", position: groundPosition, protocol: protocol}
	protocol.Mount(ground)

	links, err := protocol.UpdateLinks()
	if err != nil {
		t.Fatalf("UpdateLinks returned error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected one established link, got %d", len(links))
	}

	reachable.position = types.Vector{
		X: -(groundPosition.X + 550_000),
		Y: 0,
		Z: 0,
	}
	if orbit.GroundSatelliteVisible(groundPosition, reachable.position) {
		t.Fatal("test setup invalid: satellite should be below the horizon after moving")
	}

	links, err = protocol.UpdateLinks()
	if err != nil {
		t.Fatalf("UpdateLinks returned error after visibility loss: %v", err)
	}
	if links != nil {
		t.Fatalf("expected no links after visibility loss, got %d", len(links))
	}
	if protocol.Link() != nil {
		t.Fatal("expected active ground link to be cleared")
	}
	if len(reachable.isl.disconnected) != 1 {
		t.Fatalf("expected one disconnect call, got %d", len(reachable.isl.disconnected))
	}
}
