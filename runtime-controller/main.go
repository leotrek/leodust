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

	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	var lastSnapshotID string
	for {
		lastSnapshotID = applyLatestSnapshot(*runtimeFile, plugins, env, lastSnapshotID)
		select {
		case <-stop:
			infof("Runtime controller stopping")
			return
		case <-ticker.C:
		}
	}
}

func applyLatestSnapshot(path string, plugins []Plugin, env Environment, lastSnapshotID string) string {
	snapshot, err := LoadSnapshot(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			debugf("Runtime snapshot %s does not exist yet", path)
			return lastSnapshotID
		}
		warnf("Failed to load runtime snapshot: %v", err)
		return lastSnapshotID
	}

	snapshotID := fmt.Sprintf("%d|%s", snapshot.Version, snapshot.Time.UTC().Format(time.RFC3339Nano))
	if snapshotID == lastSnapshotID {
		return lastSnapshotID
	}

	desired, err := BuildDesiredState(snapshot, env.ClusterName)
	if err != nil {
		warnf("Failed to build desired runtime state for snapshot version %d: %v", snapshot.Version, err)
		return lastSnapshotID
	}

	for _, plugin := range plugins {
		result, err := plugin.Reconcile(snapshot, desired, env)
		if err != nil {
			warnf("Plugin %s failed for runtime snapshot version %d: %v", plugin.Name(), snapshot.Version, err)
			return lastSnapshotID
		}
		infof(
			"Plugin %s processed runtime snapshot version %d; changed=%d %s",
			plugin.Name(),
			snapshot.Version,
			result.Changed,
			result.Summary,
		)
	}

	return snapshotID
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
