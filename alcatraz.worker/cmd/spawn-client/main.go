package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	messaging "github.com/nats-io/nats.go"
)

const (
	defaultURL     = "nats://localhost:4222"
	defaultSubject = "vm.spawn"
)

var (
	natsURL          string
	natsSubject      string
	virtualMachineId string
	vcpus            int
	memory           int
	kernelArgs       string
)

type VMRequest struct {
	ID         string `json:"id,omitempty"`
	VCPUs      int    `json:"vcpus,omitempty"`
	MemoryMib  int    `json:"memory_mib,omitempty"`
	KernelArgs string `json:"kernel_args,omitempty"`
}

func main() {
	flag.StringVar(&natsURL, "nats-url", defaultURL, "NATS server URL")
	flag.StringVar(&natsSubject, "subject", defaultSubject, "NATS subject to publish to")
	flag.StringVar(&virtualMachineId, "id", "", "VM ID (auto-generated if omitted)")
	flag.IntVar(&vcpus, "vcpus", 0, "vCPU count (default: 4)")
	flag.IntVar(&memory, "mem", 0, "Memory in MiB (default: 8192)")
	flag.StringVar(&kernelArgs, "kernel-args", "", "Kernel boot args")

	flag.Parse()

	if vcpus < 0 {
		vcpus = 0
	}
	if memory < 0 {
		memory = 0
	}

	vmRequest := VMRequest{
		ID:         virtualMachineId,
		VCPUs:      vcpus,
		MemoryMib:  memory,
		KernelArgs: kernelArgs,
	}

	data, err := json.Marshal(vmRequest)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}

	connection, err := messaging.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer connection.Close()

	if err := connection.Publish(natsSubject, data); err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}

	if err := connection.Flush(); err != nil {
		log.Fatalf("Failed to flush: %v", err)
	}

	fmt.Printf("Published spawn request to %s: %s\n", natsSubject, string(data))
}
