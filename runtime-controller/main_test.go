package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

type mainTestPlugin struct {
	calls int
}

func (p *mainTestPlugin) Name() string {
	return "test"
}

func (p *mainTestPlugin) Reconcile(Snapshot, DesiredState, Environment) (PluginResult, error) {
	p.calls++
	return PluginResult{Changed: 1, Summary: "ok"}, nil
}

type mainTestRuntime struct{}

func (mainTestRuntime) ListSandboxes(string) ([]SandboxInfo, error) { return nil, nil }
func (mainTestRuntime) ContainerIPv4(string, string) (string, error) {
	return "", nil
}
func (mainTestRuntime) ApplyScript(string, string) error { return nil }
func (mainTestRuntime) SetConfig(string, string, string) error {
	return nil
}

func TestApplyLatestSnapshotProcessesNewSnapshotOnlyOnce(t *testing.T) {
	path := t.TempDir() + "/runtime.json"
	snapshot := Snapshot{
		Version: 1,
		Time:    time.Date(2026, time.March, 28, 12, 0, 0, 0, time.UTC),
		Nodes: []SnapshotNode{
			{Name: "sat-a", Kind: "satellite"},
			{Name: "sat-b", Kind: "satellite"},
		},
		Links: []SnapshotLink{
			{Source: "sat-a", Target: "sat-b", LatencyMs: 1, BandwidthBps: 100},
		},
		Workloads: []SnapshotWorkload{
			{Name: "src", HostNode: "sat-a"},
			{Name: "dst", HostNode: "sat-b"},
		},
		Routes: []SnapshotRoute{
			{SourceNode: "sat-a", TargetNode: "sat-b", Hops: []string{"sat-a", "sat-b"}, LatencyMs: 1},
		},
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile returned error: %v", err)
	}

	plugin := &mainTestPlugin{}
	env := Environment{ClusterName: "leodust", Runtime: mainTestRuntime{}}

	status, err := applyLatestSnapshot(path, []Plugin{plugin}, env, "")
	if err != nil {
		t.Fatalf("applyLatestSnapshot returned error: %v", err)
	}
	if !status.Loaded || !status.SnapshotChanged {
		t.Fatalf("unexpected status after first apply: %+v", status)
	}
	if status.LastSnapshotID == "" {
		t.Fatal("LastSnapshotID should be set after a successful apply")
	}
	if plugin.calls != 1 {
		t.Fatalf("plugin calls = %d, want 1", plugin.calls)
	}

	status, err = applyLatestSnapshot(path, []Plugin{plugin}, env, status.LastSnapshotID)
	if err != nil {
		t.Fatalf("second applyLatestSnapshot returned error: %v", err)
	}
	if status.SnapshotChanged {
		t.Fatalf("SnapshotChanged = true on unchanged snapshot: %+v", status)
	}
	if plugin.calls != 1 {
		t.Fatalf("plugin calls after unchanged snapshot = %d, want 1", plugin.calls)
	}
}

func TestRuntimeIdleMessageExplainsEmptyGraph(t *testing.T) {
	snapshot := Snapshot{
		Version: 10,
		Nodes: []SnapshotNode{
			{Name: "ground-a", Kind: "ground"},
		},
	}
	message := runtimeIdleMessage(snapshot, DesiredState{})
	if !strings.Contains(message, "no hosted workloads or routes") {
		t.Fatalf("runtimeIdleMessage missing empty-graph explanation: %q", message)
	}
	if !strings.Contains(message, "--injectTestWorkloads") {
		t.Fatalf("runtimeIdleMessage missing testing hint: %q", message)
	}
}
