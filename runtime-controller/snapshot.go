package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const DefaultRuntimeFile = "./go/results/runtime/live_runtime_plan.json"

type Snapshot struct {
	Version     int64              `json:"version"`
	Time        time.Time          `json:"time"`
	GeneratedAt time.Time          `json:"generated_at"`
	Nodes       []SnapshotNode     `json:"nodes"`
	Links       []SnapshotLink     `json:"links"`
	Workloads   []SnapshotWorkload `json:"workloads"`
	Routes      []SnapshotRoute    `json:"routes"`
}

type SnapshotNode struct {
	Name            string           `json:"name"`
	Kind            string           `json:"kind"`
	ComputingType   string           `json:"computing_type"`
	Position        SnapshotPosition `json:"position"`
	TotalCPU        float64          `json:"total_cpu"`
	AvailableCPU    float64          `json:"available_cpu"`
	TotalMemory     float64          `json:"total_memory"`
	AvailableMemory float64          `json:"available_memory"`
	HostedWorkloads []string         `json:"hosted_workloads"`
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
