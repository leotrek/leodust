# Cluster Bootstrap

`microk8s_cluster.sh` is the only supported bootstrap entrypoint for the LXC-based simulation cluster.

The tool is intentionally opinionated:

- one LXD container acts as the MicroK8s control plane
- additional LXD containers join as workers
- cluster nodes are stable execution capacity
- satellite sandboxes are separate LXD containers created only for active satellites
- applications run inside satellite sandboxes, not as MicroK8s nodes
- every node gets two interfaces:
  - `eth0` management plane for MicroK8s control traffic
  - `eth1` simulation plane for runtime-controller link programming
- the default node image is `ubuntu-minimal:22.04` to reduce per-node footprint while staying on an official Ubuntu base
- when you provide `--worker-names-file`, worker container names are derived by sanitizing the simulation node names
- the bootstrap can optionally write a JSON name-map file for external tooling or debugging
- cluster nodes and satellite sandboxes now reuse cached LXD launch images after the first build so repeated launches do not reinstall the same base packages from scratch

Note:

- this bootstrap intentionally uses privileged LXD containers because the simulation wants each node to be a container
- MicroK8s traffic should stay on `eth0`; runtime network programming should only touch `eth1`
- upstream MicroK8s documentation recommends LXD VMs instead of containers for stronger isolation and fewer systemd/AppArmor edge cases
- the bootstrap explicitly installs `snapd`, `netplan.io`, `iproute2`, `iptables`, and `ca-certificates` inside each node so `ubuntu-minimal` still satisfies the networking and MicroK8s requirements

Network topology is no longer generated from the scripts directory.

- live runtime plans should come from `RuntimeReconcilePlugin` in the simulator
- sandbox and link reconciliation should be handled by the standalone `runtime-controller/` tool

Why the older scripts were removed:

- they duplicated bootstrap logic in multiple places
- they encoded one-off assumptions like random `tc` shaping and hardcoded container names
- `generate_network_topology_json.sh` scraped current `tc` state and generated a fully connected graph, which does not match the simulator-driven topology model

This script now manages two layers:

- `cluster nodes`: the MicroK8s control plane and stable workers
- `satellite sandboxes`: lazily created runtime containers representing active satellites

That split is the basis for scaling the simulator without trying to make every simulated satellite a Kubernetes node.

Example:

```bash
./scripts/microk8s_cluster.sh install \
  --cluster-name leodust
```

From scratch, `install` and `host-setup` are the same command. They install host prerequisites, install and initialize LXD, create the management and simulation bridges, and create the node profile.

Then create the cluster:

```bash
./scripts/microk8s_cluster.sh create \
  --cluster-name leodust \
  --control-plane-name master \
  --worker-names-file ./go/resources/tle/starlink_250.tle
```

Export a host-usable kubeconfig from the control-plane container:

```bash
./scripts/microk8s_cluster.sh kubeconfig \
  --cluster-name leodust
```

By default this writes `./results/cluster/<cluster>-kubeconfig.yaml` and rewrites the API server to the control-plane management IP so host `kubectl` can reach it.

This creates workers like `starlink-1008`, `starlink-1012`, and so on, which line up with the same name sanitization used by the runtime controller for satellite sandboxes.

Add more workers later from the same names file:

```bash
./scripts/microk8s_cluster.sh add-workers \
  --cluster-name leodust \
  --control-plane-name master \
  --worker-names-file ./go/resources/tle/starlink_250.tle \
  --workers 2
```

Create a live satellite sandbox without joining it to MicroK8s:

```bash
./scripts/microk8s_cluster.sh sandbox-create \
  --cluster-name leodust \
  --satellite-id STARLINK-3393 \
  --sandbox-role endpoint
```

Create or update multiple sandboxes in one call:

```bash
./scripts/microk8s_cluster.sh sandbox-create-many \
  --cluster-name leodust \
  --sandbox-specs 'STARLINK-3393|endpoint;STARLINK-3394|relay'
```

Start an application inside that sandbox:

```bash
./scripts/microk8s_cluster.sh app-start \
  --cluster-name leodust \
  --satellite-id STARLINK-3393 \
  --app-name telemetry \
  --app-command 'python3 -m http.server 8080'
```

Inspect current live sandboxes and managed applications:

```bash
./scripts/microk8s_cluster.sh sandbox-list --cluster-name leodust
./scripts/microk8s_cluster.sh app-list --cluster-name leodust
```

Show cluster status:

```bash
./scripts/microk8s_cluster.sh status \
  --cluster-name leodust \
  --control-plane-name master
```

List every cluster the script can discover, including older MicroK8s-in-LXC clusters that were not originally created by this script:

```bash
./scripts/microk8s_cluster.sh list
```

Show detailed status for all managed clusters:

```bash
./scripts/microk8s_cluster.sh status --all-clusters true
```

Show DEBUG logs, including the concrete `lxc` and in-container commands the script is issuing:

```bash
./scripts/microk8s_cluster.sh list --debug true
```

Run a quick smoke test that proves:

- the scheduler placed a workload on a satellite worker
- the service is reachable inside the cluster

```bash
./scripts/microk8s_cluster.sh test \
  --cluster-name leodust \
  --control-plane-name master
```

Destroy the cluster containers but keep the LXD profile and networks for fast rebuilds:

```bash
./scripts/microk8s_cluster.sh destroy \
  --cluster-name leodust \
  --control-plane-name master
```

Uninstall the cluster cleanly, including the script-created profile, bridges, and generated node-map file:

```bash
./scripts/microk8s_cluster.sh uninstall \
  --cluster-name leodust \
  --control-plane-name master
```

If you also want to remove the host LXD snap:

```bash
./scripts/microk8s_cluster.sh uninstall \
  --cluster-name leodust \
  --control-plane-name master \
  --remove-lxd true
```

Run the simulator with runtime export enabled:

```bash
cd go

./leodust \
  --simulationConfig ./resources/configs/simulationManualConfig-0250.yaml \
  --simulationPlugins RuntimeReconcilePlugin \
  --runtimeOutputFile ./results/runtime/live_runtime_plan.json
```

Then run the standalone runtime controller:

```bash
cd ..

go run ./runtime-controller \
  --runtimeFile ./go/results/runtime/live_runtime_plan.json \
  --clusterName leodust \
  --plugins sandboxes,links \
  --device eth1 \
  --dryRun=false
```

The first container launch for a given cluster still has to build the cached node or sandbox image. After that, `create`, `add-workers`, and runtime-controller-driven sandbox creation reuse those cached images and should be substantially faster.

## Verifying the Cluster Network

Do not treat the generated topology snapshot by itself as proof that the cluster network is programmed.

Use the files and commands below as separate checks:

- `./go/results/topology/live_topology.json` proves the simulator computed a network graph
- `./go/results/runtime/live_runtime_plan.json` proves the simulator exported an active runtime graph for workloads and routes
- `./scripts/microk8s_cluster.sh status` and `test` prove the MicroK8s cluster is healthy on `eth0`
- `sandbox-list` plus `ip route`, `iptables`, and `tc` inside a sandbox prove the runtime controller programmed the simulated network on `eth1`

Check that the cluster itself is healthy:

```bash
./scripts/microk8s_cluster.sh status --cluster-name leodust
./scripts/microk8s_cluster.sh test --cluster-name leodust
```

Check the generated topology snapshot:

```bash
jq '{version, time, generated_at, nodes: (.nodes|length), links: (.links|length)}' \
  ./go/results/topology/live_topology.json
```

This confirms that the simulator generated a network snapshot. It does not prove that the runtime controller applied that topology to the cluster.

Check whether the runtime export contains any active network to materialize:

```bash
jq '{generated_at, workloads: (.workloads|length), routes: (.routes|length)}' \
  ./go/results/runtime/live_runtime_plan.json
./scripts/microk8s_cluster.sh sandbox-list --cluster-name leodust
```

Interpretation:

- if `workloads=0` and `routes=0`, the controller has no active runtime graph to apply yet
- if workloads and routes are nonzero, you should also see live endpoint and relay sandboxes

To generate a non-empty runtime graph for testing, run the simulator with both topology and runtime export enabled and inject a small number of synthetic workloads:

```bash
cd ./go

go run ./cmd/leodust \
  --simulationConfig ./resources/configs/simulationManualConfig-0250.yaml \
  --islConfig ./resources/configs/islMstConfig.yaml \
  --groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
  --computingConfig ./resources/configs/computingConfig.yaml \
  --routerConfig ./resources/configs/routerAStarConfig.yaml \
  --simulationPlugins TopologyExportPlugin,RuntimeReconcilePlugin \
  --injectTestWorkloads 2 \
  --topologyOutputFile ./results/topology/live_topology.json \
  --runtimeOutputFile ./results/runtime/live_runtime_plan.json
```

Then reconcile it into the cluster:

```bash
cd ..

go run ./runtime-controller \
  --runtimeFile ./go/results/runtime/live_runtime_plan.json \
  --clusterName leodust \
  --plugins sandboxes,links \
  --device eth1 \
  --dryRun=false
```

Once sandboxes exist, inspect one sandbox directly to verify that the simulated network is actually programmed on `eth1`:

```bash
lxc exec <sandbox-name> -- ip -4 addr show dev eth1
lxc exec <sandbox-name> -- ip route show table 200
lxc exec <sandbox-name> -- iptables -t mangle -S LEODUST-RUNTIME-MANGLE
lxc exec <sandbox-name> -- tc qdisc show dev eth1
```

If `lxc` is not on `PATH`, use `/snap/bin/lxc`.

What a working result looks like:

- `status` and `test` succeed
- `live_topology.json` updates over time
- `live_runtime_plan.json` shows `workloads > 0` and `routes > 0`
- `sandbox-list` shows active sandboxes
- inside a sandbox, table `200`, chain `LEODUST-RUNTIME-MANGLE`, and `tc` state exist on `eth1`

If you pass `--node-map-output-file`, the script writes `./results/cluster/<cluster>-node-map.json`. That file is now only for debugging or external tools; `runtime-controller` does not require it.

You can also put settings in an env file and pass it with `--config`, using `microk8s-cluster.env.example` as a template.

Set `DEBUG=true` in that env file if you want command-level tracing by default.

Lifecycle summary:

- `install` and `host-setup` install and initialize LXD plus the dual-network node profile
- `host-setup` installs and initializes LXD plus the dual-network node profile
- `create` brings up the control plane and workers
- `sandbox-create` creates a dual-NIC satellite sandbox with endpoint or relay role
- `sandbox-delete` removes a satellite sandbox
- `sandbox-list` shows live sandboxes and their host-worker placement
- `app-start` starts a managed application process inside a sandbox
- `app-stop` stops a managed application process inside a sandbox
- `app-list` shows which applications are live and where
- `kubeconfig` exports a host-usable kubeconfig for normal `kubectl`
- `list` summarizes every cluster the script can still discover and manage
- `test` deploys a tiny in-cluster web workload and verifies a response
- `destroy` removes only the cluster containers
- `uninstall` removes cluster containers and script-created host resources

Cluster discovery rules:

- the script writes `./results/cluster/<cluster>-inventory.tsv` for each managed cluster
- it also falls back to live LXD container metadata when inventory files are missing
- if no script metadata exists, the script will try to detect a legacy MicroK8s control-plane container and use its `kubectl get nodes` output to enumerate that cluster
- for legacy clusters, the cluster name shown by `list` is the control-plane container name
- `status`, `destroy`, `uninstall`, and `test` accept `--all-clusters true`
- `sandbox-list` and `app-list` also accept `--all-clusters true`
- `uninstall` only removes LXD profiles and bridges for clusters that are explicitly script-managed; legacy-discovered clusters are limited to container cleanup

Runtime inventory files:

- `./results/cluster/<cluster>-sandboxes.tsv` records the current live satellite sandboxes
- `./results/cluster/<cluster>-apps.tsv` records the currently managed sandbox applications
- `status` now prints both the cluster node view and the sandbox/application view
