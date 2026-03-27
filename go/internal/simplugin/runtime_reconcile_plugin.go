package simplugin

import (
	"strings"
	"time"

	"github.com/leotrek/leodust/internal/runtimeplan"
	"github.com/leotrek/leodust/pkg/logging"
	"github.com/leotrek/leodust/pkg/types"
)

var _ types.SimulationPlugin = (*RuntimeReconcilePlugin)(nil)

type RuntimeReconcilePlugin struct {
	outputFile string
	version    int64
}

func NewRuntimeReconcilePlugin(outputFile string) *RuntimeReconcilePlugin {
	outputFile = strings.TrimSpace(outputFile)
	if outputFile == "" {
		outputFile = runtimeplan.DefaultOutputFile
	}

	return &RuntimeReconcilePlugin{
		outputFile: outputFile,
	}
}

func (p *RuntimeReconcilePlugin) Name() string {
	return "RuntimeReconcilePlugin"
}

func (p *RuntimeReconcilePlugin) PostSimulationStep(simulation types.SimulationController) error {
	p.version++
	snapshot := runtimeplan.BuildSnapshot(simulation, p.version, time.Now().UTC())
	if err := runtimeplan.SaveSnapshot(p.outputFile, snapshot); err != nil {
		return err
	}

	logging.Debugf(
		"RuntimeReconcilePlugin: wrote runtime snapshot version %d with %d nodes, %d links, and %d workloads to %s",
		snapshot.Version,
		len(snapshot.Nodes),
		len(snapshot.Links),
		len(snapshot.Workloads),
		p.outputFile,
	)
	return nil
}
