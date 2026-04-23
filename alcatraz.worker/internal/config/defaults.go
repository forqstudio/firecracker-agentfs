package config

const (
	EnvFile = ".env"

	DefaultNATSURL    = "nats://localhost:4222"
	DefaultSubject    = "vm.spawn"
	DefaultMaxVMs     = 5
	DefaultQueueGroup = "vm-workers"

	FirecrackerBin = "../alcatraz.core/bin/firecracker-v1.15.1"
	KernelPath     = "../alcatraz.core/linux-amazon/vmlinux"
	RootfsPath     = "../alcatraz.core/rootfs"
	AgentfsDir     = "../alcatraz.core/.agentfs"
	AgentfsBin     = "/home/dev/.cargo/bin/agentfs"

	BaseTapDev    = "fc-tap"
	BaseHostTapIP = "172.16.0.1"
	BaseVMIP      = "172.16.0.2"
	BaseNFSPort   = 11111

	VMHostname = "alcatraz"
	GuestMAC   = "AA:FC:00:00:00:01"

	DefaultVCPUs      = 4
	DefaultMemMib     = 8192
	DefaultKernelArgs = "loglevel=7 printk.devkmsg=on"
)