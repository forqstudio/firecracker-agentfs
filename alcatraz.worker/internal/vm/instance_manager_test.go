package vm

import (
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
	if mgr.pool.Len() != 3 {
		t.Errorf("expected pool size 3, got %d", mgr.pool.Len())
	}
}

func TestInstanceManagerAllocate(t *testing.T) {
	mgr := NewInstanceManager(3)

	idx, err := mgr.Allocate()
	if err != nil {
		t.Fatalf("Allocate failed: %v", err)
	}
	if idx < 0 || idx >= 3 {
		t.Errorf("expected index 0-2, got %d", idx)
	}

	idx2, err := mgr.Allocate()
	if err != nil {
		t.Fatalf("Allocate failed: %v", err)
	}
	if idx2 == idx {
		t.Errorf("expected different index, got %d", idx2)
	}
}

func TestInstanceManagerAllocateExhausted(t *testing.T) {
	mgr := NewInstanceManager(2)

	_, err := mgr.Allocate()
	if err != nil {
		t.Fatalf("Allocate failed: %v", err)
	}
	_, err = mgr.Allocate()
	if err != nil {
		t.Fatalf("Allocate failed: %v", err)
	}

	_, err = mgr.Allocate()
	if err == nil {
		t.Fatal("expected error when pool exhausted")
	}
}

func TestInstanceManagerRelease(t *testing.T) {
	mgr := NewInstanceManager(2)

	idx1, _ := mgr.Allocate()
	if idx1 != 0 {
		t.Errorf("expected first allocation to be 0, got %d", idx1)
	}

	mgr.Release(idx1)

	idx2, err := mgr.Allocate()
	if err != nil {
		t.Fatalf("Allocate after release failed: %v", err)
	}
	if idx2 != 1 {
		t.Errorf("expected second allocation to be 1, got %d", idx2)
	}
}

func TestInstanceManagerAddRemove(t *testing.T) {
	mgr := NewInstanceManager(5)

	inst := NewInstance(WithID("test-vm-1"))
	mgr.AddInstance(inst)

	if mgr.GetInstance("test-vm-1") == nil {
		t.Error("expected to find instance after AddInstance")
	}

	mgr.RemoveInstance("test-vm-1")
	if mgr.GetInstance("test-vm-1") != nil {
		t.Error("expected instance to be nil after RemoveInstance")
	}
}
