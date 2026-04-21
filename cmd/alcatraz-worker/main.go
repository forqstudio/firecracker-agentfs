package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const (
	defaultNATSURL    = "nats://localhost:4222"
	defaultSubject    = "vm.spawn"
	defaultMaxVMs     = 5
	defaultQueueGroup = "vm-workers"

	firecrackerBin = "./bin/firecracker-v1.15.1"
	kernelPath     = "./linux-amazon/vmlinux"
	rootfsPath    = "./rootfs"
	agentfsDir    = ".agentfs"

	baseTapDev     = "fc-tap"
	baseHostTapIP = "172.16.0.1"
	baseVMIP     = "172.16.0.2"
	baseNFSPort  = 11111

	vmHostname = "alcatraz"
	guestMAC   = "AA:FC:00:00:00:01"

	defaultVCPUs   = 4
	defaultMemMib  = 8192
	defaultKernelArgs = "loglevel=7 printk.devkmsg=on"
)

var (
	natsURL      string
	subject     string
	maxVMs      int
	queueGroup string

	agentfsBin string
	fcBin     string
	rootfs   string
	kernel   string

	agentfsDirAbs string
)

type VMRequest struct {
	ID          string `json:"id,omitempty"`
	VCPUs      int    `json:"vcpus,omitempty"`
	MemoryMib  int    `json:"memory_mib,omitempty"`
	KernelArgs string `json:"kernel_args,omitempty"`
}

type VMInstance struct {
	ID          string
	VCPUs      int
	MemoryMib  int
	KernelArgs string
	Index     int

	TapDev    string
	HostTapIP string
	VMIP     string
	Subnet   string
	NFSPort  int
	Socket   string
	AgentID  string

	Machine   *firecracker.Machine
	NFSProc   *exec.Cmd
	Config   firecracker.Config
	MachineCfg *models.MachineConfiguration

	mu sync.Mutex
}

type InstanceManager struct {
	mu       sync.Mutex
	instances map[string]*VMInstance
	pool      []int
	maxVMs   int
}

func NewInstanceManager(maxVMs int) *InstanceManager {
	pool := make([]int, maxVMs)
	for i := range pool {
		pool[i] = i
	}
	return &InstanceManager{
		instances: make(map[string]*VMInstance),
		pool:      pool,
		maxVMs:   maxVMs,
	}
}

func (m *InstanceManager) allocate() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.pool) == 0 {
		return 0, fmt.Errorf("no available VM slots (max %d)", m.maxVMs)
	}
	idx := m.pool[0]
	m.pool = m.pool[1:]
	return idx, nil
}

func (m *InstanceManager) release(idx int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pool = append(m.pool, idx)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findAgentfsBin() (string, error) {
	if agentfsBin != "" {
		return agentfsBin, nil
	}
	locations := []string{
		"agentfs",
		"/home/dev/.cargo/bin/agentfs",
		"/usr/local/bin/agentfs",
	}
	for _, loc := range locations {
		if path, err := exec.LookPath(loc); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("agentfs not found")
}

func resolveFirecrackerBin() (string, error) {
	if fcBin != "" && fileExists(fcBin) {
		return fcBin, nil
	}
	if fileExists(firecrackerBin) {
		return firecrackerBin, nil
	}
	if path, err := exec.LookPath("firecracker"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("firecracker binary not found")
}

func setupTap(instance *VMInstance) error {
	if err := runCmd("ip", "link", "del", instance.TapDev); err != nil {
		log.Printf("warning: could not delete tap device: %v", err)
	}
	if err := runCmd("ip", "tuntap", "add", "dev", instance.TapDev, "mode", "tap"); err != nil {
		return fmt.Errorf("failed to create tap device: %w", err)
	}
	if err := runCmd("ip", "addr", "add", fmt.Sprintf("%s/24", instance.HostTapIP), "dev", instance.TapDev); err != nil {
		return fmt.Errorf("failed to assign IP to tap: %w", err)
	}
	if err := runCmd("ip", "link", "set", instance.TapDev, "up"); err != nil {
		return fmt.Errorf("failed to bring up tap: %w", err)
	}
	return nil
}

func setupNAT(instance *VMInstance) error {
	if err := runCmd("sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
		return fmt.Errorf("failed to enable IP forward: %w", err)
	}
	if err := runCmd("sysctl", "-w", fmt.Sprintf("net.ipv4.conf.%s.route_localnet=1", instance.TapDev)); err != nil {
		return fmt.Errorf("failed to enable route_localnet: %w", err)
	}

	hostIface, err := getHostIface()
	if err != nil {
		hostIface = "eth0"
	}

	if err := runCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", instance.Subnet, "-o", hostIface, "-j", "MASQUERADE"); err != nil {
		return fmt.Errorf("failed to add NAT rule: %w", err)
	}
	if err := runCmd("iptables", "-A", "FORWARD", "-i", hostIface, "-o", instance.TapDev, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("failed to add forward rule 1: %w", err)
	}
	if err := runCmd("iptables", "-A", "FORWARD", "-i", instance.TapDev, "-o", hostIface, "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("failed to add forward rule 2: %w", err)
	}

	return nil
}

func getHostIface() (string, error) {
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

func prepareAgentfsOverlay(instance *VMInstance) error {
	os.MkdirAll(agentfsDirAbs, 0755)

	dbPath := filepath.Join(agentfsDirAbs, instance.AgentID+".db")
	needsInit := !fileExists(dbPath)

	baseStampFile := filepath.Join(rootfs, "etc/alcatraz-release")
	var currentStamp string
	if fileExists(baseStampFile) {
		cmd := exec.Command("sha256sum", baseStampFile)
		out, err := cmd.Output()
		if err == nil {
			currentStamp = string(out[:64])
		}
	}

	baseStampPath := filepath.Join(agentfsDirAbs, instance.AgentID+".base-stamp")

	if fileExists(baseStampPath) && currentStamp != "" {
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
		cmd := exec.Command(agentfsBin, "init", "--force", "--base", rootfs, instance.AgentID)
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

func startAgentfsNFS(instance *VMInstance) error {
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

func cleanupInstance(instance *VMInstance) {
	log.Printf("Cleaning up instance %s", instance.ID)

	if instance.NFSProc != nil && instance.NFSProc.Process != nil {
		instance.NFSProc.Process.Kill()
		instance.NFSProc.Wait()
	}
	exec.Command("pkill", "-f", fmt.Sprintf("agentfs serve nfs --bind %s --port %d", instance.HostTapIP, instance.NFSPort)).Run()

	runCmd("ip", "link", "del", instance.TapDev)

	hostIface, _ := getHostIface()
	runCmd("iptables", "-t", "nat", "-D", "POSTROUTING", "-s", instance.Subnet, "-o", hostIface, "-j", "MASQUERADE")
	runCmd("iptables", "-D", "FORWARD", "-i", hostIface, "-o", instance.TapDev, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	runCmd("iptables", "-D", "FORWARD", "-i", instance.TapDev, "-o", hostIface, "-j", "ACCEPT")

	if fileExists(instance.Socket) {
		os.Remove(instance.Socket)
	}
}

func spawnVM(ctx context.Context, mgr *InstanceManager, req VMRequest) (*VMInstance, error) {
	idx, err := mgr.allocate()
	if err != nil {
		return nil, err
	}

	vmID := req.ID
	if vmID == "" {
		vmID = uuid.New().String()
	}

	vcpus := req.VCPUs
	if vcpus <= 0 {
		vcpus = defaultVCPUs
	}

	memMib := req.MemoryMib
	if memMib <= 0 {
		memMib = defaultMemMib
	}

	kernelArgs := req.KernelArgs
	if kernelArgs == "" {
		kernelArgs = defaultKernelArgs
	}

	hostTapIP := fmt.Sprintf("172.16.%d.1", idx)
	vmIP := fmt.Sprintf("172.16.%d.2", idx)
	subnet := fmt.Sprintf("172.16.%d.0/24", idx)
	nfsPort := baseNFSPort + idx
	tapDev := fmt.Sprintf("%s%d", baseTapDev, idx)
	socket := filepath.Join(agentfsDirAbs, fmt.Sprintf("fc-%s.sock", vmID))
	agentID := vmID

	instance := &VMInstance{
		ID:          vmID,
		VCPUs:       vcpus,
		MemoryMib:  memMib,
		KernelArgs: kernelArgs,
		Index:      idx,

		TapDev:    tapDev,
		HostTapIP: hostTapIP,
		VMIP:      vmIP,
		Subnet:    subnet,
		NFSPort:   nfsPort,
		Socket:    socket,
		AgentID:   agentID,
	}

	log.Printf("Spawning VM %s (vCPUs: %d, Mem: %d MiB, idx: %d)", vmID, vcpus, memMib, idx)

	if err := setupTap(instance); err != nil {
		mgr.release(idx)
		return nil, fmt.Errorf("setup tap: %w", err)
	}

	if err := setupNAT(instance); err != nil {
		cleanupInstance(instance)
		mgr.release(idx)
		return nil, fmt.Errorf("setup nat: %w", err)
	}

	if err := prepareAgentfsOverlay(instance); err != nil {
		cleanupInstance(instance)
		mgr.release(idx)
		return nil, fmt.Errorf("prepare agentfs: %w", err)
	}

	if err := startAgentfsNFS(instance); err != nil {
		cleanupInstance(instance)
		mgr.release(idx)
		return nil, fmt.Errorf("start agentfs nfs: %w", err)
	}

	subnetMask := "255.255.255.0"
	bootArgs := fmt.Sprintf(
		"console=ttyS0 reboot=k panic=1 pci=off %s ip=%s::%s:%s:%s:eth0:off root=/dev/nfs nfsroot=%s:/,nfsvers=3,tcp,nolock,port=%d,mountport=%d rw init=/init",
		kernelArgs, vmIP, hostTapIP, subnetMask, vmHostname, hostTapIP, nfsPort, nfsPort,
	)

	cfg := firecracker.Config{
		SocketPath:      instance.Socket,
		KernelImagePath: kernel,
		KernelArgs:      bootArgs,
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(int64(vcpus)),
			MemSizeMib: firecracker.Int64(int64(memMib)),
		},
		NetworkInterfaces: []firecracker.NetworkInterface{
			{
				StaticConfiguration: &firecracker.StaticNetworkConfiguration{
					MacAddress:  guestMAC,
					HostDevName: tapDev,
				},
			},
		},
	}

	fcBinPath, err := resolveFirecrackerBin()
	if err != nil {
		cleanupInstance(instance)
		mgr.release(idx)
		return nil, err
	}

	cmd := firecracker.VMCommandBuilder{}.
		WithBin(fcBinPath).
		WithSocketPath(instance.Socket).
		Build(ctx)

	m, err := firecracker.NewMachine(ctx, cfg, firecracker.WithProcessRunner(cmd))
	if err != nil {
		cleanupInstance(instance)
		mgr.release(idx)
		return nil, fmt.Errorf("create machine: %w", err)
	}

	instance.Machine = m
	instance.Config = cfg

	mgr.mu.Lock()
	mgr.instances[vmID] = instance
	mgr.mu.Unlock()

	if err := m.Start(ctx); err != nil {
		mgr.mu.Lock()
		delete(mgr.instances, vmID)
		mgr.mu.Unlock()
		cleanupInstance(instance)
		mgr.release(idx)
		return nil, fmt.Errorf("start machine: %w", err)
	}

	log.Printf("VM %s started (IP: %s)", vmID, vmIP)

	go func() {
		id := instance.ID
		if err := m.Wait(ctx); err != nil {
			log.Printf("VM %s wait error: %v", id, err)
		}
		log.Printf("VM %s exited", id)
		mgr.mu.Lock()
		delete(mgr.instances, id)
		mgr.mu.Unlock()
		cleanupInstance(instance)
		mgr.release(idx)
	}()

	return instance, nil
}

func main() {
	flag.StringVar(&natsURL, "nats-url", defaultNATSURL, "NATS server URL")
	flag.StringVar(&subject, "subject", defaultSubject, "NATS subject to subscribe")
	flag.IntVar(&maxVMs, "max-vms", defaultMaxVMs, "Maximum concurrent VMs")
	flag.StringVar(&queueGroup, "queue-group", defaultQueueGroup, "NATS queue group")

	flag.StringVar(&agentfsBin, "agentfs-bin", "", "Path to agentfs binary")
	flag.StringVar(&fcBin, "firecracker-bin", "", "Path to firecracker binary")
	flag.StringVar(&rootfs, "rootfs", rootfsPath, "Root filesystem path")
	flag.StringVar(&kernel, "kernel", kernelPath, "Kernel path")

	flag.Parse()

	if os.Getuid() != 0 {
		log.Fatal("This program must be run as root")
	}

	if runtime.GOOS != "linux" {
		log.Fatal("This program must be run on Linux")
	}

	if !fileExists(kernel) {
		log.Fatalf("Kernel not found at %s", kernel)
	}
	if !fileExists(rootfs) {
		log.Fatalf("Rootfs not found at %s", rootfs)
	}

	absDir, err := filepath.Abs(agentfsDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path for agentfs dir: %v", err)
	}
	agentfsDirAbs = absDir
	os.MkdirAll(agentfsDirAbs, 0755)

	agentfsPath, err := findAgentfsBin()
	if err != nil {
		log.Fatal(err)
	}
	agentfsBin = agentfsPath
	log.Printf("Using agentfs: %s", agentfsBin)

	ctx := context.Background()

	mgr := NewInstanceManager(maxVMs)

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Drain()
	log.Printf("Connected to NATS at %s", natsURL)

	ch := make(chan *nats.Msg, 64)
	sub, err := nc.ChanQueueSubscribe(subject, queueGroup, ch)
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	log.Printf("Subscribed to %s (queue: %s)", subject, queueGroup)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Worker ready (max VMs: %d)", maxVMs)

	for {
		select {
		case sig := <-sigCh:
			log.Printf("Received signal %v, shutting down...", sig)
			mgr.mu.Lock()
			for _, inst := range mgr.instances {
				if inst.Machine != nil {
					log.Printf("Stopping VM %s", inst.ID)
					inst.Machine.StopVMM()
				}
			}
			mgr.mu.Unlock()
			time.Sleep(2 * time.Second)
			os.Exit(0)

		case msg, ok := <-ch:
			if !ok {
				log.Printf("NATS channel closed")
				continue
			}
			var req VMRequest
			if err := json.Unmarshal(msg.Data, &req); err != nil {
				log.Printf("Failed to parse request: %v", err)
				continue
			}

			log.Printf("Received spawn request: %+v", req)

			inst, err := spawnVM(ctx, mgr, req)
			if err != nil {
				log.Printf("Failed to spawn VM: %v", err)
				continue
			}

			log.Printf("Spawned VM %s", inst.ID)
		}
	}
}