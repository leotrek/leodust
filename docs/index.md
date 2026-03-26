# LeoDust Documentation

LeoDust is a TLE-driven space-ground network simulator with:

- live simulation from TLE and ground-station inputs
- snapshot precomputation and replay
- a browser viewer for Earth-based playback
- a static export path for GitHub Pages

This documentation is organized as a tutorial first and a reference second.

## How to Read These Docs

If you are new to the project, use this order:

1. [First Simulation](tutorials/first-simulation.md)
2. [Precompute and Replay](tutorials/precompute-and-replay.md)
3. [Viewer and GitHub Pages](tutorials/viewer-and-github-pages.md)
4. [Changing Configurations](tutorials/changing-configurations.md)
5. [Configuration Reference](reference/configuration.md)
6. [Package and Type Map](reference/package-type-map.md)

## What LeoDust Simulates

LeoDust models a network made of:

- satellites propagated from TLE data with SGP4
- ground stations positioned with latitude, longitude, and optional altitude
- inter-satellite link protocols
- ground-to-satellite link selection
- routing between nodes
- optional simulation and state plugins
- serialized snapshots for replay and visualization

The current runtime uses an Earth-fixed frame. Satellite positions are propagated and converted to ECEF, ground stations are placed in ECEF, and the viewer also renders from that same Earth-fixed geometry.

## Terminology

LeoDust is written in Go. Go does not have classes in the Java or Python sense, so this documentation uses:

- `package` for a folder/module namespace
- `struct` for concrete stateful runtime objects
- `interface` for behavioral contracts

When this documentation says “class”, it is referring to a Go `struct` or `interface` used like a runtime component.

## Documentation Structure

The documentation is split into three layers:

- Tutorials:
  step-by-step workflows that start from a clean checkout and explain what to run and why
- Concepts:
  model-level explanations such as ECEF, TLE freshness, routing, and replay
- Reference:
  detailed pages for config files, runtime components, bundled presets, data files, and package/type maps

If you are preparing a Read the Docs site, the most important starting pages are:

- [First Simulation](tutorials/first-simulation.md)
- [Runtime Assembly](tutorials/runtime-assembly.md)
- [Configuration Reference](reference/configuration.md)
- [Runtime Components](reference/runtime-components.md)

## Build the Docs Locally

Install MkDocs:

```bash
python -m pip install -r docs/requirements.txt
```

Run the local docs server from the repository root:

```bash
mkdocs serve
```

Build the static docs site:

```bash
mkdocs build
```

MkDocs writes the generated site into `site/` by default.

Important:

- keep `docs/` for MkDocs source files
- do not export the viewer static bundle into `docs/`
- export the viewer into a separate folder such as `viewer-site/`

## Read the Docs

This repository already includes:

- `mkdocs.yml`
- `.readthedocs.yml`
- `docs/requirements.txt`

That is enough for a standard Read the Docs MkDocs build without extra plugins.
