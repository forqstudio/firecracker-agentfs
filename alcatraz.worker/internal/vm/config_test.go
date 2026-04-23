package vm

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxVMs != DefaultMaxVMs {
		t.Errorf("MaxVMs = %d, want %d", cfg.MaxVMs, DefaultMaxVMs)
	}
	if cfg.AgentfsBin != AgentfsBin {
		t.Errorf("AgentfsBin = %s, want %s", cfg.AgentfsBin, AgentfsBin)
	}
	if cfg.FirecrackerBin != FirecrackerBin {
		t.Errorf("FirecrackerBin = %s, want %s", cfg.FirecrackerBin, FirecrackerBin)
	}
}

func TestRequestValidate(t *testing.T) {
	tests := []struct {
		name      string
		req       CreateVMInput
		wantID    bool
		wantVCPUs int
		wantMem   int
	}{
		{
			name:      "empty request gets defaults",
			req:       CreateVMInput{},
			wantID:    true,
			wantVCPUs: DefaultVCPUs,
			wantMem:   DefaultMemMib,
		},
		{
			name: "partial request",
			req: CreateVMInput{
				VCPUs: 8,
			},
			wantVCPUs: 8,
			wantMem:   DefaultMemMib,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if tt.wantID && tt.req.ID == "" {
				t.Error("expected ID to be set")
			}
			if tt.req.VCPUs != tt.wantVCPUs {
				t.Errorf("VCPUs = %d, want %d", tt.req.VCPUs, tt.wantVCPUs)
			}
			if tt.req.MemoryMib != tt.wantMem {
				t.Errorf("MemoryMib = %d, want %d", tt.req.MemoryMib, tt.wantMem)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	if DefaultVCPUs <= 0 {
		t.Error("DefaultVCPUs should be positive")
	}
	if DefaultMemMib <= 0 {
		t.Error("DefaultMemMib should be positive")
	}
	if BaseNFSPort <= 0 {
		t.Error("BaseNFSPort should be positive")
	}
}
