package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type DesiredState struct {
	Version      int64
	Time         time.Time
	GeneratedAt  time.Time
	Nodes        map[string]DesiredNode
	Edges        map[string]DesiredEdge
	RoutesByNode map[string][]DesiredRoute
}

type DesiredNode struct {
	Name            string
	ContainerName   string
	Kind            string
	ComputingType   string
	HostedWorkloads []string
	Endpoint        bool
	Relay           bool
}

func (n DesiredNode) SandboxRole() string {
	if n.Endpoint {
		return "endpoint"
	}
	return "relay"
}

type DesiredEdge struct {
	Source       string
	Target       string
	LatencyMs    float64
	BandwidthBps float64
}

type DesiredRoute struct {
	DestinationNode string
	NextHopNode     string
}

func BuildDesiredState(snapshot Snapshot, clusterName string) (DesiredState, error) {
	nodes := make(map[string]DesiredNode, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		hosted := append([]string(nil), node.HostedWorkloads...)
		sort.Strings(hosted)
		nodes[node.Name] = DesiredNode{
			Name:            node.Name,
			ContainerName:   sandboxContainerName(clusterName, node.Name),
			Kind:            node.Kind,
			ComputingType:   node.ComputingType,
			HostedWorkloads: hosted,
		}
	}

	for _, workload := range snapshot.Workloads {
		if strings.TrimSpace(workload.HostNode) == "" {
			continue
		}
		node, ok := nodes[workload.HostNode]
		if !ok {
			return DesiredState{}, fmt.Errorf("workload %s references unknown host node %s", workload.Name, workload.HostNode)
		}
		node.Endpoint = true
		nodes[workload.HostNode] = node
	}

	linkByPair := make(map[string]SnapshotLink, len(snapshot.Links))
	for _, link := range snapshot.Links {
		linkByPair[edgeKey(link.Source, link.Target)] = link
	}

	edges := make(map[string]DesiredEdge)
	routeMap := make(map[string]map[string]string)
	for _, route := range snapshot.Routes {
		if len(route.Hops) < 2 {
			continue
		}
		for _, hop := range route.Hops {
			if _, ok := nodes[hop]; !ok {
				return DesiredState{}, fmt.Errorf("route %s->%s references unknown hop %s", route.SourceNode, route.TargetNode, hop)
			}
		}

		for i := 1; i < len(route.Hops)-1; i++ {
			node := nodes[route.Hops[i]]
			node.Relay = true
			nodes[route.Hops[i]] = node
		}

		for i := 0; i < len(route.Hops)-1; i++ {
			source := route.Hops[i]
			target := route.Hops[i+1]
			link, ok := linkByPair[edgeKey(source, target)]
			if !ok {
				return DesiredState{}, fmt.Errorf("route %s->%s uses hop %s->%s with no active link", route.SourceNode, route.TargetNode, source, target)
			}
			edges[edgeKey(source, target)] = DesiredEdge{
				Source:       source,
				Target:       target,
				LatencyMs:    link.LatencyMs,
				BandwidthBps: link.BandwidthBps,
			}
		}

		if err := addDirectionalRouteEntries(routeMap, route.Hops, route.TargetNode); err != nil {
			return DesiredState{}, err
		}
		reverse := append([]string(nil), route.Hops...)
		reversePath(reverse)
		if err := addDirectionalRouteEntries(routeMap, reverse, route.SourceNode); err != nil {
			return DesiredState{}, err
		}
	}

	routesByNode := make(map[string][]DesiredRoute, len(routeMap))
	for nodeName, destinationMap := range routeMap {
		routes := make([]DesiredRoute, 0, len(destinationMap))
		for destination, nextHop := range destinationMap {
			routes = append(routes, DesiredRoute{
				DestinationNode: destination,
				NextHopNode:     nextHop,
			})
		}
		sort.Slice(routes, func(i, j int) bool {
			if routes[i].DestinationNode == routes[j].DestinationNode {
				return routes[i].NextHopNode < routes[j].NextHopNode
			}
			return routes[i].DestinationNode < routes[j].DestinationNode
		})
		routesByNode[nodeName] = routes
	}

	activeNodes := make(map[string]DesiredNode)
	for name, node := range nodes {
		if node.Endpoint || node.Relay {
			activeNodes[name] = node
		}
	}

	return DesiredState{
		Version:      snapshot.Version,
		Time:         snapshot.Time,
		GeneratedAt:  snapshot.GeneratedAt,
		Nodes:        activeNodes,
		Edges:        edges,
		RoutesByNode: routesByNode,
	}, nil
}

func addDirectionalRouteEntries(routeMap map[string]map[string]string, hops []string, destination string) error {
	if len(hops) < 2 {
		return nil
	}
	for i := 0; i < len(hops)-1; i++ {
		nodeName := hops[i]
		nextHop := hops[i+1]
		if routeMap[nodeName] == nil {
			routeMap[nodeName] = make(map[string]string)
		}
		if existing, ok := routeMap[nodeName][destination]; ok && existing != nextHop {
			return fmt.Errorf(
				"conflicting next-hop routes for node %s to destination %s: %s and %s",
				nodeName,
				destination,
				existing,
				nextHop,
			)
		}
		routeMap[nodeName][destination] = nextHop
	}
	return nil
}

func reversePath(values []string) {
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}
}

func edgeKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "||" + b
}

func sandboxContainerName(clusterName, nodeName string) string {
	return clusterName + "-sat-" + sanitizeInstanceName(nodeName)
}

func sanitizeInstanceName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "node"
	}
	return result
}
