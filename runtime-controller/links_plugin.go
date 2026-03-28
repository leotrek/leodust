package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	routingTableID  = 200
	routingRulePref = 1100
	mangleChainName = "LEODUST-RUNTIME-MANGLE"
	defaultRootMbit = 1000000
)

type LinksPlugin struct {
	lastScripts map[string]string
}

func NewLinksPlugin() *LinksPlugin {
	return &LinksPlugin{
		lastScripts: make(map[string]string),
	}
}

func (*LinksPlugin) Name() string {
	return "links"
}

type nextHopPlan struct {
	NextHopNode  string
	NextHopIP    string
	LatencyMs    float64
	BandwidthBps float64
	Destinations []string
}

func (p *LinksPlugin) Reconcile(_ Snapshot, desired DesiredState, env Environment) (PluginResult, error) {
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

	nodeIPs := make(map[string]string, len(desired.Nodes))
	for name, node := range desired.Nodes {
		live, ok := bySatellite[name]
		if !ok {
			warnf("Links plugin cannot program %s because sandbox %s does not exist", name, node.ContainerName)
			continue
		}
		ip, err := env.Runtime.ContainerIPv4(live.ContainerName, env.Device)
		if err != nil {
			return PluginResult{}, fmt.Errorf("resolve IP for %s: %w", live.ContainerName, err)
		}
		nodeIPs[name] = ip
	}

	names := make([]string, 0, len(desired.Nodes))
	for name := range desired.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)

	changed := 0
	for _, name := range names {
		node := desired.Nodes[name]
		live, ok := bySatellite[name]
		if !ok {
			continue
		}

		plan, err := buildNextHopPlans(name, desired, nodeIPs)
		if err != nil {
			return PluginResult{}, err
		}
		script := buildLinkApplyScript(env.Device, node.Relay, plan)
		if p.lastScripts[live.ContainerName] == script {
			continue
		}
		if env.DryRun {
			debugf("Links plugin would apply script to %s with %d next-hop groups", live.ContainerName, len(plan))
			p.lastScripts[live.ContainerName] = script
			changed++
			continue
		}
		if err := env.Runtime.ApplyScript(live.ContainerName, script); err != nil {
			return PluginResult{}, fmt.Errorf("apply link state to %s: %w", live.ContainerName, err)
		}
		p.lastScripts[live.ContainerName] = script
		changed++
	}

	return PluginResult{
		Changed: changed,
		Summary: fmt.Sprintf("active edges=%d", len(desired.Edges)),
	}, nil
}

func buildNextHopPlans(source string, desired DesiredState, nodeIPs map[string]string) ([]nextHopPlan, error) {
	routes := desired.RoutesByNode[source]
	if len(routes) == 0 {
		return nil, nil
	}

	byNextHop := make(map[string]*nextHopPlan)
	for _, route := range routes {
		destinationIP, ok := nodeIPs[route.DestinationNode]
		if !ok {
			warnf("Skipping destination %s from %s because its sandbox IP is unavailable", route.DestinationNode, source)
			continue
		}
		nextHopIP, ok := nodeIPs[route.NextHopNode]
		if !ok {
			warnf("Skipping next hop %s from %s because its sandbox IP is unavailable", route.NextHopNode, source)
			continue
		}
		edge, ok := desired.Edges[edgeKey(source, route.NextHopNode)]
		if !ok {
			return nil, fmt.Errorf("missing active edge for %s -> %s", source, route.NextHopNode)
		}

		plan, exists := byNextHop[route.NextHopNode]
		if !exists {
			plan = &nextHopPlan{
				NextHopNode:  route.NextHopNode,
				NextHopIP:    nextHopIP,
				LatencyMs:    edge.LatencyMs,
				BandwidthBps: edge.BandwidthBps,
			}
			byNextHop[route.NextHopNode] = plan
		}
		plan.Destinations = append(plan.Destinations, destinationIP)
	}

	plans := make([]nextHopPlan, 0, len(byNextHop))
	for _, plan := range byNextHop {
		sort.Strings(plan.Destinations)
		plan.Destinations = compactStrings(plan.Destinations)
		plans = append(plans, *plan)
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].NextHopNode == plans[j].NextHopNode {
			return plans[i].NextHopIP < plans[j].NextHopIP
		}
		return plans[i].NextHopNode < plans[j].NextHopNode
	})
	return plans, nil
}

func buildLinkApplyScript(device string, enableForwarding bool, plans []nextHopPlan) string {
	var builder strings.Builder
	builder.WriteString("#!/bin/sh\n")
	builder.WriteString("set -eu\n\n")
	fmt.Fprintf(&builder, "DEV=%q\n", device)
	fmt.Fprintf(&builder, "CHAIN=%q\n", mangleChainName)
	fmt.Fprintf(&builder, "TABLE=%d\n", routingTableID)
	fmt.Fprintf(&builder, "RULE_PREF=%d\n\n", routingRulePref)
	builder.WriteString("sysctl -w net.ipv4.ip_forward=")
	if enableForwarding {
		builder.WriteString("1 >/dev/null\n")
	} else {
		builder.WriteString("0 >/dev/null\n")
	}
	builder.WriteString("iptables -t mangle -N \"$CHAIN\" 2>/dev/null || true\n")
	builder.WriteString("iptables -t mangle -F \"$CHAIN\"\n")
	builder.WriteString("iptables -t mangle -C OUTPUT -j \"$CHAIN\" 2>/dev/null || iptables -t mangle -I OUTPUT 1 -j \"$CHAIN\"\n")
	builder.WriteString("iptables -t mangle -C FORWARD -j \"$CHAIN\" 2>/dev/null || iptables -t mangle -I FORWARD 1 -j \"$CHAIN\"\n")
	builder.WriteString("while ip rule del pref \"$RULE_PREF\" 2>/dev/null; do :; done\n")
	builder.WriteString("ip rule add pref \"$RULE_PREF\" lookup \"$TABLE\" 2>/dev/null || true\n")
	builder.WriteString("ip route flush table \"$TABLE\" 2>/dev/null || true\n")
	builder.WriteString("tc qdisc del dev \"$DEV\" root 2>/dev/null || true\n")
	if len(plans) == 0 {
		builder.WriteString("iptables -t mangle -A \"$CHAIN\" -j RETURN\n")
		return builder.String()
	}

	for index, plan := range plans {
		mark := index + 10
		for _, destination := range plan.Destinations {
			fmt.Fprintf(&builder, "ip route replace table \"$TABLE\" %s/32 via %s dev \"$DEV\" onlink\n", destination, plan.NextHopIP)
			fmt.Fprintf(&builder, "iptables -t mangle -A \"$CHAIN\" -d %s/32 -j MARK --set-mark %d\n", destination, mark)
		}
	}
	builder.WriteString("iptables -t mangle -A \"$CHAIN\" -j RETURN\n")
	builder.WriteString("tc qdisc add dev \"$DEV\" root handle 1: htb default 1\n")
	fmt.Fprintf(&builder, "tc class add dev \"$DEV\" parent 1: classid 1:1 htb rate %dmbit ceil %dmbit\n", defaultRootMbit, defaultRootMbit)
	for index, plan := range plans {
		classID := index + 10
		rateBits := maxRateBits(plan.BandwidthBps)
		delayMs := roundDelayMs(plan.LatencyMs)
		fmt.Fprintf(&builder, "tc class add dev \"$DEV\" parent 1:1 classid 1:%d htb rate %dbit ceil %dbit\n", classID, rateBits, rateBits)
		fmt.Fprintf(&builder, "tc qdisc add dev \"$DEV\" parent 1:%d handle %d0: netem delay %.3fms\n", classID, classID, delayMs)
		fmt.Fprintf(&builder, "tc filter add dev \"$DEV\" parent 1: protocol ip prio 1 handle %d fw flowid 1:%d\n", classID, classID)
	}

	return builder.String()
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func maxRateBits(value float64) int64 {
	if value <= 1 {
		return 1
	}
	return int64(math.Round(value))
}

func roundDelayMs(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Round(value*1000) / 1000
}
