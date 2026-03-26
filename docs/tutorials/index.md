# Tutorial Path

This section is written as a guided path through the simulator.

## Recommended Order

### 1. First Simulation

Use [First Simulation](first-simulation.md) to:

- build the simulator
- understand the minimum config set
- run the bundled constellation
- read the most important logs

### 2. Precompute and Replay

Use [Precompute and Replay](precompute-and-replay.md) to:

- write a `.gob` simulation snapshot
- understand the `.json` viewer sidecar
- replay a precomputed state

### 3. Runtime Assembly

Use [Runtime Assembly](runtime-assembly.md) to:

- connect each config file to the builder that consumes it
- understand how satellites and ground stations are created
- follow the live stepping pipeline
- understand where replay mode diverges

### 4. Viewer and GitHub Pages

Use [Viewer and GitHub Pages](viewer-and-github-pages.md) to:

- open the local snapshot viewer
- switch between datasets
- export a static viewer bundle
- publish it on GitHub Pages

### 5. Changing Configurations

Use [Changing Configurations](changing-configurations.md) when you want to:

- change constellation size
- choose another ISL protocol
- switch routers
- alter ground-station input
- use fresh TLE downloads

## Suggested Reading After the Tutorials

- [Simulation Model](../concepts/simulation-model.md)
- [Orbit, Links, and Routing](../concepts/orbit-links-routing.md)
- [Configuration Reference](../reference/configuration.md)
- [Bundled Presets](../reference/bundled-presets.md)
- [Computing and Deployment](../reference/computing-and-deployment.md)
- [Runtime Components](../reference/runtime-components.md)
- [Package and Type Map](../reference/package-type-map.md)
