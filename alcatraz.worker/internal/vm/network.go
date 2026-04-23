package vm

import (
	"fmt"
	"os/exec"
	"strings"
)

func SetupTap(instance *Instance) error {
	if err := RunCmd("ip", "link", "del", instance.TapDev); err != nil {
		fmt.Printf("warning: could not delete tap device: %v\n", err)
	}
	if err := RunCmd("ip", "tuntap", "add", "dev", instance.TapDev, "mode", "tap"); err != nil {
		return fmt.Errorf("failed to create tap device: %w", err)
	}
	if err := RunCmd("ip", "addr", "add", fmt.Sprintf("%s/24", instance.HostTapIP), "dev", instance.TapDev); err != nil {
		return fmt.Errorf("failed to assign IP to tap: %w", err)
	}
	if err := RunCmd("ip", "link", "set", instance.TapDev, "up"); err != nil {
		return fmt.Errorf("failed to bring up tap: %w", err)
	}
	return nil
}

func GetHostIface() (string, error) {
	cmd := exec.Command("ip", "route", "show", "default")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	fields := string(out)
	for _, f := range strings.Fields(fields) {
		if f == "dev" {
			continue
		}
		if !strings.HasPrefix(f, "-") && !strings.HasPrefix(f, "default") {
			return f, nil
		}
	}

	return "eth0", nil
}

func SetupNAT(instance *Instance) error {
	if err := RunCmd("sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
		return fmt.Errorf("failed to enable IP forward: %w", err)
	}
	if err := RunCmd("sysctl", "-w", fmt.Sprintf("net.ipv4.conf.%s.route_localnet=1", instance.TapDev)); err != nil {
		return fmt.Errorf("failed to enable route_localnet: %w", err)
	}

	hostIface, err := GetHostIface()
	if err != nil {
		hostIface = "eth0"
	}

	if err := RunCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", instance.Subnet, "-o", hostIface, "-j", "MASQUERADE"); err != nil {
		return fmt.Errorf("failed to add NAT rule: %w", err)
	}
	if err := RunCmd("iptables", "-A", "FORWARD", "-i", hostIface, "-o", instance.TapDev, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("failed to add forward rule 1: %w", err)
	}
	if err := RunCmd("iptables", "-A", "FORWARD", "-i", instance.TapDev, "-o", hostIface, "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("failed to add forward rule 2: %w", err)
	}

	return nil
}

func CleanupNAT(instance *Instance) {
	hostIface, _ := GetHostIface()
	RunCmd("iptables", "-t", "nat", "-D", "POSTROUTING", "-s", instance.Subnet, "-o", hostIface, "-j", "MASQUERADE")
	RunCmd("iptables", "-D", "FORWARD", "-i", hostIface, "-o", instance.TapDev, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	RunCmd("iptables", "-D", "FORWARD", "-i", instance.TapDev, "-o", hostIface, "-j", "ACCEPT")
}