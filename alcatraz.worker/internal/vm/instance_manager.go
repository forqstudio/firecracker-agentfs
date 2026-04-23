package vm

import (
	"fmt"
	"sync"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
)

type Instance struct {
	id         string
	vcpus      int
	memoryMib  int
	kernelArgs string
	index      int

	tapDev    string
	hostTapIP string
	vmIP      string
	subnet    string
	nfsPort   int
	socket    string
	agentID   string

	machine *firecracker.Machine
	nfsProc NFSProcess

	mu sync.Mutex
}

func NewInstance(options ...InstanceOption) *Instance {
	inst := &Instance{}
	for _, opt := range options {
		opt(inst)
	}
	return inst
}

type InstanceOption func(*Instance)

func WithID(id string) InstanceOption {
	return func(i *Instance) {
		i.id = id
		i.agentID = id
	}
}

func WithVCPUs(vcpus int) InstanceOption {
	return func(i *Instance) {
		i.vcpus = vcpus
	}
}

func WithMemory(mib int) InstanceOption {
	return func(i *Instance) {
		i.memoryMib = mib
	}
}

func WithKernelArgs(args string) InstanceOption {
	return func(i *Instance) {
		i.kernelArgs = args
	}
}

func WithIndex(index int) InstanceOption {
	return func(vmInstance *Instance) {
		vmInstance.index = index
		vmInstance.tapDev = fmt.Sprintf("%s%d", BaseTapDev, index)
		vmInstance.hostTapIP = fmt.Sprintf("172.16.%d.1", index)
		vmInstance.vmIP = fmt.Sprintf("172.16.%d.2", index)
		vmInstance.subnet = fmt.Sprintf("172.16.%d.0/24", index)
		vmInstance.nfsPort = BaseNFSPort + index
	}
}

func WithSocket(socket string) InstanceOption {
	return func(i *Instance) {
		i.socket = socket
	}
}

func WithMachine(machine *firecracker.Machine) InstanceOption {
	return func(vmInstance *Instance) {
		vmInstance.machine = machine
	}
}

func WithNFSProcess(process NFSProcess) InstanceOption {
	return func(vmInstance *Instance) {
		vmInstance.nfsProc = process
	}
}

func (vmInstance *Instance) GetID() string                    { return vmInstance.id }
func (vmInstance *Instance) GetVCPUs() int                    { return vmInstance.vcpus }
func (vmInstance *Instance) GetMemoryMib() int                { return vmInstance.memoryMib }
func (vmInstance *Instance) GetKernelArgs() string            { return vmInstance.kernelArgs }
func (vmInstance *Instance) GetIndex() int                    { return vmInstance.index }
func (vmInstance *Instance) GetTapDev() string                { return vmInstance.tapDev }
func (vmInstance *Instance) GetHostTapIP() string             { return vmInstance.hostTapIP }
func (vmInstance *Instance) GetVMIP() string                  { return vmInstance.vmIP }
func (vmInstance *Instance) GetSubnet() string                { return vmInstance.subnet }
func (vmInstance *Instance) GetNFSPort() int                  { return vmInstance.nfsPort }
func (vmInstance *Instance) GetSocket() string                { return vmInstance.socket }
func (vmInstance *Instance) GetAgentID() string               { return vmInstance.agentID }
func (vmInstance *Instance) GetMachine() *firecracker.Machine { return vmInstance.machine }
func (vmInstance *Instance) GetNFSProcess() NFSProcess        { return vmInstance.nfsProc }

func (vmInstance *Instance) SetNFSProcess(proc NFSProcess) {
	vmInstance.mu.Lock()
	defer vmInstance.mu.Unlock()
	vmInstance.nfsProc = proc
}

type InstanceManager struct {
	mu        sync.Mutex
	instances map[string]*Instance
	pool      IntPool
	maxVMs    int
}

type IntPool struct {
	items []int
	mu    sync.Mutex
}

func NewIntPool(maxSize int) IntPool {
	pool := make([]int, maxSize)
	for i := range pool {
		pool[i] = i
	}
	return IntPool{items: pool}
}

func (p *IntPool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.items)
}

func (p *IntPool) Allocate() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.items) == 0 {
		return 0, fmt.Errorf("no available slots")
	}
	idx := p.items[0]
	p.items = p.items[1:]
	return idx, nil
}

func (p *IntPool) Release(idx int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items = append(p.items, idx)
}

func NewInstanceManager(maxVMs int) *InstanceManager {
	return &InstanceManager{
		instances: make(map[string]*Instance),
		pool:      NewIntPool(maxVMs),
		maxVMs:    maxVMs,
	}
}

func (instanceManager *InstanceManager) Allocate() (int, error) {
	instanceManager.mu.Lock()
	defer instanceManager.mu.Unlock()
	if len(instanceManager.pool.items) == 0 {
		return 0, fmt.Errorf("no available VM slots (max %d)", instanceManager.maxVMs)
	}
	idx := instanceManager.pool.items[0]
	instanceManager.pool.items = instanceManager.pool.items[1:]
	return idx, nil
}

func (instanceManager *InstanceManager) Release(idx int) {
	instanceManager.mu.Lock()
	defer instanceManager.mu.Unlock()
	instanceManager.pool.items = append(instanceManager.pool.items, idx)
}

func (instanceManager *InstanceManager) AddInstance(inst *Instance) {
	instanceManager.mu.Lock()
	defer instanceManager.mu.Unlock()
	instanceManager.instances[inst.id] = inst
}

func (instanceManager *InstanceManager) RemoveInstance(id string) {
	instanceManager.mu.Lock()
	defer instanceManager.mu.Unlock()
	delete(instanceManager.instances, id)
}

func (instanceManager *InstanceManager) GetInstance(id string) *Instance {
	instanceManager.mu.Lock()
	defer instanceManager.mu.Unlock()
	return instanceManager.instances[id]
}

func (instanceManager *InstanceManager) ListInstances() []*Instance {
	instanceManager.mu.Lock()
	defer instanceManager.mu.Unlock()
	instances := make([]*Instance, 0, len(instanceManager.instances))
	for _, inst := range instanceManager.instances {
		instances = append(instances, inst)
	}
	return instances
}
