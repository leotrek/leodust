# Configuration Reference

This page documents every configuration file that LeoDust consumes in the normal live runtime.

The config structs themselves are defined in `go/configs/config.go`. This page goes one step further and explains how those fields affect the runtime.

## How Configuration Files Are Resolved

The main CLI accepts either:

- an explicit relative path such as `./resources/configs/simulationAutorunConfig.yaml`
- an absolute path
- a remote URL for supported satellite inputs
- or, for some data-source fields inside config files, a bare filename

For data sources referenced inside `SimulationConfig`, bare filenames are resolved like this:

- TLE file names resolve under `./resources/tle/`
- ground-station names resolve under `./resources/yml/`

Examples:

- `SatelliteDataSource: starlink_250.tle` becomes `./resources/tle/starlink_250.tle`
- `GroundStationDataSource: ground_stations.yml` becomes `./resources/yml/ground_stations.yml`

If the value already contains a slash, starts with `.`, is absolute, or is an `http://` or `https://` URL, LeoDust uses it directly.

## Configuration Stack at Runtime

The normal live startup path uses five config files plus one ground-station YAML file:

1. simulation config
2. inter-satellite link config
3. ground-link config
4. router config
5. computing config
6. ground-station YAML

That maps to runtime code like this:

| Config | Main consumer | Runtime effect |
|------|---------------|----------------|
| simulation config | `main.go`, `SimulationService` | time stepping, logging default, data sources, serialization, routing-table recalculation |
| ISL config | `IslProtocolBuilder` | active satellite-to-satellite link strategy |
| ground-link config | `GroundProtocolBuilder`, `GroundStationBuilder` | how ground stations select uplinks |
| router config | `RouterBuilder` | routing algorithm on every node |
| computing config | `DefaultComputingBuilder` | CPU/memory profiles attached to nodes |
| ground-station YAML | `GroundStationYmlLoader` | actual ground-station nodes and per-station overrides |

## `SimulationConfig`

`SimulationConfig` is the top-level live runtime config.

### Full Field Reference

| Field | Type | Used by | Meaning |
|------|------|---------|---------|
| `StepInterval` | `int` | `BaseSimulationService.StartAutorun`, `main.runController` | Real-time delay between autorun steps in milliseconds. Negative values switch to the current manual CLI path. |
| `StepMultiplier` | `int` | `BaseSimulationService.StartAutorun` | Simulated seconds added per autorun step. |
| `StepCount` | `int` | autorun and manual path | Maximum number of steps to run. |
| `LogLevel` | `string` | `configureLogging` | Default log level unless overridden by CLI. |
| `SatelliteDataSource` | `string` | satellite loading path | TLE filename, path, or URL. |
| `SatelliteDataSourceType` | `string` | `SatelliteConstellationLoader` | Loader registry key. Today the live runtime uses `tle`. |
| `GroundStationDataSource` | `string` | ground-station loading path | YAML filename or path. |
| `GroundStationDataSourceType` | `string` | startup path | Loader kind. Today the live runtime uses `yml`. |
| `UsePreRouteCalc` | `bool` | `SimulationService.runSimulationStep` | Whether every node calls `CalculateRoutingTable()` on each step. |
| `SimulationStartTime` | `time.Time` | satellite creation and simulation time | Absolute UTC time used for the initial frame and subsequent stepping. |

### Step Fields in Autorun Mode

When `StepInterval >= 0`:

- autorun is enabled
- each step advances by `StepMultiplier` seconds
- the process waits `StepInterval` real milliseconds between steps
- the run stops after `StepCount` steps

Example:

```yaml
StepInterval: 250
StepMultiplier: 60
StepCount: 120
```

Meaning:

- run 120 steps
- each step moves the simulation forward by 60 seconds
- wait 250 milliseconds between steps in wall-clock time

### Step Fields in the Current Manual CLI Path

When `StepInterval < 0`:

- the current code does not start autorun
- `runController` executes a manual loop instead
- that loop still runs `StepCount` times
- each iteration currently advances by a hard-coded `600` seconds

This is important because `StepMultiplier` is not what controls the manual path today.

### `SimulationStartTime`

This field is one of the most important correctness settings in the whole simulator.

It affects:

- initial satellite positions
- all later propagated positions
- the timestamps written into serialized frames

Rule:

- keep `SimulationStartTime` close to the TLE epoch range

If it is too far away:

- SGP4 can still run
- but the resulting orbit geometry may no longer be trustworthy

### `UsePreRouteCalc`

When `true`, each step calls `CalculateRoutingTable()` on every node after links update.

That is most meaningful with:

- `dijkstra`

That is much less useful with:

- `a-star`

because A* is primarily on-demand.

### Example

```yaml
StepInterval: 1
StepMultiplier: 10
StepCount: 10
LogLevel: info
SatelliteDataSource: starlink_250.tle
SatelliteDataSourceType: tle
GroundStationDataSource: ground_stations.yml
GroundStationDataSourceType: yml
UsePreRouteCalc: false
SimulationStartTime: "2026-03-26T14:00:00Z"
```

## `InterSatelliteLinkConfig`

This config selects the inter-satellite topology strategy.

### Fields

| Field | Type | Used by | Meaning |
|------|------|---------|---------|
| `Protocol` | `string` | `IslProtocolBuilder` | Strategy name for the ISL protocol |
| `Neighbours` | `int` | nearest and loop-related protocols | Target neighbor count or loop budget |

### Supported Protocol Names

Current values from `go/internal/links/isl_protocol_builder.go`:

- `mst`
- `pst`
- `mst_loop`
- `pst_loop`
- `mst_smart_loop`
- `pst_smart_loop`
- `other_mst`
- `other_mst_loop`
- `other_mst_smart_loop`
- `nearest`

### How to Interpret `Neighbours`

`Neighbours` is not “how many links every protocol always creates.”

It is a protocol-specific control. Examples:

- `nearest` uses it directly as the top-`N` reachable neighbor count
- loop and smart-loop variants use it as a target while adding extra structure
- plain MST-style protocols do not interpret it the same way as a nearest-neighbor graph

### Fallback Behavior

Unknown ISL protocol names currently log a warning and fall back to `nearest`.

### Example

```yaml
Neighbours: 4
Protocol: mst_smart_loop
```

## `GroundLinkConfig`

This config selects how ground stations attach to satellites.

### Fields

| Field | Type | Used by | Meaning |
|------|------|---------|---------|
| `Protocol` | `string` | `GroundProtocolBuilder` | Ground-to-satellite selection strategy |

### Supported Values

- `nearest`

### What `nearest` Actually Does

The current bundled ground-link protocol:

1. checks all known satellites
2. filters out below-horizon satellites
3. sorts visible satellites by distance
4. keeps one active uplink to the nearest visible satellite

That means the ground-link layer is Earth-aware and should not create links through the planet.

## `RouterConfig`

This config selects the routing algorithm attached to every node.

### Fields

| Field | Type | Used by | Meaning |
|------|------|---------|---------|
| `Protocol` | `string` | `RouterBuilder` | Routing implementation name |

### Supported Values

- `a-star`
- `dijkstra`

### How to Choose

Use `a-star` when:

- you want simpler on-demand routing
- you are not relying on precomputed routing tables

Use `dijkstra` when:

- you want a classic shortest-path table
- you want `UsePreRouteCalc: true`
- you want service-routing behavior through the Dijkstra router

### Failure Mode

Unknown router values are fatal at startup.

## `ComputingConfig`

This file is a list of named computing profiles.

The builder does not invent CPU or memory values on its own. It only selects from this list.

### Fields

| Field | Type | Used by | Meaning |
|------|------|---------|---------|
| `Cores` | `int` | `DefaultComputingBuilder` | CPU capacity assigned to the profile |
| `Memory` | `int` | `DefaultComputingBuilder` | Memory capacity assigned to the profile |
| `Type` | `string` | `DefaultComputingBuilder` | Logical profile key: `None`, `Edge`, `Cloud`, or `Any` |

### How the Builder Uses These Profiles

The current runtime uses them like this:

- satellites request `Edge`
- ground stations use the builder default unless a YAML `ComputingType` override is present
- a station-level `ComputingType` asks the builder for that specific profile

### Example

```yaml
- Cores: 0
  Memory: 0
  Type: None
- Cores: 512
  Memory: 4096
  Type: Edge
- Cores: 1024
  Memory: 32768
  Type: Cloud
```

### Practical Guidance

Keep the profile list small.

A good starting set is:

- one empty profile
- one satellite profile
- one ground-station profile

## Ground-Station YAML

The main runtime loads ground stations from YAML instead of a struct-specific config file.

Bundled file:

- `go/resources/yml/ground_stations.yml`

### Supported Fields

| Field | Type | Used by | Meaning |
|------|------|---------|---------|
| `Name` | `string` | `GroundStationBuilder` | Ground-station display name |
| `Lat` | `float64` | `node.NewGroundStation` | Latitude in degrees |
| `Lon` | `float64` | `node.NewGroundStation` | Longitude in degrees |
| `Alt` | `float64` | `node.NewGroundStation` | Optional altitude in meters |
| `Protocol` | `string` | `GroundStationBuilder` | Optional per-station ground-link override |
| `ComputingType` | `string` | `GroundStationBuilder` | Optional per-station computing profile override |

### What Happens to These Fields

The loader path is:

1. YAML is decoded into `rawGroundStation`
2. that is normalized into `GroundStationSpec`
3. `GroundStationBuilder` creates a router, protocol, computing object, and `GroundStationStruct`

### Legacy Field Note

The bundled YAML still contains a `Router` field on many entries.

The current loader ignores it.

That means:

- changing `Router` inside the YAML currently has no effect
- router selection still comes from the shared router config file

## Bundled Files to Start From

LeoDust ships bundled presets in `go/resources/configs`.

For a file-by-file explanation of those presets, see [Bundled Presets](bundled-presets.md).
