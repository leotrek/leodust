package configs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leotrek/leodust/pkg/types"
)

func TestLoadConfigFromFileYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "simulation.yaml")
	content := []byte(`StepInterval: 5
StepMultiplier: 10
StepCount: 2
LogLevel: debug
SatelliteDataSource: sats.tle
SatelliteDataSourceType: tle
GroundStationDataSource: ground.yml
GroundStationDataSourceType: yml
UsePreRouteCalc: true
SimulationStartTime: "2025-01-01T00:00:00Z"
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := LoadConfigFromFile[SimulationConfig](path)
	if err != nil {
		t.Fatalf("LoadConfigFromFile returned error: %v", err)
	}

	if cfg.StepInterval != 5 || cfg.StepMultiplier != 10 || cfg.StepCount != 2 {
		t.Fatalf("unexpected numeric fields: %+v", cfg)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected LogLevel debug, got %q", cfg.LogLevel)
	}
	if !cfg.UsePreRouteCalc {
		t.Fatal("expected UsePreRouteCalc to be true")
	}
	wantTime := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	if !cfg.SimulationStartTime.Equal(wantTime) {
		t.Fatalf("unexpected SimulationStartTime: got %s want %s", cfg.SimulationStartTime, wantTime)
	}
}

func TestLoadConfigFromFileJSONAndComputingType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "computing.json")
	content := []byte(`[{"Cores":4,"Memory":8192,"Type":"Edge"}]`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := LoadConfigFromFile[[]ComputingConfig](path)
	if err != nil {
		t.Fatalf("LoadConfigFromFile returned error: %v", err)
	}

	if len(*cfg) != 1 {
		t.Fatalf("expected one config entry, got %d", len(*cfg))
	}
	if (*cfg)[0].Type != types.Edge {
		t.Fatalf("expected Edge computing type, got %s", (*cfg)[0].Type)
	}
}

func TestLoadConfigFromFileUnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "simulation.txt")
	if err := os.WriteFile(path, []byte("ignored"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if _, err := LoadConfigFromFile[SimulationConfig](path); err == nil {
		t.Fatal("expected unsupported extension to return an error")
	}
}
