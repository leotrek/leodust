package main

import "fmt"

type Environment struct {
	ClusterName     string
	ClusterScript   string
	Device          string
	DryRun          bool
	PruneSandboxes  bool
	Runtime         ContainerRuntime
	ManagedMetadata string
}

type Plugin interface {
	Name() string
	Reconcile(snapshot Snapshot, desired DesiredState, env Environment) (PluginResult, error)
}

type PluginResult struct {
	Changed int
	Summary string
}

func buildPlugins(names []string) ([]Plugin, error) {
	plugins := make([]Plugin, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		switch name {
		case "sandboxes":
			if !seen[name] {
				plugins = append(plugins, SandboxPlugin{})
				seen[name] = true
			}
		case "links":
			if !seen[name] {
				plugins = append(plugins, NewLinksPlugin())
				seen[name] = true
			}
		default:
			return nil, fmt.Errorf("unsupported plugin %q", name)
		}
	}
	return plugins, nil
}
