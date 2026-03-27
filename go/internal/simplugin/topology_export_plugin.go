package simplugin

import (
	"strings"
	"time"

	"github.com/leotrek/leodust/internal/topology"
	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

var _ types.SimulationPlugin = (*TopologyExportPlugin)(nil)

type TopologyExportPlugin struct {
	outputFile string
	version    int64
}

func NewTopologyExportPlugin(outputFile string) *TopologyExportPlugin {
	outputFile = strings.TrimSpace(outputFile)
	if outputFile == "" {
		outputFile = topology.DefaultOutputFile
	}

	return &TopologyExportPlugin{
		outputFile: outputFile,
	}
}

func (p *TopologyExportPlugin) Name() string {
	return "TopologyExportPlugin"
}

func (p *TopologyExportPlugin) PostSimulationStep(simulation types.SimulationController) error {
	p.version++
	snapshot := topology.BuildSnapshot(simulation, p.version, time.Now().UTC())
	if err := topology.SaveSnapshot(p.outputFile, snapshot); err != nil {
		return err
	}

	logging.Debugf(
		"TopologyExportPlugin: wrote topology snapshot version %d with %d nodes and %d links to %s",
		snapshot.Version,
		len(snapshot.Nodes),
		len(snapshot.Links),
		p.outputFile,
	)
	return nil
}
