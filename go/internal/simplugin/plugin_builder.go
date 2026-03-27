package simplugin

import (
	"fmt"

	"github.com/leotrek/leodust/internal/runtimeplan"
	"github.com/leotrek/leodust/internal/topology"
	"github.com/leotrek/leodust/pkg/types"
)

type SimPluginBuilder struct {
	topologyOutputFile string
	runtimeOutputFile  string
}

// NewPluginBuilder creates a new instance of PluginBuilder
func NewPluginBuilder() *SimPluginBuilder {
	return &SimPluginBuilder{}
}

func (pb *SimPluginBuilder) WithTopologyOutputFile(path string) *SimPluginBuilder {
	pb.topologyOutputFile = path
	return pb
}

func (pb *SimPluginBuilder) WithRuntimeOutputFile(path string) *SimPluginBuilder {
	pb.runtimeOutputFile = path
	return pb
}

// BuildPlugins constructs plugin instances based on provided names
func (pb *SimPluginBuilder) BuildPlugins(pluginNames []string) ([]types.SimulationPlugin, error) {
	var plugins []types.SimulationPlugin
	for _, name := range pluginNames {
		switch name {
		case "DummyPlugin":
			plugins = append(plugins, &DummyPlugin{})
		case "TopologyExportPlugin":
			outputFile := pb.topologyOutputFile
			if outputFile == "" {
				outputFile = topology.DefaultOutputFile
			}
			plugins = append(plugins, NewTopologyExportPlugin(outputFile))
		case "RuntimeReconcilePlugin":
			outputFile := pb.runtimeOutputFile
			if outputFile == "" {
				outputFile = runtimeplan.DefaultOutputFile
			}
			plugins = append(plugins, NewRuntimeReconcilePlugin(outputFile))
		default:
			return nil, fmt.Errorf("unknown plugin: %s", name)
		}
	}
	return plugins, nil
}
