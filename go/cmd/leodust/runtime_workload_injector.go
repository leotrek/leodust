package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/leotrek/leodust/internal/deployment"
	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

const defaultInjectedWorkloadPrefix = "runtime-test"

type workloadInjectionOptions struct {
	Count  int
	CPU    float64
	Memory float64
	Target string
	Prefix string
}

func injectSyntheticWorkloads(simulationController types.SimulationController, options workloadInjectionOptions) error {
	if options.Count == 0 {
		return nil
	}
	if options.Count < 0 {
		return fmt.Errorf("injectTestWorkloads must be >= 0, got %d", options.Count)
	}
	if options.CPU <= 0 {
		return fmt.Errorf("injectTestWorkloadCPU must be > 0, got %f", options.CPU)
	}
	if options.Memory <= 0 {
		return fmt.Errorf("injectTestWorkloadMemory must be > 0, got %f", options.Memory)
	}

	target := strings.ToLower(strings.TrimSpace(options.Target))
	if target == "" {
		target = "satellites"
	}
	prefix := strings.TrimSpace(options.Prefix)
	if prefix == "" {
		prefix = defaultInjectedWorkloadPrefix
	}

	candidates, err := workloadInjectionCandidates(simulationController, target, options.CPU, options.Memory)
	if err != nil {
		return err
	}
	if len(candidates) < options.Count {
		return fmt.Errorf(
			"injectTestWorkloads=%d requested %s with at least %.3f CPU and %.3f memory, but only %d eligible nodes are available",
			options.Count,
			target,
			options.CPU,
			options.Memory,
			len(candidates),
		)
	}

	placements := make([]string, 0, options.Count)
	for i := 0; i < options.Count; i++ {
		node := candidates[i]
		serviceName := fmt.Sprintf("%s-%d", prefix, i+1)
		service, err := deployment.NewDeployableService(serviceName, options.CPU, options.Memory)
		if err != nil {
			return fmt.Errorf("build synthetic workload %s: %w", serviceName, err)
		}
		placed, err := node.GetComputing().TryPlaceDeploymentAsync(service)
		if err != nil {
			return fmt.Errorf("place synthetic workload %s on %s: %w", serviceName, node.GetName(), err)
		}
		if !placed {
			return fmt.Errorf("place synthetic workload %s on %s: node rejected placement", serviceName, node.GetName())
		}
		if err := service.Deploy(); err != nil {
			return fmt.Errorf("mark synthetic workload %s as deployed on %s: %w", serviceName, node.GetName(), err)
		}
		placements = append(placements, fmt.Sprintf("%s=%s", node.GetName(), serviceName))
	}

	logging.Infof(
		"Injected %d synthetic test workload(s) on %s: %s",
		options.Count,
		target,
		strings.Join(placements, ", "),
	)
	return nil
}

func workloadInjectionCandidates(simulationController types.SimulationController, target string, cpu, memory float64) ([]types.Node, error) {
	var nodes []types.Node
	switch target {
	case "satellites":
		for _, satellite := range simulationController.GetSatellites() {
			nodes = append(nodes, satellite)
		}
	case "grounds":
		for _, ground := range simulationController.GetGroundStations() {
			nodes = append(nodes, ground)
		}
	case "all":
		nodes = append(nodes, simulationController.GetAllNodes()...)
	default:
		return nil, fmt.Errorf("unknown injectTestWorkloadTarget %q: want satellites, grounds, or all", target)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].GetName() < nodes[j].GetName()
	})

	candidates := make([]types.Node, 0, len(nodes))
	for _, node := range nodes {
		computing := node.GetComputing()
		if computing == nil {
			continue
		}
		if computing.CpuAvailable() < cpu || computing.MemoryAvailable() < memory {
			continue
		}
		candidates = append(candidates, node)
	}
	return candidates, nil
}
