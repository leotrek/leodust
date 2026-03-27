package main

import "testing"

func TestBuildDesiredStateIncludesEndpointsRelaysEdgesAndRoutes(t *testing.T) {
	snapshot := Snapshot{
		Version: 1,
		Nodes: []SnapshotNode{
			{Name: "sat-a", Kind: "satellite"},
			{Name: "sat-b", Kind: "satellite"},
			{Name: "sat-c", Kind: "satellite"},
			{Name: "sat-d", Kind: "satellite"},
		},
		Links: []SnapshotLink{
			{Source: "sat-a", Target: "sat-b", LatencyMs: 1, BandwidthBps: 100},
			{Source: "sat-b", Target: "sat-c", LatencyMs: 2, BandwidthBps: 90},
			{Source: "sat-c", Target: "sat-d", LatencyMs: 3, BandwidthBps: 80},
		},
		Workloads: []SnapshotWorkload{
			{Name: "src", HostNode: "sat-a"},
			{Name: "dst", HostNode: "sat-d"},
		},
		Routes: []SnapshotRoute{
			{
				SourceNode: "sat-a",
				TargetNode: "sat-d",
				Hops:       []string{"sat-a", "sat-b", "sat-c", "sat-d"},
				LatencyMs:  6,
			},
		},
	}

	desired, err := BuildDesiredState(snapshot, "leodust")
	if err != nil {
		t.Fatalf("BuildDesiredState returned error: %v", err)
	}

	if len(desired.Nodes) != 4 {
		t.Fatalf("BuildDesiredState returned %d nodes, want 4", len(desired.Nodes))
	}
	if !desired.Nodes["sat-a"].Endpoint {
		t.Fatalf("sat-a should be an endpoint")
	}
	if !desired.Nodes["sat-b"].Relay || !desired.Nodes["sat-c"].Relay {
		t.Fatalf("sat-b and sat-c should be relays: %+v %+v", desired.Nodes["sat-b"], desired.Nodes["sat-c"])
	}
	if len(desired.Edges) != 3 {
		t.Fatalf("BuildDesiredState returned %d edges, want 3", len(desired.Edges))
	}

	routesA := desired.RoutesByNode["sat-a"]
	if len(routesA) != 1 || routesA[0].DestinationNode != "sat-d" || routesA[0].NextHopNode != "sat-b" {
		t.Fatalf("unexpected routes for sat-a: %+v", routesA)
	}
	routesC := desired.RoutesByNode["sat-c"]
	if len(routesC) != 2 {
		t.Fatalf("unexpected route count for sat-c: %+v", routesC)
	}
	if desired.Nodes["sat-a"].ContainerName != "leodust-sat-sat-a" {
		t.Fatalf("unexpected container name for sat-a: %s", desired.Nodes["sat-a"].ContainerName)
	}
}
