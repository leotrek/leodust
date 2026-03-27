package topology

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/leotrek/leodust/pkg/types"
)

const (
	DefaultOutputFile = "./results/topology/live_topology.json"

	KindSatellite = "satellite"
	KindGround    = "ground"
	KindUnknown   = "unknown"
)

type Snapshot struct {
	Version     int64          `json:"version"`
	Time        time.Time      `json:"time"`
	GeneratedAt time.Time      `json:"generated_at"`
	Nodes       []SnapshotNode `json:"nodes"`
	Links       []SnapshotLink `json:"links"`
}

type SnapshotNode struct {
	Name          string           `json:"name"`
	Kind          string           `json:"kind"`
	ComputingType string           `json:"computing_type"`
	Position      SnapshotPosition `json:"position"`
}

type SnapshotPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type SnapshotLink struct {
	Source         string  `json:"source"`
	Target         string  `json:"target"`
	DistanceMeters float64 `json:"distance_meters"`
	LatencyMs      float64 `json:"latency_ms"`
	BandwidthBps   float64 `json:"bandwidth_bps"`
}

func BuildSnapshot(simulation types.SimulationController, version int64, generatedAt time.Time) Snapshot {
	nodes := simulation.GetAllNodes()
	kinds := nodeKinds(simulation)

	nodeSnapshots := make([]SnapshotNode, 0, len(nodes))
	for _, node := range nodes {
		position := node.GetPosition()
		nodeSnapshots = append(nodeSnapshots, SnapshotNode{
			Name:          node.GetName(),
			Kind:          kinds[node.GetName()],
			ComputingType: node.GetComputing().GetComputingType().String(),
			Position: SnapshotPosition{
				X: position.X,
				Y: position.Y,
				Z: position.Z,
			},
		})
	}
	sort.Slice(nodeSnapshots, func(i, j int) bool {
		return nodeSnapshots[i].Name < nodeSnapshots[j].Name
	})

	linksByKey := make(map[string]SnapshotLink)
	for _, node := range nodes {
		for _, link := range node.GetLinkNodeProtocol().Established() {
			n1, n2 := link.Nodes()
			source, target := canonicalLinkNames(n1.GetName(), n2.GetName())
			key := source + "\x00" + target
			if _, exists := linksByKey[key]; exists {
				continue
			}

			linksByKey[key] = SnapshotLink{
				Source:         source,
				Target:         target,
				DistanceMeters: link.Distance(),
				LatencyMs:      link.Latency(),
				BandwidthBps:   link.Bandwidth(),
			}
		}
	}

	linkSnapshots := make([]SnapshotLink, 0, len(linksByKey))
	for _, link := range linksByKey {
		linkSnapshots = append(linkSnapshots, link)
	}
	sort.Slice(linkSnapshots, func(i, j int) bool {
		if linkSnapshots[i].Source == linkSnapshots[j].Source {
			return linkSnapshots[i].Target < linkSnapshots[j].Target
		}
		return linkSnapshots[i].Source < linkSnapshots[j].Source
	})

	return Snapshot{
		Version:     version,
		Time:        simulation.GetSimulationTime().UTC(),
		GeneratedAt: generatedAt.UTC(),
		Nodes:       nodeSnapshots,
		Links:       linkSnapshots,
	}
}

func SaveSnapshot(path string, snapshot Snapshot) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("topology snapshot path cannot be empty")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create topology directory %s: %w", dir, err)
	}

	file, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create topology temp file: %w", err)
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
		return fmt.Errorf("encode topology snapshot: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close topology temp file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename topology snapshot into place: %w", err)
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
		return Snapshot{}, fmt.Errorf("decode topology snapshot %s: %w", path, err)
	}
	return snapshot, nil
}

func nodeKinds(simulation types.SimulationController) map[string]string {
	kinds := make(map[string]string)
	for _, satellite := range simulation.GetSatellites() {
		kinds[satellite.GetName()] = KindSatellite
	}
	for _, ground := range simulation.GetGroundStations() {
		kinds[ground.GetName()] = KindGround
	}
	for _, node := range simulation.GetAllNodes() {
		if _, exists := kinds[node.GetName()]; !exists {
			kinds[node.GetName()] = KindUnknown
		}
	}
	return kinds
}

func canonicalLinkNames(a, b string) (string, string) {
	if a <= b {
		return a, b
	}
	return b, a
}
