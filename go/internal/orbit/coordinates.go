package orbit

import (
	"math"

	"github.com/leotrek/leodust/pkg/types"
)

const (
	wgs84SemiMajorAxis = 6378137.0
	wgs84SemiMinorAxis = 6356752.314245
)

// GeodeticToECEF converts latitude, longitude, and altitude to ECEF meters.
func GeodeticToECEF(latitudeDeg, longitudeDeg, altitudeMeters float64) types.Vector {
	latRad := types.DegreesToRadians(latitudeDeg)
	lonRad := types.DegreesToRadians(longitudeDeg)

	eccentricitySquared := 1 - (wgs84SemiMinorAxis*wgs84SemiMinorAxis)/(wgs84SemiMajorAxis*wgs84SemiMajorAxis)
	radius := wgs84SemiMajorAxis / math.Sqrt(1-eccentricitySquared*math.Sin(latRad)*math.Sin(latRad))

	return types.Vector{
		X: (radius + altitudeMeters) * math.Cos(latRad) * math.Cos(lonRad),
		Y: (radius + altitudeMeters) * math.Cos(latRad) * math.Sin(lonRad),
		Z: ((1-eccentricitySquared)*radius + altitudeMeters) * math.Sin(latRad),
	}
}

// GroundSatelliteVisible reports whether the satellite is above the local horizon.
func GroundSatelliteVisible(groundStation, satellite types.Vector) bool {
	lineOfSight := types.Vector{
		X: satellite.X - groundStation.X,
		Y: satellite.Y - groundStation.Y,
		Z: satellite.Z - groundStation.Z,
	}

	// A positive projection onto the surface normal means the satellite sits above
	// the tangent plane at the observer.
	return lineOfSight.Dot(groundStation.Normalize()) > 0
}
