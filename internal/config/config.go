package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Server         ServerConfig         `yaml:"server"`
	Kubernetes     KubernetesConfig     `yaml:"kubernetes"`
	Auth           AuthConfig           `yaml:"auth"`
	Resources      ResourceConfig       `yaml:"resources"`
	Timeouts       TimeoutConfig        `yaml:"timeouts"`
	Pool           PoolConfig           `yaml:"pool"`
	Reconciliation ReconciliationConfig `yaml:"reconciliation"`
}

// ReconciliationConfig holds reconciliation loop settings
type ReconciliationConfig struct {
	// IntervalSeconds is how often the reconciliation loop runs (default: 60)
	IntervalSeconds int `yaml:"interval_seconds"`
	// MaxRetries is the maximum number of reconciliation attempts for a failed/pending environment before marking as failed (default: 5)
	MaxRetries int `yaml:"max_retries"`
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

	// Reconciliation defaults
	cfg.Reconciliation.IntervalSeconds = 60
	cfg.Reconciliation.MaxRetries = 5
}

// overrideFromEnv overrides config with environment variables
func overrideFromEnv(cfg *Config) {
	overrideServerFromEnv(&cfg.Server)
	overrideKubernetesFromEnv(&cfg.Kubernetes)
	overrideAuthFromEnv(&cfg.Auth)
	overrideResourcesFromEnv(&cfg.Resources)
	overrideTimeoutsFromEnv(&cfg.Timeouts)
	overridePoolFromEnv(&cfg.Pool)
	overrideReconciliationFromEnv(&cfg.Reconciliation)
}

// overrideServerFromEnv overrides server config from environment variables
func overrideServerFromEnv(cfg *ServerConfig) {
	if v := os.Getenv("AGENTBOX_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
		}
	}
	if v := os.Getenv("AGENTBOX_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("AGENTBOX_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
}

// overrideKubernetesFromEnv overrides Kubernetes config from environment variables
func overrideKubernetesFromEnv(cfg *KubernetesConfig) {
	if v := os.Getenv("AGENTBOX_KUBECONFIG"); v != "" {
		cfg.Kubeconfig = v
	}
	if v := os.Getenv("AGENTBOX_NAMESPACE_PREFIX"); v != "" {
		cfg.NamespacePrefix = v
	}
	if v := os.Getenv("AGENTBOX_RUNTIME_CLASS"); v != "" {
		cfg.RuntimeClass = v
	}
}

// overrideAuthFromEnv overrides auth config from environment variables
func overrideAuthFromEnv(cfg *AuthConfig) {
	if v := os.Getenv("AGENTBOX_AUTH_ENABLED"); v != "" {
		cfg.Enabled = v == "true"
	}
	if v := os.Getenv("AGENTBOX_AUTH_SECRET"); v != "" {
		cfg.Secret = v
	}
}

// overrideResourcesFromEnv overrides resources config from environment variables
func overrideResourcesFromEnv(cfg *ResourceConfig) {
	if v := os.Getenv("AGENTBOX_DEFAULT_CPU_LIMIT"); v != "" {
		cfg.DefaultCPULimit = v
	}
	if v := os.Getenv("AGENTBOX_DEFAULT_MEMORY_LIMIT"); v != "" {
		cfg.DefaultMemoryLimit = v
	}
	if v := os.Getenv("AGENTBOX_DEFAULT_STORAGE_LIMIT"); v != "" {
		cfg.DefaultStorageLimit = v
	}
	if v := os.Getenv("AGENTBOX_MAX_ENVIRONMENTS_PER_USER"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.MaxEnvironmentsPerUser = val
		}
	}
}

// overrideTimeoutsFromEnv overrides timeouts config from environment variables
func overrideTimeoutsFromEnv(cfg *TimeoutConfig) {
	if v := os.Getenv("AGENTBOX_DEFAULT_TIMEOUT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.DefaultTimeout = val
		}
	}
	if v := os.Getenv("AGENTBOX_MAX_TIMEOUT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.MaxTimeout = val
		}
	}
	if v := os.Getenv("AGENTBOX_STARTUP_TIMEOUT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.StartupTimeout = val
		}
	}
}

// overridePoolFromEnv overrides pool config from environment variables
func overridePoolFromEnv(cfg *PoolConfig) {
	if v := os.Getenv("AGENTBOX_POOL_ENABLED"); v != "" {
		cfg.Enabled = v == "true"
	}
	if v := os.Getenv("AGENTBOX_POOL_SIZE"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val >= 0 {
			cfg.Size = val
		}
	}
	if v := os.Getenv("AGENTBOX_POOL_DEFAULT_IMAGE"); v != "" {
		cfg.DefaultImage = v
	}
	if v := os.Getenv("AGENTBOX_POOL_DEFAULT_CPU"); v != "" {
		cfg.DefaultCPU = v
	}
	if v := os.Getenv("AGENTBOX_POOL_DEFAULT_MEMORY"); v != "" {
		cfg.DefaultMemory = v
	}
}

// overrideReconciliationFromEnv overrides reconciliation config from environment variables
func overrideReconciliationFromEnv(cfg *ReconciliationConfig) {
	if v := os.Getenv("AGENTBOX_RECONCILIATION_INTERVAL_SECONDS"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val > 0 {
			cfg.IntervalSeconds = val
		}
	}
	if v := os.Getenv("AGENTBOX_RECONCILIATION_MAX_RETRIES"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val >= 0 {
			cfg.MaxRetries = val
		}
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

	if cfg.Reconciliation.IntervalSeconds < 10 {
		return fmt.Errorf("reconciliation interval_seconds must be at least 10, got %d", cfg.Reconciliation.IntervalSeconds)
	}
	if cfg.Reconciliation.MaxRetries < 0 {
		return fmt.Errorf("reconciliation max_retries must be >= 0, got %d", cfg.Reconciliation.MaxRetries)
	}

	return nil
}
