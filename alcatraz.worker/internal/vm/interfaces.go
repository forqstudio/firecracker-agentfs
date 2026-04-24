package vm

type VirtualMachineInfo interface {
	GetTapDev() string
	GetHostTapIP() string
	GetVMIP() string
	GetSubnet() string
	GetNFSPort() int
	GetSocket() string
	GetAgentID() string
	GetID() string
	GetNFSProcess() NFSProcess
}
