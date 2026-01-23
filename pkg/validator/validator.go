package validator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sciffer/agentbox/pkg/models"
)

var (
	// Valid Kubernetes resource formats
	cpuRegex     = regexp.MustCompile(`^(\d+)m?$`)
	memoryRegex  = regexp.MustCompile(`^(\d+)(Mi|Gi|M|G|Ki|K)?$`)
	storageRegex = regexp.MustCompile(`^(\d+)(Mi|Gi|Ti|M|G|T|Ki|K)?$`)
	nameRegex    = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
)

// Validator handles input validation
type Validator struct {
	maxCPU     int64
	maxMemory  int64
	maxStorage int64
	maxTimeout int
}

// New creates a new validator with resource limits
func New(maxCPU, maxMemory, maxStorage int64, maxTimeout int) *Validator {
	return &Validator{
		maxCPU:     maxCPU,
		maxMemory:  maxMemory,
		maxStorage: maxStorage,
		maxTimeout: maxTimeout,
	}
}

// ValidateCreateRequest validates an environment creation request
func (v *Validator) ValidateCreateRequest(req *models.CreateEnvironmentRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}

	if !nameRegex.MatchString(req.Name) {
		return fmt.Errorf("name must be lowercase alphanumeric with hyphens")
	}

	if len(req.Name) > 63 {
		return fmt.Errorf("name must be 63 characters or less")
	}

	if req.Image == "" {
		return fmt.Errorf("image is required")
	}

	if err := v.ValidateResourceSpec(&req.Resources); err != nil {
		return fmt.Errorf("invalid resources: %w", err)
	}

	if req.Timeout > v.maxTimeout {
		return fmt.Errorf("timeout exceeds maximum allowed (%d seconds)", v.maxTimeout)
	}

	if req.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}

	// Validate environment variables
	for k := range req.Env {
		if k == "" {
			return fmt.Errorf("environment variable name cannot be empty")
		}
	}

	// Validate labels
	for k, v := range req.Labels {
		if k == "" {
			return fmt.Errorf("label key cannot be empty")
		}
		if len(k) > 63 {
			return fmt.Errorf("label key must be 63 characters or less")
		}
		if len(v) > 63 {
			return fmt.Errorf("label value must be 63 characters or less")
		}
	}

	return nil
}

// ValidateResourceSpec validates resource specifications
func (v *Validator) ValidateResourceSpec(spec *models.ResourceSpec) error {
	if spec.CPU == "" {
		return fmt.Errorf("cpu is required")
	}

	cpu, err := parseCPU(spec.CPU)
	if err != nil {
		return fmt.Errorf("invalid cpu format: %w", err)
	}

	if cpu > v.maxCPU {
		return fmt.Errorf("cpu exceeds maximum allowed (%dm)", v.maxCPU)
	}

	if cpu <= 0 {
		return fmt.Errorf("cpu must be positive")
	}

	if spec.Memory == "" {
		return fmt.Errorf("memory is required")
	}

	memory, err := parseMemory(spec.Memory)
	if err != nil {
		return fmt.Errorf("invalid memory format: %w", err)
	}

	if memory > v.maxMemory {
		return fmt.Errorf("memory exceeds maximum allowed (%d bytes)", v.maxMemory)
	}

	if memory <= 0 {
		return fmt.Errorf("memory must be positive")
	}

	if spec.Storage == "" {
		return fmt.Errorf("storage is required")
	}

	storage, err := parseStorage(spec.Storage)
	if err != nil {
		return fmt.Errorf("invalid storage format: %w", err)
	}

	if storage > v.maxStorage {
		return fmt.Errorf("storage exceeds maximum allowed (%d bytes)", v.maxStorage)
	}

	if storage <= 0 {
		return fmt.Errorf("storage must be positive")
	}

	return nil
}

// ValidateExecRequest validates a command execution request
func (v *Validator) ValidateExecRequest(req *models.ExecRequest) error {
	if len(req.Command) == 0 {
		return fmt.Errorf("command is required")
	}

	if req.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}

	if req.Timeout > v.maxTimeout {
		return fmt.Errorf("timeout exceeds maximum allowed (%d seconds)", v.maxTimeout)
	}

	return nil
}

// parseCPU parses CPU resource string to millicores
func parseCPU(cpu string) (int64, error) {
	if !cpuRegex.MatchString(cpu) {
		return 0, fmt.Errorf("invalid format (expected: 100m or 1)")
	}

	if strings.HasSuffix(cpu, "m") {
		val := strings.TrimSuffix(cpu, "m")
		return strconv.ParseInt(val, 10, 64)
	}

	val, err := strconv.ParseInt(cpu, 10, 64)
	if err != nil {
		return 0, err
	}

	return val * 1000, nil
}

// parseMemory parses memory resource string to bytes
func parseMemory(memory string) (int64, error) {
	if !memoryRegex.MatchString(memory) {
		return 0, fmt.Errorf("invalid format (expected: 512Mi, 1Gi, etc)")
	}

	matches := memoryRegex.FindStringSubmatch(memory)
	if len(matches) < 2 {
		return 0, fmt.Errorf("failed to parse memory")
	}

	val, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}

	unit := ""
	if len(matches) > 2 {
		unit = matches[2]
	}

	multiplier := int64(1)
	switch unit {
	case "Ki", "K":
		multiplier = 1024
	case "Mi", "M":
		multiplier = 1024 * 1024
	case "Gi", "G":
		multiplier = 1024 * 1024 * 1024
	}

	return val * multiplier, nil
}

// parseStorage parses storage resource string to bytes
func parseStorage(storage string) (int64, error) {
	if !storageRegex.MatchString(storage) {
		return 0, fmt.Errorf("invalid format (expected: 1Gi, 5Gi, etc)")
	}

	matches := storageRegex.FindStringSubmatch(storage)
	if len(matches) < 2 {
		return 0, fmt.Errorf("failed to parse storage")
	}

	val, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}

	unit := ""
	if len(matches) > 2 {
		unit = matches[2]
	}

	multiplier := int64(1)
	switch unit {
	case "Ki", "K":
		multiplier = 1024
	case "Mi", "M":
		multiplier = 1024 * 1024
	case "Gi", "G":
		multiplier = 1024 * 1024 * 1024
	case "Ti", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	}

	return val * multiplier, nil
}
