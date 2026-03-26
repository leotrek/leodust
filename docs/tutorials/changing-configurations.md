# Changing Configurations

This tutorial explains how to make controlled changes to LeoDust without getting lost in the config files.

## Step 1: Pick the Right Simulation Preset

Bundled simulation presets live in `go/resources/configs`.

Useful presets:

- `simulationAutorunConfig.yaml`
- `simulationManualConfig.yaml`
- `simulationManualConfig-0250.yaml`
- `simulationManualConfig-0500.yaml`
- `simulationManualConfig-1000.yaml`
- `simulationManualConfig-2000.yaml`
- `simulationManualConfig-3000.yaml`
- `simulationManualConfig-6000.yaml`
- `simulationManualConfig-newest.yaml`

The numbered presets mostly change the selected TLE subset and usually keep the rest of the startup shape the same.

If you want a detailed description of each bundled file, read [Bundled Presets](../reference/bundled-presets.md).

## Step 2: Change Constellation Size

Edit `SatelliteDataSource` in the simulation config.

Examples:

- `starlink_250.tle`
- `starlink_500.tle`
- `starlink_1000.tle`
- `starlink_current.tle`

Rule of thumb:

- smaller files are easier for local iteration
- larger files are better for scale experiments

Operational rule:

- keep the selected TLE file and `SimulationStartTime` close together in time

## Step 3: Change the Simulation Time

Edit `SimulationStartTime`.

This matters a lot because satellites are propagated from TLEs at that time.

Best practice:

- keep `SimulationStartTime` close to the TLE epoch

## Step 4: Change the Inter-Satellite Link Protocol

Edit `Protocol` in the ISL config.

Common values:

- `mst`
- `nearest`
- `mst_smart_loop`

Also available:

- `pst`
- `mst_loop`
- `pst_loop`
- `pst_smart_loop`
- `other_mst`
- `other_mst_loop`
- `other_mst_smart_loop`

If you provide an unknown ISL protocol name, LeoDust currently falls back to `nearest`.

How to think about the common choices:

- `nearest`: geometric local connectivity
- `mst`: sparse tree-like topology
- `mst_smart_loop`: sparse base topology with extra redundancy

## Step 5: Change the Router

Edit `Protocol` in the router config.

Supported values:

- `a-star`
- `dijkstra`

Unknown router names are fatal at startup.

Guideline:

- use `a-star` for simpler on-demand routing
- use `dijkstra` when you want `UsePreRouteCalc: true`

## Step 6: Change Ground Stations

Edit the YAML file in `go/resources/yml`.

Supported fields per ground-station entry:

- `Name`
- `Lat`
- `Lon`
- `Alt`
- `Protocol`
- `ComputingType`

Important:

- the current loader ignores legacy `Router` fields that still appear in the bundled YAML
- `Protocol` and `ComputingType` are optional per-station overrides
- `Alt` is in meters above the WGS84 ellipsoid

## Step 7: Change Computing Profiles

Edit `computingConfig.yaml`.

That file defines the computing profiles available to builders.

Typical entries:

- `None`
- `Edge`
- `Cloud`

Ground stations can choose a per-station `ComputingType`. Satellites are currently created with `Edge` computing in the TLE path.

Practical pattern:

1. keep the shared computing config small
2. use YAML `ComputingType` only when a specific station needs different capacity

## Step 8: Use Precomputed Routing Tables

Set:

```yaml
UsePreRouteCalc: true
```

This tells the simulation to call `CalculateRoutingTable()` on every node during each step.

This is most useful with `dijkstra`.

With `a-star`, enabling `UsePreRouteCalc` is mostly wasted work because A* does not rely on a prebuilt routing table.

## Step 9: Enable Plugins

Use CLI flags:

```bash
--simulationPlugins DummyPlugin
--statePlugins DummySunStatePlugin
```

Simulation plugins run on each step. State plugins can also persist additional data into precomputed outputs.

## Step 10: Verify the Result

After changing configuration, check:

- startup logs
- TLE epoch range warnings
- satellite count
- ground-station count
- route and link behavior
- whether the chosen router is actually the one you intended
- whether ground-station overrides were applied as expected

If you changed the viewer input pipeline, also regenerate precomputed snapshots and reload the viewer.
