# First Simulation

This tutorial walks through a complete first run of LeoDust using the bundled inputs.

## Step 1: Change Into the Go Workspace

All simulator commands are run from the `go/` directory:

```bash
cd go
```

## Step 2: Run the Tests Once

This confirms the codebase builds in your environment:

```bash
go test ./...
```

## Step 3: Build the Main Binary

```bash
go build -o leodust ./cmd/leodust
```

You now have a local `./leodust` executable.

## Step 4: Understand the Minimum Config Set

The main runtime takes five primary config files:

- simulation config
- inter-satellite link config
- ground-link config
- computing config
- router config

For the bundled quick-start run, use:

- `./resources/configs/simulationAutorunConfig.yaml`
- `./resources/configs/islMstConfig.yaml`
- `./resources/configs/groundLinkNearestConfig.yaml`
- `./resources/configs/computingConfig.yaml`
- `./resources/configs/routerAStarConfig.yaml`

What each one controls:

- simulation config: time stepping and data sources
- ISL config: how satellites choose neighbors
- ground-link config: how ground stations choose an uplink satellite
- computing config: resource profiles attached to nodes
- router config: how routes are computed across the graph

## Step 5: Run the Bundled Simulation

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml
```

## Step 6: Understand the Startup Logs

The most important startup logs are:

- log level initialization
- TLE summary
- TLE epoch range
- satellite count loaded
- ground-station count loaded
- whether the simulation is autorunning or using the manual path

What to check:

### TLE epoch range

The simulator reports the earliest and latest TLE epochs found in the input file.

This matters because `SimulationStartTime` should be close to the TLE epoch. If the simulation time is far away from the TLE epoch, the propagation can become unrealistic even if the SGP4 implementation itself is correct.

### Satellite and ground-station counts

These tell you whether the selected input files were parsed correctly.

### Autorun behavior

If `StepInterval >= 0`, the simulator starts autorun. If `StepInterval < 0`, it uses the manual CLI path instead.

## Step 7: Validate the Orbit Layer

If you want to validate the propagation implementation itself:

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --validateOrbitOnly
```

This compares the orbit implementation against published SGP4 reference vectors.

Important:

- this validates the implementation
- it does not guarantee a stale TLE file is still accurate for a chosen date

## Step 8: Use a Fresh TLE Snapshot

You can override the configured TLE source for a run:

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --downloadTLEToday \
  --downloadTLEGroup starlink
```

The downloaded TLE becomes the active source for that run.

## What Happened Under the Hood

The live path in `go/cmd/leodust/main.go` performs these major steps:

1. Load configs.
2. Configure logging.
3. Optionally download a fresh TLE.
4. Optionally validate the orbit layer.
5. Build router and computing builders.
6. Build satellite and ground loaders.
7. Inject satellites and ground stations into the simulation service.
8. Start autorun or the manual CLI loop.

## Next Step

Continue to [Precompute and Replay](precompute-and-replay.md).
