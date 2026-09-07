package config

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	envVarPhilURL          = "PHILHARMONIC_URL"
	envVarPhilToken        = "PHILHARMONIC_TOKEN"
	envVarPhilImage        = "PHILHARMONIC_IMAGE"
	envVarPhilTaskTimeout  = "PHILHARMONIC_TASK_TIMEOUT"
	envVarPhilPollInterval = "PHILHARMONIC_POLL_INTERVAL"
	envVarPhilPollTimeout  = "PHILHARMONIC_POLL_TIMEOUT"
	envVarPhilCpu          = "PHILHARMONIC_CPU"
	envVarPhilMemory       = "PHILHARMONIC_MEMORY"
)

const (
	defaultPhilURL          = "http://localhost:5555"
	defaultPhilImage        = "sandbox:latest"
	defaultPhilTaskTimeout  = 30 * time.Second
	defaultPhilPollInterval = time.Second
	defaultPhilCpu          = 0.5
	defaultPhilMemory       = 256 << 20 // bytes
)

type PhilharmonicConfig struct {
	BaseURL      string
	Token        string // empty means the manager runs without auth
	SandboxImage string
	TaskTimeout  time.Duration
	PollInterval time.Duration
	PollTimeout  time.Duration

	// per-task resource limits
	Cpu    float64
	Memory int64 // bytes

	// HTTPClient overrides the transport, mainly for tests; nil = default
	HTTPClient *http.Client
}

// called by Load
func LoadPhilharmonicConfig() (PhilharmonicConfig, error) {
	phrmCfg := PhilharmonicConfig{
		BaseURL:      envString(envVarPhilURL, defaultPhilURL),
		Token:        envString(envVarPhilToken, ""),
		SandboxImage: envString(envVarPhilImage, defaultPhilImage),
		TaskTimeout:  envDuration(envVarPhilTaskTimeout, defaultPhilTaskTimeout),
		PollInterval: envDuration(envVarPhilPollInterval, defaultPhilPollInterval),
		PollTimeout:  envDuration(envVarPhilPollTimeout, 0), // derived below
		Cpu:          envFloat(envVarPhilCpu, defaultPhilCpu),
		Memory:       envInt64(envVarPhilMemory, defaultPhilMemory),
	}

	if phrmCfg.PollTimeout <= 0 {
		phrmCfg.PollTimeout = phrmCfg.TaskTimeout + 15*time.Second
	}

	if err := validatePhilharmonic(phrmCfg); err != nil {
		return PhilharmonicConfig{}, err
	}
	return phrmCfg, nil
}

func validatePhilharmonic(opts PhilharmonicConfig) error {
	u, err := url.Parse(opts.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("%s must be a valid http(s) URL, got %q", envVarPhilURL, opts.BaseURL)
	}
	if strings.TrimSpace(opts.SandboxImage) == "" {
		return fmt.Errorf("%s must not be empty", envVarPhilImage)
	}
	if opts.TaskTimeout <= 0 {
		return fmt.Errorf("%s must be positive, got %s", envVarPhilTaskTimeout, opts.TaskTimeout)
	}
	if opts.PollInterval <= 0 {
		return fmt.Errorf("%s must be positive, got %s", envVarPhilPollInterval, opts.PollInterval)
	}
	if opts.PollTimeout < opts.TaskTimeout {
		return fmt.Errorf("%s (%s) must exceed %s (%s): the worker enforces the kill switch asynchronously",
			envVarPhilPollTimeout, opts.PollTimeout, envVarPhilTaskTimeout, opts.TaskTimeout)
	}
	if opts.Cpu <= 0 {
		return fmt.Errorf("%s must be positive, got %v", envVarPhilCpu, opts.Cpu)
	}
	if opts.Memory <= 0 {
		return fmt.Errorf("%s must be positive (bytes), got %d", envVarPhilMemory, opts.Memory)
	}
	return nil
}
