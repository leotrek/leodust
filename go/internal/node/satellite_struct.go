package node

import (
	"time"

	"github.com/leotrek/leodust/internal/orbit"
	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

var _ types.Satellite = (*SatelliteStruct)(nil) // Ensure SatelliteStruct implements Satellite

// SatelliteStruct represents a single satellite node in the simulation.
type SatelliteStruct struct {
	// Implementing Node methods via the embedded Node struct
	BaseNode // Embedding BaseNode struct to satisfy the Node interface

	propagator  orbit.Propagator
	ISLProtocol types.InterSatelliteLinkProtocol
}

// NewSatellite initializes a new Satellite object with a TLE propagator and ISL protocol.
func NewSatellite(name string, propagator orbit.Propagator, simTime time.Time, isl types.InterSatelliteLinkProtocol, router types.Router, computing types.Computing) *SatelliteStruct {
	s := &SatelliteStruct{
		BaseNode:    BaseNode{Name: name, Router: router, Computing: computing},
		propagator:  propagator,
		ISLProtocol: isl,
	}

	isl.Mount(s)
	router.Mount(s)
	computing.Mount(s)
	s.UpdatePosition(simTime)
	return s
}

// UpdatePosition propagates the TLE and stores the result in ECEF meters.
func (s *SatelliteStruct) UpdatePosition(simTime time.Time) {
	position, err := s.propagator.PositionECEF(simTime)
	if err != nil {
		logging.Errorf("Failed to propagate satellite %s at %s: %v", s.GetName(), simTime.Format(time.RFC3339), err)
		return
	}

	s.Position = position
	logging.Debugf("Satellite %s propagated to ECEF %+v at %s", s.GetName(), position, simTime.UTC().Format(time.RFC3339))
}

func (s *SatelliteStruct) GetLinkNodeProtocol() types.LinkNodeProtocol {
	return s.ISLProtocol
}

func (s *SatelliteStruct) GetISLProtocol() types.InterSatelliteLinkProtocol {
	return s.ISLProtocol
}
