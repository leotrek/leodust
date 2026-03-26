# Bundled Presets

This page explains the bundled configuration files in `go/resources/configs`.

Use it when you want to know which preset to start from instead of editing everything from scratch.

## Simulation Presets

### `simulationAutorunConfig.yaml`

Use this when you want a small bundled live run.

Current intent:

- autorun enabled
- bundled `starlink_250.tle`
- bundled ground stations
- simulation time near the bundled TLE epoch

### `simulationManualConfig.yaml`

Use this when you want the current non-autorun CLI path with a smaller constellation.

Remember:

- this is not an interactive shell
- it uses the manual loop in `main.go`
- each iteration currently advances by `600` simulated seconds

### `simulationManualConfig-0250.yaml`

Manual-mode preset for the `250` satellite subset.

### `simulationManualConfig-0500.yaml`

Manual-mode preset for the `500` satellite subset.

### `simulationManualConfig-1000.yaml`

Manual-mode preset for the `1000` satellite subset.

### `simulationManualConfig-2000.yaml`

Manual-mode preset for the `2000` satellite subset.

### `simulationManualConfig-3000.yaml`

Manual-mode preset for the `3000` satellite subset.

### `simulationManualConfig-6000.yaml`

Manual-mode preset for the larger `6000` satellite file.

Use this for scale experiments, not for the easiest first run.

### `simulationManualConfig-newest.yaml`

Manual-mode preset that targets the newest bundled full snapshot.

Use this when you want the latest bundled local TLE file rather than a curated subset.

## Inter-Satellite Link Presets

### `islNearestConfig.yaml`

Nearest-neighbor ISL topology.

Best for:

- simple behavior
- easy debugging
- experiments where you want local geometric connectivity

### `islMstConfig.yaml`

Minimum-spanning-tree-style topology.

Best for:

- sparse topologies
- deterministic tree-like connectivity
- experiments where link count needs to stay low

### `islMstSmartLoopConfig.yaml`

MST base topology with extra loop behavior.

Best for:

- exploring more resilient topologies than pure MST
- experiments where some redundancy matters

## Ground-Link Presets

### `groundLinkNearestConfig.yaml`

Ground stations connect to the nearest visible satellite.

This is the default bundled choice and the only bundled ground-link strategy today.

## Router Presets

### `routerAStarConfig.yaml`

On-demand routing.

Best for:

- simple live runs
- cases where you do not want routing-table precomputation

### `routerDijkstraConfig.yaml`

Shortest-path routing with support for precomputed routing tables.

Best for:

- repeatable routing experiments
- cases where `UsePreRouteCalc: true`

## Computing Presets

### `computingConfig.yaml`

Defines the shared computing profiles available to node builders.

Bundled profile categories:

- `None`
- `Edge`
- `Cloud`

The builders then use these categories as follows:

- satellites currently use `Edge`
- ground stations usually use `Cloud` unless overridden in YAML

## How to Pick a Starting Set

### Easiest first run

- `simulationAutorunConfig.yaml`
- `islMstConfig.yaml` or `islNearestConfig.yaml`
- `groundLinkNearestConfig.yaml`
- `routerAStarConfig.yaml`
- `computingConfig.yaml`

### Best starting point for offline playback work

- any `simulationManualConfig-*.yaml`
- `groundLinkNearestConfig.yaml`
- `routerAStarConfig.yaml`
- `computingConfig.yaml`
- plus `--simulationStateOutputFile`

### Best starting point for routing-table experiments

- `simulationAutorunConfig.yaml`
- `routerDijkstraConfig.yaml`
- `UsePreRouteCalc: true`

## Practical Rule

If you are new to the project, change one preset at a time.

The safest order is:

1. choose constellation size
2. choose ISL protocol
3. choose router
4. adjust simulation time
5. only then experiment with computing or plugins
