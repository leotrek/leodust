package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	runtimeFile := flag.String(
		"runtimeFile",
		DefaultRuntimeFile,
		"Path to the live runtime snapshot exported by RuntimeReconcilePlugin",
	)
	clusterName := flag.String(
		"clusterName",
		"leodust",
		"Cluster name used by scripts/microk8s_cluster.sh and sandbox metadata",
	)
	clusterScript := flag.String(
		"clusterScript",
		"",
		"Path to scripts/microk8s_cluster.sh; defaults to ./scripts/microk8s_cluster.sh or ../scripts/microk8s_cluster.sh",
	)
	device := flag.String(
		"device",
		"eth1",
		"Simulation interface inside each sandbox to program",
	)
	pluginsFlag := flag.String(
		"plugins",
		"sandboxes,links",
		"Comma-separated controller plugins to enable: sandboxes, links, or none",
	)
	pollInterval := flag.Duration(
		"pollInterval",
		2*time.Second,
		"Polling interval used to watch the runtime snapshot file",
	)
	once := flag.Bool(
		"once",
		false,
		"Apply the latest runtime snapshot once and exit instead of polling for changes",
	)
	pruneSandboxes := flag.Bool(
		"pruneSandboxes",
		true,
		"Delete controller-managed sandboxes that are no longer active in the latest runtime snapshot",
	)
	dryRun := flag.Bool(
		"dryRun",
		true,
		"Render controller actions without executing cluster script or LXC commands",
	)
	logLevel := flag.String(
		"logLevel",
		"info",
		"Log level: error, warn, info, debug",
	)
	flag.Parse()

	if err := configureLogLevel(*logLevel); err != nil {
		fatalf("Failed to configure log level: %v", err)
	}

	scriptPath, err := resolveClusterScript(*clusterScript)
	if err != nil {
		fatalf("Failed to resolve cluster script: %v", err)
	}

	pluginNames, err := parsePluginNames(*pluginsFlag)
	if err != nil {
		fatalf("Failed to parse plugins: %v", err)
	}
	plugins, err := buildPlugins(pluginNames)
	if err != nil {
		fatalf("Failed to build plugins: %v", err)
	}

	env := Environment{
		ClusterName:     strings.TrimSpace(*clusterName),
		ClusterScript:   scriptPath,
		Device:          strings.TrimSpace(*device),
		DryRun:          *dryRun,
		PruneSandboxes:  *pruneSandboxes,
		Runtime:         NewLXCRuntime(),
		ManagedMetadata: "user.leodust.runtime-controller",
	}

	infof(
		"Runtime controller started for %s with plugins=%s on cluster=%s device=%s (dryRun=%t)",
		*runtimeFile,
		strings.Join(pluginNames, ","),
		env.ClusterName,
		env.Device,
		env.DryRun,
	)

	if *once {
		status, err := applyLatestSnapshot(*runtimeFile, plugins, env, "")
		if err != nil {
			fatalOnSnapshotError(*runtimeFile, err)
		}
		if !status.Loaded {
			fatalf("Runtime snapshot %s was not loaded", *runtimeFile)
		}
		infof("Runtime controller processed snapshot once and is stopping")
		return
	}

	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	var lastSnapshotID string
	for {
		status, err := applyLatestSnapshot(*runtimeFile, plugins, env, lastSnapshotID)
		if err != nil {
			logSnapshotError(*runtimeFile, err)
		} else {
			lastSnapshotID = status.LastSnapshotID
		}
		select {
		case <-stop:
			infof("Runtime controller stopping")
			return
		case <-ticker.C:
		}
	}
}

type snapshotApplyStatus struct {
	LastSnapshotID  string
	Loaded          bool
	SnapshotChanged bool
}

func applyLatestSnapshot(path string, plugins []Plugin, env Environment, lastSnapshotID string) (snapshotApplyStatus, error) {
	status := snapshotApplyStatus{LastSnapshotID: lastSnapshotID}
	snapshot, err := LoadSnapshot(path)
	if err != nil {
		return status, err
	}
	status.Loaded = true

	snapshotID := fmt.Sprintf("%d|%s", snapshot.Version, snapshot.Time.UTC().Format(time.RFC3339Nano))
	if snapshotID == lastSnapshotID {
		return status, nil
	}
	status.SnapshotChanged = true

	desired, err := BuildDesiredState(snapshot, env.ClusterName)
	if err != nil {
		return status, fmt.Errorf("build desired runtime state for snapshot version %d: %w", snapshot.Version, err)
	}
	if message := runtimeIdleMessage(snapshot, desired); message != "" {
		infof("%s", message)
	}

	for _, plugin := range plugins {
		result, err := plugin.Reconcile(snapshot, desired, env)
		if err != nil {
			return status, fmt.Errorf("plugin %s failed for runtime snapshot version %d: %w", plugin.Name(), snapshot.Version, err)
		}
		infof(
			"Plugin %s processed runtime snapshot version %d; changed=%d %s",
			plugin.Name(),
			snapshot.Version,
			result.Changed,
			result.Summary,
		)
	}

	status.LastSnapshotID = snapshotID
	return status, nil
}

func runtimeIdleMessage(snapshot Snapshot, desired DesiredState) string {
	switch {
	case len(snapshot.Workloads) == 0 && len(snapshot.Routes) == 0 && len(desired.Nodes) == 0 && len(desired.Edges) == 0:
		return fmt.Sprintf(
			"Runtime snapshot version %d has no hosted workloads or routes; the controller has nothing to materialize and will remain idle until the snapshot changes. Place simulator services first or use --injectTestWorkloads on leodust for testing.",
			snapshot.Version,
		)
	case len(snapshot.Workloads) > 0 && len(snapshot.Routes) == 0 && len(desired.Edges) == 0:
		return fmt.Sprintf(
			"Runtime snapshot version %d has %d hosted workload(s) but no inter-host routes; endpoint sandboxes can still be materialized, but link projection will stay idle until at least two workload-hosting nodes are active.",
			snapshot.Version,
			len(snapshot.Workloads),
		)
	default:
		return ""
	}
}

func logSnapshotError(path string, err error) {
	if errors.Is(err, os.ErrNotExist) {
		debugf("Runtime snapshot %s does not exist yet", path)
		return
	}
	warnf("Failed to process runtime snapshot %s: %v", path, err)
}

func fatalOnSnapshotError(path string, err error) {
	if errors.Is(err, os.ErrNotExist) {
		fatalf("Runtime snapshot %s does not exist yet", path)
	}
	fatalf("Failed to process runtime snapshot %s: %v", path, err)
}

func parsePluginNames(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "none") {
		return []string{}, nil
	}

	names := make([]string, 0)
	seen := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no plugins configured")
	}
	return names, nil
}

func resolveClusterScript(value string) (string, error) {
	candidates := make([]string, 0, 3)
	if strings.TrimSpace(value) != "" {
		candidates = append(candidates, strings.TrimSpace(value))
	} else {
		candidates = append(candidates, "./scripts/microk8s_cluster.sh", "../scripts/microk8s_cluster.sh")
	}

	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			absPath, err := filepath.Abs(candidate)
			if err != nil {
				return candidate, nil
			}
			return absPath, nil
		}
	}
	return "", fmt.Errorf("could not find microk8s_cluster.sh; tried %s", strings.Join(candidates, ", "))
}
