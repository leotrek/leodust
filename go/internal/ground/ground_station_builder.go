package ground

import (
	"fmt"

	"github.com/leotrek/leodust/configs"
	"github.com/leotrek/leodust/internal/computing"
	"github.com/leotrek/leodust/internal/links"
	"github.com/leotrek/leodust/internal/node"
	"github.com/leotrek/leodust/internal/routing"
	"github.com/leotrek/leodust/pkg/types"
)

// GroundStationSpec captures the per-station fields loaded from configuration.
type GroundStationSpec struct {
	Name          string
	Latitude      float64
	Longitude     float64
	Altitude      float64
	Protocol      string
	ComputingType string
}

// GroundStationBuilder creates ground stations from immutable shared dependencies
// and a per-station spec so one build cannot leak state into the next.
type GroundStationBuilder struct {
	groundLinkConfig configs.GroundLinkConfig
	routerBuilder    *routing.RouterBuilder
	computingBuilder *computing.DefaultComputingBuilder
}

// NewGroundStationBuilder initializes a new GroundStationBuilder.
func NewGroundStationBuilder(router *routing.RouterBuilder, computing *computing.DefaultComputingBuilder, config configs.GroundLinkConfig) *GroundStationBuilder {
	return &GroundStationBuilder{
		groundLinkConfig: config,
		routerBuilder:    router,
		computingBuilder: computing,
	}
}

// Build constructs a ground station from one config record and the current satellite set.
func (b *GroundStationBuilder) Build(spec GroundStationSpec, satellites []types.Satellite) (types.GroundStation, error) {
	router, err := b.routerBuilder.Build()
	if err != nil {
		return nil, err
	}

	groundLinkConfig := b.groundLinkConfig
	if spec.Protocol != "" {
		groundLinkConfig.Protocol = spec.Protocol
	}

	protocol := links.NewGroundProtocolBuilder(groundLinkConfig).
		SetSatellites(satellites).
		Build()
	if protocol == nil {
		return nil, fmt.Errorf("unknown ground link protocol: %s", groundLinkConfig.Protocol)
	}

	computingResource, err := b.buildComputing(spec.ComputingType)
	if err != nil {
		return nil, err
	}

	return node.NewGroundStation(
		spec.Name,
		spec.Latitude,
		spec.Longitude,
		spec.Altitude,
		protocol,
		router,
		computingResource,
	), nil
}

// buildComputing applies the optional per-station override while keeping the shared
// computing builder reusable across all stations.
func (b *GroundStationBuilder) buildComputing(computingType string) (types.Computing, error) {
	if computingType == "" {
		return b.computingBuilder.Build(), nil
	}

	ctype, err := types.ToComputingType(computingType)
	if err != nil {
		return nil, err
	}

	return b.computingBuilder.BuildWithType(ctype), nil
}
