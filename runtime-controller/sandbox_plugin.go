package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type SandboxPlugin struct{}

func (SandboxPlugin) Name() string {
	return "sandboxes"
}

type sandboxSpec struct {
	SatelliteID string
	Role        string
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
	specs := make([]sandboxSpec, 0)
	managedNodes := make([]DesiredNode, 0)
	for _, name := range names {
		node := desired.Nodes[name]
		live, exists := bySatellite[name]
		needsCreate := !exists
		needsRoleUpdate := exists && live.Role != node.SandboxRole()

		if needsCreate || needsRoleUpdate {
			infof("Sandbox plugin ensuring %s as %s", name, node.SandboxRole())
			if !env.DryRun {
				specs = append(specs, sandboxSpec{SatelliteID: name, Role: node.SandboxRole()})
				managedNodes = append(managedNodes, node)
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

	if !env.DryRun && len(specs) > 0 {
		encodedSpecs, err := encodeSandboxSpecs(specs)
		if err != nil {
			return PluginResult{}, err
		}
		if err := execClusterScript(
			env.ClusterScript,
			"sandbox-create-many",
			"--cluster-name", env.ClusterName,
			"--sandbox-specs", encodedSpecs,
		); err != nil {
			return PluginResult{}, err
		}
		for _, node := range managedNodes {
			if err := env.Runtime.SetConfig(node.ContainerName, env.ManagedMetadata, "true"); err != nil {
				return PluginResult{}, err
			}
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

func encodeSandboxSpecs(specs []sandboxSpec) (string, error) {
	parts := make([]string, 0, len(specs))
	for _, spec := range specs {
		satelliteID := strings.TrimSpace(spec.SatelliteID)
		role := strings.TrimSpace(spec.Role)
		if satelliteID == "" {
			return "", fmt.Errorf("sandbox spec satellite ID cannot be empty")
		}
		if role == "" {
			return "", fmt.Errorf("sandbox spec role cannot be empty for %s", satelliteID)
		}
		if strings.ContainsAny(satelliteID, ";|") {
			return "", fmt.Errorf("sandbox spec satellite ID %q cannot contain ';' or '|'", satelliteID)
		}
		if strings.ContainsAny(role, ";|") {
			return "", fmt.Errorf("sandbox spec role %q cannot contain ';' or '|'", role)
		}
		parts = append(parts, satelliteID+"|"+role)
	}
	return strings.Join(parts, ";"), nil
}

func execClusterScript(path string, args ...string) error {
	cmdArgs := append([]string{path}, args...)
	cmd := exec.Command("bash", cmdArgs...)

	var output bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &output)
	cmd.Stderr = io.MultiWriter(os.Stderr, &output)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("cluster script failed: %w: %s", err, strings.TrimSpace(output.String()))
	}
	debugf("Cluster script %s %s", path, strings.Join(args, " "))
	return nil
}
