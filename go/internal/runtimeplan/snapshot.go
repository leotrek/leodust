package runtimeplan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/leotrek/leodust/internal/topology"
	"github.com/leotrek/leodust/pkg/types"
)

const DefaultOutputFile = "./results/runtime/live_runtime_plan.json"

type Snapshot struct {
	Version     int64                   `json:"version"`
	Time        time.Time               `json:"time"`
	GeneratedAt time.Time               `json:"generated_at"`
	Nodes       []SnapshotNode          `json:"nodes"`
	Links       []topology.SnapshotLink `json:"links"`
	Workloads   []SnapshotWorkload      `json:"workloads"`
	Routes      []SnapshotRoute         `json:"routes"`
}

type SnapshotNode struct {
	Name            string                    `json:"name"`
	Kind            string                    `json:"kind"`
	ComputingType   string                    `json:"computing_type"`
	Position        topology.SnapshotPosition `json:"position"`
	TotalCPU        float64                   `json:"total_cpu"`
	AvailableCPU    float64                   `json:"available_cpu"`
	TotalMemory     float64                   `json:"total_memory"`
	AvailableMemory float64                   `json:"available_memory"`
	HostedWorkloads []string                  `json:"hosted_workloads"`
}

type SnapshotWorkload struct {
	Name     string  `json:"name"`
	HostNode string  `json:"host_node"`
	CPU      float64 `json:"cpu"`
	Memory   float64 `json:"memory"`
}

type SnapshotRoute struct {
	SourceNode string   `json:"source_node"`
	TargetNode string   `json:"target_node"`
	Hops       []string `json:"hops"`
	LatencyMs  float64  `json:"latency_ms"`
}

func BuildSnapshot(simulation types.SimulationController, version int64, generatedAt time.Time) Snapshot {
	topologySnapshot := topology.BuildSnapshot(simulation, version, generatedAt)
	nodes := simulation.GetAllNodes()

	nodeSnapshots := make([]SnapshotNode, 0, len(nodes))
	workloadSnapshots := make([]SnapshotWorkload, 0)

	nodeKinds := make(map[string]string, len(topologySnapshot.Nodes))
	nodeLinks := topologySnapshot.Links
	nodePositions := make(map[string]topology.SnapshotPosition, len(topologySnapshot.Nodes))
	nodeComputingTypes := make(map[string]string, len(topologySnapshot.Nodes))
	for _, node := range topologySnapshot.Nodes {
		nodeKinds[node.Name] = node.Kind
		nodePositions[node.Name] = node.Position
		nodeComputingTypes[node.Name] = node.ComputingType
	}

	for _, node := range nodes {
		computing := node.GetComputing()
		hostedWorkloads := []string{}
		totalCPU := 0.0
		availableCPU := 0.0
		totalMemory := 0.0
		availableMemory := 0.0

		if computing != nil {
			availableCPU = computing.CpuAvailable()
			availableMemory = computing.MemoryAvailable()
			usedCPU := 0.0
			usedMemory := 0.0
			for _, service := range computing.GetServices() {
				hostedWorkloads = append(hostedWorkloads, service.GetServiceName())
				workloadSnapshots = append(workloadSnapshots, SnapshotWorkload{
					Name:     service.GetServiceName(),
					HostNode: node.GetName(),
					CPU:      service.GetCpuUsage(),
					Memory:   service.GetMemoryUsage(),
				})
				usedCPU += service.GetCpuUsage()
				usedMemory += service.GetMemoryUsage()
			}
			totalCPU = availableCPU + usedCPU
			totalMemory = availableMemory + usedMemory
		}

		sort.Strings(hostedWorkloads)
		nodeSnapshots = append(nodeSnapshots, SnapshotNode{
			Name:            node.GetName(),
			Kind:            nodeKinds[node.GetName()],
			ComputingType:   nodeComputingTypes[node.GetName()],
			Position:        nodePositions[node.GetName()],
			TotalCPU:        totalCPU,
			AvailableCPU:    availableCPU,
			TotalMemory:     totalMemory,
			AvailableMemory: availableMemory,
			HostedWorkloads: hostedWorkloads,
		})
	}

	sort.Slice(nodeSnapshots, func(i, j int) bool {
		return nodeSnapshots[i].Name < nodeSnapshots[j].Name
	})
	sort.Slice(workloadSnapshots, func(i, j int) bool {
		if workloadSnapshots[i].HostNode == workloadSnapshots[j].HostNode {
			return workloadSnapshots[i].Name < workloadSnapshots[j].Name
		}
		return workloadSnapshots[i].HostNode < workloadSnapshots[j].HostNode
	})

	routes := buildHostedWorkloadRoutes(nodeLinks, workloadSnapshots)

	return Snapshot{
		Version:     topologySnapshot.Version,
		Time:        topologySnapshot.Time,
		GeneratedAt: topologySnapshot.GeneratedAt,
		Nodes:       nodeSnapshots,
		Links:       nodeLinks,
		Workloads:   workloadSnapshots,
		Routes:      routes,
	}
}

func SaveSnapshot(path string, snapshot Snapshot) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("runtime snapshot path cannot be empty")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create runtime directory %s: %w", dir, err)
	}

	file, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create runtime temp file: %w", err)
	}
	tempPath := file.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode runtime snapshot: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close runtime temp file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename runtime snapshot into place: %w", err)
	}

	cleanup = false
	return nil
}

func LoadSnapshot(path string) (Snapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return Snapshot{}, err
	}
	defer file.Close()

	var snapshot Snapshot
	if err := json.NewDecoder(file).Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode runtime snapshot %s: %w", path, err)
	}
	return snapshot, nil
}

type pathEdge struct {
	to      string
	latency float64
}

func buildHostedWorkloadRoutes(links []topology.SnapshotLink, workloads []SnapshotWorkload) []SnapshotRoute {
	hostSet := make(map[string]struct{})
	for _, workload := range workloads {
		if strings.TrimSpace(workload.HostNode) == "" {
			continue
		}
		hostSet[workload.HostNode] = struct{}{}
	}

	if len(hostSet) < 2 {
		return []SnapshotRoute{}
	}

	hosts := make([]string, 0, len(hostSet))
	for host := range hostSet {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	graph := make(map[string][]pathEdge)
	for _, link := range links {
		graph[link.Source] = append(graph[link.Source], pathEdge{to: link.Target, latency: link.LatencyMs})
		graph[link.Target] = append(graph[link.Target], pathEdge{to: link.Source, latency: link.LatencyMs})
	}

	routes := make([]SnapshotRoute, 0)
	for i := 0; i < len(hosts); i++ {
		for j := i + 1; j < len(hosts); j++ {
			hops, latency, ok := shortestPath(graph, hosts[i], hosts[j])
			if !ok {
				continue
			}
			routes = append(routes, SnapshotRoute{
				SourceNode: hosts[i],
				TargetNode: hosts[j],
				Hops:       hops,
				LatencyMs:  latency,
			})
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].SourceNode == routes[j].SourceNode {
			return routes[i].TargetNode < routes[j].TargetNode
		}
		return routes[i].SourceNode < routes[j].SourceNode
	})

	return routes
}

func shortestPath(graph map[string][]pathEdge, source string, target string) ([]string, float64, bool) {
	if source == target {
		return []string{source}, 0, true
	}

	dist := map[string]float64{source: 0}
	prev := make(map[string]string)
	visited := make(map[string]bool)

	for {
		current := ""
		currentDistance := 0.0
		found := false
		for node, distance := range dist {
			if visited[node] {
				continue
			}
			if !found || distance < currentDistance || (distance == currentDistance && node < current) {
				current = node
				currentDistance = distance
				found = true
			}
		}

		if !found {
			return nil, 0, false
		}
		if current == target {
			break
		}

		visited[current] = true
		for _, edge := range graph[current] {
			if visited[edge.to] {
				continue
			}
			candidate := currentDistance + edge.latency
			existing, ok := dist[edge.to]
			if !ok || candidate < existing {
				dist[edge.to] = candidate
				prev[edge.to] = current
			}
		}
	}

	hops := []string{target}
	for current := target; current != source; {
		previous, ok := prev[current]
		if !ok {
			return nil, 0, false
		}
		hops = append(hops, previous)
		current = previous
	}

	for left, right := 0, len(hops)-1; left < right; left, right = left+1, right-1 {
		hops[left], hops[right] = hops[right], hops[left]
	}

	return hops, dist[target], true
}
