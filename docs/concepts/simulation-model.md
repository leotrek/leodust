# Simulation Model

This page explains how LeoDust is structured conceptually.

## The Main Runtime Pipeline

The normal live runtime is:

1. load configs
2. configure logging
3. resolve TLE and ground-station inputs
4. build node builders
5. load satellites
6. load ground stations
7. inject everything into the simulation service
8. execute autorun or the manual CLI path

## The Main Components

### Nodes

There are two primary live node types:

- satellites
- ground stations

Both are nodes with:

- a name
- a position
- a router
- a computing resource
- a link protocol

### Links

Links represent communication edges between nodes.

In live mode, links are selected by protocols. In replay mode, links are reconstructed from serialized state.

### Routers

Routers calculate how traffic moves through the node graph.

Current router implementations:

- A*
- Dijkstra

### Computing

Each node can have a computing resource attached to it. This is how LeoDust models deployment capacity and service hosting.

### Simulation Plugins

Simulation plugins run each step and can add scenario-specific behavior.

### State Plugins

State plugins calculate state that can be serialized and replayed later.

## Time Model

LeoDust uses explicit simulation time.

The starting point is `SimulationStartTime`.

### Autorun

Autorun advances time by `StepMultiplier` simulated seconds per step and waits `StepInterval` real milliseconds between steps.

### Manual CLI Path

The current manual CLI path advances by `600` simulated seconds per loop iteration.

### Replay

Replay mode uses the timestamps stored in the serialized state file.

## Serialization Model

The serializer writes a `SimulationMetadata` object that contains:

- satellites
- ground stations
- links
- state-plugin names
- a list of time-stamped node states

This split is important:

- topology and metadata are stored once
- time-varying node state is stored per frame

## Live vs Replay

### Live

- computes positions from TLEs
- updates links dynamically
- can precompute routing
- can write snapshots

### Replay

- rebuilds simulated nodes from metadata
- reuses stored positions and links
- is faster and deterministic

## Viewer Model

The viewer is not a live simulation renderer. It is a playback renderer over serialized JSON frames.

That makes it suitable for:

- local inspection
- demo artifacts
- static hosting
