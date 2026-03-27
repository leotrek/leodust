# Plugins

LeoDust has two plugin families:

- simulation plugins
- state plugins

They are both extension points, but they solve different problems.

Important:

- there is no separate pre-step or pre-processing plugin hook in the current runtime
- both plugin families run after a simulation step has already updated time, positions, links, and optional routing state
- state plugins also have a replay/precomputed side when their state needs to survive snapshot save/load

## Plugin Families

| Family | Interface | Built by | Runs when | Main use |
|------|-----------|----------|-----------|----------|
| simulation plugin | `types.SimulationPlugin` | `go/internal/simplugin/SimPluginBuilder` | after each step | post-step exports, side effects, analysis |
| state plugin | `types.StatePlugin` | `go/internal/stateplugin/DefaultStatePluginBuilder` | after each step and during snapshot save/load | post-step derived state that should be available during replay |

Use a simulation plugin when:

- you want to observe the current simulation state
- you want to export data to a file or external system
- you do not need extra state to survive replay

Use a state plugin when:

- you want to compute extra per-step state
- that state should be saved alongside `.gob` snapshots
- replay mode or the viewer should be able to read that state later

## Step Order

In the live runtime, `SimulationService.runSimulationStep` currently does this:

1. advance simulation time
2. update node positions
3. update links
4. optionally precompute routing tables
5. run state plugins
6. run simulation plugins
7. append state to the serializer

That order matters:

- state plugins run before simulation plugins
- a simulation plugin can read data from a state plugin through `GetStatePluginRepository()`
- a state plugin should not assume a simulation plugin has already run

## Pre-Step vs Post-Step

Today the plugin model is:

- no pre-step plugin hook
- post-step state plugins
- post-step simulation plugins

If you were looking for “pre-processing” vs “post-processing”, the closest mapping is:

- there is no pre-processing plugin API yet
- state plugins are post-step derived-state hooks
- simulation plugins are post-step side-effect/export hooks

## Built-In Plugins

### Simulation plugins

- `DummyPlugin`
- `RuntimeReconcilePlugin`

### State plugins

- `DummySunStatePlugin`

The CLI flag values come from the builder switch statements, not from generated discovery. Today that means:

- simulation plugin names are registered in `go/internal/simplugin/plugin_builder.go`
- state plugin names are registered in `go/internal/stateplugin/default_state_plugin_builder.go`

## Enable an Existing Plugin

Simulation plugins are enabled with `--simulationPlugins`:

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --simulationPlugins DummyPlugin
```

`RuntimeReconcilePlugin` is the built-in simulation plugin used for runtime execution integration. It writes a per-step runtime plan for the standalone runtime controller:

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --simulationPlugins RuntimeReconcilePlugin \
  --runtimeOutputFile ./results/runtime/live_runtime_plan.json
```

For the controller side, see [Runtime Integration](runtime-integration.md).

State plugins are enabled with `--statePlugins`:

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --statePlugins DummySunStatePlugin
```

## How to Create a New Simulation Plugin

### 1. Add a file under `go/internal/simplugin`

Typical layout:

- `go/internal/simplugin/my_plugin.go`

### 2. Implement `types.SimulationPlugin`

The interface is intentionally small:

```go
type SimulationPlugin interface {
    Name() string
    PostSimulationStep(simulation SimulationController) error
}
```

Minimal example:

```go
package simplugin

import "github.com/leotrek/leodust/pkg/types"

var _ types.SimulationPlugin = (*NodeCountPlugin)(nil)

type NodeCountPlugin struct{}

func (p *NodeCountPlugin) Name() string {
    return "NodeCountPlugin"
}

func (p *NodeCountPlugin) PostSimulationStep(simulation types.SimulationController) error {
    _ = simulation.GetSimulationTime()
    _ = simulation.GetAllNodes()
    return nil
}
```

Useful data exposed through `SimulationController`:

- `GetSimulationTime()`
- `GetAllNodes()`
- `GetSatellites()`
- `GetGroundStations()`
- `GetStatePluginRepository()`

Useful data exposed through `types.Node`:

- `GetName()`
- `GetPosition()`
- `GetComputing()`
- `GetRouter()`
- `GetLinkNodeProtocol()`

### 3. Register the plugin in the simulation plugin builder

Add a new case to `go/internal/simplugin/plugin_builder.go`:

```go
case "NodeCountPlugin":
    plugins = append(plugins, &NodeCountPlugin{})
```

If you skip this step, the CLI will fail with `unknown plugin`.

### 4. Add plugin-specific wiring if the plugin needs configuration

If your plugin needs constructor parameters:

1. add fields to `SimPluginBuilder`
2. add fluent setters such as `WithMyOutputFile(...)`
3. plumb the value from `go/cmd/leodust/main.go`
4. construct the plugin from those builder fields

Example:

- `RuntimeReconcilePlugin` uses `WithRuntimeOutputFile(...)`
- `go/cmd/leodust/main.go` wires that from `--runtimeOutputFile`

### 5. Run it

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --simulationPlugins NodeCountPlugin
```

## How to Create a New State Plugin

State plugins need more code than simulation plugins because they participate in serialization and replay.

### 1. Add a live plugin under `go/internal/stateplugin`

Implement `types.StatePlugin`:

```go
type StatePlugin interface {
    GetName() string
    GetType() reflect.Type
    PostSimulationStep(simulationController SimulationController)
    AddState(simulationController SimulationController)
    Save(filename string)
}
```

Responsibilities:

- `PostSimulationStep`: compute the current derived state
- `AddState`: append the current state to an in-memory history
- `Save`: persist that history when the simulation closes
- `GetType`: define the repository lookup key

### 2. Add a replay/precomputed variant if the state must survive replay

If the plugin writes sidecar state during live simulation and you want replay mode to expose that same state, add the matching precomputed implementation too.

The current example pair is:

- `DummySunStatePlugin`
- `DummySunStatePluginPrecomp`

### 3. Register the plugin in both state-plugin builders

Update:

- `go/internal/stateplugin/default_state_plugin_builder.go`
- `go/internal/stateplugin/default_state_plugin_precomp_builder.go`

If you only update the live builder, replay mode will not know how to load the plugin state.

### 4. Enable it with `--statePlugins`

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --statePlugins MyStatePlugin
```

## Read a State Plugin from a Simulation Plugin

This is the normal pattern when:

- a state plugin computes reusable derived data
- a simulation plugin exports or reacts to that data

Use the state plugin repository from `SimulationController` and the generic lookup helpers in `go/pkg/types/state_plugin.go`.

That keeps the expensive state calculation in one place and lets multiple exporters reuse it.

## Troubleshooting

### `unknown plugin: ...`

Cause:

- the plugin was not added to the correct builder switch

Check:

- `go/internal/simplugin/plugin_builder.go`
- `go/internal/stateplugin/default_state_plugin_builder.go`
- `go/internal/stateplugin/default_state_plugin_precomp_builder.go`

### Replay mode cannot find state plugin data

Cause:

- the live plugin writes sidecar state, but no precomputed variant was registered for replay

### Plugin has no access to the data you need

Check the narrow interfaces first:

- `SimulationController`
- `Node`
- `Link`

If the needed data is not exposed there, expand the interface intentionally instead of reaching into concrete runtime structs from plugin code.
