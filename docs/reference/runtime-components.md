# Runtime Components

This page is the narrative reference for the main runtime structs and interfaces.

The goal is not to reproduce generated API docs. The goal is to explain:

- why a type exists
- who creates it
- what it depends on
- which methods matter when you work on the simulator

For the shorter package-by-package inventory, see [Package and Type Map](package-type-map.md).

## Command Entrypoints

### `go/cmd/leodust`

Role:

- composition root for the live simulator and replay mode

Creates:

- config objects
- builders
- loader services
- `SimulationService` or `SimulationIteratorService`

Important functions:

- `main`
- `startSimulation`
- `startSimulationIteration`
- `runController`

Plugin-specific wiring:

- `parseListFlag`
- `mustBuildSimulationPlugins`
- `mustBuildStatePlugins`

### `go/cmd/leodust-viewer`

Role:

- local snapshot viewer server
- static export tool for the browser viewer

Creates:

- `ViewerServer`
- snapshot catalog
- static viewer bundle

## Configuration Types

## Plugin Construction Types

For the plugin workflow itself, see [Plugin Reference](plugins.md).

### `types.SimulationPlugin`

Purpose:

- run post-step logic after the core simulation update
- export or react to live runtime state

Important method:

- `PostSimulationStep`

### `types.StatePlugin`

Purpose:

- compute derived step state
- save and reload that state for replay

Important methods:

- `PostSimulationStep`
- `AddState`
- `Save`

### `simplugin.SimPluginBuilder`

Purpose:

- map CLI plugin names to concrete simulation plugin instances
- hold plugin-specific constructor inputs such as topology export paths

Important methods:

- `WithTopologyOutputFile`
- `BuildPlugins`

### `stateplugin.DefaultStatePluginBuilder`

Purpose:

- build the live state plugin instances used during normal simulation

Important method:

- `BuildPlugins`

### `stateplugin.DefaultStatePluginPrecompBuilder`

Purpose:

- build replay-side state plugin instances when a `.gob` snapshot is loaded

Important method:

- `BuildPlugins`

### `configs.SimulationConfig`

Purpose:

- define time stepping, data sources, routing-table behavior, and logging defaults

Used by:

- `main.go`
- `BaseSimulationService`
- serializer and replay startup

### `configs.InterSatelliteLinkConfig`

Purpose:

- choose ISL protocol and neighbor target

Used by:

- `IslProtocolBuilder`

### `configs.GroundLinkConfig`

Purpose:

- choose ground-station uplink selection strategy

Used by:

- `GroundProtocolBuilder`
- `GroundStationBuilder`

### `configs.RouterConfig`

Purpose:

- choose the router implementation

Used by:

- `RouterBuilder`

### `configs.ComputingConfig`

Purpose:

- declare one named computing capacity profile

Used by:

- `DefaultComputingBuilder`

## Satellite Construction Types

### `satellite.TLERecord`

Purpose:

- normalized parsed representation of one named TLE entry

Contains:

- satellite name
- TLE line 1
- TLE line 2
- parsed epoch metadata used by the TLE summary path

### `satellite.TleLoader`

Purpose:

- read TLE text from a stream
- parse it into `TLERecord`
- build live satellites from those records

Created by:

- `main.startSimulation`

Important method:

- `Load`

### `satellite.SatelliteBuilder`

Purpose:

- create one live `SatelliteStruct` from one `TLERecord`

Depends on:

- initial simulation time
- `RouterBuilder`
- `DefaultComputingBuilder`
- `IslProtocolBuilder`

Important method:

- `Build`

### `orbit.TLEPropagator`

Purpose:

- wrap the SGP4 propagation library
- expose ECEF position calculation to satellite nodes

Used by:

- `SatelliteStruct.UpdatePosition`

### `satellite.SatelliteConstellationLoader`

Purpose:

- registry for source-type-specific satellite loaders
- wiring candidate ISL links across the constellation

Important methods:

- `RegisterDataSourceLoader`
- `LoadSatelliteConstellation`

### `satellite.SatelliteLoaderService`

Purpose:

- bridge loader output into the simulation service

## Ground-Station Construction Types

### `ground.rawGroundStation`

Purpose:

- YAML decoding struct for one ground-station entry

Used only inside:

- `GroundStationYmlLoader`

### `ground.GroundStationSpec`

Purpose:

- normalized per-station input used by `GroundStationBuilder`

This is the cleaned representation after YAML parsing and before live object creation.

### `ground.GroundStationBuilder`

Purpose:

- create one `GroundStationStruct` from one `GroundStationSpec`

Depends on:

- `RouterBuilder`
- `DefaultComputingBuilder`
- default `GroundLinkConfig`
- current satellite slice

Important methods:

- `Build`
- internal `buildComputing`

### `ground.GroundStationYmlLoader`

Purpose:

- read the ground-station YAML file
- convert entries into `GroundStationSpec`
- build live ground stations

Important method:

- `Load`

### `ground.GroundStationLoaderService`

Purpose:

- inject built ground stations into the simulation service

## Live Node Types

### `node.BaseNode`

Purpose:

- shared node fields and basic geometry helpers

Owns:

- name
- router
- computing resource
- current position

Important method:

- `DistanceTo`

### `node.SatelliteStruct`

Purpose:

- live satellite node

Owns:

- propagator
- ISL protocol
- embedded `BaseNode`

Important methods:

- `UpdatePosition`
- `GetISLProtocol`

### `node.GroundStationStruct`

Purpose:

- live ground-station node

Owns:

- latitude
- longitude
- altitude
- ground-link protocol
- embedded `BaseNode`

Important methods:

- `UpdatePosition`
- `GetLinkNodeProtocol`

## Link Protocol Types

### `links.GroundProtocolBuilder`

Purpose:

- choose the ground-link protocol implementation for a station

Important method:

- `Build`

### `links.GroundSatelliteNearestProtocol`

Purpose:

- maintain exactly one active uplink from a ground station to the nearest visible satellite

Important behavior:

- horizon filtering happens here
- it can switch the active uplink when another visible satellite becomes closer

Important methods:

- `Mount`
- `UpdateLinks`
- `Link`

### `links.IslProtocolBuilder`

Purpose:

- choose and compose the inter-satellite link protocol

Why it matters:

- some protocols are simple concrete strategies
- others are decorators layered around MST or PST behavior

### `links.IslNearestProtocol`

Purpose:

- activate the nearest `N` reachable ISLs for a satellite

Key config input:

- `Neighbours`

### `links.IslMstProtocol`

Purpose:

- build an MST-style inter-satellite topology

### `links.IslPstProtocol`

Purpose:

- build the PST topology variant

### `links.IslSatelliteCentricMstProtocol`

Purpose:

- alternative MST-like strategy centered more directly on per-satellite choices

### `links.IslAddLoopProtocol`

Purpose:

- decorate another ISL strategy by adding loop edges

### `links.IslAddSmartLoopProtocol`

Purpose:

- decorate another ISL strategy with more selective loop logic

### `links.LinkFilterProtocol`

Purpose:

- wrap another ISL strategy and remove links that are not physically reachable

This type is important for correctness because raw candidate links alone are not enough.

## Link Value Types

### `linktypes.GroundLink`

Purpose:

- concrete ground-to-satellite link object

Important behavior:

- computes distance and latency
- checks Earth-occlusion reachability

### `linktypes.IslLink`

Purpose:

- concrete satellite-to-satellite link object

Important behavior:

- computes distance, latency, and reachability

### `linktypes.PrecomputedLink`

Purpose:

- replay-mode link object reconstructed from serialized frames

## Router Types

### `routing.RouterBuilder`

Purpose:

- create one router per node from config

Important method:

- `Build`

### `routing.AStarRouter`

Purpose:

- perform on-demand pathfinding between nodes

Important behavior:

- no meaningful pre-routing table
- route cost is latency-based
- heuristic is geometric distance converted using speed of light

Important methods:

- `RouteToNode`
- `RouteToService`
- `RouteTo`

### `routing.DijkstraRouter`

Purpose:

- calculate shortest paths and optionally cache them in a routing table

Important behavior:

- supports `CalculateRoutingTable`
- works best with `UsePreRouteCalc: true`
- also tracks service routes

Important methods:

- `CalculateRoutingTable`
- `RouteToNode`
- `RouteToService`

## Computing and Deployment Types

### `computing.Computing`

Purpose:

- concrete resource container mounted on a node

Owns:

- total CPU
- total memory
- used CPU
- used memory
- deployed services
- mounted node reference

Important methods:

- `Mount`
- `CanPlace`
- `TryPlaceDeploymentAsync`
- `RemoveDeploymentAsync`
- `Clone`

### `computing.DefaultComputingBuilder`

Purpose:

- convert config profiles into fresh `Computing` objects

Important methods:

- `Build`
- `BuildWithType`
- `WithComputingType`

### `deployment.DeployableService`

Purpose:

- concrete service request with CPU and memory requirements

### `deployment.DeploymentOrchestrator`

Purpose:

- top-level deployment coordination object

Current maturity:

- partial
- useful as structure, not yet a fully integrated scheduling system

## Simulation Types

### `simulation.BaseSimulationService`

Purpose:

- shared node storage, time state, autorun state, and stepping helpers

Important methods:

- `InjectSatellites`
- `InjectGroundStations`
- `StartAutorun`
- `StepBySeconds`
- `StepByTime`

### `simulation.SimulationService`

Purpose:

- live simulation implementation

Important method:

- `runSimulationStep`

That method is the main runtime heartbeat.

### `simulation.SimulationIteratorService`

Purpose:

- replay-mode simulation implementation over stored frames

### `simulation.SimulationStateSerializer`

Purpose:

- collect frames during live execution
- write `.gob` and `.json` outputs on close

### `simulation.SimulationStateDeserializer`

Purpose:

- rebuild replay-mode nodes, links, and metadata from serialized state

## Plugin Types

### `simplugin.SimPluginBuilder`

Purpose:

- map CLI plugin names to live simulation-plugin instances

### `simplugin.DummyPlugin`

Purpose:

- example simulation plugin

### `stateplugin.DefaultStatePluginBuilder`

Purpose:

- map CLI plugin names to live state-plugin instances

### `stateplugin.DefaultStatePluginPrecompBuilder`

Purpose:

- rebuild replay-side state plugins from serialized metadata

### `stateplugin.DummySunStatePlugin`

Purpose:

- example state plugin that computes placeholder sunlight values during live simulation

### `stateplugin.DummySunStatePluginPrecomp`

Purpose:

- replay-side reader for the dummy sunlight sidecar

### `types.StatePluginRepository`

Purpose:

- type-indexed repository of active state plugins

Important functions:

- `NewStatePluginRepository`
- `FindStatePlugin`
- `GetStatePlugin`

Use `FindStatePlugin` for optional lookups. `GetStatePlugin` panics if the plugin is absent.

## Viewer Types

### `ViewerServer`

Purpose:

- serve the local viewer assets and the static-style dataset manifest

### `snapshotCatalog`

Purpose:

- inspect precomputed JSON snapshots and build a compact dataset manifest for the viewer UI

### `viewerConfig`

Purpose:

- describe the Earth image and manifest paths for the browser app

## How to Use This Page

When you are changing behavior, follow this order:

1. find the config type
2. find the builder that consumes it
3. find the runtime struct the builder creates
4. find the simulation service or viewer code that calls it later

That path is usually enough to understand the system without reading the whole repository.
