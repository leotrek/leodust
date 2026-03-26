package types

import (
	"reflect"
)

// StatePlugin provides the interface of state plugins
type StatePlugin interface {

	// GetName returns the name of the plugin
	GetName() string

	// GetType return the type/interface of the plugin
	GetType() reflect.Type

	// PostSimulationStep does all the computation to calculate some state of the simulation
	PostSimulationStep(simulationController SimulationController)

	// AddState add the current state to be saved to output file
	AddState(simulationController SimulationController)

	// Save to save the states to file
	Save(filename string)
}

type StatePluginRepository struct {
	plugins map[reflect.Type]StatePlugin
}

// NewStatePluginRepository creates a new StatePluginRepository and initializes it with the provided plugins.
func NewStatePluginRepository(plugins []StatePlugin) *StatePluginRepository {
	repo := &StatePluginRepository{
		plugins: make(map[reflect.Type]StatePlugin),
	}
	for _, plugin := range plugins {
		// Use the concrete type of the plugin as the key
		typ := plugin.GetType()
		repo.plugins[typ] = plugin
	}
	return repo
}

// GetAllPlugins returns all registered plugins.
func (r *StatePluginRepository) GetAllPlugins() []StatePlugin {
	plugins := make([]StatePlugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// FindStatePlugin returns the registered plugin for T when present.
// It is the non-panicking lookup path for optional state plugins.
func FindStatePlugin[T StatePlugin](r *StatePluginRepository) (T, bool) {
	var zero T
	if r == nil {
		return zero, false
	}

	typ := reflect.TypeOf(*new(T))
	plugin, ok := r.plugins[typ]
	if !ok {
		return zero, false
	}

	cast, ok := plugin.(T)
	if !ok {
		return zero, false
	}
	return cast, true
}

// GetStatePlugin is a generic function that retrieves a plugin of type T from the repository.
// It panics if the plugin is not found or if the type assertion fails.
func GetStatePlugin[T StatePlugin](r *StatePluginRepository) T {
	plugin, ok := FindStatePlugin[T](r)
	if !ok {
		panic("plugin not found")
	}
	return plugin
}
