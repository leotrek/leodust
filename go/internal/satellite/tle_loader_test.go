package satellite

import (
	"strings"
	"testing"
	"time"

	"github.com/leotrek/leodust/configs"
	"github.com/leotrek/leodust/internal/computing"
	"github.com/leotrek/leodust/internal/routing"
	"github.com/leotrek/leodust/pkg/types"
)

const (
	validLine1 = "1 51880U 22022AE  26070.52430208  .00000741  00000+0  64608-4 0  9998"
	validLine2 = "2 51880  53.2187   0.6017 0001344  94.5762 265.5384 15.08845836223399"
)

func TestValidateTLELinesAcceptsValidRecord(t *testing.T) {
	if err := validateTLELines(validLine1, validLine2); err != nil {
		t.Fatalf("validateTLELines returned error for a valid TLE: %v", err)
	}
}

func TestValidateTLELinesRejectsBadMeanMotion(t *testing.T) {
	invalidLine2 := "2 51880  53.2187   0.6017 0001344  94.5762 265.5384 XX.08845836223399"

	if err := validateTLELines(validLine1, invalidLine2); err == nil {
		t.Fatal("validateTLELines accepted an invalid mean motion field")
	}
}

func TestTleLoaderLoadUsesDisplayNameLine(t *testing.T) {
	builder := NewSatelliteBuilder(
		time.Date(2026, time.March, 11, 0, 0, 0, 0, time.UTC),
		routing.NewRouterBuilder(configs.RouterConfig{Protocol: "a-star"}),
		computing.NewComputingBuilder([]configs.ComputingConfig{
			{Type: types.Edge, Cores: 1, Memory: 1},
		}),
		configs.InterSatelliteLinkConfig{Protocol: "nearest", Neighbours: 1},
	)
	loader := NewTleLoader(builder)

	satellites, err := loader.Load(strings.NewReader("STARLINK-3566\n" + validLine1 + "\n" + validLine2 + "\n"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(satellites) != 1 {
		t.Fatalf("expected one satellite, got %d", len(satellites))
	}
	if satellites[0].GetName() != "STARLINK-3566" {
		t.Fatalf("expected display name to be used, got %q", satellites[0].GetName())
	}
	if satellites[0].GetPosition().Magnitude() == 0 {
		t.Fatal("expected propagated satellite position to be initialized")
	}
}

func TestNewTLERecordFallsBackToCatalogNumber(t *testing.T) {
	record, err := NewTLERecord("", validLine1, validLine2)
	if err != nil {
		t.Fatalf("NewTLERecord returned error: %v", err)
	}
	if record.Name != "51880" {
		t.Fatalf("expected catalog-number fallback, got %q", record.Name)
	}
}
