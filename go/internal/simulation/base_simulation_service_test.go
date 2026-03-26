package simulation

import (
	"testing"
	"time"

	"github.com/leotrek/leodust/configs"
)

func TestStepBySecondsAndStepByTimeAdvanceSimulationTime(t *testing.T) {
	start := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	cfg := &configs.SimulationConfig{SimulationStartTime: start}

	var service BaseSimulationService
	service = NewBaseSimulationService(cfg, func(next func(time.Time) time.Time) {
		service.setSimulationTime(next(service.GetSimulationTime()))
	})

	service.StepBySeconds(30)
	if got, want := service.GetSimulationTime(), start.Add(30*time.Second); !got.Equal(want) {
		t.Fatalf("unexpected time after StepBySeconds: got %s want %s", got, want)
	}

	target := start.Add(2 * time.Hour)
	service.StepByTime(target)
	if got := service.GetSimulationTime(); !got.Equal(target) {
		t.Fatalf("unexpected time after StepByTime: got %s want %s", got, target)
	}
}

func TestStartAutorunRunsExactConfiguredStepCount(t *testing.T) {
	start := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	cfg := &configs.SimulationConfig{
		StepInterval:        0,
		StepMultiplier:      10,
		StepCount:           3,
		SimulationStartTime: start,
	}

	var calls int
	var service BaseSimulationService
	service = NewBaseSimulationService(cfg, func(next func(time.Time) time.Time) {
		calls++
		service.setSimulationTime(next(service.GetSimulationTime()))
	})

	<-service.StartAutorun()

	if calls != 3 {
		t.Fatalf("expected exactly 3 autorun steps, got %d", calls)
	}
	wantTime := start.Add(30 * time.Second)
	if got := service.GetSimulationTime(); !got.Equal(wantTime) {
		t.Fatalf("unexpected simulation time after autorun: got %s want %s", got, wantTime)
	}
}
