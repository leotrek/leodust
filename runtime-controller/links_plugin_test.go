package main

import (
	"strings"
	"testing"
)

func TestBuildLinkApplyScriptProgramsRoutesAndSharedClasses(t *testing.T) {
	plans := []nextHopPlan{
		{
			NextHopNode:  "sat-b",
			NextHopIP:    "10.0.0.2",
			LatencyMs:    1.25,
			BandwidthBps: 5000,
			Destinations: []string{"10.0.0.4", "10.0.0.5"},
		},
	}

	script := buildLinkApplyScript("eth1", true, plans)

	wantContains := []string{
		`sysctl -w net.ipv4.ip_forward=1 >/dev/null`,
		`ip route replace table "$TABLE" 10.0.0.4/32 via 10.0.0.2 dev "$DEV" onlink`,
		`ip route replace table "$TABLE" 10.0.0.5/32 via 10.0.0.2 dev "$DEV" onlink`,
		`iptables -t mangle -A "$CHAIN" -d 10.0.0.4/32 -j MARK --set-mark 10`,
		`iptables -t mangle -A "$CHAIN" -d 10.0.0.5/32 -j MARK --set-mark 10`,
		`tc filter add dev "$DEV" parent 1: protocol ip prio 1 handle 10 fw flowid 1:10`,
	}
	for _, want := range wantContains {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q\n%s", want, script)
		}
	}
}
