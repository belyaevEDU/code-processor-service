package config

import (
	"fmt"
	"time"
)

type AppConfig struct {
	HTTPAddr        string
	ShutdownTimeout time.Duration
}

const (
	envVarAppAddress         = "HTTP_ADDR"
	envVarAppShutdownTimeout = "SHUTDOWN_TIMEOUT"
)

const (
	defaultAppAddress         = ":8000"
	defaultAppShutdownTimeout = 10 * time.Second
)

func LoadAppConfig() (AppConfig, error) {
	appCfg := AppConfig{
		HTTPAddr:        envString(envVarAppAddress, defaultAppAddress),
		ShutdownTimeout: envDuration(envVarAppShutdownTimeout, defaultAppShutdownTimeout),
	}

	if appCfg.ShutdownTimeout <= 0 {
		return AppConfig{}, fmt.Errorf("%s must be positive, got %s", envVarAppShutdownTimeout, appCfg.ShutdownTimeout)
	}
	return appCfg, nil
}
