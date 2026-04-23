package vm

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func PrepareAgentfsOverlay(
	instance *Instance,
	agentfsBin,
	rootfsPath,
	agentfsDir string) error {
	os.MkdirAll(agentfsDir, 0755)

	dbPath := filepath.Join(agentfsDir, instance.AgentID+".db")
	needsInit := !FileExists(dbPath)

	baseStampFile := filepath.Join(rootfsPath, "etc/alcatraz-release")
	var currentStamp string
	if FileExists(baseStampFile) {
		cmd := exec.Command("sha256sum", baseStampFile)
		out, err := cmd.Output()
		if err == nil {
			currentStamp = string(out[:64])
		}
	}

	baseStampPath := filepath.Join(agentfsDir, instance.AgentID+".base-stamp")

	if FileExists(baseStampPath) && currentStamp != "" {
		existingStamp, err := os.ReadFile(baseStampPath)
		if err == nil && string(existingStamp) != currentStamp+"\n" {
			log.Printf("Rootfs changed for %s, reinitializing", instance.AgentID)
			os.Remove(dbPath)
			os.Remove(dbPath + "-wal")
			os.Remove(dbPath + "-shm")
			os.Remove(baseStampPath)
			needsInit = true
		}
	}

	if needsInit {
		log.Printf("Initializing AgentFS overlay for %s", instance.AgentID)
		cmd := exec.Command(agentfsBin, "init", "--force", "--base", rootfsPath, instance.AgentID)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to init agentfs: %w", err)
		}
	}

	if currentStamp != "" {
		os.WriteFile(baseStampPath, []byte(currentStamp), 0644)
	}

	return nil
}

func StartAgentfsNFS(
	instance *Instance,
	agentfsBin string) error {
	log.Printf("Starting AgentFS NFS on %s:%d", instance.HostTapIP, instance.NFSPort)

	exec.Command("pkill", "-f", fmt.Sprintf("agentfs serve nfs --bind %s --port %d", instance.HostTapIP, instance.NFSPort)).Run()
	time.Sleep(500 * time.Millisecond)

	cmd := exec.Command(agentfsBin, "serve", "nfs", "--bind", instance.HostTapIP, "--port", fmt.Sprintf("%d", instance.NFSPort), instance.AgentID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start agentfs NFS: %w", err)
	}
	instance.NFSProc = cmd

	time.Sleep(1 * time.Second)

	if instance.NFSProc != nil && instance.NFSProc.Process != nil {
		if err := instance.NFSProc.Process.Kill(); err == nil {
			instance.NFSProc.Wait()
		}
	}

	cmd = exec.Command(agentfsBin, "serve", "nfs", "--bind", instance.HostTapIP, "--port", fmt.Sprintf("%d", instance.NFSPort), instance.AgentID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start agentfs NFS: %w", err)
	}
	instance.NFSProc = cmd

	time.Sleep(1 * time.Second)

	return nil
}

func CleanupInstance(instance *Instance) {
	log.Printf("Cleaning up instance %s", instance.ID)

	if instance.NFSProc != nil && instance.NFSProc.Process != nil {
		instance.NFSProc.Process.Kill()
		instance.NFSProc.Wait()
	}
	exec.Command("pkill", "-f", fmt.Sprintf("agentfs serve nfs --bind %s --port %d", instance.HostTapIP, instance.NFSPort)).Run()

	RunCmd("ip", "link", "del", instance.TapDev)

	CleanupNAT(instance)

	if FileExists(instance.Socket) {
		os.Remove(instance.Socket)
	}
}
