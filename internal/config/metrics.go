package config

import (
	"fmt"
	"time"
)

type MetricsConfig struct {
	Addr            string
	ShutdownTimeout time.Duration
}

const (
	envVarMetricsAddress         = "METRICS_ADDR"
	envVarMetricsShutdownTimeout = "METRICS_SHUTDOWN_TIMEOUT"
)

const (
	defaultMetricsAddress         = ":9100"
	defaultMetricsShutdownTimeout = 5 * time.Second
)

func LoadMetricsConfig() (MetricsConfig, error) {
	metricsCfg := MetricsConfig{
		Addr:            envString(envVarMetricsAddress, defaultMetricsAddress),
		ShutdownTimeout: envDuration(envVarMetricsShutdownTimeout, defaultMetricsShutdownTimeout),
	}

	if metricsCfg.ShutdownTimeout <= 0 {
		return MetricsConfig{}, fmt.Errorf("%s must be positive, got %s", envVarMetricsShutdownTimeout, metricsCfg.ShutdownTimeout)
	}
	return metricsCfg, nil
}
