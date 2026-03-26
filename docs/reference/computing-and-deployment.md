# Computing and Deployment

This page explains the computing model in detail and shows how it connects to the still-evolving deployment subsystem.

## Why the Computing Layer Exists

LeoDust is more than a graph of links. Nodes can also carry resource capacity.

That lets the simulator answer questions like:

- can this node host a service at all?
- how much spare CPU and memory does it still have?
- should a service live on a satellite or on a ground station?
- what route reaches the node that currently hosts a service?

## Step 1: Start with `types.Computing`

The `types.Computing` interface is the contract the rest of the simulator uses.

At a high level it supports:

- mounting the resource object to a node
- asking for the computing type
- placing and removing services
- checking spare capacity
- inspecting hosted services
- cloning the resource state

This is why node code does not need to know the exact implementation class.

## Step 2: Understand `computing.Computing`

`internal/computing/Computing` is the current concrete implementation.

### Stored State

| Field | Meaning |
|------|---------|
| `Cpu` | Total CPU capacity |
| `Memory` | Total memory capacity |
| `Type` | Logical category such as `Edge` or `Cloud` |
| `CpuUsage` | Currently used CPU |
| `MemoryUsage` | Currently used memory |
| `Services` | Deployed services |
| `node` | Mounted owner node |

### Important Methods

#### `Mount`

Attaches the computing object to one node.

Why it matters:

- placement is rejected if the computing object has not been mounted
- one computing instance should not be shared across multiple nodes

#### `CanPlace`

Checks:

- enough CPU remains
- enough memory remains
- no service with the same name is already present

#### `TryPlaceDeploymentAsync`

This is the placement method used by the deployment-facing logic.

What it does:

1. verifies the computing object is mounted
2. checks `CanPlace`
3. appends the service to the local service list
4. increments CPU and memory usage
5. starts a small asynchronous follow-up placeholder goroutine

The asynchronous portion is intentionally light today. The important state change happens immediately.

#### `RemoveDeploymentAsync`

Removes a service and decrements the tracked usage.

#### `Clone`

Creates a copy of the current computing state.

This matters for replay or for places where you need to duplicate resource state without aliasing the original service slice.

## Step 3: Understand `types.ComputingType`

The logical categories are:

- `None`
- `Edge`
- `Cloud`
- `Any`

How they are used today:

- `None`: no meaningful capacity
- `Edge`: typical satellite profile
- `Cloud`: typical ground-station profile
- `Any`: wildcard-style selector, mostly useful for matching logic rather than a normal bundled profile

## Step 4: Understand `DefaultComputingBuilder`

`DefaultComputingBuilder` converts config entries into fresh computing objects.

This is the key bridge between `computingConfig.yaml` and actual runtime nodes.

### Internal State

It stores:

- the full list of computing profiles from config
- one currently selected profile

### Important Methods

#### `Build`

Creates a `Computing` object from the current selected profile.

#### `BuildWithType`

Creates a `Computing` object for a requested type without mutating the builder’s current selection.

This is the safer method when you are building many nodes with occasional overrides.

#### `WithComputingType`

Mutates the builder’s current selection and returns the builder.

This method still exists, but `BuildWithType` is the clearer option in most modern code paths.

## Step 5: Follow the Config-to-Computing Flow

The flow is:

1. `computingConfig.yaml` is loaded into `[]configs.ComputingConfig`
2. `main.go` creates `DefaultComputingBuilder`
3. node builders request profiles from that builder
4. each node receives its own mounted `Computing` object

That means the config file defines the available profiles, but the builders decide which node gets which profile.

## Step 6: See How Satellites Use Computing

Satellites are built in `SatelliteBuilder`.

Current behavior:

- every live satellite requests `types.Edge`
- the builder uses `BuildWithType(types.Edge)`
- the resulting computing object is mounted into `SatelliteStruct`

So if you change the `Edge` profile in `computingConfig.yaml`, you change the default satellite capacity across the whole constellation.

## Step 7: See How Ground Stations Use Computing

Ground stations are built in `GroundStationBuilder`.

Current behavior:

1. start from the shared builder and config list
2. read the station’s optional YAML `ComputingType`
3. if no override is present, call `Build()`
4. if an override is present, parse it and call `BuildWithType(...)`

This is why the ground-station YAML can customize capacity per station without mutating shared builder state for the next station.

## Step 8: Understand Services and Routing

The routing layer also knows about hosted services.

Notable behavior:

- `DijkstraRouter` maintains service routes in addition to node routes
- `RouteToService` checks the local computing resource first
- if the mounted node already hosts the service, latency is `0`

This is the main point where computing and routing meet in the current implementation.

## Step 9: Understand `DeployableService`

`internal/deployment/DeployableService` is the concrete service object used by the deployment subsystem.

It stores:

- service name
- CPU requirement
- memory requirement

The constructor validates:

- service name must not be empty
- CPU must be positive
- memory must be positive

## Step 10: Understand the Deployment Interfaces

The deployment-facing interfaces live in `go/pkg/types/deployment_interfaces.go`.

### `DeployableService`

Defines what a service must expose:

- name
- CPU usage
- memory usage
- lifecycle methods such as `Deploy()` and `Remove()`

### `DeploymentSpecification`

Defines what a deployment request must expose:

- deployment type
- service payload

### `DeploymentOrchestrator`

Defines the orchestration API:

- supported deployment types
- create deployment
- delete deployment
- check reschedule

## Step 11: Understand `DeploymentOrchestrator`

`internal/deployment/DeploymentOrchestrator` is the top-level concrete orchestrator.

### What It Already Does

- stores deployment specifications
- delegates actions through a resolver
- supports create, delete, and reschedule entry points

### What Is Still Incomplete

- `DeploymentTypes()` is not implemented yet
- the simulation step only has a placeholder check for orchestrator rescheduling
- there is no deeply integrated production-grade scheduler yet

The right mental model is:

- computing is active and useful today
- deployment exists structurally but is still an evolving subsystem

## Step 12: Practical Patterns

### If you only care about network simulation

- keep the bundled `computingConfig.yaml`
- let satellites stay on `Edge`
- use `Cloud` for most ground stations
- ignore deployment orchestration for now

### If you want to test placement behavior

- keep the constellation small
- keep the number of services small
- use simple CPU and memory numbers
- prefer replay or short live runs for repeatability

### If you want to evolve the deployment subsystem

Read these code areas together:

1. `internal/computing`
2. `pkg/types/deployment_interfaces.go`
3. `internal/deployment`
4. `internal/routing/dijkstra_router.go`

That is the smallest useful slice of the codebase for deployment-related changes.
