package unit

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sciffer/agentbox/internal/config"
)

func TestLoadConfig(t *testing.T) {
	t.Run("load with defaults", func(t *testing.T) {
		// Disable auth for default test since it requires a secret
		os.Setenv("AGENTBOX_AUTH_ENABLED", "false")
		defer os.Unsetenv("AGENTBOX_AUTH_ENABLED")
		
		cfg, err := config.Load("")
		require.NoError(t, err)

		assert.Equal(t, 8080, cfg.Server.Port)
		assert.Equal(t, "0.0.0.0", cfg.Server.Host)
		assert.Equal(t, "info", cfg.Server.LogLevel)
		assert.Equal(t, "agentbox-", cfg.Kubernetes.NamespacePrefix)
		assert.Equal(t, "gvisor", cfg.Kubernetes.RuntimeClass)
		assert.Equal(t, false, cfg.Auth.Enabled) // Disabled for test
		assert.Equal(t, "1000m", cfg.Resources.DefaultCPULimit)
		assert.Equal(t, "1Gi", cfg.Resources.DefaultMemoryLimit)
		assert.Equal(t, 3600, cfg.Timeouts.DefaultTimeout)
		assert.Equal(t, 86400, cfg.Timeouts.MaxTimeout)
	})

	t.Run("override with environment variables", func(t *testing.T) {
		os.Setenv("AGENTBOX_PORT", "9090")
		os.Setenv("AGENTBOX_LOG_LEVEL", "debug")
		os.Setenv("AGENTBOX_AUTH_ENABLED", "false")
		defer func() {
			os.Unsetenv("AGENTBOX_PORT")
			os.Unsetenv("AGENTBOX_LOG_LEVEL")
			os.Unsetenv("AGENTBOX_AUTH_ENABLED")
		}()

		cfg, err := config.Load("")
		require.NoError(t, err)

		assert.Equal(t, 9090, cfg.Server.Port)
		assert.Equal(t, "debug", cfg.Server.LogLevel)
		assert.Equal(t, false, cfg.Auth.Enabled)
	})

	t.Run("validation error - invalid port", func(t *testing.T) {
		os.Setenv("AGENTBOX_PORT", "99999")
		defer os.Unsetenv("AGENTBOX_PORT")

		_, err := config.Load("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})

	t.Run("validation error - auth enabled without secret", func(t *testing.T) {
		os.Setenv("AGENTBOX_AUTH_ENABLED", "true")
		os.Setenv("AGENTBOX_AUTH_SECRET", "")
		defer func() {
			os.Unsetenv("AGENTBOX_AUTH_ENABLED")
			os.Unsetenv("AGENTBOX_AUTH_SECRET")
		}()

		_, err := config.Load("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "auth secret is required")
	})

	t.Run("validation error - max timeout less than default", func(t *testing.T) {
		// Disable auth to avoid auth validation interfering
		os.Setenv("AGENTBOX_AUTH_ENABLED", "false")
		os.Setenv("AGENTBOX_DEFAULT_TIMEOUT", "10000")
		os.Setenv("AGENTBOX_MAX_TIMEOUT", "5000")
		defer func() {
			os.Unsetenv("AGENTBOX_AUTH_ENABLED")
			os.Unsetenv("AGENTBOX_DEFAULT_TIMEOUT")
			os.Unsetenv("AGENTBOX_MAX_TIMEOUT")
		}()

		_, err := config.Load("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max timeout cannot be less than default timeout")
	})
}

func TestConfigFromYAML(t *testing.T) {
	// Create temporary config file
	yamlContent := `
server:
  port: 9090
  host: "127.0.0.1"
  log_level: "debug"

kubernetes:
  namespace_prefix: "test-"
  runtime_class: "runsc"

auth:
  enabled: false

resources:
  default_cpu_limit: "2000m"
  default_memory_limit: "2Gi"

timeouts:
  default_timeout: 7200
  max_timeout: 14400
`

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(yamlContent))
	require.NoError(t, err)
	tmpfile.Close()

	cfg, err := config.Load(tmpfile.Name())
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, "debug", cfg.Server.LogLevel)
	assert.Equal(t, "test-", cfg.Kubernetes.NamespacePrefix)
	assert.Equal(t, "runsc", cfg.Kubernetes.RuntimeClass)
	assert.Equal(t, false, cfg.Auth.Enabled)
	assert.Equal(t, "2000m", cfg.Resources.DefaultCPULimit)
	assert.Equal(t, "2Gi", cfg.Resources.DefaultMemoryLimit)
	assert.Equal(t, 7200, cfg.Timeouts.DefaultTimeout)
	assert.Equal(t, 14400, cfg.Timeouts.MaxTimeout)
}
