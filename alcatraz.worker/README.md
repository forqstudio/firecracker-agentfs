## Alcatraz.Worker

The `alcatraz-worker` is a Go application that listens to NATS messages to spawn Firecracker VMs dynamically. It supports multiple concurrent VMs.

### Features

- Listens to NATS `vm.spawn` subject for VM requests
- Supports multiple concurrent VMs with isolated networking
- Auto-allocates TAP devices, IPs, and NFS ports
- Queue-based subscription for load balancing across workers
- Auto-cleanup on VM exit

### Build

```bash
cd alcatraz.worker
make build
```

### Run NATS

```bash
docker compose up -d
```

### Run alcatraz-worker

Must run as root:

```bash
sudo ./bin/alcatraz-worker
```

###CLI Flags

```bash
--nats-url string     NATS URL (default "nats://localhost:4222")
--subject string    NATS subject (default "vm.spawn")
--max-vms int       Max concurrent VMs (default 5)
--queue-group      NATS queue group (default "vm-workers")
--agentfs-bin      Path to agentfs binary
--firecracker-bin  Path to firecracker
--rootfs           Rootfs path
--kernel           Kernel path
```

### Spawn a VM

Using spawn-client:

```bash
./bin/spawn-client -vcpus 2 -mem 2048
./bin/spawn-client -id my-vm -vcpus 4
```

Or using nats CLI:

```bash
nats pub vm.spawn '{"vcpus": 2, "memory_mib": 2048}' --creds=none -
```

### VM Request Schema

```json
{
  "id": "optional-vm-id",
  "vcpus": 4,
  "memory_mib": 8192,
  "kernel_args": "quiet"
}
```

All fields are optional. Defaults: vcpus=4, memory_mib=8192, kernel_args="loglevel=7 printk.devkmsg=on"

### Network Allocation

Each VM gets allocated:

| Slot | TAP Device | Host IP   | Guest IP  | NFS Port |
|------|-----------|-----------|-----------|----------|
| 0    | fc-tap0   | 172.16.0.1 | 172.16.0.2 | 11111  |
| 1    | fc-tap1   | 172.16.1.1 | 172.16.1.2 | 11112  |
| 2    | fc-tap2   | 172.16.2.1 | 172.16.2.2 | 11113  |
| 3    | fc-tap3   | 172.16.3.1 | 172.16.3.2 | 11114  |
| 4    | fc-tap4   | 172.16.4.1 | 172.16.4.2 | 11115  |

### Connect to VM

```bash
ssh dev@172.16.0.2
```

Default password: `dev`

### Tests

```bash
cd alcatraz.worker
make test
```
