# Orbit, Links, and Routing

This page explains the most important simulation concepts that affect correctness.

## Orbit Model

The orbit pipeline is:

1. load TLE records
2. propagate them with SGP4
3. convert results to ECEF meters
4. update satellite positions at the current simulation time

The key implementation lives in:

- `go/internal/orbit/tle_propagator.go`
- `go/internal/orbit/coordinates.go`
- `go/internal/satellite/tle_loader.go`
- `go/internal/node/satellite_struct.go`

## Why ECEF Matters

LeoDust currently works in an Earth-fixed frame.

This is important because:

- satellites are converted into ECEF before link and routing calculations
- ground stations are built directly in ECEF
- the viewer also renders ECEF

That keeps Earth, ground stations, and satellites aligned.

## Ground-Station Placement

Ground stations are placed from:

- latitude
- longitude
- optional altitude

using WGS84 Earth geometry.

## Ground Links

The current bundled ground-link protocol is `nearest`.

It:

- scans candidate satellites
- filters to those above the local horizon
- chooses the nearest valid one

This avoids physically impossible links through Earth.

## Inter-Satellite Links

ISL behavior depends on the selected protocol.

Common choices:

### `mst`

Builds a minimum spanning tree style topology and then filters it through the link protocol layer.

### `nearest`

Uses nearest-neighbor logic rather than tree-building.

### `mst_smart_loop`

Starts from MST and adds extra structure for loop resilience.

## Routing

Routing happens on top of the link graph.

### A*

- on-demand routing
- no useful precomputed routing table
- heuristic-based shortest-path search

### Dijkstra

- supports precomputation
- builds shortest paths from each node
- works well with `UsePreRouteCalc: true`

## Practical Accuracy Rules

### Keep TLEs fresh

The biggest source of bad orbital results is stale TLE data.

### Validate propagation separately from freshness

`--validateOrbitOnly` validates the implementation, not the age of the input data.

### Read the TLE epoch log

LeoDust logs the epoch range it found in the TLE file. Treat that log as the first correctness check before trusting a run.
