package configs_test

import (
	"path/filepath"
	"testing"

	"github.com/leotrek/leodust/configs"
	"github.com/leotrek/leodust/internal/satellite"
)

func TestBundledSimulationConfigsUseAlignedTLESnapshots(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("..", "resources", "configs", "simulation*.yaml"))
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected bundled simulation configs to exist")
	}

	for _, path := range matches {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			cfg, err := configs.LoadConfigFromFile[configs.SimulationConfig](path)
			if err != nil {
				t.Fatalf("LoadConfigFromFile returned error: %v", err)
			}
			if cfg.SatelliteDataSourceType != "tle" {
				return
			}

			tlePath := filepath.Join("..", "resources", "tle", cfg.SatelliteDataSource)
			summary, err := satellite.SummarizeTLEFile(tlePath)
			if err != nil {
				t.Fatalf("SummarizeTLEFile returned error: %v", err)
			}
			if summary.Empty() {
				t.Fatal("expected TLE source to contain records")
			}
			if distance := summary.DistanceTo(cfg.SimulationStartTime); distance != 0 {
				t.Fatalf(
					"SimulationStartTime %s falls outside TLE epoch range %s..%s by %s",
					cfg.SimulationStartTime,
					summary.EarliestEpoch,
					summary.LatestEpoch,
					distance,
				)
			}
		})
	}
}
