package routing

import (
	"errors"
	"sort"

	"github.com/leotrek/leodust/pkg/types"
)

// DijkstraRouter implements shortest-path routing using Dijkstra's algorithm
// and supports precomputed routing tables.
type DijkstraRouter struct {
	node     types.Node
	routes   map[types.Node]routeEntry
	services map[string]routeEntry
	comparer func(a, b dijkstraEntry) bool
}

type routeEntry struct {
	OutLink types.Link
	Route   types.RouteResult
}

type dijkstraEntry struct {
	Link    types.Link
	Target  types.Node
	Via     types.Link
	Latency float64
}

// NewDijkstraRouter creates a new Dijkstra-based router
func NewDijkstraRouter() *DijkstraRouter {
	return &DijkstraRouter{
		routes:   make(map[types.Node]routeEntry),
		services: make(map[string]routeEntry),
		comparer: func(a, b dijkstraEntry) bool {
			return a.Latency < b.Latency
		},
	}
}

// Mount attaches the router to a node
func (r *DijkstraRouter) Mount(node types.Node) error {
	if r.node != nil {
		return errors.New("router already mounted")
	}
	r.node = node
	return nil
}

// CanPreRouteCalc returns true (Dijkstra supports pre-calculation)
func (r *DijkstraRouter) CanPreRouteCalc() bool { return true }

// CanOnRouteCalc returns true (also usable live)
func (r *DijkstraRouter) CanOnRouteCalc() bool { return true }

// RouteToNode finds a route to a specific node
func (r *DijkstraRouter) RouteToNode(target types.Node, payload types.Payload) (types.RouteResult, error) {
	if r.node == nil {
		return nil, errors.New("router not mounted")
	}
	if r.node == target { // Compare values directly since self is of type types.Node
		// Return a PreRouteResult with 0 latency when self == target
		return NewPreRouteResult(0), nil // Use the function to create the PreRouteResult
	}
	if entry, ok := r.routes[target]; ok {
		return entry.Route, nil
	}
	return UnreachableRouteResultInstance, nil
}

// RouteToService finds a route by service name
func (r *DijkstraRouter) RouteToService(serviceName string, payload types.Payload) (types.RouteResult, error) {
	if r.node == nil {
		return nil, errors.New("router not mounted")
	}

	// Check if the service is hosted on this node's computing
	if r.node.GetComputing().HostsService(serviceName) {
		// Create a PreRouteResult with 0 latency if the service is hosted on this node
		return NewPreRouteResult(0), nil // Use NewPreRouteResult for flexibility
	}

	// If the service exists in the routing table, return the associated route
	if entry, ok := r.services[serviceName]; ok {
		return entry.Route, nil
	}

	// If the service is not reachable, return the UnreachableRouteResultInstance
	return UnreachableRouteResultInstance, nil
}

// CalculateRoutingTable populates all shortest paths using Dijkstra
func (r *DijkstraRouter) CalculateRoutingTable() error {
	if r.node == nil {
		return errors.New("router not mounted")
	}
	r.routes = make(map[types.Node]routeEntry)
	r.services = make(map[string]routeEntry)

	queue := []dijkstraEntry{}
	r.routes[r.node] = routeEntry{}

	// Seed the queue with one-hop routes from the mounted node. Each queue entry
	// keeps the original first hop in Via so completed routes can be emitted as
	// forwarding-table entries instead of full hop-by-hop paths.
	for _, l := range r.node.GetLinkNodeProtocol().Established() {
		queue = append(queue, dijkstraEntry{
			Link:    l,
			Target:  l.GetOther(r.node),
			Via:     l,
			Latency: l.Latency(),
		})
	}

	// Sort the queue based on latency
	sort.Slice(queue, func(i, j int) bool { return r.comparer(queue[i], queue[j]) })

	// Initialize visited map
	visited := map[types.Node]bool{r.node: true}

	// Pop the current shortest candidate, fix it into the routing table, then
	// expand the destination's neighbors with accumulated latency.
	for len(queue) > 0 {
		entry := queue[0]
		queue = queue[1:]

		// Skip already visited targets
		if visited[entry.Target] {
			continue
		}

		visited[entry.Target] = true
		r.routes[entry.Target] = routeEntry{
			OutLink: entry.Via,
			Route:   NewPreRouteResult(int(entry.Latency)),
		}

		// Handle services for the target node
		r.addServicesToRoutes(entry.Target, entry.Latency)

		// Preserve the original Via link when exploring deeper so every discovered
		// route still resolves to the correct first hop from the mounted node.
		for _, link := range entry.Target.GetLinkNodeProtocol().Established() {
			neighbor := link.GetOther(entry.Target)
			if !visited[neighbor] {
				queue = append(queue, dijkstraEntry{
					Link:    link,
					Target:  neighbor,
					Via:     entry.Via,
					Latency: entry.Latency + link.Latency(),
				})
			}
		}

		// Re-sort the queue by latency
		sort.Slice(queue, func(i, j int) bool { return r.comparer(queue[i], queue[j]) })
	}

	return nil
}

// AdvertiseNewServiceAsync pushes a service to neighbors (future use)
func (r *DijkstraRouter) AdvertiseNewServiceAsync(serviceName string) error {
	return nil // placeholder for broadcast mechanism
}

// ReceiveServiceAdvertismentsAsync updates service routes
func (r *DijkstraRouter) ReceiveServiceAdvertismentsAsync(serviceName string, outlink types.Link, route types.RouteResult) error {
	if existing, ok := r.services[serviceName]; ok && existing.Route.Latency() <= route.Latency() {
		return nil
	}
	r.services[serviceName] = routeEntry{outlink, route}
	return nil
}

// addServicesToRoutes helps manage the services associated with a node in the routes map.
func (r *DijkstraRouter) addServicesToRoutes(target types.Node, latency float64) {
	for _, service := range target.GetComputing().GetServices() {
		if _, exists := r.services[service.GetServiceName()]; !exists {
			if len(target.GetLinkNodeProtocol().Links()) > 0 {
				// Services inherit the same latency as the node route. The outlink is
				// only used as a first-hop hint, so the first known link is sufficient.
				r.services[service.GetServiceName()] = routeEntry{
					OutLink: target.GetLinkNodeProtocol().Links()[0],
					Route:   NewPreRouteResult(int(latency)),
				}
			} else {
				r.services[service.GetServiceName()] = routeEntry{
					OutLink: nil,
					Route:   UnreachableRouteResultInstance,
				}
			}
		}
	}
}
