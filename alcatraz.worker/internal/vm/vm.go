package vm

import (
	"context"
	"fmt"
	"log"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

type SpawnOptions struct {
	AgentfsBin     string
	FirecrackerBin string
	Rootfs         string
	Kernel         string
	AgentfsData    string
}

type VMBuilder struct {
	instance *Instance
	options  *SpawnOptions
}

func NewVMBuilder() *VMBuilder {
	return &VMBuilder{
		instance: NewInstance(),
		options:  &SpawnOptions{},
	}
}

func (builder *VMBuilder) WithRequest(req *CreateVMInput) *VMBuilder {
	builder.instance.id = req.ID
	builder.instance.vcpus = req.VCPUs
	builder.instance.memoryMib = req.MemoryMib
	builder.instance.kernelArgs = req.KernelArgs
	return builder
}

func (builder *VMBuilder) WithIndex(idx int) *VMBuilder {
	builder.instance.index = idx
	builder.instance.tapDev = fmt.Sprintf("%s%d", BaseTapDev, idx)
	builder.instance.hostTapIP = fmt.Sprintf("172.16.%d.1", idx)
	builder.instance.vmIP = fmt.Sprintf("172.16.%d.2", idx)
	builder.instance.subnet = fmt.Sprintf("172.16.%d.0/24", idx)
	builder.instance.nfsPort = BaseNFSPort + idx
	return builder
}

func (builder *VMBuilder) WithAgentID(id string) *VMBuilder {
	builder.instance.agentID = id
	return builder
}

func (builder *VMBuilder) WithSpawnOptions(opts *SpawnOptions) *VMBuilder {
	builder.options = opts
	return builder
}

func (builder *VMBuilder) Build() *Instance {
	builder.instance.socket = fmt.Sprintf("%s/fc-%s.sock", builder.options.AgentfsData, builder.instance.agentID)
	return builder.instance
}

func Spawn(
	context context.Context,
	instanceManager *InstanceManager,
	request *CreateVMInput,
	options *SpawnOptions) (*Instance, error) {
	index, err := instanceManager.Allocate()
	if err != nil {
		return nil, err
	}

	validatedReq := request.WithDefaults()

	instance := NewVMBuilder().
		WithRequest(validatedReq).
		WithIndex(index).
		WithAgentID(validatedReq.ID).
		WithSpawnOptions(options).
		Build()

	log.Printf("Spawning VM %s (vCPUs: %d, Mem: %d MiB, idx: %d)", instance.id, instance.vcpus, instance.memoryMib, index)

	if err := SetupTap(instance); err != nil {
		instanceManager.Release(index)
		return nil, fmt.Errorf("setup tap: %w", err)
	}

	if err := SetupNAT(instance); err != nil {
		CleanupInstance(instance)
		instanceManager.Release(index)
		return nil, fmt.Errorf("setup nat: %w", err)
	}

	if err := PrepareAgentfsOverlay(instance, options.AgentfsBin, options.Rootfs, options.AgentfsData); err != nil {
		CleanupInstance(instance)
		instanceManager.Release(index)
		return nil, fmt.Errorf("prepare agentfs: %w", err)
	}

	nfsProc, err := StartAgentfsNFS(instance, options.AgentfsBin)
	if err != nil {
		CleanupInstance(instance)
		instanceManager.Release(index)
		return nil, fmt.Errorf("start agentfs nfs: %w", err)
	}
	instance.SetNFSProcess(nfsProc)

	subnetMask := "255.255.255.0"
	bootArgs := fmt.Sprintf(
		"console=ttyS0 reboot=k panic=1 pci=off %s ip=%s::%s:%s:%s:eth0:off root=/dev/nfs nfsroot=%s:/,nfsvers=3,tcp,nolock,port=%d,mountport=%d rw init=/init",
		instance.kernelArgs,
		instance.vmIP,
		instance.hostTapIP,
		subnetMask,
		VMHostname,
		instance.hostTapIP,
		instance.nfsPort,
		instance.nfsPort,
	)

	cfg := firecracker.Config{
		SocketPath:      instance.socket,
		KernelImagePath: options.Kernel,
		KernelArgs:      bootArgs,
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(int64(instance.vcpus)),
			MemSizeMib: firecracker.Int64(int64(instance.memoryMib)),
		},
		NetworkInterfaces: []firecracker.NetworkInterface{
			{
				StaticConfiguration: &firecracker.StaticNetworkConfiguration{
					MacAddress:  GuestMAC,
					HostDevName: instance.tapDev,
				},
			},
		},
	}

	fcBinPath := options.FirecrackerBin
	if !FileExists(fcBinPath) {
		CleanupInstance(instance)
		instanceManager.Release(index)
		return nil, fmt.Errorf("firecracker binary not found: %s", fcBinPath)
	}

	cmd := firecracker.VMCommandBuilder{}.
		WithBin(fcBinPath).
		WithSocketPath(instance.socket).
		Build(context)

	m, err := firecracker.NewMachine(context, cfg, firecracker.WithProcessRunner(cmd))
	if err != nil {
		CleanupInstance(instance)
		instanceManager.Release(index)
		return nil, fmt.Errorf("create machine: %w", err)
	}

	if err := m.Start(context); err != nil {
		instanceManager.RemoveInstance(instance.id)
		CleanupInstance(instance)
		instanceManager.Release(index)
		return nil, fmt.Errorf("start machine: %w", err)
	}

	log.Printf("VM %s started (IP: %s)", instance.id, instance.vmIP)

	go func() {
		id := instance.id
		if err := m.Wait(context); err != nil {
			log.Printf("VM %s wait error: %v", id, err)
		}
		log.Printf("VM %s exited", id)
		instanceManager.RemoveInstance(id)
		CleanupInstance(instance)
		instanceManager.Release(index)
	}()

	return instance, nil
}

func StopVM(inst *Instance) error {
	if inst.machine != nil {
		return inst.machine.StopVMM()
	}
	return nil
}
