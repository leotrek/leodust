# Package and Type Map

This page is the high-level type map for the runtime code.

It is written as a practical reference, not as generated API docs. Test-only helper types are omitted. Internal helper types are included when they matter to understanding the runtime.

## `go/cmd/leodust`

| Type | Kind | Role |
|------|------|------|
| `main` package entrypoint | command | Loads configs, builds the simulation, and starts autorun or the manual CLI path. |

## `go/cmd/leodust-viewer`

| Type | Kind | Role |
|------|------|------|
| `ViewerServer` | struct | Serves the local snapshot viewer and static-style asset paths. |
| `snapshotCatalog` | struct | Internal in-memory list of available snapshot datasets. |
| `snapshotCatalogEntry` | struct | One dataset entry with filename, counts, and relative path metadata. |
| `snapshotCatalogResponse` | struct | Manifest payload written for the viewer. |
| `snapshotSummary` | struct | Lightweight decoded snapshot summary used while building the catalog. |
| `viewerConfig` | struct | Static viewer config file describing the Earth image and manifest locations. |

## `go/configs`

| Type | Kind | Role |
|------|------|------|
| `SimulationConfig` | struct | Top-level simulation runtime configuration. |
| `InterSatelliteLinkConfig` | struct | ISL protocol configuration. |
| `GroundLinkConfig` | struct | Ground-link protocol configuration. |
| `RouterConfig` | struct | Router selection config. |
| `ComputingConfig` | struct | One computing profile entry. |

## `go/internal/orbit`

| Type | Kind | Role |
|------|------|------|
| `ECIState` | struct | Propagated inertial position and velocity state. |
| `Propagator` | interface | Propagation contract used by satellite nodes/builders. |
| `TLEPropagator` | struct | SGP4-based propagator wrapper used by the TLE path. |
| `ReferenceSample` | struct | One published orbit-validation sample. |
| `ReferenceCase` | struct | A validation case made of TLE plus samples. |
| `ReferenceSampleResult` | struct | Per-sample validation result. |
| `ReferenceValidationResult` | struct | Aggregate validation result for a reference case. |

## `go/internal/ground`

| Type | Kind | Role |
|------|------|------|
| `rawGroundStation` | struct | YAML-decoded ground-station record. |
| `GroundStationYmlLoader` | struct | Reads YAML and turns it into ground-station objects. |
| `GroundStationLoaderService` | struct | Loads ground stations and injects them into the simulation. |
| `GroundStationSpec` | struct | Per-station normalized input used by the builder. |
| `GroundStationBuilder` | struct | Builds live ground-station nodes from shared dependencies plus a station spec. |

## `go/internal/stateplugin`

| Type | Kind | Role |
|------|------|------|
| `SunStatePlugin` | interface | State-plugin contract for sunlight exposure access. |
| `DummySunStatePlugin` | struct | Example live state plugin that generates placeholder sunlight values. |
| `DummySunStatePluginPrecomp` | struct | Replay-time reader for the dummy sunlight sidecar. |
| `DefaultStatePluginBuilder` | struct | Builds live state plugins from CLI names. |
| `DefaultStatePluginPrecompBuilder` | struct | Builds replay-time state plugins from serialized metadata. |

## `go/internal/simulation`

| Type | Kind | Role |
|------|------|------|
| `BaseSimulationService` | struct | Shared simulation-time, node-injection, and stepping base. |
| `SimulationService` | struct | Live simulation implementation. |
| `SimulationIteratorService` | struct | Replay-mode simulation implementation. |
| `SimulationStateSerializer` | struct | Writes `.gob` and `.json` simulation snapshots. |
| `SimulationStateDeserializer` | struct | Reconstructs replay-mode nodes and links from `.gob` snapshots. |

## `go/internal/links`

| Type | Kind | Role |
|------|------|------|
| `GroundProtocolBuilder` | struct | Builds ground-to-satellite link protocols. |
| `GroundSatelliteNearestProtocol` | struct | Picks the nearest visible satellite for each ground station. |
| `IslProtocolBuilder` | struct | Selects and builds the configured ISL protocol. |
| `IslNearestProtocol` | struct | ISL nearest-neighbor strategy. |
| `IslMstProtocol` | struct | MST-based ISL topology builder. |
| `IslPstProtocol` | struct | PST-based ISL topology builder. |
| `IslSatelliteCentricMstProtocol` | struct | Satellite-centric MST variant. |
| `IslAddLoopProtocol` | struct | Decorator that adds loop edges around an inner protocol result. |
| `IslAddSmartLoopProtocol` | struct | Decorator that adds smarter loop behavior around an inner protocol result. |
| `LinkFilterProtocol` | struct | Wraps another link protocol and filters unreachable links. |
| `PrecomputedLinkProtocol` | struct | Link protocol used for replay-time precomputed links. |

## `go/internal/links/linktypes`

| Type | Kind | Role |
|------|------|------|
| `GroundLink` | struct | Concrete live ground-to-satellite link. |
| `IslLink` | struct | Concrete live inter-satellite link. |
| `PrecomputedLink` | struct | Replay-time link object. |
| `LinkPriorityQueue` | struct | Helper queue used by graph-building logic. |
| `linkItem` | struct | Internal queue item for `LinkPriorityQueue`. |

## `go/internal/deployment`

| Type | Kind | Role |
|------|------|------|
| `DeployableService` | struct | Concrete deployable service with CPU and memory requirements. |
| `DeploymentOrchestrator` | struct | Top-level deployment coordinator. |
| `DeploymentOrchestratorResolver` | struct | Chooses the right orchestrator implementation for a deployment. |

## `go/internal/satellite`

| Type | Kind | Role |
|------|------|------|
| `SatelliteBuilder` | struct | Builds live satellites from TLE-derived input. |
| `TleLoader` | struct | Parses TLE files into satellite objects. |
| `TLERecord` | struct | Parsed named TLE record. |
| `SatelliteConstellationLoader` | struct | Loads a satellite collection and wires inter-satellite links. |
| `SatelliteDataSourceLoader` | interface | Plug-in contract for satellite data loaders. |
| `SatelliteLoaderService` | struct | Loads satellites and injects them into the simulation. |
| `TLESummary` | struct | Summary of a TLE file including record counts and epoch range. |
| `DownloadResult` | struct | Output of the fresh-TLE download path. |

## `go/internal/simplugin`

| Type | Kind | Role |
|------|------|------|
| `DummyPlugin` | struct | Example simulation plugin. |
| `SimPluginBuilder` | struct | Builds simulation plugins from CLI names. |

## `go/internal/computing`

| Type | Kind | Role |
|------|------|------|
| `ComputingBuilder` | interface | Builder contract for computing instances. |
| `DefaultComputingBuilder` | struct | Concrete computing builder driven by config entries. |
| `Computing` | struct | Concrete runtime implementation of node computing resources. |

## `go/internal/routing`

| Type | Kind | Role |
|------|------|------|
| `RouterBuilder` | struct | Creates routers from config. |
| `AStarRouter` | struct | On-demand A* router. |
| `DijkstraRouter` | struct | Shortest-path router with precomputation support. |
| `PreRouteResult` | struct | Route result returned from precomputed routing. |
| `OnRouteResult` | struct | Route result returned from on-demand routing. |
| `UnreachableRouteResult` | struct | Route result used when no path exists. |
| `routeEntry` | struct | Internal Dijkstra routing-table entry. |
| `dijkstraEntry` | struct | Internal Dijkstra frontier entry. |

## `go/internal/node`

| Type | Kind | Role |
|------|------|------|
| `BaseNode` | struct | Shared base fields and helpers for live nodes. |
| `SatelliteStruct` | struct | Live satellite node. |
| `GroundStationStruct` | struct | Live ground-station node. |
| `PrecomputedNode` | interface | Replay-mode node contract. |
| `PrecomputedSatellite` | struct | Replay-mode satellite node. |
| `PrecomputedGroundStation` | struct | Replay-mode ground-station node. |

## `go/pkg/types`

| Type | Kind | Role |
|------|------|------|
| `Node` | interface | Common behavior for all nodes. |
| `Satellite` | interface | Satellite-specific node contract. |
| `GroundStation` | interface | Ground-station-specific node contract. |
| `Link` | interface | Common link behavior. |
| `LinkNodeProtocol` | interface | Common contract for node-attached link protocols. |
| `GroundSatelliteLinkProtocol` | interface | Contract for ground-to-satellite link logic. |
| `InterSatelliteLinkProtocol` | interface | Contract for inter-satellite link logic. |
| `Router` | interface | Routing contract used by all nodes. |
| `Route` | struct | Basic route hop description. |
| `RouteResult` | interface | Route calculation result contract. |
| `Payload` | interface | Placeholder payload abstraction. |
| `SimulationController` | interface | Main simulation control surface. |
| `SimulationPlugin` | interface | Per-step plugin contract. |
| `StatePlugin` | interface | State plugin contract. |
| `StatePluginBuilder` | interface | Builder contract for state plugins. |
| `StatePluginRepository` | struct | Type-based state-plugin registry. |
| `Computing` | interface | Computing-resource contract. |
| `ComputingType` | enum-like type | Node computing category. |
| `DeployableService` | interface | Service deployment contract. |
| `DeploymentSpecification` | interface | Deployment request contract. |
| `DeploymentOrchestrator` | interface | Deployment orchestrator contract. |
| `Vector` | struct | 3D vector for positions and geometry. |
| `SimulationLink` | struct | Serialized link metadata. |
| `SimulationMetadata` | struct | Top-level serialized simulation object. |
| `SimulationState` | struct | One serialized time frame. |
| `NodeState` | struct | Serialized node state for one frame. |
| `RawSatellite` | struct | Serialized satellite metadata. |
| `RawGroundStation` | struct | Serialized ground-station metadata. |

## `go/pkg/helper`

| Type | Kind | Role |
|------|------|------|
| `ManualResetEvent` | struct | Synchronization primitive used by some runtime flows. |

## `go/pkg/logging`

The logging package is function-oriented rather than class-heavy.

Key runtime type:

| Type | Kind | Role |
|------|------|------|
| `Level` | type | Global logging verbosity level used by the CLI and services. |
