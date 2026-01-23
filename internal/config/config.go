package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Auth       AuthConfig       `yaml:"auth"`
	Resources  ResourceConfig   `yaml:"resources"`
	Timeouts   TimeoutConfig    `yaml:"timeouts"`
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
	cfg.Timeouts.StartupTimeout = 60
}

// overrideFromEnv overrides config with environment variables
func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("AGENTBOX_PORT"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &cfg.Server.Port); err != nil {
			// Invalid port value, keep default
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
		if _, err := fmt.Sscanf(v, "%d", &cfg.Resources.MaxEnvironmentsPerUser); err != nil {
			// Invalid value, keep default
		}
	}
	if v := os.Getenv("AGENTBOX_DEFAULT_TIMEOUT"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &cfg.Timeouts.DefaultTimeout); err != nil {
			// Invalid value, keep default
		}
	}
	if v := os.Getenv("AGENTBOX_MAX_TIMEOUT"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &cfg.Timeouts.MaxTimeout); err != nil {
			// Invalid value, keep default
		}
	}
	if v := os.Getenv("AGENTBOX_STARTUP_TIMEOUT"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &cfg.Timeouts.StartupTimeout); err != nil {
			// Invalid value, keep default
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

	return nil
}
