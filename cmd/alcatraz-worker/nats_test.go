package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestNATSConnection(t *testing.T) {
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		t.Skipf("NATS not available: %v", err)
	}
	defer nc.Close()

	sub, err := nc.SubscribeSync("test.worker")
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	testMsg := []byte("test message")
	if err := nc.Publish("test.worker", testMsg); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	msg, err := sub.NextMsg(2 * time.Second)
	if err != nil {
		t.Fatalf("nextmsg failed: %v", err)
	}

	if string(msg.Data) != string(testMsg) {
		t.Errorf("got %q, want %q", msg.Data, testMsg)
	}
	sub.Unsubscribe()
}

func TestNATSPublishRequest(t *testing.T) {
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		t.Skipf("NATS not available: %v", err)
	}
	defer nc.Close()

	req := VMRequest{
		VCPUs:     2,
		MemoryMib: 4096,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if err := nc.Publish("vm.spawn", data); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	if err := nc.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
}