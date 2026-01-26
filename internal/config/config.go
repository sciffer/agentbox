package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Auth       AuthConfig       `yaml:"auth"`
	Resources  ResourceConfig   `yaml:"resources"`
	Timeouts   TimeoutConfig    `yaml:"timeouts"`
	Pool       PoolConfig       `yaml:"pool"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port     int    `yaml:"port"`
	Host     string `yaml:"host"`
	LogLevel string `yaml:"log_level"`
}

// KubernetesConfig holds Kubernetes connection configuration
type KubernetesConfig struct {
	Kubeconfig      string `yaml:"kubeconfig"`
	NamespacePrefix string `yaml:"namespace_prefix"`
	RuntimeClass    string `yaml:"runtime_class"`
}

// PoolConfig holds standby pod pool configuration
type PoolConfig struct {
	// Enabled enables the standby pod pool for faster execution startup
	Enabled bool `yaml:"enabled"`
	// Size is the number of standby pods to maintain per image
	Size int `yaml:"size"`
	// DefaultImage is the default image for standby pods when no environment is specified
	DefaultImage string `yaml:"default_image"`
	// DefaultCPU is the CPU limit for standby pods
	DefaultCPU string `yaml:"default_cpu"`
	// DefaultMemory is the memory limit for standby pods
	DefaultMemory string `yaml:"default_memory"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled bool   `yaml:"enabled"`
	Secret  string `yaml:"secret"`
}

// ResourceConfig holds default resource limits
type ResourceConfig struct {
	DefaultCPULimit        string `yaml:"default_cpu_limit"`
	DefaultMemoryLimit     string `yaml:"default_memory_limit"`
	DefaultStorageLimit    string `yaml:"default_storage_limit"`
	MaxEnvironmentsPerUser int    `yaml:"max_environments_per_user"`
}

// TimeoutConfig holds timeout settings
type TimeoutConfig struct {
	DefaultTimeout int `yaml:"default_timeout"`
	MaxTimeout     int `yaml:"max_timeout"`
	StartupTimeout int `yaml:"startup_timeout"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	cfg := &Config{}

	// Set defaults
	setDefaults(cfg)

	// Load from file if provided
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Override with environment variables
	overrideFromEnv(cfg)

	// Validate configuration
	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(cfg *Config) {
	cfg.Server.Port = 8080
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.LogLevel = "info"

	cfg.Kubernetes.NamespacePrefix = "agentbox-"
	cfg.Kubernetes.RuntimeClass = "gvisor"

	cfg.Auth.Enabled = true

	cfg.Resources.DefaultCPULimit = "1000m"
	cfg.Resources.DefaultMemoryLimit = "1Gi"
	cfg.Resources.DefaultStorageLimit = "5Gi"
	cfg.Resources.MaxEnvironmentsPerUser = 100

	cfg.Timeouts.DefaultTimeout = 3600
	cfg.Timeouts.MaxTimeout = 86400
	cfg.Timeouts.StartupTimeout = 120 // 2 minutes to allow for image pulls

	// Pool defaults (disabled by default)
	cfg.Pool.Enabled = false
	cfg.Pool.Size = 2
	cfg.Pool.DefaultImage = "python:3.11-slim"
	cfg.Pool.DefaultCPU = "500m"
	cfg.Pool.DefaultMemory = "512Mi"
}

// overrideFromEnv overrides config with environment variables
func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("AGENTBOX_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("AGENTBOX_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("AGENTBOX_LOG_LEVEL"); v != "" {
		cfg.Server.LogLevel = v
	}
	if v := os.Getenv("AGENTBOX_KUBECONFIG"); v != "" {
		cfg.Kubernetes.Kubeconfig = v
	}
	if v := os.Getenv("AGENTBOX_NAMESPACE_PREFIX"); v != "" {
		cfg.Kubernetes.NamespacePrefix = v
	}
	if v := os.Getenv("AGENTBOX_RUNTIME_CLASS"); v != "" {
		cfg.Kubernetes.RuntimeClass = v
	}
	if v := os.Getenv("AGENTBOX_AUTH_ENABLED"); v != "" {
		cfg.Auth.Enabled = v == "true"
	}
	if v := os.Getenv("AGENTBOX_AUTH_SECRET"); v != "" {
		cfg.Auth.Secret = v
	}
	if v := os.Getenv("AGENTBOX_DEFAULT_CPU_LIMIT"); v != "" {
		cfg.Resources.DefaultCPULimit = v
	}
	if v := os.Getenv("AGENTBOX_DEFAULT_MEMORY_LIMIT"); v != "" {
		cfg.Resources.DefaultMemoryLimit = v
	}
	if v := os.Getenv("AGENTBOX_DEFAULT_STORAGE_LIMIT"); v != "" {
		cfg.Resources.DefaultStorageLimit = v
	}
	if v := os.Getenv("AGENTBOX_MAX_ENVIRONMENTS_PER_USER"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.Resources.MaxEnvironmentsPerUser = val
		}
	}
	if v := os.Getenv("AGENTBOX_DEFAULT_TIMEOUT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.Timeouts.DefaultTimeout = val
		}
	}
	if v := os.Getenv("AGENTBOX_MAX_TIMEOUT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.Timeouts.MaxTimeout = val
		}
	}
	if v := os.Getenv("AGENTBOX_STARTUP_TIMEOUT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.Timeouts.StartupTimeout = val
		}
	}
	// Pool settings
	if v := os.Getenv("AGENTBOX_POOL_ENABLED"); v != "" {
		cfg.Pool.Enabled = v == "true"
	}
	if v := os.Getenv("AGENTBOX_POOL_SIZE"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val >= 0 {
			cfg.Pool.Size = val
		}
	}
	if v := os.Getenv("AGENTBOX_POOL_DEFAULT_IMAGE"); v != "" {
		cfg.Pool.DefaultImage = v
	}
	if v := os.Getenv("AGENTBOX_POOL_DEFAULT_CPU"); v != "" {
		cfg.Pool.DefaultCPU = v
	}
	if v := os.Getenv("AGENTBOX_POOL_DEFAULT_MEMORY"); v != "" {
		cfg.Pool.DefaultMemory = v
	}
}

// validate checks if the configuration is valid
func validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", cfg.Server.Port)
	}

	if cfg.Kubernetes.NamespacePrefix == "" {
		return fmt.Errorf("namespace prefix cannot be empty")
	}

	if cfg.Auth.Enabled && cfg.Auth.Secret == "" {
		return fmt.Errorf("auth secret is required when auth is enabled")
	}

	if cfg.Timeouts.MaxTimeout < cfg.Timeouts.DefaultTimeout {
		return fmt.Errorf("max timeout cannot be less than default timeout")
	}

	return nil
}
