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

type VirtualMachineBuilder struct {
	instance *VirtualMachine
	options  *SpawnOptions
}

func NewVirtualMachineBuilder() *VirtualMachineBuilder {
	return &VirtualMachineBuilder{
		instance: NewVirtualMachine(),
		options:  &SpawnOptions{},
	}
}

func (builder *VirtualMachineBuilder) WithInput(input *CreateVirtualMachineInput) *VirtualMachineBuilder {
	builder.instance.id = input.ID
	builder.instance.vcpus = input.VCPUs
	builder.instance.memoryMib = input.MemoryMib
	builder.instance.kernelArgs = input.KernelArgs
	return builder
}

func (builder *VirtualMachineBuilder) WithIndex(index int) *VirtualMachineBuilder {
	builder.instance.index = index
	builder.instance.nfsPort = 8000 + index
	return builder
}

func (builder *VirtualMachineBuilder) WithAgentID(id string) *VirtualMachineBuilder {
	builder.instance.agentID = id
	return builder
}

func (builder *VirtualMachineBuilder) WithSpawnOptions(spawnOptions *SpawnOptions) *VirtualMachineBuilder {
	builder.options = spawnOptions
	return builder
}

func (builder *VirtualMachineBuilder) Build() *VirtualMachine {
	builder.instance.socket = fmt.Sprintf("/tmp/alcatraz-%s.sock", builder.instance.agentID)
	return builder.instance
}

func Spawn(
	context context.Context,
	virtualMachineService *VirtualMachineService,
	createVMInput *CreateVirtualMachineInput,
	spawnOptions *SpawnOptions) (*VirtualMachine, error) {
	index, err := virtualMachineService.Allocate()
	if err != nil {
		return nil, err
	}

	input := createVMInput.WithDefaults()

	tapDev := fmt.Sprintf("fc-tap%d", index)

	instance := NewVirtualMachineBuilder().
		WithInput(input).
		WithIndex(index).
		WithAgentID(input.ID).
		WithSpawnOptions(spawnOptions).
		Build()

	instance.tapDev = tapDev
	log.Printf("Spawning VM %s (vCPUs: %d, Mem: %d MiB, index: %d)", instance.id, instance.vcpus, instance.memoryMib, index)

	if err := PrepareAgentfsOverlay(instance, spawnOptions.AgentfsBin, spawnOptions.Rootfs, spawnOptions.AgentfsData); err != nil {
		virtualMachineService.Release(index)
		return nil, fmt.Errorf("prepare agentfs: %w", err)
	}

	nfsProc, err := StartAgentfsNFS(instance, spawnOptions.AgentfsBin)
	if err != nil {
		virtualMachineService.Release(index)
		return nil, fmt.Errorf("start agentfs nfs: %w", err)
	}
	instance.SetNFSProcess(nfsProc)

	bootArgs := fmt.Sprintf(
		"console=ttyS0 reboot=k panic=1 pci=off %s root=/dev/nfs nfsroot=${GATEWAY_IP}:/,nfsvers=3,tcp,nolock,port=%d,mountport=%d rw init=/init",
		instance.kernelArgs,
		instance.nfsPort,
		instance.nfsPort,
	)

	cfg := firecracker.Config{
		SocketPath:      instance.socket,
		KernelImagePath: spawnOptions.Kernel,
		KernelArgs:      bootArgs,
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(int64(instance.vcpus)),
			MemSizeMib: firecracker.Int64(int64(instance.memoryMib)),
		},
		NetworkInterfaces: []firecracker.NetworkInterface{
			{
				CNIConfiguration: &firecracker.CNIConfiguration{
					NetworkName: "alcatraz-bridge",
					IfName:      tapDev,
					VMIfName:    "eth0",
					ConfDir:     "/etc/cni/net.d",
					BinPath:     []string{"/opt/cni/bin"},
				},
			},
		},
		VMID: instance.id,
	}

	firecrackerBinPath := spawnOptions.FirecrackerBin
	if !FileExists(firecrackerBinPath) {
		virtualMachineService.Release(index)
		return nil, fmt.Errorf("firecracker binary not found: %s", firecrackerBinPath)
	}

	cmd := firecracker.VMCommandBuilder{}.
		WithBin(firecrackerBinPath).
		WithSocketPath(instance.socket).
		Build(context)

	m, err := firecracker.NewMachine(context, cfg, firecracker.WithProcessRunner(cmd))
	if err != nil {
		virtualMachineService.Release(index)
		return nil, fmt.Errorf("create machine: %w", err)
	}

	if err := m.Start(context); err != nil {
		virtualMachineService.RemoveVirtualMachine(instance.id)
		virtualMachineService.Release(index)
		return nil, fmt.Errorf("start machine: %w", err)
	}

	log.Printf("VM %s started", instance.id)

	go func() {
		id := instance.id
		if err := m.Wait(context); err != nil {
			log.Printf("VM %s wait error: %v", id, err)
		}
		log.Printf("VM %s exited", id)
		virtualMachineService.RemoveVirtualMachine(id)
		virtualMachineService.Release(index)
	}()

	return instance, nil
}

func StopVM(virtualMachine *VirtualMachine) error {
	if virtualMachine.machine != nil {
		return virtualMachine.machine.StopVMM()
	}
	return nil
}
