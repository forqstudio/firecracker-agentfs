package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"alcatraz.worker/internal/config"
	"alcatraz.worker/internal/nats"
	"alcatraz.worker/internal/vm"
)

func main() {
	configuration, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	mgr := vm.NewInstanceManager(configuration.MaxVMs)

	handler := func(req *config.VMRequest) error {
		opts := &vm.SpawnOptions{
			AgentfsBin:     configuration.AgentfsBin,
			FirecrackerBin: configuration.FirecrackerBin,
			Rootfs:         configuration.Rootfs,
			Kernel:         configuration.Kernel,
			AgentfsDir:     configuration.AgentfsDir,
		}

		ctx := context.Background()
		_, err := vm.Spawn(ctx, mgr, req, opts)
		return err
	}

	subcriber, err := nats.NewSubscriber(configuration.NATSURL, configuration.Subject, configuration.QueueGroup, handler)
	if err != nil {
		log.Fatalf("Failed to create subscriber: %v", err)
	}

	if err := subcriber.Start(); err != nil {
		log.Fatalf("Failed to start subscriber: %v", err)
	}

	log.Printf("Alcatraz Worker started, connected to %s", subcriber.URL())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	subcriber.Stop()
}
