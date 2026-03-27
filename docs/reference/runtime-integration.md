# Runtime Integration

This page documents the supported simulator-to-runtime path:

- `RuntimeReconcilePlugin` exports the active runtime plan from LeoDust
- `runtime-controller` consumes that plan
- the controller reconciles endpoint sandboxes, relay sandboxes, and per-link network state

This path is optional. If you do not enable `RuntimeReconcilePlugin`, the simulator runs exactly as it does today.

## Simulator Side

Enable `RuntimeReconcilePlugin` with `--simulationPlugins` and point it at a runtime snapshot file with `--runtimeOutputFile`:

```bash
cd /Users/kenia/workspace/leodust/go

./leodust \
  --simulationConfig ./resources/configs/simulationManualConfig-0250.yaml \
  --simulationPlugins RuntimeReconcilePlugin \
  --runtimeOutputFile ./results/runtime/live_runtime_plan.json
```

The runtime snapshot includes:

- nodes
- active links
- hosted workloads
- per-workload host placement
- hop-by-hop routes between workload-hosting nodes

## Controller Side

Run the standalone controller from the repository root:

```bash
cd /Users/kenia/workspace/leodust

go run ./runtime-controller \
  --runtimeFile ./go/results/runtime/live_runtime_plan.json \
  --clusterName leodust \
  --plugins sandboxes,links \
  --device eth1 \
  --dryRun=false
```

The controller is internally plugin-driven.

Supported controller plugins:

- `sandboxes`
  creates endpoint sandboxes for workload hosts, relay sandboxes for intermediate hops, and prunes stale controller-managed sandboxes
- `links`
  programs `/32` policy routes, `iptables` marks, and shared `tc` queues on `eth1` so active relay links become real shared bottlenecks

Examples:

Only reconcile sandboxes:

```bash
go run ./runtime-controller \
  --runtimeFile ./go/results/runtime/live_runtime_plan.json \
  --clusterName leodust \
  --plugins sandboxes
```

Disable all controller actions but keep polling:

```bash
go run ./runtime-controller \
  --runtimeFile ./go/results/runtime/live_runtime_plan.json \
  --clusterName leodust \
  --plugins none
```

## What Gets Materialized

The controller does not turn every simulated satellite into a Kubernetes node.

Instead it materializes only the active runtime graph:

- endpoint sandboxes for satellites currently hosting workloads
- relay sandboxes for satellites that appear on active routes
- shared per-link network state for the union of active routes

That is the supported path for congestion-aware runtime execution.
