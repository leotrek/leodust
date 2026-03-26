package computing

import (
	"testing"
	"time"

	"github.com/leotrek/leodust/configs"
	"github.com/leotrek/leodust/pkg/types"
)

type computingTestNode struct{}

func (n *computingTestNode) GetName() string                             { return "node" }
func (n *computingTestNode) GetRouter() types.Router                     { return nil }
func (n *computingTestNode) GetComputing() types.Computing               { return nil }
func (n *computingTestNode) GetPosition() types.Vector                   { return types.Vector{} }
func (n *computingTestNode) DistanceTo(other types.Node) float64         { return 0 }
func (n *computingTestNode) UpdatePosition(time.Time)                    {}
func (n *computingTestNode) GetLinkNodeProtocol() types.LinkNodeProtocol { return nil }

type computingTestService struct {
	name   string
	cpu    float64
	memory float64
}

func (s computingTestService) GetServiceName() string  { return s.name }
func (s computingTestService) GetCpuUsage() float64    { return s.cpu }
func (s computingTestService) GetMemoryUsage() float64 { return s.memory }
func (s computingTestService) IsDeployed() bool        { return true }
func (s computingTestService) Deploy() error           { return nil }
func (s computingTestService) Remove() error           { return nil }

func TestTryPlaceDeploymentAsyncRequiresMount(t *testing.T) {
	computing := NewComputing(4, 8, types.Edge)

	placed, err := computing.TryPlaceDeploymentAsync(computingTestService{name: "svc", cpu: 1, memory: 1})
	if err == nil {
		t.Fatal("expected TryPlaceDeploymentAsync to fail when computing is not mounted")
	}
	if placed {
		t.Fatal("expected placement to be rejected when computing is not mounted")
	}
}

func TestTryPlaceAndRemoveDeployment(t *testing.T) {
	computing := NewComputing(4, 8, types.Edge)
	if err := computing.Mount(&computingTestNode{}); err != nil {
		t.Fatalf("Mount returned error: %v", err)
	}

	service := computingTestService{name: "svc", cpu: 1.5, memory: 2}
	placed, err := computing.TryPlaceDeploymentAsync(service)
	if err != nil {
		t.Fatalf("TryPlaceDeploymentAsync returned error: %v", err)
	}
	if !placed {
		t.Fatal("expected service placement to succeed")
	}
	if !computing.HostsService(service.name) {
		t.Fatal("expected computing to host the placed service")
	}
	if computing.CpuAvailable() != 2.5 || computing.MemoryAvailable() != 6 {
		t.Fatalf("unexpected available resources: cpu=%f memory=%f", computing.CpuAvailable(), computing.MemoryAvailable())
	}

	duplicatePlaced, err := computing.TryPlaceDeploymentAsync(service)
	if err != nil {
		t.Fatalf("duplicate TryPlaceDeploymentAsync returned error: %v", err)
	}
	if duplicatePlaced {
		t.Fatal("expected duplicate service placement to be rejected")
	}

	if err := computing.RemoveDeploymentAsync(service); err != nil {
		t.Fatalf("RemoveDeploymentAsync returned error: %v", err)
	}
	if computing.HostsService(service.name) {
		t.Fatal("expected service to be removed")
	}
	if computing.CpuAvailable() != 4 || computing.MemoryAvailable() != 8 {
		t.Fatalf("expected resources to be fully restored, cpu=%f memory=%f", computing.CpuAvailable(), computing.MemoryAvailable())
	}
}

func TestDefaultComputingBuilderSelectsRequestedConfiguration(t *testing.T) {
	builder := NewComputingBuilder([]configs.ComputingConfig{
		{Cores: 0, Memory: 0, Type: types.None},
		{Cores: 16, Memory: 1024, Type: types.Edge},
		{Cores: 32, Memory: 4096, Type: types.Cloud},
	})

	computing := builder.WithComputingType(types.Cloud).Build()
	if computing.GetComputingType() != types.Cloud {
		t.Fatalf("expected Cloud computing, got %s", computing.GetComputingType())
	}
	if computing.Cpu != 32 || computing.Memory != 4096 {
		t.Fatalf("unexpected configuration selected: cpu=%f memory=%f", computing.Cpu, computing.Memory)
	}
}
