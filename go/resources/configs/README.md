# LeoDust Configuration Reference

This directory contains bundled config files used by the simulator.

For the maintained MkDocs documentation, start at [docs/index.md](../../../docs/index.md).

Most useful pages:

- [Configuration Reference](../../../docs/reference/configuration.md)
- [Bundled Presets](../../../docs/reference/bundled-presets.md)
- [Runtime Assembly](../../../docs/tutorials/runtime-assembly.md)

## Simulation Config

Defined in `go/configs/config.go`.

| Field | Type | Meaning |
|------|------|---------|
| `StepInterval` | `int` | Real-time delay between autorun steps in milliseconds. `< 0` selects the current manual CLI path. |
| `StepMultiplier` | `int` | Simulated seconds added per autorun step. |
| `StepCount` | `int` | Number of steps to run. |
| `LogLevel` | `string` | `error`, `warn`, `info`, or `debug`. |
| `SatelliteDataSource` | `string` | Satellite file name or path. |
| `SatelliteDataSourceType` | `string` | Currently `tle`. |
| `GroundStationDataSource` | `string` | Ground-station file name or path. |
| `GroundStationDataSourceType` | `string` | The main runtime currently wires `yml`. |
| `UsePreRouteCalc` | `bool` | Whether routers should precompute routing tables each step. |
| `SimulationStartTime` | `time.Time` | Absolute UTC start time used for TLE propagation. |

Example:

```yaml
StepInterval: 1
StepMultiplier: 10
StepCount: 10
LogLevel: info
SatelliteDataSource: starlink_500.tle
SatelliteDataSourceType: tle
GroundStationDataSource: ground_stations.yml
GroundStationDataSourceType: yml
UsePreRouteCalc: false
SimulationStartTime: "2026-03-26T14:00:00Z"
```

## Inter-Satellite Link Config

| Field | Type | Meaning |
|------|------|---------|
| `Neighbours` | `int` | Neighbor target for protocols that use it. |
| `Protocol` | `string` | ISL protocol name. |

Supported protocol names are documented in [Configuration Reference](../../../docs/reference/configuration.md).

## Ground Link Config

| Field | Type | Meaning |
|------|------|---------|
| `Protocol` | `string` | Ground-to-satellite protocol name. |

The current bundled and supported value is `nearest`.

## Router Config

| Field | Type | Meaning |
|------|------|---------|
| `Protocol` | `string` | Router protocol name. |

The current bundled values are `a-star` and `dijkstra`.

## Computing Config

This file is a list of computing profiles.

| Field | Type | Meaning |
|------|------|---------|
| `Cores` | `int` | CPU capacity. |
| `Memory` | `int` | Memory capacity. |
| `Type` | `string` | `None`, `Edge`, `Cloud`, or `Any`. |

Example:

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

## Ground-Station YAML Fields

The bundled ground-station file lives in `../yml/ground_stations.yml`.

Supported fields:

| Field | Type | Meaning |
|------|------|---------|
| `Name` | `string` | Ground-station name. |
| `Lat` | `float64` | Latitude in degrees. |
| `Lon` | `float64` | Longitude in degrees. |
| `Alt` | `float64` | Optional altitude in meters. |
| `Protocol` | `string` | Optional per-station ground-link override. |
| `ComputingType` | `string` | Optional per-station computing override. |

Note:

- some bundled entries still contain a legacy `Router` field
- the current loader ignores it
