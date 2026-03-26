package ground

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leotrek/leodust/configs"
	"github.com/leotrek/leodust/internal/computing"
	"github.com/leotrek/leodust/internal/routing"
	"github.com/leotrek/leodust/pkg/types"
)

func TestGroundStationYmlLoaderLoadDoesNotLeakBuilderState(t *testing.T) {
	loader := newTestGroundStationLoader()

	path := writeGroundStationFixture(t, `
- Name: edge-station
  Lat: 48.2
  Lon: 16.3
  Alt: 180
  Protocol: nearest
  ComputingType: Edge
- Name: default-station
  Lat: 40.7
  Lon: -74.0
  Protocol: nearest
`)

	stations, err := loader.Load(path, nil)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(stations) != 2 {
		t.Fatalf("expected 2 ground stations, got %d", len(stations))
	}

	if got := stations[0].GetComputing().GetComputingType(); got != types.Edge {
		t.Fatalf("first station computing type = %s, want %s", got, types.Edge)
	}
	if got := stations[1].GetComputing().GetComputingType(); got != types.Cloud {
		t.Fatalf("second station computing type = %s, want %s", got, types.Cloud)
	}
}

func TestGroundStationYmlLoaderLoadRejectsInvalidComputingType(t *testing.T) {
	loader := newTestGroundStationLoader()

	path := writeGroundStationFixture(t, `
- Name: invalid-station
  Lat: 48.2
  Lon: 16.3
  Protocol: nearest
  ComputingType: NotAType
`)

	if _, err := loader.Load(path, nil); err == nil {
		t.Fatal("expected invalid computing type to return an error")
	}
}

func newTestGroundStationLoader() *GroundStationYmlLoader {
	computingBuilder := computing.NewComputingBuilder([]configs.ComputingConfig{
		{Type: types.Cloud, Cores: 8, Memory: 32},
		{Type: types.Edge, Cores: 2, Memory: 4},
	})
	builder := NewGroundStationBuilder(
		routing.NewRouterBuilder(configs.RouterConfig{Protocol: "a-star"}),
		computingBuilder,
		configs.GroundLinkConfig{Protocol: "nearest"},
	)
	return NewGroundStationYmlLoader(builder)
}

func writeGroundStationFixture(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ground_stations.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
	return path
}
