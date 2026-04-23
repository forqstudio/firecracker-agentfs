package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	NATSURL        string
	Subject        string
	MaxVMs         int
	QueueGroup     string
	AgentfsBin     string
	AgentfsDir     string
	FirecrackerBin string
	Rootfs         string
	Kernel         string
}

func Load() (*Config, error) {
	if err := godotenv.Load(EnvFile); err != nil {
		return nil, err
	}

	cfg := &Config{}

	if v := os.Getenv("NATS_URL"); v != "" {
		cfg.NATSURL = v
	}
	if v := os.Getenv("NATS_SUBJECT"); v != "" {
		cfg.Subject = v
	}
	if v := os.Getenv("MAX_VMS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxVMs = n
		}
	}
	if v := os.Getenv("QUEUE_GROUP"); v != "" {
		cfg.QueueGroup = v
	}
	if v := os.Getenv("FIRECRACKER_BIN"); v != "" {
		cfg.FirecrackerBin = v
	}
	if v := os.Getenv("KERNEL_PATH"); v != "" {
		cfg.Kernel = v
	}
	if v := os.Getenv("ROOTFS"); v != "" {
		cfg.Rootfs = v
	}
	if v := os.Getenv("AGENTFS_DIR"); v != "" {
		cfg.AgentfsDir = v
	}
	if v := os.Getenv("AGENTFS_BIN"); v != "" {
		cfg.AgentfsBin = v
	}

	return cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		NATSURL:        DefaultNATSURL,
		Subject:        DefaultSubject,
		MaxVMs:         DefaultMaxVMs,
		QueueGroup:     DefaultQueueGroup,
		AgentfsBin:     AgentfsBin,
		FirecrackerBin: FirecrackerBin,
		Rootfs:         RootfsPath,
		Kernel:         KernelPath,
		AgentfsDir:     AgentfsDir,
	}
}
