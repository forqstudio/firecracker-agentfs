package vm

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"

	"alcatraz.worker/internal/config"
)

type Instance struct {
	ID         string
	VCPUs      int
	MemoryMib  int
	KernelArgs string
	Index      int

	TapDev    string
	HostTapIP string
	VMIP      string
	Subnet    string
	NFSPort   int
	Socket    string
	AgentID   string

	Machine    *firecracker.Machine
	NFSProc    *exec.Cmd
	Config     firecracker.Config
	MachineCfg *models.MachineConfiguration

	mutex sync.Mutex
}

type InstanceManager struct {
	mutex     sync.Mutex
	instances map[string]*Instance
	pool      []int
	maxVMs    int
}

func NewInstanceManager(maxVMs int) *InstanceManager {
	pool := make([]int, maxVMs)
	for i := range pool {
		pool[i] = i
	}
	return &InstanceManager{
		instances: make(map[string]*Instance),
		pool:      pool,
		maxVMs:    maxVMs,
	}
}

func (instanceManager *InstanceManager) Allocate() (int, error) {
	instanceManager.mutex.Lock()
	defer instanceManager.mutex.Unlock()
	if len(instanceManager.pool) == 0 {
		return 0, fmt.Errorf("no available VM slots (max %d)", instanceManager.maxVMs)
	}
	idx := instanceManager.pool[0]
	instanceManager.pool = instanceManager.pool[1:]
	return idx, nil
}

func (instanceManager *InstanceManager) Release(idx int) {
	instanceManager.mutex.Lock()
	defer instanceManager.mutex.Unlock()
	instanceManager.pool = append(instanceManager.pool, idx)
}

func (instanceManager *InstanceManager) AddInstance(inst *Instance) {
	instanceManager.mutex.Lock()
	defer instanceManager.mutex.Unlock()
	instanceManager.instances[inst.ID] = inst
}

func (instanceManager *InstanceManager) RemoveInstance(id string) {
	instanceManager.mutex.Lock()
	defer instanceManager.mutex.Unlock()
	delete(instanceManager.instances, id)
}

func (instanceManager *InstanceManager) GetInstance(id string) *Instance {
	instanceManager.mutex.Lock()
	defer instanceManager.mutex.Unlock()
	return instanceManager.instances[id]
}

func (instanceManager *InstanceManager) ListInstances() []*Instance {
	instanceManager.mutex.Lock()
	defer instanceManager.mutex.Unlock()
	instances := make([]*Instance, 0, len(instanceManager.instances))
	for _, inst := range instanceManager.instances {
		instances = append(instances, inst)
	}
	return instances
}

func FormatHostTapIP(index int) string {
	return fmt.Sprintf("172.16.%d.1", index)
}

func FormatVMIP(index int) string {
	return fmt.Sprintf("172.16.%d.2", index)
}

func FormatSubnet(index int) string {
	return fmt.Sprintf("172.16.%d.0/24", index)
}

func FormatNFSPort(index int) int {
	return config.BaseNFSPort + index
}

func FormatTapDev(index int) string {
	return fmt.Sprintf("%s%d", config.BaseTapDev, index)
}

func FormatSocket(agentfsDirectory, virtualMachineId string) string {
	return filepath.Join(agentfsDirectory, fmt.Sprintf("fc-%s.sock", virtualMachineId))
}
