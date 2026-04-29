# CNI Integration via Firecracker Go SDK

## Overview

This document describes the integration of CNI-based networking using the Firecracker Go SDK's built-in CNI support. The SDK automatically handles CNI invocation, TAP device creation, and network namespace setup.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     alcatraz-worker                         │
│  ┌─────────────┐    ┌──────────────────────────────────┐   │
│  │  VM Request │───▶│  Firecracker Go SDK             │   │
│  └─────────────┘    │  (CNIConfiguration)             │   │
│                      └──────────────┬───────────────────┘   │
│                                     │                       │
│                                     ▼                       │
│                      ┌──────────────────────────────────┐   │
│                      │  CNI Plugins (tc-redirect-tap)  │   │
│                      │  - bridge                       │   │
│                      │  - host-local (IPAM)            │   │
│                      │  - tc-redirect-tap              │   │
│                      └──────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Key Components

### 1. CNI Configuration (`cni/alcatraz-bridge.conflist`)

Uses CNI list format with chained plugins:
- **bridge**: Creates a bridge device (`alcatraz0`)
- **host-local**: Handles IP address management
- **tc-redirect-tap**: Creates TAP device for Firecracker VM

```json
{
  "cniVersion": "0.4.0",
  "name": "alcatraz-bridge",
  "plugins": [
    {
      "type": "bridge",
      "bridge": "alcatraz0",
      "ipMasq": true,
      "ipam": {
        "type": "host-local",
        "subnet": "172.16.0.0/24",
        "rangeStart": "172.16.0.10",
        "rangeEnd": "172.16.0.250"
      }
    },
    {
      "type": "tc-redirect-tap"
    }
  ]
}
```

### 2. VM Configuration (`internal/vm/vm.go`)

Uses `CNIConfiguration` instead of manual network setup:

```go
cfg := firecracker.Config{
    ...
    NetworkInterfaces: []firecracker.NetworkInterface{
        {
            CNIConfiguration: &firecracker.CNIConfiguration{
                NetworkName: "alcatraz-bridge",
                IfName:      tapDev,      // e.g., "fc-tap0"
                VMIfName:    "eth0",      // interface name inside VM
                ConfDir:     "/etc/cni/net.d",
                BinPath:     []string{"/opt/cni/bin"},
            },
        },
    },
    VMID: instance.id,
}
```

The SDK automatically:
- Invokes CNI plugins
- Creates network namespace
- Sets up TAP device
- Configures IP addresses
- Generates `ip=` kernel boot parameter

## Implementation Changes

### Files Removed
| File | Reason |
|------|--------|
| `internal/vm/network.go` | Manual network setup replaced by CNI |
| `pkg/cni/cni.go` | Custom IPAM replaced by CNI host-local |

### Files Modified
| File | Change |
|------|--------|
| `internal/vm/vm.go` | Use `CNIConfiguration`, remove `SetupTAPDev()`, `SetupNAT()`, `SetupIsolationRules()` |
| `internal/vm/instance_manager.go` | Remove `ipamClient`, `AllocateNetwork()`, `ReleaseNetwork()` |

### Flow: VM Spawn (New)

```
1. Allocate index from pool
2. Configure VM with CNIConfiguration
3. Prepare AgentFS overlay
4. Start AgentFS NFS
5. SDK invokes CNI, creates TAP, starts VM
6. CNI automatically handles:
   - Network namespace creation
   - TAP device creation (via tc-redirect-tap)
   - IP allocation (via host-local)
   - NAT/masquerade (via bridge plugin)
```

### Flow: VM Cleanup (New)

```
1. VM exits
2. SDK automatically invokes CNI DEL
3. CNI cleans up:
   - TAP device removal
   - Network namespace cleanup
   - IP release
4. Release index back to pool
```

## Prerequisites

### Required CNI Plugins

Install to `/opt/cni/bin`:

1. **Standard CNI plugins** (bridge, host-local, etc.):
   ```bash
   curl -sL https://github.com/containernetworking/plugins/releases/download/v1.0.1/cni-plugins-linux-amd64-v1.0.1.tgz | \
     tar -xz -C /opt/cni/bin/
   ```

2. **tc-redirect-tap** (Firecracker-specific):
   ```bash
   git clone https://github.com/firecracker-microvm/tc-redirect-tap.git
   cd tc-redirect-tap
   make
   cp tc-redirect-tap /opt/cni/bin/
   ```

### CNI Configuration

Install the CNI config file:
```bash
mkdir -p /etc/cni/net.d
cp cni/alcatraz-bridge.conflist /etc/cni/net.d/
```

## Benefits

| Before | After |
|--------|-------|
| Manual iptables/ip commands | CNI plugins handle networking |
| Custom IPAM implementation | Standard CNI host-local IPAM |
| Manual TAP device creation | tc-redirect-tap creates TAP automatically |
| No network namespace support | CNI manages network namespaces |
| Custom cleanup code | CNI DEL automatically cleans up |
| Firecracker Go SDK | Firecracker Go SDK (CNIConfiguration) |

## Kernel Boot Arguments

The SDK automatically generates the `ip=` parameter from CNI results. For NFS root, manually specify NFS options:

```go
bootArgs := fmt.Sprintf(
    "console=ttyS0 reboot=k panic=1 pci=off %s root=/dev/nfs nfsroot=${GATEWAY_IP}:/,nfsvers=3,tcp,nolock,port=%d,mountport=%d rw init=/init",
    instance.kernelArgs,
    instance.nfsPort,
    instance.nfsPort,
)
```

Note: `${GATEWAY_IP}` is replaced by the SDK with the gateway IP from CNI result.

## Deployment

```bash
# Build
go build -o bin/alcatraz-worker ./cmd/alcatraz-worker

# Run (requires CAP_NET_ADMIN, CAP_SYS_ADMIN)
sudo ./bin/alcatraz-worker
```
