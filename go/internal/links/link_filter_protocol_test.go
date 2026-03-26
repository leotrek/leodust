package links

import (
	"testing"
	"time"

	"github.com/leotrek/leodust/pkg/types"
)

type filterTestNode struct {
	name     string
	protocol types.LinkNodeProtocol
}

func (n *filterTestNode) GetName() string                             { return n.name }
func (n *filterTestNode) GetRouter() types.Router                     { return nil }
func (n *filterTestNode) GetComputing() types.Computing               { return nil }
func (n *filterTestNode) GetPosition() types.Vector                   { return types.Vector{} }
func (n *filterTestNode) DistanceTo(other types.Node) float64         { return 0 }
func (n *filterTestNode) UpdatePosition(time.Time)                    {}
func (n *filterTestNode) GetLinkNodeProtocol() types.LinkNodeProtocol { return n.protocol }

type filterTestLink struct {
	a types.Node
	b types.Node
}

func (l *filterTestLink) Distance() float64  { return 0 }
func (l *filterTestLink) Latency() float64   { return 0 }
func (l *filterTestLink) Bandwidth() float64 { return 0 }
func (l *filterTestLink) GetOther(self types.Node) types.Node {
	if self == l.a {
		return l.b
	}
	if self == l.b {
		return l.a
	}
	return nil
}
func (l *filterTestLink) IsReachable() bool               { return true }
func (l *filterTestLink) Nodes() (types.Node, types.Node) { return l.a, l.b }

type filterTestInnerProtocol struct {
	updates      [][]types.Link
	updateIndex  int
	added        []types.Link
	connected    []types.Link
	disconnected []types.Link
}

func (p *filterTestInnerProtocol) Mount(types.Node) {}
func (p *filterTestInnerProtocol) AddLink(link types.Link) {
	p.added = append(p.added, link)
}
func (p *filterTestInnerProtocol) ConnectLink(link types.Link) error {
	p.connected = append(p.connected, link)
	return nil
}
func (p *filterTestInnerProtocol) DisconnectLink(link types.Link) error {
	p.disconnected = append(p.disconnected, link)
	return nil
}
func (p *filterTestInnerProtocol) UpdateLinks() ([]types.Link, error) {
	result := p.updates[p.updateIndex]
	if p.updateIndex < len(p.updates)-1 {
		p.updateIndex++
	}
	return result, nil
}
func (p *filterTestInnerProtocol) Established() []types.Link { return nil }
func (p *filterTestInnerProtocol) Links() []types.Link       { return nil }

func TestLinkFilterProtocolFiltersLocalLinks(t *testing.T) {
	a := &filterTestNode{name: "A"}
	b := &filterTestNode{name: "B"}
	c := &filterTestNode{name: "C"}

	ab := &filterTestLink{a: a, b: b}
	bc := &filterTestLink{a: b, b: c}

	inner := &filterTestInnerProtocol{updates: [][]types.Link{{ab, bc}}}
	protocol := NewLinkFilterProtocol(inner)
	protocol.Mount(a)
	protocol.AddLink(ab)
	protocol.AddLink(bc)

	links, err := protocol.UpdateLinks()
	if err != nil {
		t.Fatalf("UpdateLinks returned error: %v", err)
	}
	if len(inner.added) != 2 {
		t.Fatalf("expected inner protocol to receive both links, got %d", len(inner.added))
	}
	if len(protocol.Links()) != 1 {
		t.Fatalf("expected only one local candidate link, got %d", len(protocol.Links()))
	}
	if len(links) != 1 || links[0] != ab {
		t.Fatalf("expected only the A-B link to remain after filtering, got %+v", links)
	}
}

func TestLinkFilterProtocolRemovesStaleEstablishedLinks(t *testing.T) {
	a := &filterTestNode{name: "A"}
	b := &filterTestNode{name: "B"}
	ab := &filterTestLink{a: a, b: b}

	inner := &filterTestInnerProtocol{updates: [][]types.Link{{ab}, {}}}
	protocol := NewLinkFilterProtocol(inner)
	protocol.Mount(a)
	protocol.AddLink(ab)

	if _, err := protocol.UpdateLinks(); err != nil {
		t.Fatalf("first UpdateLinks returned error: %v", err)
	}
	if len(protocol.Established()) != 1 {
		t.Fatalf("expected one established link, got %d", len(protocol.Established()))
	}

	links, err := protocol.UpdateLinks()
	if err != nil {
		t.Fatalf("second UpdateLinks returned error: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("expected no filtered links after removal, got %d", len(links))
	}
	if len(protocol.Established()) != 0 {
		t.Fatalf("expected stale established links to be cleared, got %d", len(protocol.Established()))
	}
}
