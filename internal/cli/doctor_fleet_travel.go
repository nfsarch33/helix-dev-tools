// runx-public-repo-gate: allow-file fleet_host_alias
// runx-public-repo-gate: allow-file network_topology
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// fleetLatencyProfile holds SSH latency SLO thresholds derived from
// reports/research/win3-fleet-connectivity-australia-china-2026-05-30.md
type fleetLatencyProfile struct {
	Name         string
	DirectGreen  time.Duration
	DirectYellow time.Duration
	JumpGreen    time.Duration
	JumpYellow   time.Duration
	ProbeTimeout time.Duration
}

func fleetLatencyProfileFromEnv() fleetLatencyProfile {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FLEET_LATENCY_PROFILE"))) {
	case "home", "au", "australia":
		return fleetLatencyProfile{
			Name:         "home",
			DirectGreen:  3 * time.Second,
			DirectYellow: 8 * time.Second,
			JumpGreen:    5 * time.Second,
			JumpYellow:   12 * time.Second,
			ProbeTimeout: 20 * time.Second,
		}
	case "china", "chengdu", "travel":
		return fleetLatencyProfile{
			Name:         "china",
			DirectGreen:  12 * time.Second,
			DirectYellow: 25 * time.Second,
			JumpGreen:    18 * time.Second,
			JumpYellow:   30 * time.Second,
			ProbeTimeout: 35 * time.Second,
		}
	default:
		// auto: permissive china thresholds (travel-safe default from win3)
		return fleetLatencyProfile{
			Name:         "auto",
			DirectGreen:  12 * time.Second,
			DirectYellow: 25 * time.Second,
			JumpGreen:    18 * time.Second,
			JumpYellow:   30 * time.Second,
			ProbeTimeout: 35 * time.Second,
		}
	}
}

func isTravelMode() bool {
	if doctorFleetFlags.travel {
		return true
	}
	v := strings.ToLower(strings.TrimSpace(os.Getenv("FLEET_TRAVEL_MODE")))
	return v == "1" || v == "true" || v == "yes"
}

func buildFleetTravelProbes() []FleetProbe {
	prof := fleetLatencyProfileFromEnv()
	raw := "hostname"
	makeProbe := func(name, target string, viaJump bool) FleetProbe {
		green, yellow := prof.DirectGreen, prof.DirectYellow
		if viaJump {
			green, yellow = prof.JumpGreen, prof.JumpYellow
		}
		return FleetProbe{
			Name: name,
			Command: []string{
				"runx", "ssh", "exec", "--target", target, "--raw", raw,
			},
			Expect:         travelProbeExpect(target),
			Timeout:        prof.ProbeTimeout,
			MaxLatencyWarn: yellow,
			MaxLatencyFail: yellow + 5*time.Second,
			LatencyGreen:   green,
		}
	}
	return []FleetProbe{
		makeProbe("Travel SSH wsl1 direct", "wsl1", false),
		makeProbe("Travel SSH wsl1 via jump", "wsl1-travel", true),
		makeProbe("Travel SSH wsl2 direct", "wsl2", false),
		makeProbe("Travel SSH wsl2 via jump", "wsl2-travel", true),
		makeProbe("Travel SSH oracle-jump", "oracle-jump", true),
	}
}

func travelProbeExpect(target string) string {
	if target == "oracle-jump" {
		return "ocijump"
	}
	return "DESKTOP"
}

func applyLatencyStatus(result *FleetProbeResult, p FleetProbe) {
	if result.Status == FleetRed || p.LatencyGreen == 0 {
		return
	}
	if p.MaxLatencyFail > 0 && result.Duration > p.MaxLatencyFail {
		result.Status = FleetRed
		result.Error = fmt.Sprintf("latency %s exceeds fail threshold %s", result.Duration, p.MaxLatencyFail)
		return
	}
	if p.MaxLatencyWarn > 0 && result.Duration > p.MaxLatencyWarn {
		result.Status = FleetYellow
		if result.Error == "" {
			result.Error = fmt.Sprintf("slow: %s (warn > %s)", result.Duration, p.MaxLatencyWarn)
		}
		return
	}
	if p.LatencyGreen > 0 && result.Duration > p.LatencyGreen && result.Status == FleetGreen {
		// Within SLO but slower than ideal — stay GREEN, annotate output
		result.Output = strings.TrimSpace(result.Output) +
			fmt.Sprintf(" [%s latency ok]", result.Duration)
	}
}
