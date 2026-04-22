# Alcatraz

A sanboxed environment for letting your coding agents run wild.

## Overview

Alcatraz spins up ephemeral Firecracker microVMs on demand. Each VM backs its root filesystem with AgentFS overlay on NFS, so agent changes persist across restarts while keeping the base image clean.



- **CLI** runs on user's machine, talks to API
- **API** handles authentication via Keycloak, sends spawn requests to NATS
- **Worker** runs on VM server(s), manages VM lifecycle and listens to NATS 
- **Keycloak** handles authentication

## Components

| Component | Description |
|-----------|-------------|
| [alcatraz.core](alcatraz.core/) | Firecracker microVM with AgentFS overlay |
| [alcatraz.worker](alcatraz.worker/) | NATS-driven dynamic VM spawner |
| alcatraz.api | Stateless API for auth + VM coordination ([TODO](#alcatrazapi)) |
| alcatraz.cli | User Interface ([TODO](#alcatrazcli)) |

### alcatraz.core

See [alcatraz.core/README.md](alcatraz.core/README.md) for details.

### alcatraz.worker

See [alcatraz.worker/README.md](alcatraz.worker/README.md) for details.

### alcatraz.api (TODO)

Stateless API that the CLI communicates with:

- **Authentication** via Keycloak device flow
- **VM coordination** - publishes to NATS

**API Endpoints:**

```
POST   /v1/sandboxes     - Create sandbox, returns connection info
GET    /v1/sandboxes     - List user's sandboxes
GET    /v1/sandboxes/:id - Get sandbox status
DELETE /v1/sandboxes/:id - Destroy sandbox
GET    /v1/auth/login    - Keycloak device flow initiation
GET    /v1/auth/me       - Current user info
```

### alcatraz.cli (TODO)

.NET CLI application that is the user's primary interface.

**Commands:**

```bash
alcatraz login                                   # Keycloak device flow login
alcatraz sandbox create                          # Create sandbox
alcatraz sandbox create --vcpus 4 --mem 8        # Custom resources
alcatraz sandbox list                            # List sandboxes
alcatraz sandbox status <id>                     # Get sandbox status
alcatraz sandbox destroy <id>                    # Destroy sandbox
```
