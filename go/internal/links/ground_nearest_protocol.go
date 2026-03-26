package links

import (
	"errors"
	"sort"
	"sync"

	"github.com/leotrek/leodust/internal/links/linktypes"
	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

var _ types.GroundSatelliteLinkProtocol = (*GroundSatelliteNearestProtocol)(nil)

// GroundSatelliteNearestProtocol maintains a single active link from the ground station
// to the nearest satellite at any given time.
type GroundSatelliteNearestProtocol struct {
	link          *linktypes.GroundLink // Current active ground link
	satellites    []types.Satellite     // Available satellites
	groundStation types.Node            // The ground station node
	mu            sync.Mutex
}

// NewGroundSatelliteNearestProtocol creates a new protocol with an initial list of satellites.
func NewGroundSatelliteNearestProtocol(satellites []types.Satellite) types.GroundSatelliteLinkProtocol {
	return &GroundSatelliteNearestProtocol{
		satellites: satellites,
	}
}

// Mount binds this protocol to a ground station.
func (p *GroundSatelliteNearestProtocol) Mount(gs types.Node) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.groundStation == nil {
		p.groundStation = gs
	}
}

// AddLink is a no-op for this protocol.
func (p *GroundSatelliteNearestProtocol) AddLink(link types.Link) {}

// ConnectLink is a no-op for this protocol.
func (p *GroundSatelliteNearestProtocol) ConnectLink(link types.Link) error {
	return nil
}

// DisconnectLink is a no-op for this protocol.
func (p *GroundSatelliteNearestProtocol) DisconnectLink(link types.Link) error {
	return nil
}

// UpdateLinks selects the closest visible satellite and updates the active ground link.
func (p *GroundSatelliteNearestProtocol) UpdateLinks() ([]types.Link, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.groundStation == nil {
		return nil, errors.New("protocol not mounted to ground station")
	}
	if len(p.satellites) == 0 {
		return nil, errors.New("no satellites available")
	}

	reachable := make([]types.Satellite, 0, len(p.satellites))
	for _, satellite := range p.satellites {
		// Filter below-horizon satellites before sorting by range so a geometrically
		// closer but hidden satellite never wins the uplink.
		if linktypes.NewGroundLink(p.groundStation, satellite).IsReachable() {
			reachable = append(reachable, satellite)
		}
	}

	if len(reachable) == 0 {
		if p.link != nil {
			p.link.Satellite.GetLinkNodeProtocol().DisconnectLink(p.link)
			logging.Debugf("Ground station %s lost visibility to satellite %s", p.groundStation.GetName(), p.link.Satellite.GetName())
			p.link = nil
		}
		return nil, nil
	}

	sort.Slice(reachable, func(i, j int) bool {
		var nodeA types.Node = reachable[i]
		var nodeB types.Node = reachable[j]
		return p.groundStation.DistanceTo(nodeA) < p.groundStation.DistanceTo(nodeB)
	})

	nearest := reachable[0]
	if p.link != nil && p.link.Satellite.GetName() == nearest.GetName() {
		return []types.Link{p.link}, nil // Already linked to the nearest
	}

	old := p.link
	p.link = linktypes.NewGroundLink(p.groundStation, nearest)

	// Add new link to satellite if it supports ground links
	nearest.GetLinkNodeProtocol().ConnectLink(p.link)

	// Remove old link from previous satellite if supported
	if old != nil {
		old.Satellite.GetLinkNodeProtocol().DisconnectLink(old)
	}
	if old == nil {
		logging.Debugf("Ground station %s connected to satellite %s", p.groundStation.GetName(), nearest.GetName())
	} else {
		logging.Debugf("Ground station %s switched uplink from %s to %s", p.groundStation.GetName(), old.Satellite.GetName(), nearest.GetName())
	}

	return []types.Link{p.link}, nil
}

// Links returns the current active link if any.
func (p *GroundSatelliteNearestProtocol) Links() []types.Link {
	if p.link != nil {
		return []types.Link{p.link}
	}
	return nil
}

// Established returns the current active link if any.
func (p *GroundSatelliteNearestProtocol) Established() []types.Link {
	return p.Links()
}

// Link returns the currently active GroundLink.
func (p *GroundSatelliteNearestProtocol) Link() types.Link {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.link == nil {
		return nil
	}
	return p.link
}

// AddSatellite adds a satellite to the trackable list.
func (p *GroundSatelliteNearestProtocol) AddSatellite(sat types.Node) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if satellite, ok := sat.(types.Satellite); ok {
		p.satellites = append(p.satellites, satellite)
	}
}

// RemoveSatellite removes a satellite from the list and resets the link if needed.
func (p *GroundSatelliteNearestProtocol) RemoveSatellite(toRemove types.Node) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if satellite, ok := toRemove.(types.Satellite); ok {

		// Filter out the satellite
		filtered := make([]types.Satellite, 0, len(p.satellites))
		for _, s := range p.satellites {
			if s.GetName() != satellite.GetName() {
				filtered = append(filtered, s)
			}
		}
		p.satellites = filtered

		// Remove the link if it's pointing to the removed satellite
		if p.link != nil && p.link.Satellite.GetName() == satellite.GetName() {
			satellite.GetLinkNodeProtocol().DisconnectLink(p.link)
			p.link = nil
		}
	}
}
