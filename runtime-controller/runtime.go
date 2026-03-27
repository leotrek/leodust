package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type ContainerRuntime interface {
	ListSandboxes(clusterName string) ([]SandboxInfo, error)
	ContainerIPv4(containerName, device string) (string, error)
	ApplyScript(containerName, script string) error
	SetConfig(containerName, key, value string) error
}

type SandboxInfo struct {
	ContainerName string
	SatelliteID   string
	Role          string
	HostWorker    string
	Managed       bool
}

type LXCRuntime struct{}

func NewLXCRuntime() *LXCRuntime {
	return &LXCRuntime{}
}

func (r *LXCRuntime) ListSandboxes(clusterName string) ([]SandboxInfo, error) {
	cmd := exec.Command("lxc", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lxc list failed: %w", err)
	}

	var instances []struct {
		Name   string            `json:"name"`
		Config map[string]string `json:"config"`
	}
	if err := json.Unmarshal(output, &instances); err != nil {
		return nil, fmt.Errorf("decode lxc list json: %w", err)
	}

	sandboxes := make([]SandboxInfo, 0)
	for _, instance := range instances {
		if instance.Config["user.leodust.cluster"] != clusterName {
			continue
		}
		if instance.Config["user.leodust.kind"] != "sandbox" {
			continue
		}
		sandboxes = append(sandboxes, SandboxInfo{
			ContainerName: instance.Name,
			SatelliteID:   instance.Config["user.leodust.satellite-id"],
			Role:          instance.Config["user.leodust.sandbox-role"],
			HostWorker:    instance.Config["user.leodust.host-worker"],
			Managed:       strings.EqualFold(instance.Config["user.leodust.runtime-controller"], "true"),
		})
	}
	return sandboxes, nil
}

func (r *LXCRuntime) ContainerIPv4(containerName, device string) (string, error) {
	cmd := exec.Command(
		"lxc",
		"exec",
		containerName,
		"--",
		"sh",
		"-lc",
		`ip -o -4 addr show dev "$1" | awk 'NR==1 {print $4}' | cut -d/ -f1`,
		"runtime-controller",
		device,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lxc exec ip lookup failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	ip := strings.TrimSpace(string(output))
	if ip == "" {
		return "", fmt.Errorf("no IPv4 address reported for %s on %s", containerName, device)
	}
	return ip, nil
}

func (r *LXCRuntime) ApplyScript(containerName, script string) error {
	cmd := exec.Command("lxc", "exec", containerName, "--", "sh", "-s")
	cmd.Stdin = strings.NewReader(script)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("lxc exec apply failed: %w: %s", err, message)
	}
	return nil
}

func (r *LXCRuntime) SetConfig(containerName, key, value string) error {
	cmd := exec.Command("lxc", "config", "set", containerName, key, value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc config set %s %s failed: %w: %s", containerName, key, err, strings.TrimSpace(string(output)))
	}
	return nil
}
