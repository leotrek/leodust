package satellite

import (
	"time"

	"github.com/leotrek/leodust/configs"
	"github.com/leotrek/leodust/internal/computing"
	"github.com/leotrek/leodust/internal/links"
	"github.com/leotrek/leodust/internal/node"
	"github.com/leotrek/leodust/internal/orbit"
	"github.com/leotrek/leodust/internal/routing"
	"github.com/leotrek/leodust/pkg/types"
)

// SatelliteBuilder helps construct Satellite instances with ISL, routing, and computing configuration.
type SatelliteBuilder struct {
	initialTime time.Time

	routerBuilder    *routing.RouterBuilder
	computingBuilder *computing.DefaultComputingBuilder
	islBuilder       *links.IslProtocolBuilder
}

// NewSatelliteBuilder creates a new SatelliteBuilder with required dependencies.
func NewSatelliteBuilder(initialTime time.Time, routerBuilder *routing.RouterBuilder, computing *computing.DefaultComputingBuilder, islConfig configs.InterSatelliteLinkConfig) *SatelliteBuilder {
	return &SatelliteBuilder{
		initialTime:      initialTime,
		routerBuilder:    routerBuilder,
		computingBuilder: computing,
		islBuilder:       links.NewIslProtocolBuilder(islConfig),
	}
}

// Build constructs one satellite from a validated TLE record.
func (b *SatelliteBuilder) Build(record TLERecord) (types.Satellite, error) {
	router, err := b.routerBuilder.Build()
	if err != nil {
		return nil, err
	}

	propagator, err := orbit.NewTLEPropagator(record.Line1, record.Line2)
	if err != nil {
		return nil, err
	}

	return node.NewSatellite(
		record.Name,
		propagator,
		b.initialTime,
		// Keep one shared ISL builder so protocols that rely on shared inner state,
		// such as MST-style variants, still coordinate across the whole constellation.
		b.islBuilder.Build(),
		router,
		b.computingBuilder.BuildWithType(types.Edge),
	), nil
}
