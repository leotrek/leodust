# LeoDust Simulator Guide

This is the detailed reference for the LeoDust simulator.

Use [README.md](./README.md) for the shortest possible quick start. For the maintained Read the Docs and MkDocs content, start at [docs/index.md](./docs/index.md). Use this guide when you want a single-file overview of runtime modes, CLI flags, config properties, data files, outputs, and current behavior.

## Overview

LeoDust simulates a space-ground network made of:

- satellites loaded from TLE data
- ground stations loaded from YAML
- inter-satellite links
- ground-to-satellite links
- routing across that topology
- optional simulation and state plugins
- precomputed snapshots for replay and visualization

The current orbital path is TLE-based and uses SGP4 propagation. Satellite positions are converted into Earth-Centered Earth-Fixed coordinates before link and routing calculations. Ground stations are also represented in ECEF, so the simulator and the viewer both operate in the same Earth-fixed frame.

## Repository Layout

Key directories:

- `go/cmd/leodust`
  Main simulator CLI.
- `go/cmd/leodust-viewer`
  Snapshot viewer and static export tool.
- `go/resources/configs`
  Bundled simulator config files.
- `go/resources/tle`
  Bundled TLE inputs.
- `go/resources/yml`
  Bundled ground-station YAML files.
- `go/results/precomputed`
  Precomputed snapshot outputs for the viewer.
- `go/internal/orbit`
  SGP4 propagation and coordinate conversion.
- `go/internal/simulation`
  Live simulation and replay services.

## Core Runtime Modes

### Live Autorun

This is the normal mode when `StepInterval >= 0`.

The simulator:

- loads satellites from TLE
- loads ground stations from YAML
- advances simulation time automatically
- recomputes positions and links each step
- optionally precomputes routing tables
- optionally writes a `.gob` snapshot and `.json` sidecar

### CLI Manual Mode

This is what happens when `StepInterval < 0`.

Important: the current CLI manual path is not a generic interactive console. The logic in `go/cmd/leodust/main.go`:

- loads the simulation once
- runs `StepCount` manual iterations
- advances by `600` simulated seconds per iteration
- logs a sample route between two bundled ground stations

So the “manual” configs are selecting the manual code path, not opening a REPL.

If you want real external time control, the underlying controller already supports:

- `StartAutorun`
- `StopAutorun`
- `StepBySeconds`
- `StepByTime`

through `SimulationController`.

### Precompute Mode

If you pass `--simulationStateOutputFile`, the live simulation writes:

- `name.gob`
- `name.gob.json`

State plugins can also write their own sidecars.

### Replay Mode

If you pass `--simulationStateInputFile`, the simulator reconstructs the topology from a serialized `.gob` file instead of rebuilding from TLEs.

### Viewer Mode

The browser viewer is snapshot-based. It renders:

- Earth
- satellites
- ground stations
- active links
- frame playback
- dataset selection from precomputed snapshots

The viewer can run:

- locally through the Go server
- as a static bundle for GitHub Pages

## Typical Workflows

### 1. Run a Bundled Simulation

From `go/`:

```bash
go build -o leodust ./cmd/leodust

./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml
```

### 2. Validate the Orbit Layer

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --validateOrbitOnly
```

This validates the propagation implementation against published SGP4 reference vectors. It does not prove that a stale TLE file is still physically accurate.

### 3. Download a Fresh TLE Before Running

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --downloadTLEToday \
  --downloadTLEGroup starlink
```

This downloads a fresh CelesTrak group snapshot and temporarily overrides `SatelliteDataSource` for that run.

### 4. Precompute a Snapshot

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --simulationStateOutputFile ./results/precomputed/precomputed_data-local.gob
```

Output:

- `./results/precomputed/precomputed_data-local.gob`
- `./results/precomputed/precomputed_data-local.gob.json`

### 5. Replay a Precomputed State

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --simulationStateInputFile ./results/precomputed/precomputed_data-local.gob
```

### 6. View Snapshots Locally

```bash
go run ./cmd/leodust-viewer \
  --snapshotDir ./results/precomputed \
  --earthImage ./resources/image/World_Map_Blank.svg \
  --addr :8080
```

### 7. Export the Viewer for GitHub Pages

```bash
go run ./cmd/leodust-viewer \
  --snapshotDir ./results/precomputed \
  --earthImage ./resources/image/World_Map_Blank.svg \
  --exportStaticDir ../viewer-site
```

This writes a self-contained static bundle.

## `leodust` CLI Reference

The main simulator CLI lives in `go/cmd/leodust/main.go`.

### Main Runtime Flags

| Flag | Meaning |
|------|---------|
| `--simulationConfig` | Path to the simulation config file |
| `--islConfig` | Path to the inter-satellite link config |
| `--groundLinkConfig` | Path to the ground-link config |
| `--computingConfig` | Path to the computing config |
| `--routerConfig` | Path to the router config |
| `--simulationStateOutputFile` | Optional `.gob` output path for precompute mode |
| `--simulationStateInputFile` | Optional `.gob` input path for replay mode |

### Optional Flags

| Flag | Meaning |
|------|---------|
| `--simulationPlugins` | Comma-separated simulation plugin names |
| `--statePlugins` | Comma-separated state plugin names |
| `--logLevel` | Runtime log level override |
| `--validateOrbit` | Run orbit validation before starting |
| `--validateOrbitOnly` | Run orbit validation and exit |
| `--downloadTLEToday` | Download a fresh TLE snapshot before starting |
| `--downloadTLEGroup` | CelesTrak group for `--downloadTLEToday` |
| `--downloadTLEOutput` | Optional output path for the downloaded TLE file |

### Supported Plugin Names

Simulation plugins:

- `DummyPlugin`

State plugins:

- `DummySunStatePlugin`

If a state plugin is active during serialization, its data can be reloaded in replay mode through the corresponding precomputed plugin builder.

## `leodust-viewer` CLI Reference

The viewer CLI lives in `go/cmd/leodust-viewer/main.go`.

| Flag | Meaning |
|------|---------|
| `--snapshot` | Default snapshot file to preselect if one specific file should be active first |
| `--snapshotDir` | Directory scanned for `.gob.json` snapshot files |
| `--earthImage` | Equirectangular world map image used for the Earth surface |
| `--exportStaticDir` | If set, writes a static site bundle and exits |
| `--addr` | HTTP listen address for local viewer mode |
| `--logLevel` | Viewer log level |

## Orbital Model

The current orbital path is:

- TLE input
- SGP4 propagation
- ECEF output positions
- WGS84 ground-station placement
- horizon-filtered ground links

Important behaviors:

- satellite positions depend directly on `SimulationStartTime`
- ground stations stay Earth-fixed
- the viewer assumes the same Earth-fixed frame
- stale TLEs can produce unrealistic results even if SGP4 itself is correct

### How to Judge Orbit Quality

A run is only trustworthy when the TLE snapshot is near the requested simulation time.

Good practice:

- same day as the TLE snapshot
- ideally within hours, not months

The simulator already logs the TLE epoch range and warns when the simulation time is too far away.

## Configuration Reference

Config structs are defined in `go/configs/config.go`.

### Simulation Config

Fields:

| Field | Type | Meaning |
|------|------|---------|
| `StepInterval` | `int` | Real-time delay between autorun steps in milliseconds. `< 0` switches the CLI into the current manual path. |
| `StepMultiplier` | `int` | Simulated seconds added per autorun step. |
| `StepCount` | `int` | Number of steps in autorun, and also the loop count used by the current manual CLI path. |
| `LogLevel` | `string` | `error`, `warn`, `info`, or `debug`. |
| `SatelliteDataSource` | `string` | Satellite file name or path. A bare file name resolves under `./resources/tle/`. |
| `SatelliteDataSourceType` | `string` | Currently wired as `tle`. |
| `GroundStationDataSource` | `string` | Ground-station file name or path. A bare file name resolves under `./resources/yml/`. |
| `GroundStationDataSourceType` | `string` | Currently wired as `yml` in the main runtime. |
| `UsePreRouteCalc` | `bool` | Calls `CalculateRoutingTable()` for every node each step. Most useful with Dijkstra. |
| `SimulationStartTime` | `time.Time` | Absolute UTC simulation start time used for propagation. |

Important notes:

- `StepMultiplier` only matters in autorun mode.
- `UsePreRouteCalc` is safe with A*, but A* treats routing-table calculation as a no-op.
- extra YAML keys are ignored by Go’s YAML unmarshaler, so legacy keys in sample files do not currently break startup.

### Inter-Satellite Link Config

| Field | Type | Meaning |
|------|------|---------|
| `Protocol` | `string` | ISL strategy name |
| `Neighbours` | `int` | Neighbor target for protocols that use it |

Supported protocol values from `go/internal/links/isl_protocol_builder.go`:

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

Behavioral note:

- unknown ISL protocol names currently fall back to `nearest`

### Ground Link Config

| Field | Type | Meaning |
|------|------|---------|
| `Protocol` | `string` | Ground-to-satellite link strategy |

Supported value:

- `nearest`

The current implementation only links to satellites above the local horizon.

### Router Config

| Field | Type | Meaning |
|------|------|---------|
| `Protocol` | `string` | Router strategy name |

Supported values:

- `dijkstra`
- `a-star`

Behavioral note:

- unknown router names are fatal at startup

### Computing Config

This file is a list of computing profiles.

| Field | Type | Meaning |
|------|------|---------|
| `Cores` | `int` | CPU capacity |
| `Memory` | `int` | Memory capacity |
| `Type` | `string` | `None`, `Edge`, `Cloud`, or `Any` |

The bundled configs primarily use `None`, `Edge`, and `Cloud`.

## Ground-Station YAML Reference

The runtime loader is `go/internal/ground/yml_loader.go`.

Supported fields per entry:

| Field | Type | Meaning |
|------|------|---------|
| `Name` | `string` | Ground-station display name |
| `Lat` | `float64` | Latitude in degrees |
| `Lon` | `float64` | Longitude in degrees |
| `Alt` | `float64` | Optional altitude in meters |
| `Protocol` | `string` | Optional per-station ground-link override |
| `ComputingType` | `string` | Optional per-station computing override |

Example:

```yaml
- Name: Graz
  Lat: 47.0707
  Lon: 15.4409
  Alt: 353
  Protocol: nearest
  ComputingType: Cloud
```

Important note:

- the bundled YAML file still contains a legacy `Router` field on many entries
- the current loader does not read it
- the field is ignored

## Bundled Inputs

### TLE Files

Bundled TLE files live in `go/resources/tle`.

Key conventions:

- `starlink_current.tle`
  latest bundled full snapshot
- `starlink_<count>.tle`
  maintained subset for repeatable local runs
- `starlink_6000.tle`
  large raw subset kept as-is

### Ground Stations

Bundled ground stations live in `go/resources/yml/ground_stations.yml`.

### Config Presets

Bundled presets live in `go/resources/configs`.

Common presets:

- `simulationAutorunConfig.yaml`
- `simulationManualConfig.yaml`
- `simulationManualConfig-0250.yaml`
- `simulationManualConfig-0500.yaml`
- `simulationManualConfig-1000.yaml`
- `simulationManualConfig-2000.yaml`
- `simulationManualConfig-3000.yaml`
- `simulationManualConfig-6000.yaml`
- `simulationManualConfig-newest.yaml`

The numbered manual configs mainly change the TLE subset size.

## Outputs

### Simulation Outputs

If `--simulationStateOutputFile ./path/name.gob` is set, the simulator writes:

- `./path/name.gob`
- `./path/name.gob.json`

State plugins can also write sidecars. For example:

- `name.gob.dummySimPlugin`

### Viewer Static Outputs

The viewer export writes:

- `index.html`
- `app.js`
- `styles.css`
- `viewer-config.json`
- `snapshots/index.json`
- `snapshots/*.gob.json`
- `earth/<image>`

## Current Gotchas

### TLE freshness still matters

Correct SGP4 code does not make an old TLE trustworthy.

### Manual CLI mode is specialized

It currently runs the hardcoded route-demo loop in `main.go`.

### Ground-station JSON is not wired into the main entrypoint

Older docs sometimes implied broader support. The current `leodust` startup path wires the YAML loader.

### Extra YAML keys are ignored

That is why some legacy fields can still exist in bundled config files.

### The viewer is snapshot-based

It is not a live simulation streamer.

## Where to Start

If you are new to the repo:

1. Read [README.md](./README.md).
2. Run the bundled autorun config.
3. Generate a local precomputed snapshot.
4. Open the viewer against `./results/precomputed`.
5. Come back to this guide when you want to change protocols, routing, or data sources.
