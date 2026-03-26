package orbit

import (
	"math"
	"testing"
	"time"

	gosatellite "github.com/joshuaferrara/go-satellite"
	"github.com/leotrek/leodust/pkg/types"
)

const (
	testTLELine1 = "1 51880U 22022AE  26070.52430208  .00000741  00000+0  64608-4 0  9998"
	testTLELine2 = "2 51880  53.2187   0.6017 0001344  94.5762 265.5384 15.08845836223399"
)

func TestTLEPropagatorMatchesLibraryECEFOutput(t *testing.T) {
	propagator, err := NewTLEPropagator(testTLELine1, testTLELine2)
	if err != nil {
		t.Fatalf("NewTLEPropagator returned error: %v", err)
	}

	at := time.Date(2026, time.March, 11, 12, 34, 59, 0, time.UTC)
	got, err := propagator.PositionECEF(at)
	if err != nil {
		t.Fatalf("PositionECEF returned error: %v", err)
	}

	sat := gosatellite.TLEToSat(testTLELine1, testTLELine2, gosatellite.GravityWGS72)
	positionECI, _ := gosatellite.Propagate(sat, at.Year(), int(at.Month()), at.Day(), at.Hour(), at.Minute(), at.Second())
	gmst := gosatellite.GSTimeFromDate(at.Year(), int(at.Month()), at.Day(), at.Hour(), at.Minute(), at.Second())
	positionECEF := gosatellite.ECIToECEF(positionECI, gmst)
	want := types.Vector{
		X: positionECEF.X * 1000,
		Y: positionECEF.Y * 1000,
		Z: positionECEF.Z * 1000,
	}

	assertVectorClose(t, got, want, 1e-6)
}

func TestTLEPropagatorTruncatesSubSecondInput(t *testing.T) {
	propagator, err := NewTLEPropagator(testTLELine1, testTLELine2)
	if err != nil {
		t.Fatalf("NewTLEPropagator returned error: %v", err)
	}

	base := time.Date(2026, time.March, 11, 12, 34, 59, 100_000_000, time.UTC)
	gotA, err := propagator.PositionECEF(base)
	if err != nil {
		t.Fatalf("PositionECEF returned error: %v", err)
	}
	gotB, err := propagator.PositionECEF(base.Add(800 * time.Millisecond))
	if err != nil {
		t.Fatalf("PositionECEF returned error: %v", err)
	}

	assertVectorClose(t, gotA, gotB, 1e-6)
}

func TestGeodeticToECEFAtEquatorPrimeMeridian(t *testing.T) {
	position := GeodeticToECEF(0, 0, 0)

	if math.Abs(position.X-wgs84SemiMajorAxis) > 1e-6 {
		t.Fatalf("unexpected X coordinate: got %f want %f", position.X, wgs84SemiMajorAxis)
	}
	if math.Abs(position.Y) > 1e-6 {
		t.Fatalf("unexpected Y coordinate: got %f want 0", position.Y)
	}
	if math.Abs(position.Z) > 1e-6 {
		t.Fatalf("unexpected Z coordinate: got %f want 0", position.Z)
	}
}

func TestGroundSatelliteVisible(t *testing.T) {
	ground := GeodeticToECEF(0, 0, 0)
	visible := types.Vector{X: ground.X + 550_000, Y: ground.Y, Z: ground.Z}
	hidden := types.Vector{X: -(ground.X + 550_000), Y: 0, Z: 0}

	if !GroundSatelliteVisible(ground, visible) {
		t.Fatal("expected overhead satellite to be visible")
	}
	if GroundSatelliteVisible(ground, hidden) {
		t.Fatal("expected opposite-side satellite to be hidden")
	}
}

func TestValidateECIStateRejectsImpossibleRadius(t *testing.T) {
	err := validateECIState(ECIState{
		PositionKM:       types.Vector{X: 100, Y: 0, Z: 0},
		VelocityKMPerSec: types.Vector{X: 1, Y: 1, Z: 1},
	})
	if err == nil {
		t.Fatal("expected impossible orbital radius to return an error")
	}
}

func assertVectorClose(t *testing.T, got, want types.Vector, tolerance float64) {
	t.Helper()

	if math.Abs(got.X-want.X) > tolerance || math.Abs(got.Y-want.Y) > tolerance || math.Abs(got.Z-want.Z) > tolerance {
		t.Fatalf("vectors differ beyond tolerance %.6f:\n got:  %+v\n want: %+v", tolerance, got, want)
	}
}
