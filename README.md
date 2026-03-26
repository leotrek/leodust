# LeoDust

LeoDust is a TLE-driven space-ground network simulator with precomputed snapshot playback and a browser viewer.

Full documentation:

- [MkDocs Home](./docs/index.md)
- [First Simulation Tutorial](./docs/tutorials/first-simulation.md)
- [Configuration Reference](./docs/reference/configuration.md)
- [Runtime Components](./docs/reference/runtime-components.md)

## Quick Start

From `go/`:

```bash
go build -o leodust ./cmd/leodust
```

Run the bundled simulator:

```bash
./leodust \
  --simulationConfig ./resources/configs/simulationAutorunConfig.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml
```

Open the snapshot viewer:

```bash
go run ./cmd/leodust-viewer \
  --snapshotDir ./results/precomputed \
  --earthImage ./resources/image/World_Map_Blank.svg \
  --addr :8080
```

Then open `http://localhost:8080`.

Build the documentation locally:

```bash
python -m pip install -r docs/requirements.txt
mkdocs serve
```
