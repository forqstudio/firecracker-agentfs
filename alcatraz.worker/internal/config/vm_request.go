package config

import (
	"github.com/google/uuid"
)

type VMRequest struct {
	ID         string `json:"id,omitempty"`
	VCPUs      int    `json:"vcpus,omitempty"`
	MemoryMib  int    `json:"memory_mib,omitempty"`
	KernelArgs string `json:"kernel_args,omitempty"`
}

func (r *VMRequest) WithDefaults() *VMRequest {
	r.Validate()
	return r
}

func (r *VMRequest) Validate() error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	if r.VCPUs <= 0 {
		r.VCPUs = DefaultVCPUs
	}
	if r.MemoryMib <= 0 {
		r.MemoryMib = DefaultMemMib
	}
	if r.KernelArgs == "" {
		r.KernelArgs = DefaultKernelArgs
	}
	return nil
}
