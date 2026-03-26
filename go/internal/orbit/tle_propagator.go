package orbit

import (
	"fmt"
	"math"
	"time"

	gosatellite "github.com/joshuaferrara/go-satellite"
	"github.com/leotrek/leodust/pkg/types"
)

const (
	minReasonableOrbitRadiusKM = 6000.0
	maxReasonableOrbitRadiusKM = 500000.0
)

// ECIState contains one propagated state vector in the inertial frame.
// Position is expressed in kilometers and velocity in kilometers per second
// because those are the native SGP4 units from the upstream library.
type ECIState struct {
	PositionKM       types.Vector
	VelocityKMPerSec types.Vector
}

// Propagator produces Earth-fixed satellite positions for a given simulation time.
type Propagator interface {
	PositionECEF(time.Time) (types.Vector, error)
}

// TLEPropagator wraps an SGP4 satellite record built from a TLE.
type TLEPropagator struct {
	satellite gosatellite.Satellite
}

// NewTLEPropagator builds an SGP4 propagator from validated TLE lines.
func NewTLEPropagator(line1, line2 string) (*TLEPropagator, error) {
	if len(line1) < 69 || len(line2) < 69 {
		return nil, fmt.Errorf("invalid TLE line length")
	}

	return &TLEPropagator{
		satellite: gosatellite.TLEToSat(line1, line2, gosatellite.GravityWGS72),
	}, nil
}

// StateECI propagates the TLE to the requested time and returns the inertial
// state in the upstream library's native kilometer and kilometer-per-second units.
func (p *TLEPropagator) StateECI(at time.Time) (ECIState, error) {
	// The upstream propagator accepts whole-second timestamps, so align here to make
	// the simulator's behavior explicit and deterministic.
	utc := at.UTC().Truncate(time.Second)

	positionECI, velocityECI := gosatellite.Propagate(
		p.satellite,
		utc.Year(),
		int(utc.Month()),
		utc.Day(),
		utc.Hour(),
		utc.Minute(),
		utc.Second(),
	)

	state := ECIState{
		PositionKM: types.Vector{
			X: positionECI.X,
			Y: positionECI.Y,
			Z: positionECI.Z,
		},
		VelocityKMPerSec: types.Vector{
			X: velocityECI.X,
			Y: velocityECI.Y,
			Z: velocityECI.Z,
		},
	}

	return state, validateECIState(state)
}

// PositionECEF propagates the TLE to the requested time and returns meters in ECEF.
func (p *TLEPropagator) PositionECEF(at time.Time) (types.Vector, error) {
	state, err := p.StateECI(at)
	if err != nil {
		return types.Vector{}, err
	}

	utc := at.UTC().Truncate(time.Second)
	gmst := gosatellite.GSTimeFromDate(
		utc.Year(),
		int(utc.Month()),
		utc.Day(),
		utc.Hour(),
		utc.Minute(),
		utc.Second(),
	)
	positionECEF := gosatellite.ECIToECEF(gosatellite.Vector3{
		X: state.PositionKM.X,
		Y: state.PositionKM.Y,
		Z: state.PositionKM.Z,
	}, gmst)

	return types.Vector{
		X: positionECEF.X * 1000,
		Y: positionECEF.Y * 1000,
		Z: positionECEF.Z * 1000,
	}, nil
}

func validateECIState(state ECIState) error {
	if !vectorFinite(state.PositionKM) || !vectorFinite(state.VelocityKMPerSec) {
		return fmt.Errorf("propagation produced non-finite state vector")
	}

	radius := state.PositionKM.Magnitude()
	if radius < minReasonableOrbitRadiusKM || radius > maxReasonableOrbitRadiusKM {
		return fmt.Errorf("propagation produced unreasonable orbital radius %.3f km", radius)
	}

	return nil
}

func vectorFinite(v types.Vector) bool {
	return !(math.IsNaN(v.X) || math.IsNaN(v.Y) || math.IsNaN(v.Z) ||
		math.IsInf(v.X, 0) || math.IsInf(v.Y, 0) || math.IsInf(v.Z, 0))
}
