# CLI Reference

This page documents the current command-line interfaces.

## `leodust`

The main simulator command lives in `go/cmd/leodust/main.go`.

### Required Runtime Flags

| Flag | Meaning |
|------|---------|
| `--simulationConfig` | Simulation config file |
| `--islConfig` | ISL config file |
| `--groundLinkConfig` | Ground-link config file |
| `--computingConfig` | Computing profile config file |
| `--routerConfig` | Router config file |

### Optional Runtime Flags

| Flag | Meaning |
|------|---------|
| `--simulationStateOutputFile` | Write a precomputed `.gob` state and `.json` sidecar |
| `--simulationStateInputFile` | Replay from a precomputed `.gob` state |
| `--simulationPlugins` | Comma-separated simulation plugin names |
| `--statePlugins` | Comma-separated state plugin names |
| `--logLevel` | Runtime log-level override |
| `--validateOrbit` | Validate orbit propagation before running |
| `--validateOrbitOnly` | Validate orbit propagation and exit |
| `--downloadTLEToday` | Download a fresh CelesTrak TLE snapshot |
| `--downloadTLEGroup` | TLE group used with `--downloadTLEToday` |
| `--downloadTLEOutput` | Output path for the downloaded TLE |

### Examples

Run a normal live simulation:

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml
```

Validate the orbit layer:

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --validateOrbitOnly
```

Replay from a precomputed state:

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --simulationStateInputFile ./results/precomputed/precomputed_data-local.gob
```

### Supported Plugin Names

Simulation plugins:

- `DummyPlugin`

State plugins:

- `DummySunStatePlugin`

## `leodust-viewer`

The viewer command lives in `go/cmd/leodust-viewer/main.go`.

### Flags

| Flag | Meaning |
|------|---------|
| `--snapshot` | Default snapshot to preselect |
| `--snapshotDir` | Directory scanned for `.gob.json` files |
| `--earthImage` | Equirectangular Earth image |
| `--exportStaticDir` | Static export output directory |
| `--addr` | HTTP listen address for local viewer mode |
| `--logLevel` | Viewer log level |

### Examples

Run the local viewer:

```bash
go run ./cmd/leodust-viewer \
  --snapshotDir ./results/precomputed \
  --earthImage ./resources/image/World_Map_Blank.svg \
  --addr :8080
```

Export a static viewer bundle:

```bash
go run ./cmd/leodust-viewer \
  --snapshotDir ./results/precomputed \
  --earthImage ./resources/image/World_Map_Blank.svg \
  --exportStaticDir ../viewer-site
```
