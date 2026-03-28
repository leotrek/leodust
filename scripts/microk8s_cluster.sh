#!/usr/bin/env bash

set -euo pipefail

PATH="/snap/bin:${PATH}"

COMMAND=""
CONFIG_FILE=""
CLUSTER_NAME="leodust"
CONTROL_PLANE_NAME=""
WORKER_BASENAME=""
WORKER_NAMES_FILE=""
IMAGE="ubuntu-minimal:22.04"
MICROK8S_CHANNEL="1.30/stable"
LXD_PROFILE_NAME=""
NODE_IMAGE_ALIAS=""
SANDBOX_IMAGE_ALIAS=""
WORKER_COUNT="0"
CONTAINER_CPU="2"
CONTAINER_MEMORY="4GiB"
SANDBOX_CPU="1"
SANDBOX_MEMORY="1GiB"
WAIT_TIMEOUT_SECONDS="900"
ADDONS="dns,hostpath-storage"
MGMT_NETWORK_NAME=""
MGMT_NETWORK_SUBNET="10.180.0.1/16"
MGMT_NETWORK_NAT="true"
SIM_NETWORK_NAME=""
SIM_NETWORK_SUBNET="10.181.0.1/16"
SIM_NETWORK_NAT="false"
TAINT_CONTROL_PLANE="true"
NODE_MAP_OUTPUT_FILE=""
KUBECONFIG_OUTPUT_FILE=""
REMOVE_LXD="false"
ALL_CLUSTERS="false"
DEBUG="false"
SATELLITE_ID=""
SANDBOX_NAME=""
SANDBOX_ROLE="endpoint"
SANDBOX_HOST_WORKER=""
SANDBOX_SPECS=""
APP_NAME=""
APP_COMMAND=""

MGMT_INTERFACE_NAME="eth0"
SIM_INTERFACE_NAME="eth1"

CLUSTER_RESULTS_DIR="./results/cluster"
CLUSTER_INVENTORY_SUFFIX="-inventory.tsv"
SANDBOX_INVENTORY_SUFFIX="-sandboxes.tsv"
APP_INVENTORY_SUFFIX="-apps.tsv"

SMOKE_TEST_NAMESPACE_SUFFIX="smoke"
SMOKE_TEST_DEPLOYMENT_NAME="leodust-smoke"
SMOKE_TEST_SERVICE_NAME="leodust-smoke"
SMOKE_TEST_CHECK_NAME="leodust-smoke-check"
SMOKE_TEST_IMAGE="busybox:1.36"
SMOKE_TEST_EXPECTED_BODY="leodust smoke ok"

declare -a WORKER_NODE_NAMES=()
declare -a WORKER_CONTAINER_NAMES=()
declare -a SELECTED_WORKER_NODE_NAMES=()
declare -a SELECTED_WORKER_CONTAINER_NAMES=()
declare -a FAST_INSTANCE_NAMES=()

FAST_LXC_INSTANCE_CACHE_LOADED="false"
FAST_LXC_RESOURCE_CACHE_LOADED="false"
FAST_LXC_CACHE_SOURCE=""
FAST_LXC_ASSOC_ARRAYS_INITIALIZED="false"

log() {
    local level="$1"
    shift
    if [[ "${level}" == "DEBUG" && "${DEBUG}" != "true" ]]; then
        return 0
    fi
    printf '[%s] %s\n' "$level" "$*" >&2
}

debug() {
    log DEBUG "$*"
}

fatal() {
    log FATAL "$*"
    exit 1
}

command_to_string() {
    local arg result=""
    for arg in "$@"; do
        if [[ -n "${result}" ]]; then
            result+=" "
        fi
        result+="$(printf '%q' "${arg}")"
    done
    printf '%s' "${result}"
}

summarize_text() {
    local text="$1"
    text="${text//$'\n'/\\n}"
    if (( ${#text} > 240 )); then
        printf '%s...' "${text:0:240}"
    else
        printf '%s' "${text}"
    fi
}

usage() {
    cat <<'EOF'
Usage:
  ./scripts/microk8s_cluster.sh <command> [options]

Commands:
  install       Install host dependencies, initialize LXD, create the cluster bridges, and prepare the node profile
  host-setup    Install LXD if needed, initialize it, create the cluster bridges, and prepare the node profile
  create        Create the control-plane node and an initial set of workers
  add-workers   Add more worker containers and join them to the cluster
  sandbox-create Create or update a satellite sandbox without joining it to MicroK8s
  sandbox-create-many Create or update multiple satellite sandboxes from --sandbox-specs
  sandbox-delete Delete a managed satellite sandbox
  sandbox-list  List live satellite sandboxes and their logical host-worker placement
  app-start     Start a managed application process inside a satellite sandbox
  app-stop      Stop a managed application process inside a satellite sandbox
  app-list      List managed sandbox applications and where they are running
  kubeconfig    Export a host-usable kubeconfig from the control-plane container
  list          List all discovered clusters from script inventory, LXD metadata, and legacy MicroK8s control planes
  test          Deploy a small smoke workload and verify in-cluster scheduling and service reachability
  status        Show cluster nodes, live satellite sandboxes, managed apps, and current MicroK8s nodes
  destroy       Stop and delete all containers belonging to the cluster
  uninstall     Destroy the cluster and remove script-created LXD resources; optionally remove the LXD snap

Options:
  --config <file>                Source configuration values from a shell env file
  --cluster-name <name>          Logical cluster name used for metadata and defaults
  --control-plane-name <name>    Explicit control-plane container name
  --worker-base-name <name>      Worker container prefix; workers are named <prefix>-N
  --worker-names-file <file>     Plain text or TLE file used to derive worker container names
  --image <image>                LXD image to launch (default: ubuntu-minimal:22.04)
  --microk8s-channel <chan>      Snap channel to install (default: 1.30/stable)
  --lxd-profile-name <name>      LXD profile name for the dual-NIC simulation nodes
  --workers <count>              Number of workers for create, or number to add for add-workers
  --cpu <count>                  limits.cpu applied to each container
  --memory <value>               limits.memory applied to each container
  --sandbox-cpu <count>          limits.cpu applied to each satellite sandbox
  --sandbox-memory <value>       limits.memory applied to each satellite sandbox
  --wait-timeout <seconds>       Timeout for boot/install/join operations
  --addons <csv>                 MicroK8s addons enabled on the control plane; use none to skip
  --management-network-name <name>
                                 LXD bridge used for stable MicroK8s traffic
  --management-subnet <cidr>     IPv4 subnet for the management bridge
  --management-nat <bool>        Whether the management bridge should NAT outbound traffic
  --simulation-network-name <name>
                                 LXD bridge used for the simulated satellite interface
  --simulation-subnet <cidr>     IPv4 subnet for the simulation bridge
  --simulation-nat <bool>        Whether the simulation bridge should NAT outbound traffic
  --taint-control-plane <bool>   Apply a NoSchedule taint to the control-plane node
  --node-map-output-file <file>  JSON file written for simulation-to-container name mappings
  --kubeconfig-output-file <file>
                                 Host path for exported kubeconfig (default: ./results/cluster/<cluster>-kubeconfig.yaml)
  --satellite-id <id>            Satellite identifier used for sandbox creation and lookup
  --sandbox-specs <specs>        Semicolon-separated sandbox specs as satellite|role or satellite|role|host-worker
  --sandbox-name <name>          Explicit LXD container name for a satellite sandbox
  --sandbox-role <role>          Sandbox role: endpoint or relay (default: endpoint)
  --sandbox-host-worker <name>   Logical worker responsible for the sandbox; defaults to least-loaded worker
  --app-name <name>              Managed application name inside a sandbox
  --app-command <cmd>            Shell command to start for app-start inside the sandbox
  --remove-lxd <bool>            Remove the LXD snap during uninstall (default: false)
  --all-clusters <bool>          For list/status/destroy/uninstall/test/sandbox-list/app-list, operate across every discovered cluster
  --debug <bool>                 Emit DEBUG logs with concrete LXC and in-container commands
  -h, --help                     Show this help text

Examples:
  ./scripts/microk8s_cluster.sh install --cluster-name leodust
  ./scripts/microk8s_cluster.sh create --cluster-name leodust --workers 6
  ./scripts/microk8s_cluster.sh create --cluster-name leodust --worker-names-file ./go/resources/tle/starlink_250.tle
  ./scripts/microk8s_cluster.sh add-workers --cluster-name leodust --workers 4
  ./scripts/microk8s_cluster.sh sandbox-create --cluster-name leodust --satellite-id STARLINK-3393
  ./scripts/microk8s_cluster.sh sandbox-create-many --cluster-name leodust --sandbox-specs 'STARLINK-3393|endpoint;STARLINK-3394|relay'
  ./scripts/microk8s_cluster.sh app-start --cluster-name leodust --satellite-id STARLINK-3393 --app-name ping --app-command 'sleep 3600'
  ./scripts/microk8s_cluster.sh sandbox-list --cluster-name leodust
  ./scripts/microk8s_cluster.sh kubeconfig --cluster-name leodust
  ./scripts/microk8s_cluster.sh list
  ./scripts/microk8s_cluster.sh status --all-clusters true
  ./scripts/microk8s_cluster.sh list --debug true
  ./scripts/microk8s_cluster.sh test --cluster-name leodust
  ./scripts/microk8s_cluster.sh status --cluster-name leodust
  ./scripts/microk8s_cluster.sh destroy --cluster-name leodust
  ./scripts/microk8s_cluster.sh uninstall --cluster-name leodust --remove-lxd true
EOF
}

run_privileged() {
    if [[ "${EUID}" -eq 0 ]]; then
        "$@"
    else
        if [[ -t 0 || -t 1 || -t 2 ]]; then
            sudo "$@"
        else
            sudo -n "$@" || fatal "sudo access is required for: $(command_to_string "$@"). Re-run the script interactively or refresh sudo first."
        fi
    fi
}

find_binary_path() {
    local name="$1"
    local path=""

    path="$(command -v "${name}" 2>/dev/null || true)"
    if [[ -n "${path}" && -x "${path}" ]]; then
        printf '%s\n' "${path}"
        return 0
    fi

    if [[ -x "/snap/bin/${name}" ]]; then
        printf '%s\n' "/snap/bin/${name}"
        return 0
    fi

    return 1
}

resolve_binary() {
    local name="$1"
    local path=""

    path="$(find_binary_path "${name}" || true)"
    [[ -n "${path}" ]] || fatal "Required binary not found: ${name}"
    printf '%s\n' "${path}"
}

binary_available() {
    local name="$1"
    find_binary_path "${name}" >/dev/null 2>&1
}

lxc_cmd() {
    local binary
    binary="$(resolve_binary lxc)"
    "${binary}" "$@" </dev/null
}

lxd_cmd() {
    local binary
    binary="$(resolve_binary lxd)"
    "${binary}" "$@" </dev/null
}

container_exec_direct() {
    local name="$1"
    shift
    lxc_cmd exec "${name}" -- "$@" </dev/null
}

container_exec_capture_direct() {
    local name="$1"
    shift
    lxc_cmd exec "${name}" -- "$@" </dev/null
}

container_exec() {
    local name="$1"
    shift
    lxc_cmd exec "${name}" -- env PATH="/snap/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" bash -lc "$*" </dev/null
}

container_exec_capture() {
    local name="$1"
    shift
    lxc_cmd exec "${name}" -- env PATH="/snap/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" bash -lc "$*" </dev/null
}

wait_until() {
    local timeout_seconds="$1"
    local description="$2"
    shift 2

    local start_epoch now_epoch elapsed
    start_epoch="$(date +%s)"
    debug "wait_until start: ${description} timeout=${timeout_seconds}s"

    until "$@" >/dev/null 2>&1; do
        now_epoch="$(date +%s)"
        elapsed=$((now_epoch - start_epoch))
        if (( elapsed >= timeout_seconds )); then
            fatal "Timed out waiting for ${description} after ${timeout_seconds}s"
        fi
        debug "wait_until retry: ${description} elapsed=${elapsed}s"
        sleep 5
    done
    debug "wait_until done: ${description}"
}

wait_until_container_exec() {
    local name="$1"
    local description="$2"
    local script="$3"
    wait_until "${WAIT_TIMEOUT_SECONDS}" "${description}" container_exec "${name}" "${script}"
}

trim_whitespace() {
    local value="$1"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    printf '%s' "${value}"
}

validate_bool() {
    case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
        true|false)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

sanitize_instance_name() {
    local value lowered result char
    value="$1"
    lowered="$(printf '%s' "${value}" | tr '[:upper:]' '[:lower:]')"
    result=""

    local i
    for (( i = 0; i < ${#lowered}; i++ )); do
        char="${lowered:i:1}"
        case "${char}" in
            [a-z0-9])
                result+="${char}"
                ;;
            *)
                if [[ -n "${result}" && "${result: -1}" != "-" ]]; then
                    result+="-"
                fi
                ;;
        esac
    done

    result="${result#-}"
    result="${result%-}"
    [[ -n "${result}" ]] || result="node"
    printf '%s\n' "${result}"
}

json_escape() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    value="${value//$'\n'/\\n}"
    printf '%s' "${value}"
}

smoke_test_namespace() {
    printf '%s-%s\n' "${CLUSTER_NAME}" "${SMOKE_TEST_NAMESPACE_SUFFIX}"
}

cluster_inventory_file() {
    local cluster_name="${1:-${CLUSTER_NAME}}"
    printf '%s/%s%s\n' "${CLUSTER_RESULTS_DIR}" "${cluster_name}" "${CLUSTER_INVENTORY_SUFFIX}"
}

sandbox_inventory_file() {
    local cluster_name="${1:-${CLUSTER_NAME}}"
    printf '%s/%s%s\n' "${CLUSTER_RESULTS_DIR}" "${cluster_name}" "${SANDBOX_INVENTORY_SUFFIX}"
}

app_inventory_file() {
    local cluster_name="${1:-${CLUSTER_NAME}}"
    printf '%s/%s%s\n' "${CLUSTER_RESULTS_DIR}" "${cluster_name}" "${APP_INVENTORY_SUFFIX}"
}

inventory_value() {
    local file_path="$1"
    local key="$2"
    awk -F '\t' -v key="${key}" '$1 == key {print substr($0, index($0, FS) + 1); exit}' "${file_path}"
}

set_cluster_defaults() {
    local cluster_name="$1"

    CLUSTER_NAME="${cluster_name}"
    CONTROL_PLANE_NAME="${cluster_name}-control-plane"
    WORKER_BASENAME="${cluster_name}-worker"
    LXD_PROFILE_NAME="${cluster_name}-node"
    MGMT_NETWORK_NAME="${cluster_name}-mgmt"
    SIM_NETWORK_NAME="${cluster_name}-sim"
    NODE_MAP_OUTPUT_FILE="${CLUSTER_RESULTS_DIR}/${cluster_name}-node-map.json"
    KUBECONFIG_OUTPUT_FILE="${CLUSTER_RESULTS_DIR}/${cluster_name}-kubeconfig.yaml"
    SANDBOX_NAME=""
    SATELLITE_ID=""
    SANDBOX_HOST_WORKER=""
}

csv_item_count() {
    local csv="$1"
    local count=0 item
    [[ -n "${csv}" ]] || {
        printf '0\n'
        return 0
    }

    IFS=',' read -r -a item_array <<< "${csv}"
    for item in "${item_array[@]}"; do
        item="$(trim_whitespace "${item}")"
        [[ -n "${item}" ]] || continue
        count=$((count + 1))
    done
    printf '%s\n' "${count}"
}

fast_lxc_metadata_supported() {
    command -v jq >/dev/null 2>&1 || command -v python3 >/dev/null 2>&1
}

initialize_fast_lxc_cache_storage() {
    if [[ "${FAST_LXC_ASSOC_ARRAYS_INITIALIZED}" == "true" ]]; then
        return 0
    fi

    declare -gA FAST_INSTANCE_STATUS=()
    declare -gA FAST_INSTANCE_CLUSTER=()
    declare -gA FAST_INSTANCE_KIND=()
    declare -gA FAST_INSTANCE_ROLE=()
    declare -gA FAST_INSTANCE_SATELLITE_ID=()
    declare -gA FAST_INSTANCE_SANDBOX_ROLE=()
    declare -gA FAST_INSTANCE_HOST_WORKER=()
    declare -gA FAST_INSTANCE_APP_COUNT=()
    declare -gA FAST_INSTANCE_MGMT_IP=()
    declare -gA FAST_INSTANCE_SIM_IP=()
    declare -gA FAST_INSTANCE_SIMULATION_NAME=()
    declare -gA FAST_INSTANCE_WORKER_BASE=()
    declare -gA FAST_INSTANCE_MGMT_NETWORK=()
    declare -gA FAST_INSTANCE_SIM_NETWORK=()
    declare -gA FAST_INSTANCE_PROFILE=()
    declare -gA FAST_PROFILE_EXISTS=()
    declare -gA FAST_NETWORK_EXISTS=()

    FAST_LXC_ASSOC_ARRAYS_INITIALIZED="true"
}

load_fast_lxc_instance_cache() {
    local instance_json="" name status cluster kind role satellite_id sandbox_role host_worker
    local app_count mgmt_ip sim_ip simulation_name worker_base mgmt_network sim_network profile

    if [[ "${FAST_LXC_INSTANCE_CACHE_LOADED}" == "true" ]]; then
        return 0
    fi

    FAST_LXC_INSTANCE_CACHE_LOADED="true"
    FAST_LXC_CACHE_SOURCE=""
    initialize_fast_lxc_cache_storage
    FAST_INSTANCE_NAMES=()
    FAST_INSTANCE_STATUS=()
    FAST_INSTANCE_CLUSTER=()
    FAST_INSTANCE_KIND=()
    FAST_INSTANCE_ROLE=()
    FAST_INSTANCE_SATELLITE_ID=()
    FAST_INSTANCE_SANDBOX_ROLE=()
    FAST_INSTANCE_HOST_WORKER=()
    FAST_INSTANCE_APP_COUNT=()
    FAST_INSTANCE_MGMT_IP=()
    FAST_INSTANCE_SIM_IP=()
    FAST_INSTANCE_SIMULATION_NAME=()
    FAST_INSTANCE_WORKER_BASE=()
    FAST_INSTANCE_MGMT_NETWORK=()
    FAST_INSTANCE_SIM_NETWORK=()
    FAST_INSTANCE_PROFILE=()

    binary_available lxc || return 1
    fast_lxc_metadata_supported || return 1

    instance_json="$(lxc_cmd list --format json 2>/dev/null || true)"
    [[ -n "${instance_json}" ]] || return 1

    if command -v jq >/dev/null 2>&1; then
        FAST_LXC_CACHE_SOURCE="jq"
        while IFS=$'\t' read -r name status cluster kind role satellite_id sandbox_role host_worker app_count mgmt_ip sim_ip simulation_name worker_base mgmt_network sim_network profile; do
            [[ -n "${name}" ]] || continue
            FAST_INSTANCE_NAMES+=("${name}")
            FAST_INSTANCE_STATUS["${name}"]="${status}"
            FAST_INSTANCE_CLUSTER["${name}"]="${cluster}"
            FAST_INSTANCE_KIND["${name}"]="${kind}"
            FAST_INSTANCE_ROLE["${name}"]="${role}"
            FAST_INSTANCE_SATELLITE_ID["${name}"]="${satellite_id}"
            FAST_INSTANCE_SANDBOX_ROLE["${name}"]="${sandbox_role}"
            FAST_INSTANCE_HOST_WORKER["${name}"]="${host_worker}"
            FAST_INSTANCE_APP_COUNT["${name}"]="${app_count:-0}"
            FAST_INSTANCE_MGMT_IP["${name}"]="${mgmt_ip}"
            FAST_INSTANCE_SIM_IP["${name}"]="${sim_ip}"
            FAST_INSTANCE_SIMULATION_NAME["${name}"]="${simulation_name}"
            FAST_INSTANCE_WORKER_BASE["${name}"]="${worker_base}"
            FAST_INSTANCE_MGMT_NETWORK["${name}"]="${mgmt_network}"
            FAST_INSTANCE_SIM_NETWORK["${name}"]="${sim_network}"
            FAST_INSTANCE_PROFILE["${name}"]="${profile}"
        done < <(
            printf '%s\n' "${instance_json}" | jq -r '
                .[] |
                [
                    .name,
                    (.status // ""),
                    (.config["user.leodust.cluster"] // ""),
                    (.config["user.leodust.kind"] // "node"),
                    (.config["user.leodust.role"] // ""),
                    (.config["user.leodust.satellite-id"] // ""),
                    (.config["user.leodust.sandbox-role"] // ""),
                    (.config["user.leodust.host-worker"] // ""),
                    (.config["user.leodust.app-count"] // "0"),
                    (.config["user.leodust.management-ip"] // ""),
                    (.config["user.leodust.simulation-ip"] // ""),
                    (.config["user.leodust.simulation-name"] // ""),
                    (.config["user.leodust.worker-base-name"] // ""),
                    (.config["user.leodust.management-network"] // ""),
                    (.config["user.leodust.simulation-network"] // ""),
                    (.config["user.leodust.profile"] // (.profiles[0] // ""))
                ] | @tsv
            '
        )
    else
        FAST_LXC_CACHE_SOURCE="python3"
        while IFS=$'\t' read -r name status cluster kind role satellite_id sandbox_role host_worker app_count mgmt_ip sim_ip simulation_name worker_base mgmt_network sim_network profile; do
            [[ -n "${name}" ]] || continue
            FAST_INSTANCE_NAMES+=("${name}")
            FAST_INSTANCE_STATUS["${name}"]="${status}"
            FAST_INSTANCE_CLUSTER["${name}"]="${cluster}"
            FAST_INSTANCE_KIND["${name}"]="${kind}"
            FAST_INSTANCE_ROLE["${name}"]="${role}"
            FAST_INSTANCE_SATELLITE_ID["${name}"]="${satellite_id}"
            FAST_INSTANCE_SANDBOX_ROLE["${name}"]="${sandbox_role}"
            FAST_INSTANCE_HOST_WORKER["${name}"]="${host_worker}"
            FAST_INSTANCE_APP_COUNT["${name}"]="${app_count:-0}"
            FAST_INSTANCE_MGMT_IP["${name}"]="${mgmt_ip}"
            FAST_INSTANCE_SIM_IP["${name}"]="${sim_ip}"
            FAST_INSTANCE_SIMULATION_NAME["${name}"]="${simulation_name}"
            FAST_INSTANCE_WORKER_BASE["${name}"]="${worker_base}"
            FAST_INSTANCE_MGMT_NETWORK["${name}"]="${mgmt_network}"
            FAST_INSTANCE_SIM_NETWORK["${name}"]="${sim_network}"
            FAST_INSTANCE_PROFILE["${name}"]="${profile}"
        done < <(
            printf '%s\n' "${instance_json}" | python3 -c '
import json, sys

def clean(value):
    return str(value).replace("\t", " ").replace("\n", " ")

instances = json.load(sys.stdin)
for item in instances:
    config = item.get("config") or {}
    profiles = item.get("profiles") or []
    row = [
        item.get("name", ""),
        item.get("status", ""),
        config.get("user.leodust.cluster", ""),
        config.get("user.leodust.kind", "node"),
        config.get("user.leodust.role", ""),
        config.get("user.leodust.satellite-id", ""),
        config.get("user.leodust.sandbox-role", ""),
        config.get("user.leodust.host-worker", ""),
        config.get("user.leodust.app-count", "0"),
        config.get("user.leodust.management-ip", ""),
        config.get("user.leodust.simulation-ip", ""),
        config.get("user.leodust.simulation-name", ""),
        config.get("user.leodust.worker-base-name", ""),
        config.get("user.leodust.management-network", ""),
        config.get("user.leodust.simulation-network", ""),
        config.get("user.leodust.profile", profiles[0] if profiles else ""),
    ]
    print("\t".join(clean(value) for value in row))
'
        )
    fi

    debug "Loaded cached LXD instance metadata for ${#FAST_INSTANCE_NAMES[@]} instances using ${FAST_LXC_CACHE_SOURCE}"
    return 0
}

load_fast_lxc_resource_cache() {
    local name

    if [[ "${FAST_LXC_RESOURCE_CACHE_LOADED}" == "true" ]]; then
        return 0
    fi

    FAST_LXC_RESOURCE_CACHE_LOADED="true"
    initialize_fast_lxc_cache_storage
    FAST_PROFILE_EXISTS=()
    FAST_NETWORK_EXISTS=()

    binary_available lxc || return 1

    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        FAST_PROFILE_EXISTS["${name}"]="1"
    done < <(lxc_cmd profile list --format csv -c n 2>/dev/null || true)

    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        FAST_NETWORK_EXISTS["${name}"]="1"
    done < <(lxc_cmd network list --format csv -c n 2>/dev/null || true)

    return 0
}

managed_cluster_names_fast() {
    local cluster_name inventory_file name found=0
    declare -A seen_clusters=()

    if [[ -d "${CLUSTER_RESULTS_DIR}" ]]; then
        while IFS= read -r inventory_file; do
            [[ -n "${inventory_file}" ]] || continue
            cluster_name="$(inventory_value "${inventory_file}" cluster_name)"
            if [[ -z "${cluster_name}" ]]; then
                cluster_name="$(basename "${inventory_file}")"
                cluster_name="${cluster_name%"${CLUSTER_INVENTORY_SUFFIX}"}"
            fi
            if [[ -n "${cluster_name}" && -z "${seen_clusters[${cluster_name}]:-}" ]]; then
                seen_clusters["${cluster_name}"]="1"
                printf '%s\n' "${cluster_name}"
                found=1
            fi
        done < <(find "${CLUSTER_RESULTS_DIR}" -maxdepth 1 -type f -name "*${CLUSTER_INVENTORY_SUFFIX}" 2>/dev/null | sort)
    fi

    if load_fast_lxc_instance_cache; then
        debug "Using cached LXD instance metadata for discovery"
        for name in "${FAST_INSTANCE_NAMES[@]}"; do
            cluster_name="${FAST_INSTANCE_CLUSTER[${name}]:-}"
            [[ -n "${cluster_name}" ]] || continue
            if [[ -z "${seen_clusters[${cluster_name}]:-}" ]]; then
                seen_clusters["${cluster_name}"]="1"
                printf '%s\n' "${cluster_name}"
                found=1
            fi
        done
    fi

    (( found > 0 ))
}

fast_cache_has_legacy_candidates() {
    local name

    if ! load_fast_lxc_instance_cache; then
        return 1
    fi

    for name in "${FAST_INSTANCE_NAMES[@]}"; do
        [[ -n "${FAST_INSTANCE_CLUSTER[${name}]:-}" ]] && continue
        case "${name}" in
            master|master-*|*-master|*-master-*|control-plane|control-plane-*|*-control-plane|*-control-plane-*|controller|controller-*|*-controller|*-controller-*)
                return 0
                ;;
        esac
    done

    return 1
}

fast_instance_exists() {
    local name="$1"

    load_fast_lxc_instance_cache >/dev/null 2>&1 || return 1
    [[ "${FAST_INSTANCE_STATUS[${name}]+set}" == "set" ]]
}

join_csv() {
    local joined="" item
    for item in "$@"; do
        [[ -n "${item}" ]] || continue
        if [[ -n "${joined}" ]]; then
            joined+=","
        fi
        joined+="${item}"
    done
    printf '%s\n' "${joined}"
}

load_worker_names() {
    WORKER_NODE_NAMES=()
    WORKER_CONTAINER_NAMES=()

    [[ -n "${WORKER_NAMES_FILE}" ]] || return 0
    [[ -f "${WORKER_NAMES_FILE}" ]] || fatal "Worker names file not found: ${WORKER_NAMES_FILE}"

    local line node_name container_name
    declare -A seen_container_names=()

    while IFS= read -r line || [[ -n "${line}" ]]; do
        node_name="$(trim_whitespace "${line}")"
        [[ -n "${node_name}" ]] || continue
        [[ "${node_name}" == \#* ]] && continue
        [[ "${node_name}" =~ ^[12][[:space:]] ]] && continue
        if [[ "${node_name}" =~ ^0[[:space:]]+ ]]; then
            node_name="$(trim_whitespace "${node_name#0 }")"
        fi
        [[ -n "${node_name}" ]] || continue

        container_name="$(sanitize_instance_name "${node_name}")"
        if [[ -n "${seen_container_names[${container_name}]:-}" ]]; then
            fatal "Worker names ${seen_container_names[${container_name}]} and ${node_name} both sanitize to ${container_name}"
        fi
        seen_container_names["${container_name}"]="${node_name}"

        WORKER_NODE_NAMES+=("${node_name}")
        WORKER_CONTAINER_NAMES+=("${container_name}")
    done < "${WORKER_NAMES_FILE}"

    (( ${#WORKER_NODE_NAMES[@]} > 0 )) || fatal "No worker names were loaded from ${WORKER_NAMES_FILE}"

    if [[ -z "${NODE_MAP_OUTPUT_FILE}" ]]; then
        NODE_MAP_OUTPUT_FILE="./results/cluster/${CLUSTER_NAME}-node-map.json"
    fi
}

discover_config_file() {
    local argv=("$@")
    local i=0
    while (( i < ${#argv[@]} )); do
        case "${argv[$i]}" in
            --config)
                (( i + 1 < ${#argv[@]} )) || fatal "--config requires a file path"
                CONFIG_FILE="${argv[$((i + 1))]}"
                return
                ;;
        esac
        ((i += 1))
    done
}

load_config_file() {
    [[ -n "${CONFIG_FILE}" ]] || return 0
    [[ -f "${CONFIG_FILE}" ]] || fatal "Config file not found: ${CONFIG_FILE}"
    # shellcheck disable=SC1090
    source "${CONFIG_FILE}"
}

parse_args() {
    (( $# > 0 )) || {
        usage
        exit 1
    }

    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
    esac

    COMMAND="$1"
    shift

    while (( $# > 0 )); do
        case "$1" in
            --config)
                (( $# >= 2 )) || fatal "--config requires a file path"
                shift 2
                ;;
            --cluster-name)
                (( $# >= 2 )) || fatal "--cluster-name requires a value"
                CLUSTER_NAME="$2"
                shift 2
                ;;
            --control-plane-name)
                (( $# >= 2 )) || fatal "--control-plane-name requires a value"
                CONTROL_PLANE_NAME="$2"
                shift 2
                ;;
            --worker-base-name)
                (( $# >= 2 )) || fatal "--worker-base-name requires a value"
                WORKER_BASENAME="$2"
                shift 2
                ;;
            --worker-names-file)
                (( $# >= 2 )) || fatal "--worker-names-file requires a value"
                WORKER_NAMES_FILE="$2"
                shift 2
                ;;
            --image)
                (( $# >= 2 )) || fatal "--image requires a value"
                IMAGE="$2"
                shift 2
                ;;
            --microk8s-channel)
                (( $# >= 2 )) || fatal "--microk8s-channel requires a value"
                MICROK8S_CHANNEL="$2"
                shift 2
                ;;
            --lxd-profile-name)
                (( $# >= 2 )) || fatal "--lxd-profile-name requires a value"
                LXD_PROFILE_NAME="$2"
                shift 2
                ;;
            --workers)
                (( $# >= 2 )) || fatal "--workers requires a value"
                WORKER_COUNT="$2"
                shift 2
                ;;
            --cpu)
                (( $# >= 2 )) || fatal "--cpu requires a value"
                CONTAINER_CPU="$2"
                shift 2
                ;;
            --memory)
                (( $# >= 2 )) || fatal "--memory requires a value"
                CONTAINER_MEMORY="$2"
                shift 2
                ;;
            --sandbox-cpu)
                (( $# >= 2 )) || fatal "--sandbox-cpu requires a value"
                SANDBOX_CPU="$2"
                shift 2
                ;;
            --sandbox-memory)
                (( $# >= 2 )) || fatal "--sandbox-memory requires a value"
                SANDBOX_MEMORY="$2"
                shift 2
                ;;
            --wait-timeout)
                (( $# >= 2 )) || fatal "--wait-timeout requires a value"
                WAIT_TIMEOUT_SECONDS="$2"
                shift 2
                ;;
            --addons)
                (( $# >= 2 )) || fatal "--addons requires a value"
                ADDONS="$2"
                shift 2
                ;;
            --management-network-name)
                (( $# >= 2 )) || fatal "--management-network-name requires a value"
                MGMT_NETWORK_NAME="$2"
                shift 2
                ;;
            --management-subnet)
                (( $# >= 2 )) || fatal "--management-subnet requires a value"
                MGMT_NETWORK_SUBNET="$2"
                shift 2
                ;;
            --management-nat)
                (( $# >= 2 )) || fatal "--management-nat requires a value"
                MGMT_NETWORK_NAT="$2"
                shift 2
                ;;
            --simulation-network-name)
                (( $# >= 2 )) || fatal "--simulation-network-name requires a value"
                SIM_NETWORK_NAME="$2"
                shift 2
                ;;
            --simulation-subnet)
                (( $# >= 2 )) || fatal "--simulation-subnet requires a value"
                SIM_NETWORK_SUBNET="$2"
                shift 2
                ;;
            --simulation-nat)
                (( $# >= 2 )) || fatal "--simulation-nat requires a value"
                SIM_NETWORK_NAT="$2"
                shift 2
                ;;
            --taint-control-plane)
                (( $# >= 2 )) || fatal "--taint-control-plane requires a value"
                TAINT_CONTROL_PLANE="$2"
                shift 2
                ;;
            --node-map-output-file)
                (( $# >= 2 )) || fatal "--node-map-output-file requires a value"
                NODE_MAP_OUTPUT_FILE="$2"
                shift 2
                ;;
            --kubeconfig-output-file)
                (( $# >= 2 )) || fatal "--kubeconfig-output-file requires a value"
                KUBECONFIG_OUTPUT_FILE="$2"
                shift 2
                ;;
            --satellite-id)
                (( $# >= 2 )) || fatal "--satellite-id requires a value"
                SATELLITE_ID="$2"
                shift 2
                ;;
            --sandbox-specs)
                (( $# >= 2 )) || fatal "--sandbox-specs requires a value"
                SANDBOX_SPECS="$2"
                shift 2
                ;;
            --sandbox-name)
                (( $# >= 2 )) || fatal "--sandbox-name requires a value"
                SANDBOX_NAME="$2"
                shift 2
                ;;
            --sandbox-role)
                (( $# >= 2 )) || fatal "--sandbox-role requires a value"
                SANDBOX_ROLE="$2"
                shift 2
                ;;
            --sandbox-host-worker)
                (( $# >= 2 )) || fatal "--sandbox-host-worker requires a value"
                SANDBOX_HOST_WORKER="$2"
                shift 2
                ;;
            --app-name)
                (( $# >= 2 )) || fatal "--app-name requires a value"
                APP_NAME="$2"
                shift 2
                ;;
            --app-command)
                (( $# >= 2 )) || fatal "--app-command requires a value"
                APP_COMMAND="$2"
                shift 2
                ;;
            --remove-lxd)
                (( $# >= 2 )) || fatal "--remove-lxd requires a value"
                REMOVE_LXD="$2"
                shift 2
                ;;
            --all-clusters)
                (( $# >= 2 )) || fatal "--all-clusters requires a value"
                ALL_CLUSTERS="$2"
                shift 2
                ;;
            --debug)
                (( $# >= 2 )) || fatal "--debug requires a value"
                DEBUG="$2"
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                fatal "Unknown option: $1"
                ;;
        esac
    done

    CONTROL_PLANE_NAME="${CONTROL_PLANE_NAME:-${CLUSTER_NAME}-control-plane}"
    WORKER_BASENAME="${WORKER_BASENAME:-${CLUSTER_NAME}-worker}"
    LXD_PROFILE_NAME="${LXD_PROFILE_NAME:-${CLUSTER_NAME}-node}"
    NODE_IMAGE_ALIAS="${NODE_IMAGE_ALIAS:-${CLUSTER_NAME}-node-cache-$(sanitize_instance_name "${IMAGE}")}"
    SANDBOX_IMAGE_ALIAS="${SANDBOX_IMAGE_ALIAS:-${CLUSTER_NAME}-sandbox-cache-$(sanitize_instance_name "${IMAGE}")}"
    MGMT_NETWORK_NAME="${MGMT_NETWORK_NAME:-${CLUSTER_NAME}-mgmt}"
    SIM_NETWORK_NAME="${SIM_NETWORK_NAME:-${CLUSTER_NAME}-sim}"

    [[ "${WORKER_COUNT}" =~ ^[0-9]+$ ]] || fatal "--workers must be a non-negative integer"
    [[ "${WAIT_TIMEOUT_SECONDS}" =~ ^[0-9]+$ ]] || fatal "--wait-timeout must be a non-negative integer"
    validate_bool "${MGMT_NETWORK_NAT}" || fatal "--management-nat must be true or false"
    validate_bool "${SIM_NETWORK_NAT}" || fatal "--simulation-nat must be true or false"
    validate_bool "${TAINT_CONTROL_PLANE}" || fatal "--taint-control-plane must be true or false"
    validate_bool "${REMOVE_LXD}" || fatal "--remove-lxd must be true or false"
    validate_bool "${ALL_CLUSTERS}" || fatal "--all-clusters must be true or false"
    validate_bool "${DEBUG}" || fatal "--debug must be true or false"
    case "${SANDBOX_ROLE}" in
        endpoint|relay)
            ;;
        *)
            fatal "--sandbox-role must be endpoint or relay"
            ;;
    esac
}

require_host_tools() {
    command -v awk >/dev/null 2>&1 || fatal "awk is required"
    command -v base64 >/dev/null 2>&1 || fatal "base64 is required"
    command -v grep >/dev/null 2>&1 || fatal "grep is required"
    command -v mkdir >/dev/null 2>&1 || fatal "mkdir is required"
    command -v sed >/dev/null 2>&1 || fatal "sed is required"
}

ensure_lxd_installed() {
    if binary_available lxc && binary_available lxd; then
        log INFO "LXD is already installed"
        log INFO "Using LXC client at $(resolve_binary lxc)"
        log INFO "Using LXD daemon client at $(resolve_binary lxd)"
        return
    fi

    log INFO "Installing LXD host dependencies"
    log INFO "Running apt-get update"
    run_privileged apt-get update
    log INFO "Installing snapd and CA certificates"
    run_privileged apt-get install -y snapd ca-certificates
    log INFO "Installing LXD snap"
    run_privileged snap install lxd
    log INFO "LXD snap installed"

    binary_available lxc || fatal "LXD was installed but 'lxc' is still unavailable. Open a new shell or run 'hash -r' and retry."
    binary_available lxd || fatal "LXD was installed but 'lxd' is still unavailable. Open a new shell or run 'hash -r' and retry."
    log INFO "Using LXC client at $(resolve_binary lxc)"
    log INFO "Using LXD daemon client at $(resolve_binary lxd)"
    log INFO "If your interactive shell still resolves an old lxc path, run 'hash -r' or open a new shell."
}

ensure_lxd_initialized() {
    if lxc_cmd profile list >/dev/null 2>&1; then
        debug "LXD is already initialized"
        return
    fi

    log INFO "Initializing LXD with minimal configuration"
    lxd_cmd init --minimal
}

require_lxd_runtime() {
    binary_available lxc && binary_available lxd && return 0
    fatal "LXD is not installed or not available in PATH. Run './microk8s_cluster.sh install --cluster-name ${CLUSTER_NAME}' first."
}

network_exists() {
    local name="$1"
    lxc_cmd network show "${name}" >/dev/null 2>&1
}

profile_exists() {
    local name="$1"
    lxc_cmd profile show "${name}" >/dev/null 2>&1
}

ensure_bridge_network() {
    local name="$1"
    local subnet="$2"
    local nat="$3"
    local description="$4"

    if network_exists "${name}"; then
        log INFO "${description} network ${name} already exists"
        return
    fi

    log INFO "Creating ${description} network ${name} (${subnet}, nat=${nat})"
    lxc_cmd network create "${name}" \
        ipv4.address="${subnet}" \
        ipv4.nat="${nat}" \
        ipv6.address=none
}

ensure_cluster_networks() {
    ensure_bridge_network "${MGMT_NETWORK_NAME}" "${MGMT_NETWORK_SUBNET}" "${MGMT_NETWORK_NAT}" "management"
    ensure_bridge_network "${SIM_NETWORK_NAME}" "${SIM_NETWORK_SUBNET}" "${SIM_NETWORK_NAT}" "simulation"
}

default_storage_pool() {
    local pool

    pool="$(lxc_cmd profile show default | awk '/^[[:space:]]+pool:/ {print $2; exit}')"
    if [[ -n "${pool}" ]]; then
        printf '%s\n' "${pool}"
        return 0
    fi

    pool="$(lxc_cmd storage list --format csv -c n 2>/dev/null | awk 'NF {print; exit}')"
    if [[ -n "${pool}" ]]; then
        printf '%s\n' "${pool}"
        return 0
    fi

    pool="default"
    log INFO "No LXD storage pool exists; creating ${pool} with dir driver"
    lxc_cmd storage create "${pool}" dir >/dev/null
    printf '%s\n' "${pool}"
}

profile_has_device() {
    local profile_name="$1"
    local device_name="$2"
    lxc_cmd profile device show "${profile_name}" | grep -Eq "^${device_name}:"
}

ensure_profile_root_disk() {
    local profile_name="$1"
    local pool existing_pool

    if lxc_cmd profile device show "${profile_name}" | awk '
        /^[^[:space:]].*:$/ {current=$1; gsub(":", "", current)}
        /^[[:space:]]+path:[[:space:]]*\/$/ {has_path=1}
        /^[[:space:]]+type:[[:space:]]*disk$/ && has_path == 1 {found=1; exit}
        /^[^[:space:]].*:$/ && has_path == 1 {has_path=0}
        END {exit found ? 0 : 1}
    '; then
        existing_pool="$(lxc_cmd profile device show "${profile_name}" | awk '
            $1 == "root:" {in_root=1; next}
            in_root == 1 && /^[^[:space:]].*:$/ {exit}
            in_root == 1 && $1 == "pool:" {print $2; exit}
        ')"
        if [[ -n "${existing_pool}" ]] && lxc_cmd storage show "${existing_pool}" >/dev/null 2>&1; then
            return 0
        fi

        pool="$(default_storage_pool)"
        log INFO "Updating root disk device on profile ${profile_name} to use storage pool ${pool}"
        lxc_cmd profile device set "${profile_name}" root pool "${pool}"
        return 0
    fi

    pool="$(default_storage_pool)"
    log INFO "Adding root disk device to profile ${profile_name}"
    lxc_cmd profile device add "${profile_name}" root disk path=/ pool="${pool}"
}

ensure_profile_nic() {
    local profile_name="$1"
    local device_name="$2"
    local interface_name="$3"
    local network_name="$4"

    if profile_has_device "${profile_name}" "${device_name}"; then
        lxc_cmd profile device set "${profile_name}" "${device_name}" network "${network_name}"
        lxc_cmd profile device set "${profile_name}" "${device_name}" name "${interface_name}"
        return
    fi

    lxc_cmd profile device add "${profile_name}" "${device_name}" nic network="${network_name}" name="${interface_name}"
}

ensure_node_profile() {
    if ! lxc_cmd profile show "${LXD_PROFILE_NAME}" >/dev/null 2>&1; then
        log INFO "Creating LXD profile ${LXD_PROFILE_NAME} from default"
        lxc_cmd profile copy default "${LXD_PROFILE_NAME}"
    fi

    ensure_profile_root_disk "${LXD_PROFILE_NAME}"
    ensure_profile_nic "${LXD_PROFILE_NAME}" "${MGMT_INTERFACE_NAME}" "${MGMT_INTERFACE_NAME}" "${MGMT_NETWORK_NAME}"
    ensure_profile_nic "${LXD_PROFILE_NAME}" "sim" "${SIM_INTERFACE_NAME}" "${SIM_NETWORK_NAME}"

    lxc_cmd profile set "${LXD_PROFILE_NAME}" boot.autostart "true"
    lxc_cmd profile set "${LXD_PROFILE_NAME}" linux.kernel_modules "overlay,br_netfilter,nf_nat,ip_tables,ip6_tables,nf_conntrack"
    lxc_cmd profile set "${LXD_PROFILE_NAME}" security.nesting "true"
    lxc_cmd profile set "${LXD_PROFILE_NAME}" security.privileged "true"
    lxc_cmd profile set "${LXD_PROFILE_NAME}" raw.lxc "$(cat <<'EOF'
lxc.apparmor.profile=unconfined
lxc.cap.drop=
lxc.cgroup.devices.allow=a
lxc.mount.auto=proc:rw sys:rw cgroup:rw
EOF
)"

    if ! profile_has_device "${LXD_PROFILE_NAME}" "kmsg"; then
        lxc_cmd profile device add "${LXD_PROFILE_NAME}" kmsg unix-char source=/dev/kmsg path=/dev/kmsg
    fi
}

image_alias_exists() {
    local alias="$1"
    lxc_cmd image show "${alias}" >/dev/null 2>&1
}

cached_image_alias_for_kind() {
    local kind="$1"
    case "${kind}" in
        sandbox)
            printf '%s\n' "${SANDBOX_IMAGE_ALIAS}"
            ;;
        *)
            printf '%s\n' "${NODE_IMAGE_ALIAS}"
            ;;
    esac
}

base_packages_for_kind() {
    local kind="$1"
    case "${kind}" in
        sandbox)
            printf '%s\n' \
                ca-certificates \
                iproute2 \
                iptables \
                nftables \
                netplan.io
            ;;
        *)
            printf '%s\n' \
                ca-certificates \
                iproute2 \
                iptables \
                nftables \
                netplan.io \
                snapd
            ;;
    esac
}

ensure_container_packages() {
    local name="$1"
    shift

    local package_name
    local -a packages=("$@")
    local -a missing_packages=()

    for package_name in "${packages[@]}"; do
        if ! container_has_package "${name}" "${package_name}" >/dev/null 2>&1; then
            missing_packages+=("${package_name}")
        fi
    done

    if (( ${#missing_packages[@]} == 0 )); then
        return
    fi

    log INFO "Installing base packages in ${name}: ${missing_packages[*]}"
    container_exec "${name}" "export DEBIAN_FRONTEND=noninteractive; apt-get update >/dev/null && apt-get install -y ${missing_packages[*]} >/dev/null"
}

prepare_container_for_image_publish() {
    local name="$1"

    container_exec "${name}" "apt-get clean >/dev/null 2>&1 || true"
    container_exec "${name}" "cloud-init clean --logs --machine-id >/dev/null 2>&1 || cloud-init clean --logs >/dev/null 2>&1 || true"
    container_exec "${name}" "truncate -s 0 /etc/machine-id >/dev/null 2>&1 || true; rm -f /var/lib/dbus/machine-id >/dev/null 2>&1 || true"
}

build_cached_launch_image() {
    local kind="$1"
    local alias seed_name

    alias="$(cached_image_alias_for_kind "${kind}")"
    [[ -n "${alias}" ]] || fatal "Missing cached image alias for kind ${kind}"

    if image_alias_exists "${alias}"; then
        debug "Cached image ${alias} already exists"
        return
    fi

    seed_name="${alias}-seed"
    if container_exists "${seed_name}"; then
        log INFO "Removing stale image seed container ${seed_name}"
        lxc_cmd stop "${seed_name}" --force >/dev/null 2>&1 || true
        lxc_cmd delete "${seed_name}" >/dev/null 2>&1 || true
    fi

    log INFO "Building cached ${kind} image ${alias} from ${IMAGE}"
    lxc_cmd launch -p "${LXD_PROFILE_NAME}" "${IMAGE}" "${seed_name}" >/dev/null
    wait_for_container_boot "${seed_name}"
    ensure_container_base_packages "${seed_name}" "${kind}"
    configure_container_networking "${seed_name}"
    prepare_container_for_image_publish "${seed_name}"
    lxc_cmd stop "${seed_name}" --force >/dev/null 2>&1 || true
    lxc_cmd publish "${seed_name}" --alias "${alias}" >/dev/null
    lxc_cmd delete "${seed_name}" >/dev/null 2>&1 || true
    log INFO "Cached ${kind} image ${alias} is ready"
}

provisioning_image_for_kind() {
    local kind="$1"
    local alias

    alias="$(cached_image_alias_for_kind "${kind}")"
    if ! image_alias_exists "${alias}"; then
        build_cached_launch_image "${kind}"
    fi

    printf '%s\n' "${alias}"
}

delete_cached_image_if_present() {
    local alias="$1"
    if image_alias_exists "${alias}"; then
        log INFO "Deleting cached image ${alias}"
        lxc_cmd image delete "${alias}"
    fi
}

host_setup() {
    log INFO "Preparing host dependencies and LXD resources for cluster ${CLUSTER_NAME}"
    require_host_tools
    ensure_lxd_installed
    log INFO "Waiting for LXD daemon readiness"
    lxd_cmd waitready || fatal "LXD daemon is not ready"
    log INFO "Ensuring LXD is initialized"
    ensure_lxd_initialized
    log INFO "Ensuring cluster networks exist"
    ensure_cluster_networks
    log INFO "Ensuring node profile exists"
    ensure_node_profile
    write_cluster_inventory
    refresh_runtime_inventory
    log INFO "Host bootstrap completed"
}

ensure_runtime_host_setup() {
    log INFO "Preparing runtime host resources for cluster ${CLUSTER_NAME}"
    lxd_cmd waitready || fatal "LXD daemon is not ready"

    if network_exists "${MGMT_NETWORK_NAME}" && network_exists "${SIM_NETWORK_NAME}" && profile_exists "${LXD_PROFILE_NAME}"; then
        log INFO "Runtime host resources already exist; skipping full host bootstrap"
        write_cluster_inventory
        refresh_runtime_inventory
        return
    fi

    host_setup
}

container_exists() {
    local name="$1"
    lxc_cmd info "${name}" >/dev/null 2>&1
}

container_has_microk8s() {
    local name="$1"
    container_exec_direct "${name}" microk8s status >/dev/null 2>&1
}

container_is_microk8s_control_plane() {
    local name="$1"
    local output
    output="$(container_exec_capture_direct "${name}" microk8s kubectl get nodes -o name 2>/dev/null || true)"
    [[ -n "${output}" ]] && printf '%s\n' "${output}" | grep -Eq '^node/'
}

microk8s_cluster_node_names() {
    local control_plane_name="$1"
    container_exec_capture_direct "${control_plane_name}" microk8s kubectl get nodes -o name \
        | sed 's|^node/||'
}

legacy_microk8s_control_plane_names() {
    local name lxc_names_output found=0

    lxc_names_output="$(lxc_cmd list --format csv -c n 2>/dev/null || true)"
    [[ -n "${lxc_names_output}" ]] || return 0
    debug "Scanning legacy control-plane candidates"

    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        case "${name}" in
            master|master-*|*-master|*-master-*|control-plane|control-plane-*|*-control-plane|*-control-plane-*|controller|controller-*|*-controller|*-controller-*)
                if container_is_microk8s_control_plane "${name}" >/dev/null 2>&1; then
                    printf '%s\n' "${name}"
                    found=$((found + 1))
                fi
                ;;
        esac
    done <<< "${lxc_names_output}"

    if (( found > 0 )); then
        debug "Legacy control-plane discovery matched preferred names"
        return 0
    fi

    debug "Preferred legacy control-plane names did not match; scanning all containers"
    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        if container_is_microk8s_control_plane "${name}" >/dev/null 2>&1; then
            printf '%s\n' "${name}"
        fi
    done <<< "${lxc_names_output}"

    return 0
}

single_legacy_microk8s_control_plane_name() {
    local count=0 control_plane control_planes_output selected=""

    control_planes_output="$(legacy_microk8s_control_plane_names || true)"
    if [[ -n "${control_planes_output}" ]]; then
        while IFS= read -r control_plane; do
            [[ -n "${control_plane}" ]] || continue
            count=$((count + 1))
            selected="${control_plane}"
            if (( count > 1 )); then
                return 1
            fi
        done <<< "${control_planes_output}"
    fi

    if (( count == 1 )); then
        printf '%s\n' "${selected}"
        return 0
    fi

    return 1
}

legacy_control_plane_for_cluster() {
    local candidate
    for candidate in "${CONTROL_PLANE_NAME}" "${CLUSTER_NAME}"; do
        [[ -n "${candidate}" ]] || continue
        if container_exists "${candidate}" && container_is_microk8s_control_plane "${candidate}" >/dev/null 2>&1; then
            printf '%s\n' "${candidate}"
            return 0
        fi
    done

    if [[ "${CLUSTER_NAME}" == "leodust" ]]; then
        candidate="$(single_legacy_microk8s_control_plane_name || true)"
        if [[ -n "${candidate}" ]]; then
            printf '%s\n' "${candidate}"
            return 0
        fi
    fi

    return 1
}

worker_name_is_declared() {
    local name="$1"
    local worker_name
    for worker_name in "${WORKER_CONTAINER_NAMES[@]}"; do
        if [[ "${worker_name}" == "${name}" ]]; then
            return 0
        fi
    done
    return 1
}

cluster_container_names() {
    local name cluster_tag legacy_control_plane node_name found=0
    local lxc_names_output legacy_node_names_output
    declare -A emitted_names=()

    if load_fast_lxc_instance_cache; then
        for name in "${FAST_INSTANCE_NAMES[@]}"; do
            [[ -n "${name}" ]] || continue
            cluster_tag="${FAST_INSTANCE_CLUSTER[${name}]:-}"
            if [[ "${cluster_tag}" == "${CLUSTER_NAME}" || "${name}" == "${CONTROL_PLANE_NAME}" || "${name}" == "${WORKER_BASENAME}-"* ]] || worker_name_is_declared "${name}"; then
                if [[ -z "${emitted_names[${name}]:-}" ]]; then
                    printf '%s\n' "${name}"
                    emitted_names["${name}"]="1"
                    found=$((found + 1))
                fi
            fi
        done
        if (( found > 0 )); then
            return 0
        fi
    fi

    binary_available lxc || return 0
    lxc_names_output="$(lxc_cmd list --format csv -c n 2>/dev/null || true)"
    if [[ -n "${lxc_names_output}" ]]; then
        while IFS= read -r name; do
            [[ -n "${name}" ]] || continue
            cluster_tag="$(lxc_cmd config get "${name}" user.leodust.cluster 2>/dev/null || true)"
            if [[ "${cluster_tag}" == "${CLUSTER_NAME}" ]]; then
                if [[ -z "${emitted_names[${name}]:-}" ]]; then
                    printf '%s\n' "${name}"
                    emitted_names["${name}"]="1"
                    found=$((found + 1))
                fi
                continue
            fi
            if [[ "${name}" == "${CONTROL_PLANE_NAME}" || "${name}" == "${WORKER_BASENAME}-"* ]]; then
                if [[ -z "${emitted_names[${name}]:-}" ]]; then
                    printf '%s\n' "${name}"
                    emitted_names["${name}"]="1"
                    found=$((found + 1))
                fi
                continue
            fi
            if worker_name_is_declared "${name}"; then
                if [[ -z "${emitted_names[${name}]:-}" ]]; then
                    printf '%s\n' "${name}"
                    emitted_names["${name}"]="1"
                    found=$((found + 1))
                fi
            fi
        done <<< "${lxc_names_output}"
    fi

    legacy_control_plane="$(legacy_control_plane_for_cluster || true)"
    [[ -n "${legacy_control_plane}" ]] || return 0

    legacy_node_names_output="$(microk8s_cluster_node_names "${legacy_control_plane}" 2>/dev/null || true)"
    if [[ -n "${legacy_node_names_output}" ]]; then
        while IFS= read -r node_name; do
            [[ -n "${node_name}" ]] || continue
            if container_exists "${node_name}" && [[ -z "${emitted_names[${node_name}]:-}" ]]; then
                printf '%s\n' "${node_name}"
                emitted_names["${node_name}"]="1"
                found=$((found + 1))
            fi
        done <<< "${legacy_node_names_output}"
    fi

    if [[ -z "${emitted_names[${legacy_control_plane}]:-}" ]] && container_exists "${legacy_control_plane}"; then
        printf '%s\n' "${legacy_control_plane}"
        emitted_names["${legacy_control_plane}"]="1"
        found=$((found + 1))
    fi

    return 0
}

container_kind() {
    local name="$1"
    local kind

    if load_fast_lxc_instance_cache >/dev/null 2>&1 && [[ "${FAST_INSTANCE_KIND[${name}]+set}" == "set" ]]; then
        kind="${FAST_INSTANCE_KIND[${name}]:-}"
        if [[ -n "${kind}" ]]; then
            printf '%s\n' "${kind}"
        else
            printf 'node\n'
        fi
        return 0
    fi

    kind="$(lxc_cmd config get "${name}" user.leodust.kind 2>/dev/null || true)"
    if [[ -n "${kind}" ]]; then
        printf '%s\n' "${kind}"
    else
        printf 'node\n'
    fi
}

cluster_node_container_names() {
    local name kind
    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        kind="$(container_kind "${name}")"
        [[ "${kind}" == "sandbox" ]] && continue
        printf '%s\n' "${name}"
    done < <(cluster_container_names)
}

sandbox_container_names() {
    local name kind
    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        kind="$(container_kind "${name}")"
        [[ "${kind}" == "sandbox" ]] || continue
        printf '%s\n' "${name}"
    done < <(cluster_container_names)
}

sandbox_container_name_for_satellite() {
    local satellite_id="$1"
    printf '%s-sat-%s\n' "${CLUSTER_NAME}" "$(sanitize_instance_name "${satellite_id}")"
}

resolve_sandbox_name() {
    if [[ -n "${SANDBOX_NAME}" ]]; then
        printf '%s\n' "${SANDBOX_NAME}"
        return 0
    fi
    if [[ -n "${SATELLITE_ID}" ]]; then
        sandbox_container_name_for_satellite "${SATELLITE_ID}"
        return 0
    fi
    fatal "A sandbox target requires --sandbox-name or --satellite-id"
}

sandbox_exists() {
    local name="$1"
    container_exists "${name}" || return 1
    [[ "$(container_kind "${name}")" == "sandbox" ]]
}

container_primary_profile() {
    local name="$1"

    if load_fast_lxc_instance_cache >/dev/null 2>&1 && [[ "${FAST_INSTANCE_PROFILE[${name}]+set}" == "set" ]]; then
        printf '%s\n' "${FAST_INSTANCE_PROFILE[${name}]}"
        return 0
    fi

    lxc_cmd config show "${name}" | awk '
        /^profiles:$/ {in_profiles=1; next}
        in_profiles == 1 && /^[^[:space:]-]/ {exit}
        in_profiles == 1 && /^[[:space:]]*-[[:space:]]*/ {
            sub(/^[[:space:]]*-[[:space:]]*/, "", $0)
            print
            exit
        }
    '
}

write_cluster_inventory() {
    local inventory_file output_dir
    inventory_file="$(cluster_inventory_file "${CLUSTER_NAME}")"
    output_dir="$(dirname "${inventory_file}")"
    mkdir -p "${output_dir}"

    {
        printf 'cluster_name\t%s\n' "${CLUSTER_NAME}"
        printf 'control_plane_name\t%s\n' "${CONTROL_PLANE_NAME}"
        printf 'worker_basename\t%s\n' "${WORKER_BASENAME}"
        printf 'lxd_profile_name\t%s\n' "${LXD_PROFILE_NAME}"
        printf 'management_network_name\t%s\n' "${MGMT_NETWORK_NAME}"
        printf 'simulation_network_name\t%s\n' "${SIM_NETWORK_NAME}"
        printf 'node_map_output_file\t%s\n' "${NODE_MAP_OUTPUT_FILE}"
        printf 'kubeconfig_output_file\t%s\n' "${KUBECONFIG_OUTPUT_FILE}"
    } > "${inventory_file}"

    log INFO "Wrote cluster inventory to ${inventory_file}"
}

cleanup_cluster_inventory_file() {
    local inventory_file
    inventory_file="$(cluster_inventory_file "${CLUSTER_NAME}")"
    [[ -f "${inventory_file}" ]] || return 0
    rm -f "${inventory_file}"
    log INFO "Removed cluster inventory ${inventory_file}"
}

cluster_has_inventory() {
    [[ -f "$(cluster_inventory_file "${CLUSTER_NAME}")" ]]
}

cluster_has_container_metadata() {
    local name

    if load_fast_lxc_instance_cache; then
        for name in "${FAST_INSTANCE_NAMES[@]}"; do
            if [[ "${FAST_INSTANCE_CLUSTER[${name}]:-}" == "${CLUSTER_NAME}" ]]; then
                return 0
            fi
        done
        return 1
    fi

    binary_available lxc || return 1
    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        if [[ "$(lxc_cmd config get "${name}" user.leodust.cluster 2>/dev/null || true)" == "${CLUSTER_NAME}" ]]; then
            return 0
        fi
    done < <(lxc_cmd list --format csv -c n 2>/dev/null || true)

    return 1
}

cluster_is_script_managed() {
    cluster_has_inventory || cluster_has_container_metadata
}

cluster_is_legacy_control_plane_name() {
    local name="$1"
    local cluster_tag

    [[ -n "${name}" ]] || return 1
    container_exists "${name}" || return 1
    cluster_tag="$(lxc_cmd config get "${name}" user.leodust.cluster 2>/dev/null || true)"
    [[ -z "${cluster_tag}" ]] || return 1
    container_is_microk8s_control_plane "${name}" >/dev/null 2>&1
}

hydrate_cluster_context_from_inventory() {
    local inventory_file value
    inventory_file="$(cluster_inventory_file "${CLUSTER_NAME}")"
    [[ -f "${inventory_file}" ]] || return 0

    value="$(inventory_value "${inventory_file}" control_plane_name)"
    [[ -n "${value}" ]] && CONTROL_PLANE_NAME="${value}"
    value="$(inventory_value "${inventory_file}" worker_basename)"
    [[ -n "${value}" ]] && WORKER_BASENAME="${value}"
    value="$(inventory_value "${inventory_file}" lxd_profile_name)"
    [[ -n "${value}" ]] && LXD_PROFILE_NAME="${value}"
    value="$(inventory_value "${inventory_file}" management_network_name)"
    [[ -n "${value}" ]] && MGMT_NETWORK_NAME="${value}"
    value="$(inventory_value "${inventory_file}" simulation_network_name)"
    [[ -n "${value}" ]] && SIM_NETWORK_NAME="${value}"
    value="$(inventory_value "${inventory_file}" node_map_output_file)"
    [[ -n "${value}" ]] && NODE_MAP_OUTPUT_FILE="${value}"
    value="$(inventory_value "${inventory_file}" kubeconfig_output_file)"
    [[ -n "${value}" ]] && KUBECONFIG_OUTPUT_FILE="${value}"
    return 0
}

hydrate_cluster_context_from_containers() {
    local name first_container="" role value legacy_control_plane

    binary_available lxc || return 0
    if ! lxc_cmd list --format csv -c n >/dev/null 2>&1; then
        return 0
    fi

    if load_fast_lxc_instance_cache; then
        for name in "${FAST_INSTANCE_NAMES[@]}"; do
            [[ -n "${name}" ]] || continue
            if [[ "${FAST_INSTANCE_CLUSTER[${name}]:-}" != "${CLUSTER_NAME}" && "${name}" != "${CONTROL_PLANE_NAME}" && "${name}" != "${WORKER_BASENAME}-"* ]] && ! worker_name_is_declared "${name}"; then
                continue
            fi
            if [[ "${FAST_INSTANCE_KIND[${name}]:-node}" == "sandbox" ]]; then
                continue
            fi
            [[ -n "${first_container}" ]] || first_container="${name}"
            role="${FAST_INSTANCE_ROLE[${name}]:-}"
            if [[ "${role}" == "control-plane" ]]; then
                CONTROL_PLANE_NAME="${name}"
                first_container="${name}"
                break
            fi
        done

        if [[ -n "${first_container}" ]]; then
            value="${FAST_INSTANCE_WORKER_BASE[${first_container}]:-}"
            [[ -n "${value}" ]] && WORKER_BASENAME="${value}"
            value="${FAST_INSTANCE_MGMT_NETWORK[${first_container}]:-}"
            [[ -n "${value}" ]] && MGMT_NETWORK_NAME="${value}"
            value="${FAST_INSTANCE_SIM_NETWORK[${first_container}]:-}"
            [[ -n "${value}" ]] && SIM_NETWORK_NAME="${value}"
            value="${FAST_INSTANCE_PROFILE[${first_container}]:-}"
            [[ -n "${value}" ]] && LXD_PROFILE_NAME="${value}"
            return 0
        fi
    fi

    legacy_control_plane="$(legacy_control_plane_for_cluster || true)"
    if [[ -n "${legacy_control_plane}" ]]; then
        CONTROL_PLANE_NAME="${legacy_control_plane}"
        first_container="${legacy_control_plane}"
    fi

    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        [[ -n "${first_container}" ]] || first_container="${name}"
        role="$(lxc_cmd config get "${name}" user.leodust.role 2>/dev/null || true)"
        if [[ "${role}" == "control-plane" ]]; then
            CONTROL_PLANE_NAME="${name}"
            first_container="${name}"
        fi
    done < <(cluster_node_container_names)

    [[ -n "${first_container}" ]] || return 0

    value="$(lxc_cmd config get "${first_container}" user.leodust.worker-base-name 2>/dev/null || true)"
    [[ -n "${value}" ]] && WORKER_BASENAME="${value}"
    value="$(lxc_cmd config get "${first_container}" user.leodust.management-network 2>/dev/null || true)"
    [[ -n "${value}" ]] && MGMT_NETWORK_NAME="${value}"
    value="$(lxc_cmd config get "${first_container}" user.leodust.simulation-network 2>/dev/null || true)"
    [[ -n "${value}" ]] && SIM_NETWORK_NAME="${value}"
    value="$(lxc_cmd config get "${first_container}" user.leodust.profile 2>/dev/null || true)"
    if [[ -n "${value}" ]]; then
        LXD_PROFILE_NAME="${value}"
    else
        value="$(container_primary_profile "${first_container}" || true)"
        [[ -n "${value}" ]] && LXD_PROFILE_NAME="${value}"
    fi
    value="$(lxc_cmd config get "${first_container}" user.leodust.node-map-output-file 2>/dev/null || true)"
    [[ -n "${value}" ]] && NODE_MAP_OUTPUT_FILE="${value}"
    return 0
}

setup_cluster_context() {
    local cluster_name="$1"
    set_cluster_defaults "${cluster_name}"
    hydrate_cluster_context_from_inventory
    hydrate_cluster_context_from_containers
    return 0
}

sync_current_cluster_context() {
    log INFO "Resolving cluster context for ${CLUSTER_NAME}"
    hydrate_cluster_context_from_inventory
    hydrate_cluster_context_from_containers

    if ! cluster_has_inventory && ! cluster_has_container_metadata && [[ "${CLUSTER_NAME}" == "leodust" ]]; then
        local discovered_cluster
        log INFO "No inventory or metadata found for ${CLUSTER_NAME}; probing discovered clusters"
        discovered_cluster="$(single_discovered_cluster_name || true)"
        if [[ -n "${discovered_cluster}" ]]; then
            log INFO "Using discovered cluster ${discovered_cluster}"
            setup_cluster_context "${discovered_cluster}"
        fi
    fi
    return 0
}

managed_cluster_names_slow() {
    local cluster_name inventory_file name
    declare -A seen_clusters=()
    local lxc_names_output

    if [[ -d "${CLUSTER_RESULTS_DIR}" ]]; then
        while IFS= read -r inventory_file; do
            [[ -n "${inventory_file}" ]] || continue
            cluster_name="$(inventory_value "${inventory_file}" cluster_name)"
            if [[ -z "${cluster_name}" ]]; then
                cluster_name="$(basename "${inventory_file}")"
                cluster_name="${cluster_name%"${CLUSTER_INVENTORY_SUFFIX}"}"
            fi
            if [[ -n "${cluster_name}" && -z "${seen_clusters[${cluster_name}]:-}" ]]; then
                seen_clusters["${cluster_name}"]="1"
                printf '%s\n' "${cluster_name}"
            fi
        done < <(find "${CLUSTER_RESULTS_DIR}" -maxdepth 1 -type f -name "*${CLUSTER_INVENTORY_SUFFIX}" 2>/dev/null | sort)
    fi

    if binary_available lxc; then
        lxc_names_output="$(lxc_cmd list --format csv -c n 2>/dev/null || true)"

        if [[ -n "${lxc_names_output}" ]]; then
            debug "LXC returned container names for discovery"
            while IFS= read -r name; do
                [[ -n "${name}" ]] || continue
                cluster_name="$(lxc_cmd config get "${name}" user.leodust.cluster 2>/dev/null || true)"
                if [[ -n "${cluster_name}" && -z "${seen_clusters[${cluster_name}]:-}" ]]; then
                    seen_clusters["${cluster_name}"]="1"
                    printf '%s\n' "${cluster_name}"
                fi
            done <<< "${lxc_names_output}"

            while IFS= read -r name; do
                [[ -n "${name}" ]] || continue
                cluster_name="$(lxc_cmd config get "${name}" user.leodust.cluster 2>/dev/null || true)"
                if [[ -n "${cluster_name}" && -n "${seen_clusters[${cluster_name}]:-}" ]]; then
                    continue
                fi
                if [[ -z "${seen_clusters[${name}]:-}" ]]; then
                    debug "Discovered legacy cluster ${name}"
                    seen_clusters["${name}"]="1"
                    printf '%s\n' "${name}"
                fi
            done < <(legacy_microk8s_control_plane_names)
        else
            debug "LXC returned no container names during discovery"
        fi
    fi

    return 0
}

managed_cluster_names() {
    local fast_output slow_output cluster_name
    declare -A seen_clusters=()

    fast_output="$(managed_cluster_names_fast || true)"
    if [[ -n "${fast_output}" ]]; then
        while IFS= read -r cluster_name; do
            [[ -n "${cluster_name}" ]] || continue
            [[ -n "${seen_clusters[${cluster_name}]:-}" ]] && continue
            seen_clusters["${cluster_name}"]="1"
            printf '%s\n' "${cluster_name}"
        done <<< "${fast_output}"

        if fast_cache_has_legacy_candidates; then
            slow_output="$(managed_cluster_names_slow || true)"
            while IFS= read -r cluster_name; do
                [[ -n "${cluster_name}" ]] || continue
                [[ -n "${seen_clusters[${cluster_name}]:-}" ]] && continue
                seen_clusters["${cluster_name}"]="1"
                printf '%s\n' "${cluster_name}"
            done <<< "${slow_output}"
        fi
        return 0
    fi

    managed_cluster_names_slow
}

single_discovered_cluster_name() {
    local cluster_name count=0 selected="" discovered_clusters_output

    discovered_clusters_output="$(managed_cluster_names || true)"
    if [[ -n "${discovered_clusters_output}" ]]; then
        while IFS= read -r cluster_name; do
            [[ -n "${cluster_name}" ]] || continue
            count=$((count + 1))
            selected="${cluster_name}"
            if (( count > 1 )); then
                return 1
            fi
        done <<< "${discovered_clusters_output}"
    fi

    if (( count == 1 )); then
        printf '%s\n' "${selected}"
        return 0
    fi

    return 1
}

wait_for_container_boot() {
    local name="$1"
    wait_until_container_exec "${name}" "container ${name} to accept commands" "true"

    if container_exec "${name}" "command -v cloud-init >/dev/null 2>&1" >/dev/null 2>&1; then
        log INFO "Waiting for cloud-init in ${name}"
        container_exec "${name}" "cloud-init status --wait >/dev/null 2>&1 || true"
    fi
}

container_ipv4() {
    local name="$1"
    local iface="$2"
    container_exec_capture "${name}" "ip -o -4 addr show dev ${iface} | awk 'NR==1 {print \$4}' | cut -d/ -f1"
}

container_has_package() {
    local name="$1"
    local package_name="$2"
    container_exec "${name}" "dpkg-query -W -f='\${Status}' '${package_name}' 2>/dev/null | grep -q 'install ok installed'"
}

ensure_container_base_packages() {
    local name="$1"
    local kind="${2:-node}"
    local package_name
    local -a packages=()

    while IFS= read -r package_name; do
        [[ -n "${package_name}" ]] || continue
        packages+=("${package_name}")
    done < <(base_packages_for_kind "${kind}")

    ensure_container_packages "${name}" "${packages[@]}"
}

configure_container_networking() {
    local name="$1"

    wait_until_container_exec "${name}" "dual interfaces in ${name}" "ip link show dev ${MGMT_INTERFACE_NAME} >/dev/null 2>&1 && ip link show dev ${SIM_INTERFACE_NAME} >/dev/null 2>&1"

    container_exec "${name}" "cat > /etc/netplan/90-leodust-dual-network.yaml <<'EOF'
network:
  version: 2
  ethernets:
    ${MGMT_INTERFACE_NAME}:
      dhcp4: true
      dhcp6: false
      optional: true
    ${SIM_INTERFACE_NAME}:
      dhcp4: true
      dhcp6: false
      optional: true
      dhcp4-overrides:
        use-routes: false
EOF
chmod 600 /etc/netplan/90-leodust-dual-network.yaml
netplan generate
netplan apply"

    wait_until_container_exec "${name}" "IPv4 on ${MGMT_INTERFACE_NAME} in ${name}" "ip -o -4 addr show dev ${MGMT_INTERFACE_NAME} | grep -q ."
    wait_until_container_exec "${name}" "IPv4 on ${SIM_INTERFACE_NAME} in ${name}" "ip -o -4 addr show dev ${SIM_INTERFACE_NAME} | grep -q ."
}

apply_container_metadata() {
    local name="$1"
    local role="$2"
    local simulation_name="${3:-}"
    local kind="${4:-node}"
    local mgmt_ip sim_ip

    lxc_cmd config set "${name}" user.leodust.cluster "${CLUSTER_NAME}"
    lxc_cmd config set "${name}" user.leodust.kind "${kind}"
    lxc_cmd config set "${name}" user.leodust.role "${role}"
    lxc_cmd config set "${name}" user.leodust.control-plane-name "${CONTROL_PLANE_NAME}"
    lxc_cmd config set "${name}" user.leodust.worker-base-name "${WORKER_BASENAME}"
    lxc_cmd config set "${name}" user.leodust.profile "${LXD_PROFILE_NAME}"
    lxc_cmd config set "${name}" user.leodust.management-network "${MGMT_NETWORK_NAME}"
    lxc_cmd config set "${name}" user.leodust.simulation-network "${SIM_NETWORK_NAME}"
    lxc_cmd config set "${name}" user.leodust.management-interface "${MGMT_INTERFACE_NAME}"
    lxc_cmd config set "${name}" user.leodust.simulation-interface "${SIM_INTERFACE_NAME}"
    if [[ "${kind}" == "sandbox" ]]; then
        lxc_cmd config set "${name}" limits.cpu "${SANDBOX_CPU}"
        lxc_cmd config set "${name}" limits.memory "${SANDBOX_MEMORY}"
    else
        lxc_cmd config set "${name}" limits.cpu "${CONTAINER_CPU}"
        lxc_cmd config set "${name}" limits.memory "${CONTAINER_MEMORY}"
    fi
    if [[ -n "${NODE_MAP_OUTPUT_FILE}" ]]; then
        lxc_cmd config set "${name}" user.leodust.node-map-output-file "${NODE_MAP_OUTPUT_FILE}"
    else
        lxc_cmd config unset "${name}" user.leodust.node-map-output-file >/dev/null 2>&1 || true
    fi

    if [[ -n "${simulation_name}" ]]; then
        lxc_cmd config set "${name}" user.leodust.simulation-name "${simulation_name}"
    else
        lxc_cmd config unset "${name}" user.leodust.simulation-name >/dev/null 2>&1 || true
    fi

    mgmt_ip="$(container_ipv4 "${name}" "${MGMT_INTERFACE_NAME}" || true)"
    sim_ip="$(container_ipv4 "${name}" "${SIM_INTERFACE_NAME}" || true)"

    if [[ -n "${mgmt_ip}" ]]; then
        lxc_cmd config set "${name}" user.leodust.management-ip "${mgmt_ip}"
    fi
    if [[ -n "${sim_ip}" ]]; then
        lxc_cmd config set "${name}" user.leodust.simulation-ip "${sim_ip}"
    fi
}

launch_container() {
    local name="$1"
    local role="$2"
    local simulation_name="${3:-}"
    local kind="${4:-node}"
    local launch_image

    if container_exists "${name}"; then
        log INFO "Container ${name} already exists"
        wait_for_container_boot "${name}"
        ensure_container_base_packages "${name}" "${kind}"
        configure_container_networking "${name}"
        apply_container_metadata "${name}" "${role}" "${simulation_name}" "${kind}"
        return
    fi

    launch_image="$(provisioning_image_for_kind "${kind}")"
    log INFO "Launching container ${name} from ${launch_image}"
    lxc_cmd launch -p "${LXD_PROFILE_NAME}" "${launch_image}" "${name}"
    wait_for_container_boot "${name}"
    ensure_container_base_packages "${name}" "${kind}"
    configure_container_networking "${name}"
    apply_container_metadata "${name}" "${role}" "${simulation_name}" "${kind}"
}

sandbox_app_state_dir() {
    printf '/var/lib/leodust/apps'
}

sandbox_app_log_dir() {
    printf '/var/log/leodust/apps'
}

sandbox_app_key() {
    sanitize_instance_name "$1"
}

sandbox_app_names() {
    local name="$1"
    local csv item
    csv="$(lxc_cmd config get "${name}" user.leodust.apps 2>/dev/null || true)"
    [[ -n "${csv}" ]] || return 0
    IFS=',' read -r -a app_array <<< "${csv}"
    for item in "${app_array[@]}"; do
        item="$(trim_whitespace "${item}")"
        [[ -n "${item}" ]] || continue
        printf '%s\n' "${item}"
    done
}

update_sandbox_apps_metadata() {
    local name="$1"
    shift
    local csv count
    csv="$(join_csv "$@")"
    if [[ -n "${csv}" ]]; then
        lxc_cmd config set "${name}" user.leodust.apps "${csv}"
    else
        lxc_cmd config unset "${name}" user.leodust.apps >/dev/null 2>&1 || true
    fi
    count="$(csv_item_count "${csv}")"
    lxc_cmd config set "${name}" user.leodust.app-count "${count}"
}

append_sandbox_app() {
    local name="$1"
    local app_name="$2"
    local current item
    local exists="false"
    local -a apps=()

    current="$(lxc_cmd config get "${name}" user.leodust.apps 2>/dev/null || true)"
    if [[ -n "${current}" ]]; then
        IFS=',' read -r -a current_array <<< "${current}"
        for item in "${current_array[@]}"; do
            item="$(trim_whitespace "${item}")"
            [[ -n "${item}" ]] || continue
            [[ "${item}" == "${app_name}" ]] && exists="true"
            apps+=("${item}")
        done
    fi

    [[ "${exists}" == "true" ]] || apps+=("${app_name}")
    update_sandbox_apps_metadata "${name}" "${apps[@]}"
}

remove_sandbox_app() {
    local name="$1"
    local app_name="$2"
    local current item
    local -a apps=()

    current="$(lxc_cmd config get "${name}" user.leodust.apps 2>/dev/null || true)"
    if [[ -n "${current}" ]]; then
        IFS=',' read -r -a current_array <<< "${current}"
        for item in "${current_array[@]}"; do
            item="$(trim_whitespace "${item}")"
            [[ -n "${item}" ]] || continue
            [[ "${item}" == "${app_name}" ]] && continue
            apps+=("${item}")
        done
    fi

    update_sandbox_apps_metadata "${name}" "${apps[@]}"
}

sandbox_app_is_running() {
    local name="$1"
    local app_name="$2"
    local key
    key="$(sandbox_app_key "${app_name}")"
    container_exec "${name}" "pid_file='$(sandbox_app_state_dir)/${key}/pid'; [[ -f \"\${pid_file}\" ]] && kill -0 \"\$(cat \"\${pid_file}\")\" >/dev/null 2>&1"
}

sandbox_app_metadata_value() {
    local name="$1"
    local app_name="$2"
    local key_name="$3"
    local key
    key="$(sandbox_app_key "${app_name}")"
    container_exec_capture "${name}" "meta_file='$(sandbox_app_state_dir)/${key}/meta.env'; [[ -f \"\${meta_file}\" ]] && sed -n 's/^${key_name}=//p' \"\${meta_file}\" | head -n 1"
}

auto_select_sandbox_host_worker() {
    local best_worker="" name role worker count best_count=-1

    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        role="$(lxc_cmd config get "${name}" user.leodust.role 2>/dev/null || true)"
        [[ "${role}" == "worker" ]] || continue
        count=0
        while IFS= read -r worker; do
            [[ -n "${worker}" ]] || continue
            if [[ "$(lxc_cmd config get "${worker}" user.leodust.host-worker 2>/dev/null || true)" == "${name}" ]]; then
                count=$((count + 1))
            fi
        done < <(sandbox_container_names)
        if (( best_count == -1 || count < best_count )); then
            best_worker="${name}"
            best_count="${count}"
        fi
    done < <(cluster_node_container_names)

    if [[ -z "${best_worker}" && -n "${CONTROL_PLANE_NAME}" ]] && container_exists "${CONTROL_PLANE_NAME}"; then
        best_worker="${CONTROL_PLANE_NAME}"
    fi

    printf '%s\n' "${best_worker}"
}

configure_sandbox_runtime() {
    local name="$1"
    local role="$2"

    container_exec "${name}" "mkdir -p $(sandbox_app_state_dir) $(sandbox_app_log_dir)"
    if [[ "${role}" == "relay" ]]; then
        container_exec "${name}" "sysctl -w net.ipv4.ip_forward=1 >/dev/null"
    fi
}

apply_sandbox_metadata() {
    local name="$1"
    local satellite_id="$2"
    local role="$3"
    local host_worker="$4"

    lxc_cmd config set "${name}" user.leodust.satellite-id "${satellite_id}"
    lxc_cmd config set "${name}" user.leodust.sandbox-role "${role}"
    if [[ -n "${host_worker}" ]]; then
        lxc_cmd config set "${name}" user.leodust.host-worker "${host_worker}"
    else
        lxc_cmd config unset "${name}" user.leodust.host-worker >/dev/null 2>&1 || true
    fi
    lxc_cmd config set "${name}" user.leodust.app-count "$(csv_item_count "$(lxc_cmd config get "${name}" user.leodust.apps 2>/dev/null || true)")"
}

launch_sandbox() {
    local name="$1"
    local satellite_id="$2"
    local role="$3"
    local host_worker="$4"

    launch_container "${name}" "sandbox" "${satellite_id}" "sandbox"
    configure_sandbox_runtime "${name}" "${role}"
    apply_sandbox_metadata "${name}" "${satellite_id}" "${role}" "${host_worker}"
}

ensure_snapd_ready() {
    local name="$1"

    ensure_container_packages "${name}" snapd
    container_exec "${name}" "systemctl enable --now snapd.socket snapd.service >/dev/null 2>&1 || true"
    wait_until_container_exec "${name}" "snapd seeding in ${name}" "snap wait system seed.loaded"
}

ensure_microk8s_installed() {
    local name="$1"

    if container_exec "${name}" "snap list microk8s >/dev/null 2>&1"; then
        log INFO "MicroK8s already installed in ${name}"
    else
        log INFO "Installing MicroK8s ${MICROK8S_CHANNEL} in ${name}"
        ensure_snapd_ready "${name}"
        container_exec "${name}" "snap install microk8s --classic --channel='${MICROK8S_CHANNEL}'"
    fi

    log INFO "Waiting for MicroK8s to become ready in ${name}"
    container_exec "${name}" "microk8s status --wait-ready >/dev/null 2>&1"
}

enable_addons() {
    [[ -n "${ADDONS}" ]] || return 0
    [[ "${ADDONS}" != "none" ]] || return 0

    local raw addon
    local -a addons_array=()
    IFS=',' read -r -a raw <<< "${ADDONS}"
    for addon in "${raw[@]}"; do
        addon="$(trim_whitespace "${addon}")"
        [[ -n "${addon}" ]] || continue
        addons_array+=("${addon}")
    done

    (( ${#addons_array[@]} > 0 )) || return 0

    log INFO "Enabling addons on ${CONTROL_PLANE_NAME}: ${addons_array[*]}"
    for addon in "${addons_array[@]}"; do
        container_exec "${CONTROL_PLANE_NAME}" "microk8s enable ${addon}"
    done
}

control_plane_has_node() {
    local node_name="$1"
    container_exec "${CONTROL_PLANE_NAME}" "microk8s kubectl get nodes -o name | grep -Fxq 'node/${node_name}'"
}

label_node() {
    local node_name="$1"
    local role="$2"
    container_exec "${CONTROL_PLANE_NAME}" "microk8s kubectl label node ${node_name} \
        leodust.io/role=${role} \
        leodust.io/management-interface=${MGMT_INTERFACE_NAME} \
        leodust.io/simulation-interface=${SIM_INTERFACE_NAME} \
        --overwrite"
}

taint_control_plane() {
    [[ "${TAINT_CONTROL_PLANE}" == "true" ]] || return 0
    container_exec "${CONTROL_PLANE_NAME}" "microk8s kubectl taint node ${CONTROL_PLANE_NAME} node-role.kubernetes.io/control-plane=true:NoSchedule --overwrite"
}

generate_worker_join_command() {
    local output join_cmd
    output="$(container_exec_capture "${CONTROL_PLANE_NAME}" "microk8s add-node --token-ttl 3600")"
    join_cmd="$(printf '%s\n' "${output}" | awk '
        /microk8s join/ && /--worker/ {gsub(/^[[:space:]]+/, "", $0); print; exit}
        /microk8s join/ && candidate == "" {candidate = $0}
        END {
            gsub(/^[[:space:]]+/, "", candidate)
            if (candidate != "") {
                print candidate
            }
        }
    ')"
    [[ -n "${join_cmd}" ]] || fatal "Unable to parse MicroK8s join command"
    if [[ "${join_cmd}" != *"--worker"* ]]; then
        join_cmd="${join_cmd} --worker"
    fi
    printf '%s\n' "${join_cmd}"
}

join_worker() {
    local worker_name="$1"
    local join_cmd="$2"

    if control_plane_has_node "${worker_name}" >/dev/null 2>&1; then
        log INFO "Worker ${worker_name} is already joined"
        return
    fi

    log INFO "Joining worker ${worker_name} to the cluster"
    container_exec "${worker_name}" "microk8s leave >/dev/null 2>&1 || true"
    container_exec "${worker_name}" "${join_cmd}"
    wait_until "${WAIT_TIMEOUT_SECONDS}" "worker ${worker_name} to register" control_plane_has_node "${worker_name}"
}

next_worker_index() {
    local name max_index suffix
    max_index=0
    while IFS= read -r name; do
        [[ "${name}" == "${WORKER_BASENAME}-"* ]] || continue
        suffix="${name#${WORKER_BASENAME}-}"
        [[ "${suffix}" =~ ^[0-9]+$ ]] || continue
        if (( suffix > max_index )); then
            max_index="${suffix}"
        fi
    done < <(cluster_node_container_names)
    printf '%s\n' $((max_index + 1))
}

select_worker_plan() {
    local requested="$1"
    SELECTED_WORKER_NODE_NAMES=()
    SELECTED_WORKER_CONTAINER_NAMES=()

    if (( ${#WORKER_CONTAINER_NAMES[@]} > 0 )); then
        local i
        for (( i = 0; i < ${#WORKER_CONTAINER_NAMES[@]}; i++ )); do
            if container_exists "${WORKER_CONTAINER_NAMES[$i]}"; then
                continue
            fi
            SELECTED_WORKER_CONTAINER_NAMES+=("${WORKER_CONTAINER_NAMES[$i]}")
            SELECTED_WORKER_NODE_NAMES+=("${WORKER_NODE_NAMES[$i]}")
            if (( requested > 0 && ${#SELECTED_WORKER_CONTAINER_NAMES[@]} >= requested )); then
                break
            fi
        done

        if (( requested > 0 && ${#SELECTED_WORKER_CONTAINER_NAMES[@]} < requested )); then
            fatal "Requested ${requested} workers but only ${#SELECTED_WORKER_CONTAINER_NAMES[@]} remain in ${WORKER_NAMES_FILE}"
        fi
        return
    fi

    (( requested > 0 )) || return

    local worker_index worker_name
    worker_index="$(next_worker_index)"
    while (( ${#SELECTED_WORKER_CONTAINER_NAMES[@]} < requested )); do
        worker_name="${WORKER_BASENAME}-${worker_index}"
        SELECTED_WORKER_CONTAINER_NAMES+=("${worker_name}")
        SELECTED_WORKER_NODE_NAMES+=("")
        worker_index=$((worker_index + 1))
    done
}

write_node_map_file() {
    [[ -n "${NODE_MAP_OUTPUT_FILE}" ]] || return 0

    local output_dir tmp_file first_entry name simulation_name
    output_dir="$(dirname "${NODE_MAP_OUTPUT_FILE}")"
    mkdir -p "${output_dir}"
    tmp_file="$(mktemp)"

    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        simulation_name="$(lxc_cmd config get "${name}" user.leodust.simulation-name 2>/dev/null || true)"
        [[ -n "${simulation_name}" ]] || continue
        printf '%s\t%s\n' "${simulation_name}" "${name}" >> "${tmp_file}"
    done < <(cluster_node_container_names)

    {
        printf '{\n'
        first_entry="true"
        while IFS=$'\t' read -r simulation_name name; do
            [[ -n "${simulation_name}" ]] || continue
            if [[ "${first_entry}" == "true" ]]; then
                first_entry="false"
            else
                printf ',\n'
            fi
            printf '  "%s": "%s"' "$(json_escape "${simulation_name}")" "$(json_escape "${name}")"
        done < "${tmp_file}"
        printf '\n}\n'
    } > "${NODE_MAP_OUTPUT_FILE}"

    rm -f "${tmp_file}"
    log INFO "Wrote simulation/container name map to ${NODE_MAP_OUTPUT_FILE}"
}

cleanup_node_map_file() {
    [[ -n "${NODE_MAP_OUTPUT_FILE}" ]] || return 0
    [[ -f "${NODE_MAP_OUTPUT_FILE}" ]] || return 0
    rm -f "${NODE_MAP_OUTPUT_FILE}"
    log INFO "Removed node map file ${NODE_MAP_OUTPUT_FILE}"
}

write_sandbox_inventory() {
    local output_file output_dir name satellite_id sandbox_role host_worker mgmt_ip sim_ip app_count state

    output_file="$(sandbox_inventory_file "${CLUSTER_NAME}")"
    output_dir="$(dirname "${output_file}")"
    mkdir -p "${output_dir}"

    {
        printf 'sandbox_name\tsatellite_id\tsandbox_role\thost_worker\tstate\tmanagement_ip\tsimulation_ip\tapp_count\n'
        while IFS= read -r name; do
            [[ -n "${name}" ]] || continue
            satellite_id="$(lxc_cmd config get "${name}" user.leodust.satellite-id 2>/dev/null || true)"
            sandbox_role="$(lxc_cmd config get "${name}" user.leodust.sandbox-role 2>/dev/null || true)"
            host_worker="$(lxc_cmd config get "${name}" user.leodust.host-worker 2>/dev/null || true)"
            mgmt_ip="$(lxc_cmd config get "${name}" user.leodust.management-ip 2>/dev/null || true)"
            sim_ip="$(lxc_cmd config get "${name}" user.leodust.simulation-ip 2>/dev/null || true)"
            app_count="$(lxc_cmd config get "${name}" user.leodust.app-count 2>/dev/null || true)"
            [[ -n "${app_count}" ]] || app_count="0"
            state="$(lxc_cmd list "${name}" --format csv -c s 2>/dev/null | head -n 1 || true)"
            [[ -n "${state}" ]] || state="UNKNOWN"
            printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
                "${name}" \
                "${satellite_id}" \
                "${sandbox_role}" \
                "${host_worker}" \
                "${state}" \
                "${mgmt_ip}" \
                "${sim_ip}" \
                "${app_count}"
        done < <(sandbox_container_names)
    } > "${output_file}"

    debug "Wrote sandbox inventory to ${output_file}"
}

write_app_inventory() {
    local output_file output_dir sandbox_name app_name app_state pid started_at host_worker satellite_id log_path

    output_file="$(app_inventory_file "${CLUSTER_NAME}")"
    output_dir="$(dirname "${output_file}")"
    mkdir -p "${output_dir}"

    {
        printf 'sandbox_name\tsatellite_id\thost_worker\tapp_name\tstate\tpid\tstarted_at\tlog_path\n'
        while IFS= read -r sandbox_name; do
            [[ -n "${sandbox_name}" ]] || continue
            satellite_id="$(lxc_cmd config get "${sandbox_name}" user.leodust.satellite-id 2>/dev/null || true)"
            host_worker="$(lxc_cmd config get "${sandbox_name}" user.leodust.host-worker 2>/dev/null || true)"
            while IFS= read -r app_name; do
                [[ -n "${app_name}" ]] || continue
                if sandbox_app_is_running "${sandbox_name}" "${app_name}" >/dev/null 2>&1; then
                    app_state="running"
                else
                    app_state="stopped"
                fi
                pid="$(sandbox_app_metadata_value "${sandbox_name}" "${app_name}" PID || true)"
                started_at="$(sandbox_app_metadata_value "${sandbox_name}" "${app_name}" STARTED_AT || true)"
                log_path="$(sandbox_app_metadata_value "${sandbox_name}" "${app_name}" LOG_PATH || true)"
                printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
                    "${sandbox_name}" \
                    "${satellite_id}" \
                    "${host_worker}" \
                    "${app_name}" \
                    "${app_state}" \
                    "${pid}" \
                    "${started_at}" \
                    "${log_path}"
            done < <(sandbox_app_names "${sandbox_name}")
        done < <(sandbox_container_names)
    } > "${output_file}"

    debug "Wrote app inventory to ${output_file}"
}

refresh_runtime_inventory() {
    write_sandbox_inventory
    write_app_inventory
}

cleanup_runtime_inventory_files() {
    local output_file

    output_file="$(sandbox_inventory_file "${CLUSTER_NAME}")"
    [[ ! -f "${output_file}" ]] || rm -f "${output_file}"

    output_file="$(app_inventory_file "${CLUSTER_NAME}")"
    [[ ! -f "${output_file}" ]] || rm -f "${output_file}"
}

ensure_satellite_sandbox() {
    local satellite_id="$1"
    local role="$2"
    local host_worker="$3"
    local sandbox_name

    [[ -n "${satellite_id}" ]] || fatal "sandbox-create requires --satellite-id"
    sandbox_name="$(sandbox_container_name_for_satellite "${satellite_id}")"
    if [[ -z "${host_worker}" ]]; then
        host_worker="$(auto_select_sandbox_host_worker)"
    fi

    log INFO "Ensuring sandbox ${sandbox_name} for satellite ${satellite_id} role=${role} host-worker=${host_worker:-unassigned}"
    launch_sandbox "${sandbox_name}" "${satellite_id}" "${role}" "${host_worker}"
}

create_satellite_sandbox() {
    ensure_runtime_host_setup
    ensure_satellite_sandbox "${SATELLITE_ID}" "${SANDBOX_ROLE}" "${SANDBOX_HOST_WORKER}"
    refresh_runtime_inventory
    sandbox_list
}

create_satellite_sandboxes() {
    local spec satellite_id role host_worker
    local -a specs=()

    [[ -n "${SANDBOX_SPECS}" ]] || fatal "sandbox-create-many requires --sandbox-specs"
    IFS=';' read -r -a specs <<< "${SANDBOX_SPECS}"

    ensure_runtime_host_setup
    for spec in "${specs[@]}"; do
        spec="$(trim_whitespace "${spec}")"
        [[ -n "${spec}" ]] || continue
        IFS='|' read -r satellite_id role host_worker <<< "${spec}"
        satellite_id="$(trim_whitespace "${satellite_id}")"
        role="$(trim_whitespace "${role}")"
        host_worker="$(trim_whitespace "${host_worker:-}")"
        case "${role}" in
            endpoint|relay)
                ;;
            *)
                fatal "sandbox-create-many received invalid role '${role}' for satellite '${satellite_id}'"
                ;;
        esac
        ensure_satellite_sandbox "${satellite_id}" "${role}" "${host_worker}"
    done
    refresh_runtime_inventory
    sandbox_list
}

delete_satellite_sandbox() {
    local sandbox_name
    sandbox_name="$(resolve_sandbox_name)"
    sandbox_exists "${sandbox_name}" || fatal "Sandbox ${sandbox_name} does not exist"

    log INFO "Deleting satellite sandbox ${sandbox_name}"
    lxc_cmd stop "${sandbox_name}" --force >/dev/null 2>&1 || true
    lxc_cmd delete "${sandbox_name}" >/dev/null 2>&1 || true
    refresh_runtime_inventory
}

fast_sandbox_names_for_cluster() {
    local name

    load_fast_lxc_instance_cache >/dev/null 2>&1 || return 1
    for name in "${FAST_INSTANCE_NAMES[@]}"; do
        [[ -n "${name}" ]] || continue
        [[ "${FAST_INSTANCE_CLUSTER[${name}]:-}" == "${CLUSTER_NAME}" ]] || continue
        [[ "${FAST_INSTANCE_KIND[${name}]:-node}" == "sandbox" ]] || continue
        printf '%s\n' "${name}"
    done
    return 0
}

sandbox_list_fast() {
    local found=0 name

    load_fast_lxc_instance_cache >/dev/null 2>&1 || return 1

    log INFO "Satellite sandboxes for cluster ${CLUSTER_NAME}:"
    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        found=1
        printf 'sandbox=%s satellite=%s role=%s host-worker=%s state=%s apps=%s management=%s simulation=%s\n' \
            "${name}" \
            "${FAST_INSTANCE_SATELLITE_ID[${name}]:-unknown}" \
            "${FAST_INSTANCE_SANDBOX_ROLE[${name}]:-unknown}" \
            "${FAST_INSTANCE_HOST_WORKER[${name}]:-unassigned}" \
            "${FAST_INSTANCE_STATUS[${name}]:-UNKNOWN}" \
            "${FAST_INSTANCE_APP_COUNT[${name}]:-0}" \
            "${FAST_INSTANCE_MGMT_IP[${name}]:-unknown}" \
            "${FAST_INSTANCE_SIM_IP[${name}]:-unknown}"
    done < <(fast_sandbox_names_for_cluster)

    if (( found == 0 )); then
        log INFO "No satellite sandboxes found for cluster ${CLUSTER_NAME}"
    fi

    return 0
}

sandbox_list() {
    if sandbox_list_fast; then
        return 0
    fi

    local found=0 name satellite_id sandbox_role host_worker mgmt_ip sim_ip app_count state

    log INFO "Satellite sandboxes for cluster ${CLUSTER_NAME}:"
    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        found=1
        satellite_id="$(lxc_cmd config get "${name}" user.leodust.satellite-id 2>/dev/null || true)"
        sandbox_role="$(lxc_cmd config get "${name}" user.leodust.sandbox-role 2>/dev/null || true)"
        host_worker="$(lxc_cmd config get "${name}" user.leodust.host-worker 2>/dev/null || true)"
        mgmt_ip="$(lxc_cmd config get "${name}" user.leodust.management-ip 2>/dev/null || true)"
        sim_ip="$(lxc_cmd config get "${name}" user.leodust.simulation-ip 2>/dev/null || true)"
        app_count="$(lxc_cmd config get "${name}" user.leodust.app-count 2>/dev/null || true)"
        [[ -n "${app_count}" ]] || app_count="0"
        state="$(lxc_cmd list "${name}" --format csv -c s 2>/dev/null | head -n 1 || true)"
        printf 'sandbox=%s satellite=%s role=%s host-worker=%s state=%s apps=%s management=%s simulation=%s\n' \
            "${name}" \
            "${satellite_id:-unknown}" \
            "${sandbox_role:-unknown}" \
            "${host_worker:-unassigned}" \
            "${state:-UNKNOWN}" \
            "${app_count}" \
            "${mgmt_ip:-unknown}" \
            "${sim_ip:-unknown}"
    done < <(sandbox_container_names)

    if (( found == 0 )); then
        log INFO "No satellite sandboxes found for cluster ${CLUSTER_NAME}"
    fi
}

start_sandbox_application() {
    local sandbox_name app_key encoded_cmd started_at state_dir log_dir app_dir script_path pid_path log_path meta_path pid

    sandbox_name="$(resolve_sandbox_name)"
    [[ -n "${APP_NAME}" ]] || fatal "app-start requires --app-name"
    [[ -n "${APP_COMMAND}" ]] || fatal "app-start requires --app-command"
    sandbox_exists "${sandbox_name}" || fatal "Sandbox ${sandbox_name} does not exist"

    app_key="$(sandbox_app_key "${APP_NAME}")"
    encoded_cmd="$(printf '%s' "${APP_COMMAND}" | base64 | tr -d '\n')"
    state_dir="$(sandbox_app_state_dir)"
    log_dir="$(sandbox_app_log_dir)"
    app_dir="${state_dir}/${app_key}"
    script_path="${app_dir}/run.sh"
    pid_path="${app_dir}/pid"
    log_path="${log_dir}/${app_key}.log"
    meta_path="${app_dir}/meta.env"
    started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

    if sandbox_app_is_running "${sandbox_name}" "${APP_NAME}" >/dev/null 2>&1; then
        fatal "Application ${APP_NAME} is already running in ${sandbox_name}"
    fi

    log INFO "Starting application ${APP_NAME} in sandbox ${sandbox_name}"
    container_exec "${sandbox_name}" "mkdir -p '${app_dir}' '${log_dir}'
cat > '${script_path}' <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cmd_b64='${encoded_cmd}'
cmd=\"\$(printf '%s' \"\${cmd_b64}\" | base64 -d)\"
exec bash -lc \"\${cmd}\"
EOF
chmod +x '${script_path}'
nohup '${script_path}' > '${log_path}' 2>&1 &
pid=\$!
echo \"\${pid}\" > '${pid_path}'
cat > '${meta_path}' <<EOF
APP_NAME=${APP_NAME}
PID=\${pid}
STARTED_AT=${started_at}
LOG_PATH=${log_path}
EOF"
    pid="$(container_exec_capture "${sandbox_name}" "cat '${pid_path}'" | tr -d '[:space:]')"
    append_sandbox_app "${sandbox_name}" "${APP_NAME}"
    lxc_cmd config set "${sandbox_name}" user.leodust.app-count "$(csv_item_count "$(lxc_cmd config get "${sandbox_name}" user.leodust.apps 2>/dev/null || true)")"
    refresh_runtime_inventory
    log INFO "Started application ${APP_NAME} in ${sandbox_name} with pid ${pid}"
}

stop_sandbox_application() {
    local sandbox_name app_key app_dir pid_path

    sandbox_name="$(resolve_sandbox_name)"
    [[ -n "${APP_NAME}" ]] || fatal "app-stop requires --app-name"
    sandbox_exists "${sandbox_name}" || fatal "Sandbox ${sandbox_name} does not exist"

    app_key="$(sandbox_app_key "${APP_NAME}")"
    app_dir="$(sandbox_app_state_dir)/${app_key}"
    pid_path="${app_dir}/pid"

    log INFO "Stopping application ${APP_NAME} in sandbox ${sandbox_name}"
    container_exec "${sandbox_name}" "if [[ -f '${pid_path}' ]]; then pid=\$(cat '${pid_path}'); kill \"\${pid}\" >/dev/null 2>&1 || true; sleep 1; kill -9 \"\${pid}\" >/dev/null 2>&1 || true; rm -f '${pid_path}'; fi"
    remove_sandbox_app "${sandbox_name}" "${APP_NAME}"
    refresh_runtime_inventory
}

app_list_fast() {
    local found=0 sandbox_name satellite_id host_worker app_name app_state pid started_at log_path
    local inventory_file

    inventory_file="$(app_inventory_file "${CLUSTER_NAME}")"
    [[ -f "${inventory_file}" ]] || return 1

    log INFO "Managed sandbox applications for cluster ${CLUSTER_NAME}:"
    while IFS=$'\t' read -r sandbox_name satellite_id host_worker app_name app_state pid started_at log_path; do
        [[ -n "${sandbox_name}" ]] || continue
        [[ "${sandbox_name}" == "sandbox_name" ]] && continue
        found=1
        printf 'app=%s sandbox=%s satellite=%s host-worker=%s state=%s pid=%s started=%s log=%s\n' \
            "${app_name}" \
            "${sandbox_name}" \
            "${satellite_id:-unknown}" \
            "${host_worker:-unassigned}" \
            "${app_state:-unknown}" \
            "${pid:-unknown}" \
            "${started_at:-unknown}" \
            "${log_path:-unknown}"
    done < "${inventory_file}"

    if (( found == 0 )); then
        log INFO "No managed sandbox applications found for cluster ${CLUSTER_NAME}"
    fi

    return 0
}

app_list() {
    if app_list_fast; then
        return 0
    fi

    local found=0 sandbox_name app_name app_state satellite_id host_worker pid started_at log_path

    log INFO "Managed sandbox applications for cluster ${CLUSTER_NAME}:"
    while IFS= read -r sandbox_name; do
        [[ -n "${sandbox_name}" ]] || continue
        satellite_id="$(lxc_cmd config get "${sandbox_name}" user.leodust.satellite-id 2>/dev/null || true)"
        host_worker="$(lxc_cmd config get "${sandbox_name}" user.leodust.host-worker 2>/dev/null || true)"
        while IFS= read -r app_name; do
            [[ -n "${app_name}" ]] || continue
            found=1
            if sandbox_app_is_running "${sandbox_name}" "${app_name}" >/dev/null 2>&1; then
                app_state="running"
            else
                app_state="stopped"
            fi
            pid="$(sandbox_app_metadata_value "${sandbox_name}" "${app_name}" PID || true)"
            started_at="$(sandbox_app_metadata_value "${sandbox_name}" "${app_name}" STARTED_AT || true)"
            log_path="$(sandbox_app_metadata_value "${sandbox_name}" "${app_name}" LOG_PATH || true)"
            printf 'app=%s sandbox=%s satellite=%s host-worker=%s state=%s pid=%s started=%s log=%s\n' \
                "${app_name}" \
                "${sandbox_name}" \
                "${satellite_id:-unknown}" \
                "${host_worker:-unassigned}" \
                "${app_state}" \
                "${pid:-unknown}" \
                "${started_at:-unknown}" \
                "${log_path:-unknown}"
        done < <(sandbox_app_names "${sandbox_name}")
    done < <(sandbox_container_names)

    if (( found == 0 )); then
        log INFO "No managed sandbox applications found for cluster ${CLUSTER_NAME}"
    fi
}

create_control_plane() {
    log INFO "Preparing control-plane container ${CONTROL_PLANE_NAME}"
    launch_container "${CONTROL_PLANE_NAME}" "control-plane"
    ensure_microk8s_installed "${CONTROL_PLANE_NAME}"
    log INFO "Waiting for control-plane node ${CONTROL_PLANE_NAME} to register"
    wait_until "${WAIT_TIMEOUT_SECONDS}" "control-plane node registration" control_plane_has_node "${CONTROL_PLANE_NAME}"
    enable_addons
    log INFO "Applying labels and taints to control-plane node ${CONTROL_PLANE_NAME}"
    label_node "${CONTROL_PLANE_NAME}" "control-plane"
    taint_control_plane
}

add_workers() {
    local workers_to_add="$1"
    local join_cmd worker_name simulation_name i

    log INFO "Planning worker additions for cluster ${CLUSTER_NAME}"
    if [[ -z "${WORKER_NAMES_FILE}" && "${workers_to_add}" == "0" ]]; then
        log INFO "No workers requested"
        return
    fi

    select_worker_plan "${workers_to_add}"
    (( ${#SELECTED_WORKER_CONTAINER_NAMES[@]} > 0 )) || {
        log INFO "No workers remain to add"
        return
    }
    log INFO "Adding ${#SELECTED_WORKER_CONTAINER_NAMES[@]} worker containers to cluster ${CLUSTER_NAME}"
    join_cmd="$(generate_worker_join_command)"
    for (( i = 0; i < ${#SELECTED_WORKER_CONTAINER_NAMES[@]}; i++ )); do
        worker_name="${SELECTED_WORKER_CONTAINER_NAMES[$i]}"
        simulation_name="${SELECTED_WORKER_NODE_NAMES[$i]}"
        if [[ -n "${simulation_name}" ]]; then
            log INFO "Provisioning worker ${worker_name} for simulation node ${simulation_name}"
        else
            log INFO "Provisioning worker ${worker_name}"
        fi
        launch_container "${worker_name}" "worker" "${simulation_name}"
        ensure_microk8s_installed "${worker_name}"
        join_worker "${worker_name}" "${join_cmd}"
        log INFO "Labeling worker node ${worker_name} as satellite"
        label_node "${worker_name}" "satellite"
    done
    write_node_map_file
    write_cluster_inventory
    refresh_runtime_inventory
    log INFO "Worker add operation completed for cluster ${CLUSTER_NAME}"
}

create_cluster() {
    log INFO "Creating MicroK8s cluster ${CLUSTER_NAME}"
    host_setup
    create_control_plane
    add_workers "${WORKER_COUNT}"
    status_cluster
    log INFO "Cluster ${CLUSTER_NAME} creation completed"
}

run_smoke_test() {
    local namespace scheduled_node response pod_targeting_yaml=""
    namespace="$(smoke_test_namespace)"

    log INFO "Starting smoke test for cluster ${CLUSTER_NAME}"
    container_exists "${CONTROL_PLANE_NAME}" || fatal "Control-plane container ${CONTROL_PLANE_NAME} does not exist"
    control_plane_has_node "${CONTROL_PLANE_NAME}" >/dev/null 2>&1 || fatal "Control-plane node ${CONTROL_PLANE_NAME} is not registered in MicroK8s"
    if container_exec "${CONTROL_PLANE_NAME}" "microk8s kubectl get nodes -l leodust.io/role=satellite -o name | grep -q ." >/dev/null 2>&1; then
        pod_targeting_yaml=$'      nodeSelector:\n        leodust.io/role: satellite'
    fi

    log INFO "Deploying smoke test workload in namespace ${namespace}"
    container_exec "${CONTROL_PLANE_NAME}" "microk8s kubectl create namespace ${namespace} --dry-run=client -o yaml | microk8s kubectl apply -f -"
    container_exec "${CONTROL_PLANE_NAME}" "microk8s kubectl delete pod -n ${namespace} ${SMOKE_TEST_CHECK_NAME} --ignore-not-found >/dev/null 2>&1 || true"
    container_exec "${CONTROL_PLANE_NAME}" "cat <<EOF | microk8s kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${SMOKE_TEST_DEPLOYMENT_NAME}
  namespace: ${namespace}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${SMOKE_TEST_DEPLOYMENT_NAME}
  template:
    metadata:
      labels:
        app: ${SMOKE_TEST_DEPLOYMENT_NAME}
    spec:
${pod_targeting_yaml}
      containers:
        - name: web
          image: ${SMOKE_TEST_IMAGE}
          command:
            - /bin/sh
            - -c
            - mkdir -p /www && printf '%s\n' '${SMOKE_TEST_EXPECTED_BODY}' > /www/index.html && exec httpd -f -p 8080 -h /www
          ports:
            - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: ${SMOKE_TEST_SERVICE_NAME}
  namespace: ${namespace}
spec:
  selector:
    app: ${SMOKE_TEST_DEPLOYMENT_NAME}
  ports:
    - port: 8080
      targetPort: 8080
EOF"

    log INFO "Waiting for smoke test deployment rollout in namespace ${namespace}"
    container_exec "${CONTROL_PLANE_NAME}" "microk8s kubectl rollout status deployment/${SMOKE_TEST_DEPLOYMENT_NAME} -n ${namespace} --timeout=${WAIT_TIMEOUT_SECONDS}s"
    scheduled_node="$(container_exec_capture "${CONTROL_PLANE_NAME}" "microk8s kubectl get pod -n ${namespace} -l app=${SMOKE_TEST_DEPLOYMENT_NAME} -o jsonpath='{.items[0].spec.nodeName}'")"
    [[ -n "${scheduled_node}" ]] || fatal "Smoke test pod was not scheduled"

    log INFO "Running smoke test client pod from cluster ${CLUSTER_NAME}"
    container_exec "${CONTROL_PLANE_NAME}" "microk8s kubectl run ${SMOKE_TEST_CHECK_NAME} -n ${namespace} --restart=Never --image=${SMOKE_TEST_IMAGE} --command -- /bin/sh -c 'wget -qO- http://${SMOKE_TEST_SERVICE_NAME}:8080'"
    wait_until_container_exec "${CONTROL_PLANE_NAME}" "smoke test client pod completion" \
        "microk8s kubectl get pod ${SMOKE_TEST_CHECK_NAME} -n ${namespace} -o jsonpath='{.status.phase}' | grep -Fxq Succeeded"
    response="$(container_exec_capture "${CONTROL_PLANE_NAME}" "microk8s kubectl logs pod/${SMOKE_TEST_CHECK_NAME} -n ${namespace}")"
    if [[ "${response}" != *"${SMOKE_TEST_EXPECTED_BODY}"* ]]; then
        fatal "Smoke test response did not match expected body. Got: ${response}"
    fi

    log INFO "Smoke test scheduled on worker ${scheduled_node}"
    printf 'Smoke test response: %s\n' "${response}"
    container_exec "${CONTROL_PLANE_NAME}" "microk8s kubectl get pods,svc -n ${namespace} -o wide"
}

list_clusters_fast() {
    local found=0 cluster_name inventory_file container_count worker_count sandbox_count app_count
    local control_plane_display inventory_state profile_state mgmt_state sim_state profile_name
    local mgmt_network_name sim_network_name worker_base_name first_container value name kind role
    local discovered_clusters_output
    local -a cluster_instances=()

    if fast_cache_has_legacy_candidates; then
        return 1
    fi

    discovered_clusters_output="$(managed_cluster_names_fast || true)"
    [[ -n "${discovered_clusters_output}" ]] || return 1

    load_fast_lxc_instance_cache >/dev/null 2>&1 || true
    load_fast_lxc_resource_cache >/dev/null 2>&1 || true

    log INFO "Discovering managed MicroK8s clusters"
    while IFS= read -r cluster_name; do
        [[ -n "${cluster_name}" ]] || continue
        found=1
        log INFO "Inspecting cluster ${cluster_name}"

        inventory_file="$(cluster_inventory_file "${cluster_name}")"
        control_plane_display=""
        profile_name=""
        mgmt_network_name=""
        sim_network_name=""
        worker_base_name=""

        if [[ -f "${inventory_file}" ]]; then
            value="$(inventory_value "${inventory_file}" control_plane_name)"
            [[ -n "${value}" ]] && control_plane_display="${value}"
            value="$(inventory_value "${inventory_file}" worker_basename)"
            [[ -n "${value}" ]] && worker_base_name="${value}"
            value="$(inventory_value "${inventory_file}" lxd_profile_name)"
            [[ -n "${value}" ]] && profile_name="${value}"
            value="$(inventory_value "${inventory_file}" management_network_name)"
            [[ -n "${value}" ]] && mgmt_network_name="${value}"
            value="$(inventory_value "${inventory_file}" simulation_network_name)"
            [[ -n "${value}" ]] && sim_network_name="${value}"
        fi

        cluster_instances=()
        if [[ "${FAST_LXC_INSTANCE_CACHE_LOADED}" == "true" ]]; then
            for name in "${FAST_INSTANCE_NAMES[@]}"; do
                if [[ "${FAST_INSTANCE_CLUSTER[${name}]:-}" == "${cluster_name}" ]]; then
                    cluster_instances+=("${name}")
                fi
            done
        fi

        if (( ${#cluster_instances[@]} == 0 )) && [[ ! -f "${inventory_file}" ]]; then
            return 1
        fi

        container_count=0
        worker_count=0
        sandbox_count=0
        app_count=0
        first_container=""

        for name in "${cluster_instances[@]}"; do
            kind="${FAST_INSTANCE_KIND[${name}]:-node}"
            if [[ "${kind}" == "sandbox" ]]; then
                sandbox_count=$((sandbox_count + 1))
                app_count=$((app_count + ${FAST_INSTANCE_APP_COUNT[${name}]:-0}))
                continue
            fi

            container_count=$((container_count + 1))
            [[ -n "${first_container}" ]] || first_container="${name}"
            role="${FAST_INSTANCE_ROLE[${name}]:-}"
            if [[ "${role}" == "control-plane" ]]; then
                control_plane_display="${name}"
                first_container="${name}"
            else
                worker_count=$((worker_count + 1))
            fi
        done

        if [[ -n "${first_container}" ]]; then
            [[ -n "${worker_base_name}" ]] || worker_base_name="${FAST_INSTANCE_WORKER_BASE[${first_container}]:-}"
            [[ -n "${profile_name}" ]] || profile_name="${FAST_INSTANCE_PROFILE[${first_container}]:-}"
            [[ -n "${mgmt_network_name}" ]] || mgmt_network_name="${FAST_INSTANCE_MGMT_NETWORK[${first_container}]:-}"
            [[ -n "${sim_network_name}" ]] || sim_network_name="${FAST_INSTANCE_SIM_NETWORK[${first_container}]:-}"
        fi

        [[ -n "${control_plane_display}" ]] || control_plane_display="${cluster_name}-control-plane"
        [[ -n "${worker_base_name}" ]] || worker_base_name="${cluster_name}-worker"
        [[ -n "${profile_name}" ]] || profile_name="${cluster_name}-node"
        [[ -n "${mgmt_network_name}" ]] || mgmt_network_name="${cluster_name}-mgmt"
        [[ -n "${sim_network_name}" ]] || sim_network_name="${cluster_name}-sim"

        if [[ -f "${inventory_file}" ]]; then
            inventory_state="present"
        else
            inventory_state="missing"
        fi

        if [[ "${FAST_LXC_RESOURCE_CACHE_LOADED}" == "true" ]]; then
            if [[ -n "${FAST_PROFILE_EXISTS[${profile_name}]:-}" ]]; then profile_state="present"; else profile_state="missing"; fi
            if [[ -n "${FAST_NETWORK_EXISTS[${mgmt_network_name}]:-}" ]]; then mgmt_state="present"; else mgmt_state="missing"; fi
            if [[ -n "${FAST_NETWORK_EXISTS[${sim_network_name}]:-}" ]]; then sim_state="present"; else sim_state="missing"; fi
        else
            profile_state="unknown"
            mgmt_state="unknown"
            sim_state="unknown"
        fi

        printf 'cluster=%s control-plane=%s containers=%s workers=%s sandboxes=%s apps=%s profile=%s(%s) management-network=%s(%s) simulation-network=%s(%s) inventory=%s\n' \
            "${cluster_name}" \
            "${control_plane_display}" \
            "${container_count}" \
            "${worker_count}" \
            "${sandbox_count}" \
            "${app_count}" \
            "${profile_name}" \
            "${profile_state}" \
            "${mgmt_network_name}" \
            "${mgmt_state}" \
            "${sim_network_name}" \
            "${sim_state}" \
            "${inventory_state}"
    done <<< "${discovered_clusters_output}"

    if (( found == 0 )); then
        log INFO "No discovered clusters found"
    fi

    return 0
}

list_clusters() {
    if list_clusters_fast; then
        return 0
    fi

    local found=0 cluster_name inventory_file container_count worker_count sandbox_count app_count
    local control_plane_display inventory_state profile_state mgmt_state sim_state name role kind
    local script_managed="false"
    local discovered_clusters_output
    local legacy_node_names_output profile_name

    log INFO "Discovering managed MicroK8s clusters"
    discovered_clusters_output="$(managed_cluster_names || true)"
    if [[ -n "${discovered_clusters_output}" ]]; then
        while IFS= read -r cluster_name; do
            [[ -n "${cluster_name}" ]] || continue
            found=1
            log INFO "Inspecting cluster ${cluster_name}"

            inventory_file="$(cluster_inventory_file "${cluster_name}")"
            if [[ ! -f "${inventory_file}" ]] && cluster_is_legacy_control_plane_name "${cluster_name}"; then
                container_count=0
                worker_count=0
                sandbox_count=0
                app_count=0
                control_plane_display="${cluster_name}"
                legacy_node_names_output="$(microk8s_cluster_node_names "${cluster_name}" 2>/dev/null || true)"
                if [[ -n "${legacy_node_names_output}" ]]; then
                    while IFS= read -r name; do
                        [[ -n "${name}" ]] || continue
                        container_count=$((container_count + 1))
                        if [[ "${name}" != "${cluster_name}" ]]; then
                            worker_count=$((worker_count + 1))
                        fi
                    done <<< "${legacy_node_names_output}"
                fi
                if (( container_count == 0 )); then
                    container_count=1
                fi
                profile_name="$(container_primary_profile "${cluster_name}" || true)"
                [[ -n "${profile_name}" ]] || profile_name="${LXD_PROFILE_NAME}"
                printf 'cluster=%s control-plane=%s containers=%s workers=%s sandboxes=%s apps=%s profile=%s(%s) management-network=%s(%s) simulation-network=%s(%s) inventory=%s\n' \
                    "${cluster_name}" \
                    "${control_plane_display}" \
                    "${container_count}" \
                    "${worker_count}" \
                    "${sandbox_count}" \
                    "${app_count}" \
                    "${profile_name}" \
                    "unmanaged" \
                    "${cluster_name}-mgmt" \
                    "unmanaged" \
                    "${cluster_name}-sim" \
                    "unmanaged" \
                    "legacy"
                continue
            fi

            setup_cluster_context "${cluster_name}"
            if cluster_is_script_managed; then
                script_managed="true"
            else
                script_managed="false"
            fi

            container_count=0
            worker_count=0
            sandbox_count=0
            app_count=0
            control_plane_display="${CONTROL_PLANE_NAME}"

            while IFS= read -r name; do
                [[ -n "${name}" ]] || continue
                kind="$(container_kind "${name}")"
                if [[ "${kind}" == "sandbox" ]]; then
                    sandbox_count=$((sandbox_count + 1))
                    app_count=$((app_count + $(csv_item_count "$(lxc_cmd config get "${name}" user.leodust.apps 2>/dev/null || true)")))
                    continue
                fi
                container_count=$((container_count + 1))
                role="$(lxc_cmd config get "${name}" user.leodust.role 2>/dev/null || true)"
                if [[ "${role}" == "worker" ]]; then
                    worker_count=$((worker_count + 1))
                elif [[ "${role}" == "control-plane" || "${name}" == "${CONTROL_PLANE_NAME}" ]]; then
                    control_plane_display="${name}"
                else
                    worker_count=$((worker_count + 1))
                fi
            done < <(cluster_container_names)

            if [[ "${script_managed}" == "false" ]]; then
                inventory_state="legacy"
                profile_state="unmanaged"
                mgmt_state="unmanaged"
                sim_state="unmanaged"
            elif [[ -f "${inventory_file}" ]]; then
                inventory_state="present"
            else
                inventory_state="missing"
            fi

            if [[ "${script_managed}" == "true" ]]; then
                if binary_available lxc && lxc_cmd list --format csv -c n >/dev/null 2>&1; then
                    if profile_exists "${LXD_PROFILE_NAME}"; then profile_state="present"; else profile_state="missing"; fi
                    if network_exists "${MGMT_NETWORK_NAME}"; then mgmt_state="present"; else mgmt_state="missing"; fi
                    if network_exists "${SIM_NETWORK_NAME}"; then sim_state="present"; else sim_state="missing"; fi
                else
                    profile_state="unknown"
                    mgmt_state="unknown"
                    sim_state="unknown"
                fi
            fi

            printf 'cluster=%s control-plane=%s containers=%s workers=%s sandboxes=%s apps=%s profile=%s(%s) management-network=%s(%s) simulation-network=%s(%s) inventory=%s\n' \
                "${cluster_name}" \
                "${control_plane_display}" \
                "${container_count}" \
                "${worker_count}" \
                "${sandbox_count}" \
                "${app_count}" \
                "${LXD_PROFILE_NAME}" \
                "${profile_state}" \
                "${MGMT_NETWORK_NAME}" \
                "${mgmt_state}" \
                "${SIM_NETWORK_NAME}" \
                "${sim_state}" \
                "${inventory_state}"
        done <<< "${discovered_clusters_output}"
    fi

    if (( found == 0 )); then
        log INFO "No discovered clusters found"
    fi

    return 0
}

status_cluster_fast() {
    local have_node_containers=0
    local name role simulation_name

    load_fast_lxc_instance_cache >/dev/null 2>&1 || return 1

    log INFO "Collecting status for cluster ${CLUSTER_NAME}"
    if ! binary_available lxc; then
        log INFO "LXD is not installed on this host"
        return 0
    fi

    log INFO "LXD node containers for cluster ${CLUSTER_NAME}:"
    for name in "${FAST_INSTANCE_NAMES[@]}"; do
        [[ -n "${name}" ]] || continue
        if [[ "${FAST_INSTANCE_CLUSTER[${name}]:-}" != "${CLUSTER_NAME}" && "${name}" != "${CONTROL_PLANE_NAME}" && "${name}" != "${WORKER_BASENAME}-"* ]] && ! worker_name_is_declared "${name}"; then
            continue
        fi
        if [[ "${FAST_INSTANCE_KIND[${name}]:-node}" == "sandbox" ]]; then
            continue
        fi
        have_node_containers=1
        role="${FAST_INSTANCE_ROLE[${name}]:-}"
        if [[ -z "${role}" ]]; then
            if [[ "${name}" == "${CONTROL_PLANE_NAME}" ]]; then
                role="control-plane"
            else
                role="worker"
            fi
        fi
        simulation_name="${FAST_INSTANCE_SIMULATION_NAME[${name}]:-}"
        printf '  - %s role=%s simulation-name=%s management=%s simulation=%s\n' \
            "${name}" \
            "${role:-unknown}" \
            "${simulation_name:-none}" \
            "${FAST_INSTANCE_MGMT_IP[${name}]:-unknown}" \
            "${FAST_INSTANCE_SIM_IP[${name}]:-unknown}"
    done

    if (( have_node_containers == 0 )); then
        if ! cluster_has_inventory && ! cluster_has_container_metadata; then
            return 1
        fi
        log INFO "No node containers found for cluster ${CLUSTER_NAME}"
    fi

    if (( have_node_containers > 0 )); then
        if fast_instance_exists "${CONTROL_PLANE_NAME}" || container_exists "${CONTROL_PLANE_NAME}"; then
            log INFO "MicroK8s nodes reported by ${CONTROL_PLANE_NAME}:"
            container_exec "${CONTROL_PLANE_NAME}" "microk8s kubectl get nodes -o wide"
        fi
    fi

    sandbox_list_fast || sandbox_list
    app_list_fast || app_list

    return 0
}

status_cluster() {
    if status_cluster_fast; then
        return 0
    fi

    local have_node_containers=0
    local name role simulation_name mgmt_ip sim_ip
    local cluster_names_output

    log INFO "Collecting status for cluster ${CLUSTER_NAME}"
    if ! binary_available lxc; then
        log INFO "LXD is not installed on this host"
        return 0
    fi
    log INFO "LXD node containers for cluster ${CLUSTER_NAME}:"
    cluster_names_output="$(cluster_node_container_names || true)"
    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        have_node_containers=1
        role="$(lxc_cmd config get "${name}" user.leodust.role 2>/dev/null || true)"
        if [[ -z "${role}" ]]; then
            if [[ "${name}" == "${CONTROL_PLANE_NAME}" ]]; then
                role="control-plane"
            else
                role="worker"
            fi
        fi
        simulation_name="$(lxc_cmd config get "${name}" user.leodust.simulation-name 2>/dev/null || true)"
        mgmt_ip="$(lxc_cmd config get "${name}" user.leodust.management-ip 2>/dev/null || true)"
        sim_ip="$(lxc_cmd config get "${name}" user.leodust.simulation-ip 2>/dev/null || true)"
        printf '  - %s role=%s simulation-name=%s management=%s simulation=%s\n' "${name}" "${role:-unknown}" "${simulation_name:-none}" "${mgmt_ip:-unknown}" "${sim_ip:-unknown}"
    done <<< "${cluster_names_output}"

    if (( have_node_containers == 0 )); then
        log INFO "No node containers found for cluster ${CLUSTER_NAME}"
    fi

    if (( have_node_containers > 0 )) && container_exists "${CONTROL_PLANE_NAME}"; then
        log INFO "MicroK8s nodes reported by ${CONTROL_PLANE_NAME}:"
        container_exec "${CONTROL_PLANE_NAME}" "microk8s kubectl get nodes -o wide"
    fi

    sandbox_list
    app_list

    return 0
}

control_plane_management_ip() {
    local mgmt_ip=""

    mgmt_ip="$(lxc_cmd config get "${CONTROL_PLANE_NAME}" user.leodust.management-ip 2>/dev/null || true)"
    if [[ -z "${mgmt_ip}" ]]; then
        mgmt_ip="$(container_ipv4 "${CONTROL_PLANE_NAME}" "${MGMT_INTERFACE_NAME}" || true)"
        if [[ -n "${mgmt_ip}" ]]; then
            lxc_cmd config set "${CONTROL_PLANE_NAME}" user.leodust.management-ip "${mgmt_ip}" >/dev/null 2>&1 || true
        fi
    fi

    [[ -n "${mgmt_ip}" ]] || fatal "Unable to determine management IP for control-plane ${CONTROL_PLANE_NAME}"
    printf '%s\n' "${mgmt_ip}"
}

export_kubeconfig() {
    local mgmt_ip raw_config rewritten_config server_endpoint output_file output_dir

    log INFO "Exporting kubeconfig for cluster ${CLUSTER_NAME}"
    container_exists "${CONTROL_PLANE_NAME}" || fatal "Control-plane container ${CONTROL_PLANE_NAME} does not exist"
    control_plane_has_node "${CONTROL_PLANE_NAME}" >/dev/null 2>&1 || fatal "Control-plane node ${CONTROL_PLANE_NAME} is not registered in MicroK8s"

    mgmt_ip="$(control_plane_management_ip)"
    server_endpoint="https://${mgmt_ip}:16443"
    raw_config="$(container_exec_capture_direct "${CONTROL_PLANE_NAME}" microk8s config)"
    [[ -n "${raw_config}" ]] || fatal "Failed to read kubeconfig from ${CONTROL_PLANE_NAME}"

    rewritten_config="$(printf '%s\n' "${raw_config}" | awk -v server="${server_endpoint}" '
        BEGIN { replaced=0 }
        /^[[:space:]]*server:[[:space:]]*https:\/\// && replaced == 0 {
            sub(/https:\/\/[^[:space:]]+/, server)
            replaced=1
        }
        { print }
    ')"

    output_file="${KUBECONFIG_OUTPUT_FILE}"
    [[ -n "${output_file}" ]] || output_file="${CLUSTER_RESULTS_DIR}/${CLUSTER_NAME}-kubeconfig.yaml"
    if [[ "${output_file}" == "-" ]]; then
        printf '%s\n' "${rewritten_config}"
        return 0
    fi

    output_dir="$(dirname "${output_file}")"
    mkdir -p "${output_dir}"
    printf '%s\n' "${rewritten_config}" > "${output_file}"
    log INFO "Wrote kubeconfig to ${output_file}"
    log INFO "Use it with: kubectl --kubeconfig ${output_file} get nodes"
}

destroy_cluster() {
    local name deleted=0
    local cluster_names_output

    log INFO "Destroying cluster ${CLUSTER_NAME}"
    log INFO "Resolving containers for cluster ${CLUSTER_NAME}"
    cluster_names_output="$(cluster_container_names || true)"
    while IFS= read -r name; do
        [[ -n "${name}" ]] || continue
        log INFO "Deleting container ${name}"
        lxc_cmd stop "${name}" --force >/dev/null 2>&1 || true
        lxc_cmd delete "${name}" >/dev/null 2>&1 || true
        deleted=1
    done <<< "${cluster_names_output}"

    if (( deleted == 0 )); then
        log INFO "No containers found for cluster ${CLUSTER_NAME}"
    else
        log INFO "Cluster ${CLUSTER_NAME} containers removed"
    fi
    refresh_runtime_inventory
}

uninstall_cluster() {
    local script_managed="false"

    log INFO "Uninstalling cluster ${CLUSTER_NAME}"
    if cluster_is_script_managed; then
        script_managed="true"
    fi

    destroy_cluster
    cleanup_node_map_file
    cleanup_cluster_inventory_file
    cleanup_runtime_inventory_files

    if [[ "${script_managed}" != "true" ]]; then
        log INFO "Cluster ${CLUSTER_NAME} is legacy-discovered; skipping profile and network removal because they are not script-managed"
        return
    fi

    if profile_exists "${LXD_PROFILE_NAME}"; then
        log INFO "Deleting LXD profile ${LXD_PROFILE_NAME}"
        lxc_cmd profile delete "${LXD_PROFILE_NAME}"
    fi

    delete_cached_image_if_present "${SANDBOX_IMAGE_ALIAS}"
    delete_cached_image_if_present "${NODE_IMAGE_ALIAS}"

    if network_exists "${SIM_NETWORK_NAME}"; then
        log INFO "Deleting simulation network ${SIM_NETWORK_NAME}"
        lxc_cmd network delete "${SIM_NETWORK_NAME}"
    fi
    if network_exists "${MGMT_NETWORK_NAME}"; then
        log INFO "Deleting management network ${MGMT_NETWORK_NAME}"
        lxc_cmd network delete "${MGMT_NETWORK_NAME}"
    fi

    if [[ "${REMOVE_LXD}" == "true" && "${ALL_CLUSTERS}" != "true" ]]; then
        if command -v snap >/dev/null 2>&1; then
            log INFO "Removing LXD snap"
            run_privileged snap remove lxd
        fi
    fi
}

run_for_all_clusters() {
    local action="$1"
    local cluster_name found=0
    local discovered_clusters_output

    log INFO "Resolving all discovered clusters for command ${COMMAND}"
    discovered_clusters_output="$(managed_cluster_names || true)"
    if [[ -n "${discovered_clusters_output}" ]]; then
        while IFS= read -r cluster_name; do
            [[ -n "${cluster_name}" ]] || continue
            found=1
            setup_cluster_context "${cluster_name}"
            log INFO "Running ${COMMAND} for cluster ${CLUSTER_NAME}"
            "${action}"
        done <<< "${discovered_clusters_output}"
    fi

    if (( found == 0 )); then
        log INFO "No discovered clusters found"
        return 0
    fi

    if [[ "${COMMAND}" == "uninstall" && "${REMOVE_LXD}" == "true" ]]; then
        if command -v snap >/dev/null 2>&1; then
            log INFO "Removing LXD snap after all cluster uninstall operations"
            run_privileged snap remove lxd
        fi
    fi
}

main() {
    discover_config_file "$@"
    load_config_file
    parse_args "$@"

    log INFO "Running command ${COMMAND} for requested cluster ${CLUSTER_NAME}"
    debug "Effective configuration: cluster=${CLUSTER_NAME} control-plane=${CONTROL_PLANE_NAME} worker-base=${WORKER_BASENAME} workers=${WORKER_COUNT} image=${IMAGE} all-clusters=${ALL_CLUSTERS}"
    require_host_tools

    if [[ "${ALL_CLUSTERS}" == "true" ]]; then
        case "${COMMAND}" in
            list|status|destroy|uninstall|test|sandbox-list|app-list)
                ;;
            *)
                fatal "--all-clusters is only supported with list, status, destroy, uninstall, test, sandbox-list, and app-list"
                ;;
        esac
    fi

    case "${COMMAND}" in
        install|host-setup)
            host_setup
            ;;
        create)
            require_lxd_runtime
            load_worker_names
            create_cluster
            ;;
        add-workers)
            require_lxd_runtime
            load_worker_names
            host_setup
            create_control_plane
            add_workers "${WORKER_COUNT}"
            status_cluster
            ;;
        sandbox-create)
            require_lxd_runtime
            sync_current_cluster_context
            create_satellite_sandbox
            ;;
        sandbox-create-many)
            require_lxd_runtime
            sync_current_cluster_context
            create_satellite_sandboxes
            ;;
        sandbox-delete)
            require_lxd_runtime
            sync_current_cluster_context
            delete_satellite_sandbox
            ;;
        sandbox-list)
            require_lxd_runtime
            if [[ "${ALL_CLUSTERS}" == "true" ]]; then
                run_for_all_clusters sandbox_list
            else
                sync_current_cluster_context
                sandbox_list
            fi
            ;;
        app-start)
            require_lxd_runtime
            sync_current_cluster_context
            start_sandbox_application
            ;;
        app-stop)
            require_lxd_runtime
            sync_current_cluster_context
            stop_sandbox_application
            ;;
        app-list)
            require_lxd_runtime
            if [[ "${ALL_CLUSTERS}" == "true" ]]; then
                run_for_all_clusters app_list
            else
                sync_current_cluster_context
                app_list
            fi
            ;;
        kubeconfig)
            require_lxd_runtime
            sync_current_cluster_context
            export_kubeconfig
            ;;
        list)
            require_lxd_runtime
            list_clusters
            ;;
        test)
            require_lxd_runtime
            if [[ "${ALL_CLUSTERS}" == "true" ]]; then
                run_for_all_clusters run_smoke_test
            else
                sync_current_cluster_context
                run_smoke_test
            fi
            ;;
        status)
            require_lxd_runtime
            if [[ "${ALL_CLUSTERS}" == "true" ]]; then
                run_for_all_clusters status_cluster
            else
                sync_current_cluster_context
                status_cluster
            fi
            ;;
        destroy)
            require_lxd_runtime
            if [[ "${ALL_CLUSTERS}" == "true" ]]; then
                run_for_all_clusters destroy_cluster
            else
                sync_current_cluster_context
                destroy_cluster
            fi
            ;;
        uninstall)
            require_lxd_runtime
            if [[ "${ALL_CLUSTERS}" == "true" ]]; then
                run_for_all_clusters uninstall_cluster
            else
                sync_current_cluster_context
                uninstall_cluster
            fi
            ;;
        *)
            usage
            fatal "Unknown command: ${COMMAND}"
            ;;
    esac
}

main "$@"
