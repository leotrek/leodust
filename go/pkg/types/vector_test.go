package types

import (
	"math"
	"testing"
	"time"
)

func TestVectorOperations(t *testing.T) {
	v := Vector{X: 3, Y: 0, Z: 4}
	normalized := v.Normalize()
	if math.Abs(normalized.X-0.6) > 1e-9 || math.Abs(normalized.Z-0.8) > 1e-9 {
		t.Fatalf("unexpected normalized vector: %+v", normalized)
	}

	if dot := v.Dot(Vector{X: 1, Y: 2, Z: 3}); dot != 15 {
		t.Fatalf("unexpected dot product: got %f want 15", dot)
	}

	cross := Vector{X: 1, Y: 0, Z: 0}.Cross(Vector{X: 0, Y: 1, Z: 0})
	if cross != (Vector{X: 0, Y: 0, Z: 1}) {
		t.Fatalf("unexpected cross product: %+v", cross)
	}
}

func TestSubtractAndDegreesToRadians(t *testing.T) {
	result := Vector{X: 1, Y: 2, Z: 3}.Subtract(Vector{X: 4, Y: 6, Z: 8})
	if result != (Vector{X: 3, Y: 4, Z: 5}) {
		t.Fatalf("unexpected subtract result: %+v", result)
	}

	if got := DegreesToRadians(180); math.Abs(got-math.Pi) > 1e-12 {
		t.Fatalf("unexpected radians conversion: got %f want %f", got, math.Pi)
	}
}

func TestAsNodesPreservesOrder(t *testing.T) {
	nodes := []Node{
		testNode{name: "A"},
		testNode{name: "B"},
	}

	result := AsNodes(nodes)
	if len(result) != 2 {
		t.Fatalf("expected two nodes, got %d", len(result))
	}
	if result[0].GetName() != "A" || result[1].GetName() != "B" {
		t.Fatalf("unexpected node order: %s, %s", result[0].GetName(), result[1].GetName())
	}
}

type testNode struct {
	name string
}

func (n testNode) GetName() string                       { return n.name }
func (n testNode) GetRouter() Router                     { return nil }
func (n testNode) GetComputing() Computing               { return nil }
func (n testNode) GetPosition() Vector                   { return Vector{} }
func (n testNode) DistanceTo(other Node) float64         { return 0 }
func (n testNode) UpdatePosition(time.Time)              {}
func (n testNode) GetLinkNodeProtocol() LinkNodeProtocol { return nil }
