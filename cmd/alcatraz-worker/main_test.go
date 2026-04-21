package main

import (
	"fmt"
	"testing"
)

func TestNewInstanceManager(t *testing.T) {
	mgr := NewInstanceManager(3)
	if mgr == nil {
		t.Fatal("NewInstanceManager returned nil")
	}
	if mgr.maxVMs != 3 {
		t.Errorf("expected maxVMs 3, got %d", mgr.maxVMs)
	}
	if len(mgr.pool) != 3 {
		t.Errorf("expected pool size 3, got %d", len(mgr.pool))
	}
}

func TestInstanceManagerAllocate(t *testing.T) {
	mgr := NewInstanceManager(3)

	idx, err := mgr.allocate()
	if err != nil {
		t.Fatalf("allocate failed: %v", err)
	}
	if idx < 0 || idx >= 3 {
		t.Errorf("expected index 0-2, got %d", idx)
	}

	idx2, err := mgr.allocate()
	if err != nil {
		t.Fatalf("allocate failed: %v", err)
	}
	if idx2 == idx {
		t.Errorf("expected different index, got %d", idx2)
	}
}

func TestInstanceManagerAllocateExhausted(t *testing.T) {
	mgr := NewInstanceManager(2)

	_, err := mgr.allocate()
	if err != nil {
		t.Fatalf("allocate failed: %v", err)
	}
	_, err = mgr.allocate()
	if err != nil {
		t.Fatalf("allocate failed: %v", err)
	}

	_, err = mgr.allocate()
	if err == nil {
		t.Fatal("expected error when pool exhausted")
	}
}

func TestInstanceManagerRelease(t *testing.T) {
	mgr := NewInstanceManager(2)

	idx1, _ := mgr.allocate()
	if idx1 != 0 {
		t.Errorf("expected first allocation to be 0, got %d", idx1)
	}

	mgr.release(idx1)

	idx2, err := mgr.allocate()
	if err != nil {
		t.Fatalf("allocate after release failed: %v", err)
	}
	if idx2 != 1 {
		t.Errorf("expected second allocation to be 1, got %d", idx2)
	}
}

func TestIPAllocation(t *testing.T) {
	tests := []struct {
		idx     int
		wantIP  string
		wantNet string
	}{
		{0, "172.16.0.1", "172.16.0.0/24"},
		{1, "172.16.1.1", "172.16.1.0/24"},
		{2, "172.16.2.1", "172.16.2.0/24"},
		{4, "172.16.4.1", "172.16.4.0/24"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			gotIP := fmt.Sprintf("172.16.%d.1", tt.idx)
			if gotIP != tt.wantIP {
				t.Errorf("formatHostTapIP(%d) = %s, want %s", tt.idx, gotIP, tt.wantIP)
			}
			gotNet := fmt.Sprintf("172.16.%d.0/24", tt.idx)
			if gotNet != tt.wantNet {
				t.Errorf("formatSubnet(%d) = %s, want %s", tt.idx, gotNet, tt.wantNet)
			}
		})
	}
}

func TestNFSPortAllocation(t *testing.T) {
	for i := 0; i < 5; i++ {
		want := baseNFSPort + i
		got := baseNFSPort + i
		if got != want {
			t.Errorf("NFSPort(%d) = %d, want %d", i, got, want)
		}
	}
}

func TestTapDeviceName(t *testing.T) {
	for i := 0; i < 5; i++ {
		want := fmt.Sprintf("fc-tap%d", i)
		got := fmt.Sprintf("%s%d", baseTapDev, i)
		if got != want {
			t.Errorf("tapDev(%d) = %s, want %s", i, got, want)
		}
	}
}