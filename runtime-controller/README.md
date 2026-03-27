# Runtime Controller

`runtime-controller` consumes the `RuntimeReconcilePlugin` snapshot and reconciles the active runtime graph into:

- endpoint sandboxes for nodes that currently host workloads
- relay sandboxes for intermediate hops on active routes
- per-link route and queue programming on the sandbox simulation interface

It is a standalone daemon. Nothing changes in the simulator unless you enable `RuntimeReconcilePlugin`, and nothing changes in the runtime unless you run this controller.

## Plugins

Controller behavior is split into optional internal plugins:

- `sandboxes`: create/update/delete active sandboxes through `scripts/microk8s_cluster.sh`
- `links`: project the active route union into `eth1` using policy routes, `iptables` marks, and `tc`

Enable only the parts you want:

```bash
cd /Users/kenia/workspace/leodust

go run ./runtime-controller \
  --runtimeFile ./go/results/runtime/live_runtime_plan.json \
  --clusterName leodust \
  --plugins sandboxes,links \
  --dryRun=true
```

To only reconcile sandboxes:

```bash
go run ./runtime-controller \
  --runtimeFile ./go/results/runtime/live_runtime_plan.json \
  --clusterName leodust \
  --plugins sandboxes
```

## How Link Projection Works

The controller does not collapse a multi-hop route into a direct endpoint tunnel. It uses the active route union from the snapshot:

- for a route `A -> B -> C -> D`, `A`, `B`, `C`, and `D` are all active
- `A-B`, `B-C`, and `C-D` become active shared links
- each sandbox gets `/32` policy routes for endpoint destinations via its next hop
- `tc` classes are grouped per next hop so multiple destinations sharing the same link also share the same queue

That means two application flows that both traverse `B-C` contend on the same `B-C` queue.

## Important Boundaries

- This controller only creates sandboxes and network state. It does not launch application processes yet.
- Sandboxes do not join MicroK8s. The stable cluster nodes remain the only Kubernetes nodes.
- The `links` plugin assumes all active sandboxes are attached to the simulation bridge and that `eth1` is the simulation interface.
