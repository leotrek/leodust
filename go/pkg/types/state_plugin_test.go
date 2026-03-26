package types

import (
	"reflect"
	"testing"
)

type statePluginTestInterface interface {
	StatePlugin
	TestValue() string
}

type statePluginTestImpl struct{}

func (p *statePluginTestImpl) GetName() string { return "test" }
func (p *statePluginTestImpl) GetType() reflect.Type {
	var plugin statePluginTestInterface
	return reflect.TypeOf(plugin)
}
func (p *statePluginTestImpl) PostSimulationStep(SimulationController) {}
func (p *statePluginTestImpl) AddState(SimulationController)           {}
func (p *statePluginTestImpl) Save(string)                             {}
func (p *statePluginTestImpl) TestValue() string                       { return "ok" }

func TestFindStatePluginReturnsRegisteredPlugin(t *testing.T) {
	repo := NewStatePluginRepository([]StatePlugin{&statePluginTestImpl{}})

	plugin, ok := FindStatePlugin[statePluginTestInterface](repo)
	if !ok {
		t.Fatal("FindStatePlugin did not find registered plugin")
	}
	if got := plugin.TestValue(); got != "ok" {
		t.Fatalf("FindStatePlugin returned plugin value %q, want %q", got, "ok")
	}
}

func TestFindStatePluginReturnsFalseWhenMissing(t *testing.T) {
	repo := NewStatePluginRepository(nil)

	plugin, ok := FindStatePlugin[statePluginTestInterface](repo)
	if ok {
		t.Fatal("FindStatePlugin unexpectedly found a plugin")
	}
	if plugin != nil {
		t.Fatal("FindStatePlugin returned a non-nil plugin for missing entry")
	}
}
