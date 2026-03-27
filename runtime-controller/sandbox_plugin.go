package main

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type SandboxPlugin struct{}

func (SandboxPlugin) Name() string {
	return "sandboxes"
}

func (SandboxPlugin) Reconcile(_ Snapshot, desired DesiredState, env Environment) (PluginResult, error) {
	liveSandboxes, err := env.Runtime.ListSandboxes(env.ClusterName)
	if err != nil {
		return PluginResult{}, err
	}

	bySatellite := make(map[string]SandboxInfo, len(liveSandboxes))
	for _, sandbox := range liveSandboxes {
		if strings.TrimSpace(sandbox.SatelliteID) == "" {
			continue
		}
		bySatellite[sandbox.SatelliteID] = sandbox
	}

	names := make([]string, 0, len(desired.Nodes))
	for name := range desired.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)

	changed := 0
	for _, name := range names {
		node := desired.Nodes[name]
		live, exists := bySatellite[name]
		needsCreate := !exists
		needsRoleUpdate := exists && live.Role != node.SandboxRole()

		if needsCreate || needsRoleUpdate {
			infof("Sandbox plugin ensuring %s as %s", name, node.SandboxRole())
			if !env.DryRun {
				if err := execClusterScript(
					env.ClusterScript,
					"sandbox-create",
					"--cluster-name", env.ClusterName,
					"--satellite-id", name,
					"--sandbox-role", node.SandboxRole(),
				); err != nil {
					return PluginResult{}, err
				}
				if err := env.Runtime.SetConfig(node.ContainerName, env.ManagedMetadata, "true"); err != nil {
					return PluginResult{}, err
				}
			}
			changed++
			continue
		}

		if !live.Managed && !env.DryRun {
			if err := env.Runtime.SetConfig(live.ContainerName, env.ManagedMetadata, "true"); err != nil {
				return PluginResult{}, err
			}
			changed++
		}
	}

	if env.PruneSandboxes {
		for _, sandbox := range liveSandboxes {
			if !sandbox.Managed {
				continue
			}
			if _, keep := desired.Nodes[sandbox.SatelliteID]; keep {
				continue
			}
			infof("Sandbox plugin deleting stale sandbox %s for %s", sandbox.ContainerName, sandbox.SatelliteID)
			if !env.DryRun {
				if err := execClusterScript(
					env.ClusterScript,
					"sandbox-delete",
					"--cluster-name", env.ClusterName,
					"--satellite-id", sandbox.SatelliteID,
				); err != nil {
					return PluginResult{}, err
				}
			}
			changed++
		}
	}

	return PluginResult{
		Changed: changed,
		Summary: fmt.Sprintf("active sandboxes=%d", len(desired.Nodes)),
	}, nil
}

func execClusterScript(path string, args ...string) error {
	cmdArgs := append([]string{path}, args...)
	cmd := exec.Command("bash", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cluster script failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	debugf("Cluster script %s %s", path, strings.Join(args, " "))
	return nil
}
