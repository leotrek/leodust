# Runtime Assembly

This tutorial explains, step by step, how LeoDust turns configuration files into a running simulation.

It is the best page to read when you want to understand:

- which config file controls which subsystem
- which struct builds which runtime object
- how the simulator startup sequence fits together

## Step 1: Start at `go/cmd/leodust/main.go`

The main command is the composition root of the application.

Its job is to:

1. parse CLI flags
2. load config files
3. configure logging
4. optionally download a fresh TLE
5. optionally run orbit validation
6. choose live mode or replay mode

The important consequence is that LeoDust is configured mostly by files, but assembled by code.

## Step 2: Load the Top-Level Config Objects

The config structs live in `go/configs/config.go`.

The main startup path loads:

- `SimulationConfig`
- `InterSatelliteLinkConfig`
- `GroundLinkConfig`
- `RouterConfig`
- `[]ComputingConfig`

At this point nothing has been simulated yet. LeoDust has only parsed configuration data into Go structs.

## Step 3: Build the Shared Builders

After config loading, the main command creates the shared builders that the rest of the runtime depends on.

### `DefaultComputingBuilder`

Built from `[]ComputingConfig`.

Purpose:

- keep the list of available computing profiles
- create fresh computing objects for nodes
- support per-node computing-type overrides

### `RouterBuilder`

Built from `RouterConfig`.

Purpose:

- create one router per node
- select `AStarRouter` or `DijkstraRouter`

### `IslProtocolBuilder`

Created inside `SatelliteBuilder` from `InterSatelliteLinkConfig`.

Purpose:

- decide how satellites will activate inter-satellite links
- reuse shared MST or PST inner objects when the selected protocol needs shared state

## Step 4: Build the Satellite Path

The live satellite pipeline is:

1. `SatelliteBuilder`
2. `TleLoader`
3. `SatelliteConstellationLoader`
4. `SatelliteLoaderService`

### `SatelliteBuilder`

`SatelliteBuilder` receives:

- the initial simulation time
- the `RouterBuilder`
- the `DefaultComputingBuilder`
- the ISL config

For every parsed TLE record it creates:

- a `TLEPropagator`
- a router
- an ISL protocol
- an `Edge` computing instance
- a `SatelliteStruct`

### `TleLoader`

`TleLoader` reads a TLE stream and turns it into `TLERecord` values.

For each record it calls `SatelliteBuilder.Build`.

### `SatelliteConstellationLoader`

This loader is the format registry for satellite sources.

In the current runtime it is configured with:

- source type `tle`
- loader `TleLoader`

After it loads the satellites, it wires every satellite pair as a candidate ISL. That does not mean every link becomes active. It only means the chosen ISL protocol has the full candidate graph available to filter later.

### `SatelliteLoaderService`

This service is the last satellite-side startup step.

Its job is to:

- load satellites from the configured source
- inject them into the simulation service

## Step 5: Build the Ground-Station Path

The live ground-station pipeline is:

1. `GroundStationBuilder`
2. `GroundStationYmlLoader`
3. `GroundStationLoaderService`

### `GroundStationBuilder`

`GroundStationBuilder` receives:

- the shared `RouterBuilder`
- the shared `DefaultComputingBuilder`
- the default `GroundLinkConfig`

For each `GroundStationSpec` it creates:

- a router
- a ground-link protocol
- a computing object
- a `GroundStationStruct`

The important detail is that per-station overrides such as `Protocol` and `ComputingType` are applied here.

### `GroundStationYmlLoader`

This loader reads `ground_stations.yml` and converts each YAML record into a `GroundStationSpec`.

For each spec it calls `GroundStationBuilder.Build`.

### `GroundStationLoaderService`

This service loads the full YAML file and injects the resulting ground stations into the simulation service.

## Step 6: Build the Simulation Service

In live mode, LeoDust creates `SimulationService`.

`SimulationService` contains:

- the shared node collections
- the current simulation time
- the optional serializer
- simulation plugins
- a state-plugin repository
- the base stepping logic from `BaseSimulationService`

If `--simulationStateOutputFile` is set, `SimulationService` also creates a `SimulationStateSerializer`.

## Step 7: Inject Optional Plugins

LeoDust has two plugin families.

### Simulation plugins

Built by `internal/simplugin/SimPluginBuilder`.

They run after each simulation step and are intended for scenario logic.
See [Plugin Reference](../reference/plugins.md) for registration and authoring details.

### State plugins

Built by `internal/stateplugin/DefaultStatePluginBuilder`.

They:

- compute additional state after each step
- can serialize that extra state for replay
See [Plugin Reference](../reference/plugins.md) for the live vs replay implementation pattern.

At runtime they are stored in `types.StatePluginRepository`.

## Step 8: Inject the Deployment Orchestrator

The startup path also creates a `DeploymentOrchestrator` and injects it into the simulation service.

Today this is best understood as a placeholder subsystem with some structure already present:

- deployable service model
- deployment interfaces
- orchestrator resolver

The orbit, link, routing, and viewer systems are more mature than the deployment flow.

## Step 9: Run Live Stepping

Once satellites and ground stations are injected, the simulation can step.

For each live step, `SimulationService.runSimulationStep` does this:

1. advance simulation time
2. update all node positions
3. update all node links
4. optionally calculate routing tables
5. run state plugins
6. run simulation plugins
7. append the frame to the serializer, if enabled

That sequence is the core of the live runtime.

## Step 10: Understand Replay Mode

Replay mode skips the TLE and YAML loaders entirely.

Instead, `SimulationStateDeserializer` reads a `.gob` file and constructs `SimulationIteratorService`.

In replay mode:

- positions come from stored frames
- links come from stored frames
- the viewer-facing behavior is deterministic

This is why replay mode is useful for demos, tests, and static visualization.

## Step 11: Map Config Files to Runtime Objects

This is the practical map most users need:

| Input | Consumed by | Produces |
|------|-------------|----------|
| `simulation*.yaml` | `main.go`, `SimulationService` | timing, data-source, serialization, routing-precalc behavior |
| `isl*.yaml` | `IslProtocolBuilder` | ISL protocol instances on satellites |
| `groundLink*.yaml` | `GroundStationBuilder`, `GroundProtocolBuilder` | ground-to-satellite link protocol instances |
| `router*.yaml` | `RouterBuilder` | routers on all nodes |
| `computing*.yaml` | `DefaultComputingBuilder` | computing instances on satellites and ground stations |
| `ground_stations.yml` | `GroundStationYmlLoader` | `GroundStationStruct` nodes |
| `*.tle` | `TleLoader`, `SatelliteBuilder` | `SatelliteStruct` nodes |

## Step 12: Trace a Single Satellite

If you want to understand one concrete object end-to-end, a single satellite goes through this path:

1. a named TLE record is parsed into `TLERecord`
2. `SatelliteBuilder.Build` creates a router, propagator, ISL protocol, and computing object
3. `node.NewSatellite` mounts those dependencies into `SatelliteStruct`
4. `SatelliteConstellationLoader` attaches candidate ISLs
5. `SimulationService` updates its position every step through the propagator
6. the chosen ISL protocol decides which candidate links are currently active

## Step 13: Trace a Single Ground Station

A ground station goes through this path:

1. one YAML record is parsed into `rawGroundStation`
2. the loader converts it to `GroundStationSpec`
3. `GroundStationBuilder.Build` creates a router, ground-link protocol, and computing object
4. `node.NewGroundStation` converts lat/lon/alt to ECEF and mounts dependencies
5. on each simulation step the ground-link protocol chooses the best visible satellite

## Next Step

After this page, the two most useful references are:

- [Configuration](../reference/configuration.md)
- [Runtime Components](../reference/runtime-components.md)
