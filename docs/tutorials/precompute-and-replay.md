# Precompute and Replay

This tutorial shows how to serialize a simulation, inspect the output files, and replay them later.

## Why Precompute?

Precomputation is useful when:

- you want to reuse an expensive constellation setup
- you want deterministic viewer input
- you want to compare replay mode and live mode
- you want to ship the results to the static viewer

## Step 1: Generate a Snapshot

From `go/`:

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --simulationStateOutputFile ./results/precomputed/precomputed_data-local.gob
```

## Step 2: Inspect the Output Files

LeoDust writes:

- `./results/precomputed/precomputed_data-local.gob`
- `./results/precomputed/precomputed_data-local.gob.json`

What they are for:

- `.gob`: replay input for the simulator
- `.json`: viewer input for the browser UI

If state plugins are enabled, you can also get plugin-specific sidecar files.

## Step 3: Understand What Is Serialized

The serializer in `go/internal/simulation/simulation_state_serializer.go` writes:

- the names and computing types of satellites
- the names and computing types of ground stations
- the link list
- one simulation state per frame
- the state-plugin names needed for replay

Each frame contains:

- a timestamp
- node positions
- indices of established links

## Step 4: Replay the Snapshot

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --simulationStateInputFile ./results/precomputed/precomputed_data-local.gob
```

Replay mode reconstructs:

- simulated satellites
- simulated ground stations
- precomputed links
- stored state-plugin data

It does not rerun the original TLE propagation pipeline.

## Step 5: Understand Replay vs Live

### Live simulation

- loads from TLE and YAML
- recomputes positions from orbital data
- recomputes links each step
- optionally recomputes routing tables

### Replay mode

- loads from `.gob`
- reuses serialized positions and link states
- is usually much faster
- is the basis for the viewer workflow

## Step 6: Use the Bundled Precomputed Folder

The repository already contains viewer-ready sample data in:

- `go/results/precomputed`

These files are what the viewer dataset dropdown reads by default.

## Next Step

Continue to [Viewer and GitHub Pages](viewer-and-github-pages.md).
