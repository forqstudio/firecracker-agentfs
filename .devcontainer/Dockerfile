FROM ubuntu:24.04

# Metadata for better tracking and reproducibility
LABEL maintainer="Firecracker AgentFS Project"
LABEL description="Dev container for Firecracker microVM with AgentFS"
LABEL version="1.0.0"

ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=1000

RUN apt-get update && apt-get install -y \
    build-essential flex bison libssl-dev libelf-dev bc kmod cpio rsync tar xz-utils curl git sudo \
    libkmod-dev pkg-config llvm-dev \
    && rm -rf /var/lib/apt/lists/*

# Install Rust toolchain
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y \
    && . "$HOME/.cargo/env" \
    && rustup default stable \
    && cargo --version \
    && rustc --version

# Ensure cargo is available in PATH for all users
ENV PATH="/root/.cargo/bin:${PATH}"

RUN groupadd -g ${USER_GID} ${USERNAME} \
    && useradd -m -u ${USER_UID} -g ${USERNAME} -G kvm,sudo ${USERNAME}

RUN curl -L --connect-timeout 10 -o /tmp/firecracker.tgz "https://github.com/firecracker-microvm/firecracker/releases/download/v1.15.1/firecracker-v1.15.1-x86_64.tgz" \
    && tar xf /tmp/firecracker.tgz -C /tmp \
    && mv "/tmp/release-v1.15.1-x86_64/firecracker-v1.15.1-x86_64" /usr/local/bin/firecracker \
    && rm -rf /tmp/release-* /tmp/firecracker.tgz \
    && chmod +x /usr/local/bin/firecracker

# Install AgentFS - properly install the Rust binary
RUN cargo install agentfs \
    && mv ~/.cargo/bin/agentfs /usr/local/bin/agentfs \
    && chmod +x /usr/local/bin/agentfs \
    && agentfs --version

# Install opencode - AI-powered coding assistant
RUN curl -fsSL https://api.opencode.ai/install | sh \
    && mv ~/.local/bin/opencode /usr/local/bin/opencode \
    && chmod +x /usr/local/bin/opencode \
    && opencode --version



# Set up environment variables for the project
ENV FIRECRACKER_PATH="/usr/local/bin/firecracker"
ENV AGENTFS_PATH="/usr/local/bin/agentfs"
ENV OPCODE_PATH="/usr/local/bin/opencode"
ENV FIRECRACKER_VERSION="v1.15.1"
ENV AGENTFS_VERSION="0.6.4"
ENV AGENTFS_DB_PATH="/home/${USERNAME}/.agentfs"


COPY --chown=${USERNAME}:${USERNAME} . /home/${USERNAME}/workspaces/firecracker-agentfs

WORKDIR /home/${USERNAME}/workspaces/firecracker-agentfs

USER ${USERNAME}
