package node

import (
	"time"

	"github.com/leotrek/leodust/internal/orbit"
	"github.com/leotrek/leodust/pkg/types"
)

var _ types.GroundStation = (*GroundStationStruct)(nil)

// GroundStationStruct represents an Earth-based node that links to satellites
// It updates its position over time and tracks the nearest satellites
type GroundStationStruct struct {
	BaseNode

	Latitude                    float64
	Longitude                   float64
	Altitude                    float64
	GroundSatelliteLinkProtocol types.GroundSatelliteLinkProtocol
}

// NewGroundStation creates and initializes a new ground station with link protocol and position
func NewGroundStation(name string, lat float64, lon float64, altitude float64, protocol types.GroundSatelliteLinkProtocol, router types.Router, computing types.Computing) *GroundStationStruct {
	gs := &GroundStationStruct{
		BaseNode: BaseNode{
			Name:      name,
			Router:    router,
			Computing: computing,
		},
		Latitude:                    lat,
		Longitude:                   lon,
		Altitude:                    altitude,
		GroundSatelliteLinkProtocol: protocol,
	}
	protocol.Mount(gs)
	router.Mount(gs)
	computing.Mount(gs)
	gs.Position = orbit.GeodeticToECEF(gs.Latitude, gs.Longitude, gs.Altitude)
	return gs
}

// UpdatePosition refreshes the fixed ECEF ground-station coordinates.
func (gs *GroundStationStruct) UpdatePosition(_ time.Time) {
	gs.Position = orbit.GeodeticToECEF(gs.Latitude, gs.Longitude, gs.Altitude)
}

func (gs *GroundStationStruct) GetLinkNodeProtocol() types.LinkNodeProtocol {
	return gs.GroundSatelliteLinkProtocol
}
